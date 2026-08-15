package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const inspectSchemaVersion = 1

type inspectFormat string

const (
	inspectFormatText inspectFormat = "text"
	inspectFormatJSON inspectFormat = "json"
)

type inspectOptions struct {
	path      string
	dir       string
	workspace string
	format    inspectFormat
}

type inspectReport struct {
	SchemaVersion int                `json:"schemaVersion"`
	Package       inspectPackage     `json:"package"`
	Entrypoints   inspectEntrypoints `json:"entrypoints"`
	Artifacts     []inspectArtifact  `json:"artifacts"`
	Edges         []inspectEdge      `json:"edges"`
	Summary       inspectSummary     `json:"summary"`
}

type inspectPackage struct {
	Name             string `json:"name"`
	Compiler         string `json:"compiler"`
	ToolchainVersion string `json:"toolchainVersion"`
	HashAssets       bool   `json:"hashAssets"`
	Preload          bool   `json:"preload"`
}

type inspectEntrypoints struct {
	HTML    string   `json:"html"`
	WASM    string   `json:"wasm"`
	Runtime string   `json:"runtime"`
	Styles  []string `json:"styles"`
}

type inspectArtifact struct {
	Path         string   `json:"path"`
	LogicalName  string   `json:"logicalName"`
	MediaType    string   `json:"mediaType"`
	Bytes        int64    `json:"bytes"`
	SHA256       string   `json:"sha256"`
	DeclaredHash string   `json:"declaredHash"`
	Encoding     string   `json:"encoding"`
	Roles        []string `json:"roles"`
}

type inspectEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Kind     string `json:"kind"`
	Encoding string `json:"encoding"`
}

type inspectSummary struct {
	ArtifactCount int   `json:"artifactCount"`
	EdgeCount     int   `json:"edgeCount"`
	TotalBytes    int64 `json:"totalBytes"`
}

func inspectCommand(args []string) error {
	return runInspectCommand(args, os.Stdout)
}

func runInspectCommand(args []string, stdout io.Writer) error {
	options, err := parseInspectOptions(args)
	if err != nil {
		return err
	}
	packageRoot, err := resolveInspectPackageRoot(options)
	if err != nil {
		return err
	}
	report, err := inspectPackageGraph(packageRoot)
	if err != nil {
		return err
	}

	switch options.format {
	case inspectFormatText:
		if err := writeInspectText(stdout, report); err != nil {
			return fmt.Errorf("write inspect text report: %w", err)
		}
	case inspectFormatJSON:
		if err := writeInspectJSON(stdout, report); err != nil {
			return fmt.Errorf("write inspect json report: %w", err)
		}
	default:
		return fmt.Errorf("unsupported inspect format %q", options.format)
	}
	return nil
}

func parseInspectOptions(args []string) (inspectOptions, error) {
	options := inspectOptions{format: inspectFormatText}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case strings.HasPrefix(arg, "--format="):
			format, err := parseInspectFormat(strings.TrimPrefix(arg, "--format="))
			if err != nil {
				return inspectOptions{}, err
			}
			options.format = format
		case arg == "--format":
			index++
			if index >= len(args) {
				return inspectOptions{}, errors.New("--format requires a value")
			}
			format, err := parseInspectFormat(args[index])
			if err != nil {
				return inspectOptions{}, err
			}
			options.format = format
		case strings.HasPrefix(arg, "--workspace="):
			options.workspace = strings.TrimPrefix(arg, "--workspace=")
			if options.workspace == "" {
				return inspectOptions{}, errors.New("--workspace requires a value")
			}
		case arg == "--workspace":
			index++
			if index >= len(args) {
				return inspectOptions{}, errors.New("--workspace requires a value")
			}
			options.workspace = args[index]
		case strings.HasPrefix(arg, "--dir="):
			options.dir = strings.TrimPrefix(arg, "--dir=")
			if options.dir == "" {
				return inspectOptions{}, errors.New("--dir requires a value")
			}
		case arg == "--dir":
			index++
			if index >= len(args) {
				return inspectOptions{}, errors.New("--dir requires a value")
			}
			options.dir = args[index]
		case strings.HasPrefix(arg, "-"):
			return inspectOptions{}, fmt.Errorf("unknown inspect flag %q", arg)
		case options.path == "":
			options.path = arg
		default:
			return inspectOptions{}, fmt.Errorf("multiple inspect input paths are not supported: %q and %q", options.path, arg)
		}
	}
	if options.dir != "" && options.path != "" {
		return inspectOptions{}, errors.New("--dir cannot be combined with a positional path")
	}
	if options.dir != "" && options.workspace != "" {
		return inspectOptions{}, errors.New("--dir cannot be combined with --workspace")
	}
	if options.dir == "" && options.path == "" {
		return inspectOptions{}, errors.New("usage: goxc inspect <app-or-package-directory> [--workspace=directory] [--format=text|json] or goxc inspect --dir=<package-directory> [--format=text|json]")
	}
	return options, nil
}

func parseInspectFormat(value string) (inspectFormat, error) {
	switch inspectFormat(value) {
	case inspectFormatText:
		return inspectFormatText, nil
	case inspectFormatJSON:
		return inspectFormatJSON, nil
	default:
		return "", fmt.Errorf("unsupported inspect format %q; expected text or json", value)
	}
}

func resolveInspectPackageRoot(options inspectOptions) (string, error) {
	if options.dir != "" {
		return validateInspectPackageRoot(options.dir)
	}
	if err := directoryNoFollow(options.path, "inspect input"); err != nil {
		return "", err
	}
	metadataExists, err := pathExistsNoFollow(filepath.Join(options.path, packageMetadataName))
	if err != nil {
		return "", err
	}
	if metadataExists {
		return validateInspectPackageRoot(options.path)
	}
	if validLegacyPackageSignature(options.path) {
		return "", legacyInspectPackageError(options.path)
	}
	if err := ensureAppDirectory(options.path); err != nil {
		return "", err
	}
	layout, err := newBuildLayout(layoutOptions{appDir: options.path, workspace: options.workspace})
	if err != nil {
		return "", err
	}
	if err := validateWorkspaceRoot(layout); err != nil {
		return "", err
	}
	if _, err := os.Lstat(layout.PackageDir); errors.Is(err, os.ErrNotExist) {
		return "", noCurrentInspectPackageError(options.path)
	} else if err != nil {
		return "", fmt.Errorf("inspect standalone package %s: %w", layout.PackageDir, err)
	}
	if err := validatePathBelowRoot(layout.WorkspaceRoot, layout.PackageDir, "standalone package directory", false); err != nil {
		return "", err
	}
	root, err := validateInspectPackageRoot(layout.PackageDir)
	if err != nil {
		return "", fmt.Errorf("no complete current standalone package found for %s; run `goxc package %s` first: %w", options.path, options.path, err)
	}
	return root, nil
}

func validateInspectPackageRoot(root string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve package directory %s: %w", root, err)
	}
	absolute = filepath.Clean(absolute)
	if err := validateExplicitPathRoot(absolute, "package directory", false); err != nil {
		return "", err
	}
	if err := directoryNoFollow(absolute, "package directory"); err != nil {
		return "", err
	}
	ownership := inspectPackageOwnership(absolute)
	switch ownership.State {
	case packageOwnedCurrent:
		return absolute, nil
	case packageOwnedLegacy:
		return "", legacyInspectPackageError(absolute)
	case packageIncompleteOrInvalid:
		return "", fmt.Errorf("package directory %s is not a complete current GoFrame package: %s", absolute, ownership.Reason)
	case packageUnowned:
		return "", fmt.Errorf("package directory %s does not contain a complete current GoFrame package: %s is required (%s)", absolute, packageMetadataName, ownership.Reason)
	default:
		return "", fmt.Errorf("package directory %s has unknown ownership state", absolute)
	}
}

func noCurrentInspectPackageError(app string) error {
	return fmt.Errorf("no complete current standalone package found for %s; run `goxc package %s` first", app, app)
}

func legacyInspectPackageError(root string) error {
	return fmt.Errorf("legacy GoFrame packages are not supported by `goxc inspect`: %s; repackage the application with the current goxc package format", root)
}

type inspectAssetSpec struct {
	logicalName string
	path        string
	asset       packageAsset
	roles       []string
}

type inspectSidecarSpec struct {
	logicalName string
	path        string
	mediaType   string
	encoding    string
	parentPath  string
}

func inspectPackageGraph(root string) (inspectReport, error) {
	root, err := validateInspectPackageRoot(root)
	if err != nil {
		return inspectReport{}, err
	}
	metadata, err := readCurrentPackageMetadata(filepath.Join(root, packageMetadataName))
	if err != nil {
		return inspectReport{}, err
	}
	if strings.TrimSpace(metadata.ToolchainVersion) == "" {
		return inspectReport{}, errors.New("package metadata toolchainVersion must not be empty")
	}
	manifest, err := readAssetManifestMetadata(filepath.Join(root, assetManifestName))
	if err != nil {
		return inspectReport{}, err
	}
	if err := validateInspectLogicalNames(filepath.Join(root, assetManifestName)); err != nil {
		return inspectReport{}, err
	}
	if metadata.Entrypoints.WASM != manifest.Entrypoints.WASM || metadata.Entrypoints.Runtime != manifest.Entrypoints.Runtime {
		return inspectReport{}, errors.New("package metadata and asset manifest entrypoints do not match")
	}

	htmlPath, err := normalizeInspectPath(metadata.Entrypoints.HTML, "HTML entrypoint")
	if err != nil {
		return inspectReport{}, err
	}
	wasmPath, err := normalizeInspectPath(manifest.Entrypoints.WASM, "WASM entrypoint")
	if err != nil {
		return inspectReport{}, err
	}
	runtimePath, err := normalizeInspectPath(manifest.Entrypoints.Runtime, "runtime entrypoint")
	if err != nil {
		return inspectReport{}, err
	}
	if !strings.EqualFold(path.Ext(wasmPath), ".wasm") {
		return inspectReport{}, fmt.Errorf("WASM entrypoint must end in .wasm: %q", wasmPath)
	}
	if wasmPath == runtimePath {
		return inspectReport{}, errors.New("WASM and runtime entrypoints must resolve to distinct assets")
	}

	occupied := map[string]string{
		packageMetadataName: "package metadata",
		assetManifestName:   "asset metadata",
	}
	if previous, exists := occupied[htmlPath]; exists {
		return inspectReport{}, fmt.Errorf("HTML entrypoint path %q collides with %s", htmlPath, previous)
	}
	occupied[htmlPath] = "HTML entrypoint"

	logicalNames := make([]string, 0, len(manifest.Assets))
	for logicalName := range manifest.Assets {
		logicalNames = append(logicalNames, logicalName)
	}
	sort.Strings(logicalNames)
	assets := make([]inspectAssetSpec, 0, len(logicalNames))
	assetByPath := make(map[string]int, len(logicalNames))
	for _, logicalName := range logicalNames {
		if logicalName == "" {
			return inspectReport{}, errors.New("asset manifest logical asset name is empty")
		}
		asset := manifest.Assets[logicalName]
		assetPath, err := normalizeInspectPath(asset.Path, fmt.Sprintf("declared asset %q", logicalName))
		if err != nil {
			return inspectReport{}, err
		}
		if previous, exists := occupied[assetPath]; exists {
			if strings.HasPrefix(previous, "ordinary asset ") {
				return inspectReport{}, fmt.Errorf("asset path %q is declared by more than one asset: %s and %q", assetPath, previous, logicalName)
			}
			return inspectReport{}, fmt.Errorf("asset path %q for %q collides with %s", assetPath, logicalName, previous)
		}
		occupied[assetPath] = fmt.Sprintf("ordinary asset %q", logicalName)
		assets = append(assets, inspectAssetSpec{
			logicalName: logicalName,
			path:        assetPath,
			asset:       asset,
			roles:       []string{"asset"},
		})
		assetByPath[assetPath] = len(assets) - 1
	}

	wasmIndex, ok := assetByPath[wasmPath]
	if !ok {
		return inspectReport{}, fmt.Errorf("WASM entrypoint %q is not declared as exactly one asset", wasmPath)
	}
	assets[wasmIndex].roles = append(assets[wasmIndex].roles, "wasm-entrypoint")
	runtimeIndex, ok := assetByPath[runtimePath]
	if !ok {
		return inspectReport{}, fmt.Errorf("runtime entrypoint %q is not declared as exactly one asset", runtimePath)
	}
	assets[runtimeIndex].roles = append(assets[runtimeIndex].roles, "runtime-entrypoint")

	stylePaths := make([]string, 0, len(manifest.Entrypoints.Styles))
	styleSet := make(map[string]struct{}, len(manifest.Entrypoints.Styles))
	for _, declared := range manifest.Entrypoints.Styles {
		stylePath, err := normalizeInspectPath(declared, "style entrypoint")
		if err != nil {
			return inspectReport{}, err
		}
		if _, exists := styleSet[stylePath]; exists {
			return inspectReport{}, fmt.Errorf("duplicate style entrypoint %q", stylePath)
		}
		styleSet[stylePath] = struct{}{}
		styleIndex, ok := assetByPath[stylePath]
		if !ok {
			return inspectReport{}, fmt.Errorf("style entrypoint %q is not declared as exactly one asset", stylePath)
		}
		assets[styleIndex].roles = append(assets[styleIndex].roles, "style-entrypoint")
		stylePaths = append(stylePaths, stylePath)
	}
	sort.Strings(stylePaths)

	sidecars := make([]inspectSidecarSpec, 0)
	for index := range assets {
		asset := &assets[index]
		encodings := make([]string, 0, len(asset.asset.Compressed))
		for encoding := range asset.asset.Compressed {
			encodings = append(encodings, encoding)
		}
		sort.Strings(encodings)
		for _, encoding := range encodings {
			if encoding == "" {
				return inspectReport{}, fmt.Errorf("compressed asset encoding name is empty for %q", asset.logicalName)
			}
			sidecarPath, err := normalizeInspectPath(asset.asset.Compressed[encoding], fmt.Sprintf("compressed asset %q for %q", encoding, asset.logicalName))
			if err != nil {
				return inspectReport{}, err
			}
			if previous, exists := occupied[sidecarPath]; exists {
				return inspectReport{}, fmt.Errorf("compressed path %q for %q encoding %q collides with %s", sidecarPath, asset.logicalName, encoding, previous)
			}
			occupied[sidecarPath] = fmt.Sprintf("compressed asset %q for %q", encoding, asset.logicalName)
			sidecars = append(sidecars, inspectSidecarSpec{
				logicalName: asset.logicalName,
				path:        sidecarPath,
				mediaType:   asset.asset.Type,
				encoding:    encoding,
				parentPath:  asset.path,
			})
		}
	}

	report := inspectReport{
		SchemaVersion: inspectSchemaVersion,
		Package: inspectPackage{
			Name:             metadata.Name,
			Compiler:         metadata.Compiler,
			ToolchainVersion: metadata.ToolchainVersion,
			HashAssets:       metadata.HashAssets,
			Preload:          metadata.Preload,
		},
		Entrypoints: inspectEntrypoints{
			HTML:    htmlPath,
			WASM:    wasmPath,
			Runtime: runtimePath,
			Styles:  stylePaths,
		},
		Artifacts: make([]inspectArtifact, 0, len(occupied)),
		Edges:     make([]inspectEdge, 0, 2+len(stylePaths)+len(sidecars)),
	}

	for _, fixed := range []struct {
		path      string
		mediaType string
		role      string
	}{
		{packageMetadataName, "application/json", "package-metadata"},
		{assetManifestName, "application/json", "asset-metadata"},
		{htmlPath, "text/html; charset=utf-8", "html-entrypoint"},
	} {
		artifact, err := inspectArtifactAt(root, fixed.path, "", fixed.mediaType, "", "", []string{fixed.role}, fixed.role)
		if err != nil {
			return inspectReport{}, err
		}
		report.Artifacts = append(report.Artifacts, artifact)
	}

	for _, asset := range assets {
		if metadata.HashAssets != (asset.asset.Hash != "") {
			if metadata.HashAssets {
				return inspectReport{}, fmt.Errorf("hashAssets=true requires a declared hash for asset %q", asset.logicalName)
			}
			return inspectReport{}, fmt.Errorf("hashAssets=false requires an empty declared hash for asset %q", asset.logicalName)
		}
		artifact, err := inspectArtifactAt(root, asset.path, asset.logicalName, asset.asset.Type, asset.asset.Hash, "", asset.roles, fmt.Sprintf("declared asset %q", asset.logicalName))
		if err != nil {
			return inspectReport{}, err
		}
		if asset.asset.Hash != "" {
			if !validInspectShortHash(asset.asset.Hash) {
				return inspectReport{}, fmt.Errorf("declared hash %q for asset %q must be exactly eight lowercase hexadecimal characters", asset.asset.Hash, asset.logicalName)
			}
			if artifact.SHA256[:packageHashLength] != asset.asset.Hash {
				return inspectReport{}, fmt.Errorf("declared hash %q for asset %q does not match actual SHA-256 %s", asset.asset.Hash, asset.logicalName, artifact.SHA256)
			}
		}
		report.Artifacts = append(report.Artifacts, artifact)
	}

	for _, sidecar := range sidecars {
		artifact, err := inspectArtifactAt(root, sidecar.path, sidecar.logicalName, sidecar.mediaType, "", sidecar.encoding, []string{"compressed"}, fmt.Sprintf("compressed asset %q for %q", sidecar.encoding, sidecar.logicalName))
		if err != nil {
			return inspectReport{}, err
		}
		report.Artifacts = append(report.Artifacts, artifact)
		report.Edges = append(report.Edges, inspectEdge{From: sidecar.parentPath, To: sidecar.path, Kind: "compressed", Encoding: sidecar.encoding})
	}

	report.Edges = append(report.Edges,
		inspectEdge{From: htmlPath, To: wasmPath, Kind: "wasm-entrypoint", Encoding: ""},
		inspectEdge{From: htmlPath, To: runtimePath, Kind: "runtime-entrypoint", Encoding: ""},
	)
	for _, stylePath := range stylePaths {
		report.Edges = append(report.Edges, inspectEdge{From: htmlPath, To: stylePath, Kind: "style-entrypoint", Encoding: ""})
	}
	sort.Slice(report.Artifacts, func(first, second int) bool {
		return report.Artifacts[first].Path < report.Artifacts[second].Path
	})
	sort.Slice(report.Edges, func(first, second int) bool {
		return lessInspectEdge(report.Edges[first], report.Edges[second])
	})
	for _, artifact := range report.Artifacts {
		report.Summary.TotalBytes += artifact.Bytes
	}
	report.Summary.ArtifactCount = len(report.Artifacts)
	report.Summary.EdgeCount = len(report.Edges)
	return report, nil
}

func normalizeInspectPath(value, description string) (string, error) {
	if !safeChildPath(value) {
		return "", fmt.Errorf("%s %q must be a safe package-relative path", description, value)
	}
	return path.Clean(manifestPath(value)), nil
}

func inspectArtifactAt(root, relative, logicalName, mediaType, declaredHash, encoding string, roles []string, description string) (inspectArtifact, error) {
	if mediaType == "" {
		return inspectArtifact{}, fmt.Errorf("%s %q has an empty media type", description, relative)
	}
	if err := validatePackageOwnedPath(root, relative, description); err != nil {
		return inspectArtifact{}, err
	}
	fullPath := filepath.Join(root, filepath.FromSlash(relative))
	if _, err := regularFileNoFollow(fullPath, description); err != nil {
		return inspectArtifact{}, err
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return inspectArtifact{}, fmt.Errorf("read %s %s: %w", description, fullPath, err)
	}
	sum := sha256.Sum256(content)
	roles = append([]string(nil), roles...)
	sort.Strings(roles)
	roles = uniqueInspectRoles(roles)
	return inspectArtifact{
		Path:         relative,
		LogicalName:  logicalName,
		MediaType:    mediaType,
		Bytes:        int64(len(content)),
		SHA256:       hex.EncodeToString(sum[:]),
		DeclaredHash: declaredHash,
		Encoding:     encoding,
		Roles:        roles,
	}, nil
}

func validInspectShortHash(value string) bool {
	if len(value) != packageHashLength {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}

func uniqueInspectRoles(roles []string) []string {
	if len(roles) == 0 {
		return []string{}
	}
	result := roles[:1]
	for _, role := range roles[1:] {
		if role != result[len(result)-1] {
			result = append(result, role)
		}
	}
	return result
}

func lessInspectEdge(first, second inspectEdge) bool {
	if first.From != second.From {
		return first.From < second.From
	}
	if first.Kind != second.Kind {
		return first.Kind < second.Kind
	}
	if first.Encoding != second.Encoding {
		return first.Encoding < second.Encoding
	}
	return first.To < second.To
}

type inspectLogicalAssetNames map[string]struct{}

func (names *inspectLogicalAssetNames) UnmarshalJSON(content []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return errors.New("assets must be a JSON object")
	}
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		name, ok := token.(string)
		if !ok {
			return errors.New("asset logical name must be a string")
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("duplicate logical asset name %q", name)
		}
		seen[name] = struct{}{}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	*names = seen
	return nil
}

func validateInspectLogicalNames(manifestPath string) error {
	if _, err := regularFileNoFollow(manifestPath, "asset manifest"); err != nil {
		return err
	}
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", manifestPath, err)
	}
	var document struct {
		Assets inspectLogicalAssetNames `json:"assets"`
	}
	if err := json.Unmarshal(content, &document); err != nil {
		return fmt.Errorf("parse %s for graph inspection: %w", manifestPath, err)
	}
	return nil
}

func writeInspectJSON(output io.Writer, report inspectReport) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func writeInspectText(output io.Writer, report inspectReport) error {
	var text strings.Builder
	fmt.Fprintln(&text, "Package")
	fmt.Fprintf(&text, "  Name: %s\n", report.Package.Name)
	fmt.Fprintf(&text, "  Compiler: %s\n", report.Package.Compiler)
	fmt.Fprintf(&text, "  Toolchain version: %s\n", report.Package.ToolchainVersion)
	fmt.Fprintf(&text, "  Hash assets: %t\n", report.Package.HashAssets)
	fmt.Fprintf(&text, "  Preload: %t\n", report.Package.Preload)
	fmt.Fprintln(&text)

	fmt.Fprintln(&text, "Entrypoints")
	fmt.Fprintf(&text, "  HTML: %s\n", report.Entrypoints.HTML)
	fmt.Fprintf(&text, "  WASM: %s\n", report.Entrypoints.WASM)
	fmt.Fprintf(&text, "  Runtime: %s\n", report.Entrypoints.Runtime)
	if len(report.Entrypoints.Styles) == 0 {
		fmt.Fprintln(&text, "  Styles: (none)")
	} else {
		fmt.Fprintln(&text, "  Styles:")
		for _, style := range report.Entrypoints.Styles {
			fmt.Fprintf(&text, "    - %s\n", style)
		}
	}
	fmt.Fprintln(&text)

	fmt.Fprintln(&text, "Artifacts")
	if len(report.Artifacts) == 0 {
		fmt.Fprintln(&text, "  (none)")
	}
	for _, artifact := range report.Artifacts {
		fmt.Fprintf(&text, "  - %s\n", artifact.Path)
		fmt.Fprintf(&text, "    Logical name: %q\n", artifact.LogicalName)
		fmt.Fprintf(&text, "    Media type: %s\n", artifact.MediaType)
		fmt.Fprintf(&text, "    Bytes: %d\n", artifact.Bytes)
		fmt.Fprintf(&text, "    SHA-256: %s\n", artifact.SHA256)
		fmt.Fprintf(&text, "    Declared hash: %q\n", artifact.DeclaredHash)
		fmt.Fprintf(&text, "    Encoding: %q\n", artifact.Encoding)
		fmt.Fprintf(&text, "    Roles: %s\n", strings.Join(artifact.Roles, ", "))
	}
	fmt.Fprintln(&text)

	fmt.Fprintln(&text, "Edges")
	if len(report.Edges) == 0 {
		fmt.Fprintln(&text, "  (none)")
	}
	for _, edge := range report.Edges {
		fmt.Fprintf(&text, "  - %s -> %s\n", edge.From, edge.To)
		fmt.Fprintf(&text, "    Kind: %s\n", edge.Kind)
		fmt.Fprintf(&text, "    Encoding: %q\n", edge.Encoding)
	}
	fmt.Fprintln(&text)

	fmt.Fprintln(&text, "Summary")
	fmt.Fprintf(&text, "  Artifact count: %d\n", report.Summary.ArtifactCount)
	fmt.Fprintf(&text, "  Edge count: %d\n", report.Summary.EdgeCount)
	fmt.Fprintf(&text, "  Total bytes: %d\n", report.Summary.TotalBytes)
	_, err := io.WriteString(output, text.String())
	return err
}
