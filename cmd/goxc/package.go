package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	assetDirectoryName    = "assets"
	assetManifestName     = "asset-manifest.json"
	packageMetadataName   = "goframe-package.json"
	legacyPackageManifest = "manifest.json"
	runtimeAssetName      = "wasm_exec.js"
	packageHashLength     = 8
	indexHTMLAssetName    = "index.html"
	preloadBlockName      = "preload"
	runtimeBlockName      = "runtime"
	bootstrapBlockName    = "bootstrap"
)

type packageOptions struct {
	appDir          string
	compiler        string
	outDir          string
	workspace       string
	compress        map[string]bool
	assetHash       bool
	preload         bool
	recordEmbedPlan func(embedInputPlan)
}

type packageAssetPlan struct {
	Assets          []plannedPackageAsset
	CustomIndexPath string
	GenerateIndex   bool
}

type plannedPackageAsset struct {
	LogicalName string
	SourcePath  string
	Exists      bool
}

type assetManifest struct {
	Version     int                     `json:"version"`
	Assets      map[string]packageAsset `json:"assets"`
	Entrypoints packageEntrypoints      `json:"entrypoints"`
}

type packageAsset struct {
	Path       string            `json:"path"`
	Hash       string            `json:"hash,omitempty"`
	Type       string            `json:"type"`
	Compressed map[string]string `json:"compressed,omitempty"`
}

type packageEntrypoints struct {
	WASM    string   `json:"wasm"`
	Runtime string   `json:"runtime"`
	Styles  []string `json:"styles,omitempty"`
}

type packageMetadata struct {
	Version          int                `json:"version"`
	Name             string             `json:"name"`
	Compiler         string             `json:"compiler"`
	ToolchainVersion string             `json:"toolchainVersion"`
	AssetsDir        string             `json:"assetsDir"`
	HashAssets       bool               `json:"hashAssets"`
	Preload          bool               `json:"preload"`
	Entrypoints      metadataEntrypoint `json:"entrypoints"`
	GeneratedAt      string             `json:"generatedAt"`
}

type metadataEntrypoint struct {
	HTML    string `json:"html"`
	WASM    string `json:"wasm"`
	Runtime string `json:"runtime"`
}

func packageCommand(args []string) error {
	options, err := parsePackageOptions(args)
	if err != nil {
		return err
	}
	return packageApp(options)
}

func parsePackageOptions(args []string) (packageOptions, error) {
	options := packageOptions{compress: map[string]bool{}}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case strings.HasPrefix(arg, "--compiler="):
			options.compiler = strings.TrimPrefix(arg, "--compiler=")
		case arg == "--compiler":
			index++
			if index >= len(args) {
				return packageOptions{}, errors.New("--compiler requires a value")
			}
			options.compiler = args[index]
		case strings.HasPrefix(arg, "--out="):
			options.outDir = strings.TrimPrefix(arg, "--out=")
		case arg == "--out":
			index++
			if index >= len(args) {
				return packageOptions{}, errors.New("--out requires a value")
			}
			options.outDir = args[index]
		case strings.HasPrefix(arg, "--workspace="):
			options.workspace = strings.TrimPrefix(arg, "--workspace=")
		case arg == "--workspace":
			index++
			if index >= len(args) {
				return packageOptions{}, errors.New("--workspace requires a value")
			}
			options.workspace = args[index]
		case strings.HasPrefix(arg, "--compress="):
			if err := parseCompression(strings.TrimPrefix(arg, "--compress="), options.compress); err != nil {
				return packageOptions{}, err
			}
		case arg == "--compress":
			index++
			if index >= len(args) {
				return packageOptions{}, errors.New("--compress requires gzip, br, or gzip,br")
			}
			if err := parseCompression(args[index], options.compress); err != nil {
				return packageOptions{}, err
			}
		case arg == "--asset-hash":
			options.assetHash = true
		case arg == "--preload":
			options.preload = true
		case strings.HasPrefix(arg, "-"):
			return packageOptions{}, fmt.Errorf("unknown package flag %q", arg)
		case options.appDir == "":
			options.appDir = arg
		default:
			return packageOptions{}, fmt.Errorf("unexpected package argument %q", arg)
		}
	}
	if options.appDir == "" {
		return packageOptions{}, errors.New("usage: goxc package <app-directory> [--compiler=go|tinygo] [--out=directory] [--workspace=directory] [--asset-hash] [--preload] [--compress=gzip,br]")
	}
	return options, nil
}

func parseCompression(value string, result map[string]bool) error {
	for _, format := range strings.Split(value, ",") {
		switch strings.TrimSpace(format) {
		case "gzip":
			result["gzip"] = true
		case "br":
			result["br"] = true
		default:
			return fmt.Errorf("unsupported compression %q; use gzip, br, or gzip,br", format)
		}
	}
	return nil
}

func packageApp(options packageOptions) error {
	manifest, err := loadManifest(options.appDir)
	if err != nil {
		return err
	}
	if options.compiler == "" {
		options.compiler = manifest.Compiler
	}
	if err := validateCompiler(options.compiler); err != nil {
		return err
	}
	if err := ensureAppDirectory(options.appDir); err != nil {
		return err
	}
	wasmLogicalName := path.Base(filepath.ToSlash(filepath.Clean(manifest.WASM)))
	if strings.ToLower(path.Ext(filepath.ToSlash(manifest.WASM))) != ".wasm" || strings.ToLower(path.Ext(wasmLogicalName)) != ".wasm" {
		return fmt.Errorf("wasm %q in %s must end with .wasm", manifest.WASM, manifestName)
	}
	if wasmLogicalName == runtimeAssetName {
		return fmt.Errorf("wasm %q in %s collides with runtime asset %s", manifest.WASM, manifestName, runtimeAssetName)
	}
	assetPlan, err := planPackageAssets(options.appDir, manifest, wasmLogicalName, options)
	if err != nil {
		return err
	}
	layout, err := newBuildLayout(layoutOptions{
		appDir:    options.appDir,
		compiler:  options.compiler,
		profile:   packageProfile(options.assetHash, options.preload, options.compress),
		workspace: options.workspace,
	})
	if err != nil {
		return err
	}
	if err := validateWorkspaceRoot(layout); err != nil {
		return err
	}
	explicitOutDir := options.outDir != ""
	options.outDir = packageOutputDirectory(options, layout)
	if explicitOutDir {
		if err := ensureNoPhysicalOverlap(options.outDir, layout.AppDir, "package output directory", "application directory"); err != nil {
			return err
		}
		if err := validateExplicitPathRoot(options.outDir, "package output directory", true); err != nil {
			return err
		}
		if err := validatePackageDestination(options.outDir); err != nil {
			return err
		}
	} else if err := validatePathBelowRoot(layout.WorkspaceRoot, options.outDir, "package output directory", true); err != nil {
		return err
	}
	workspaceResult, err := prepareBuildWorkspaceResult(layout, manifest)
	if options.recordEmbedPlan != nil {
		options.recordEmbedPlan(workspaceResult.EmbedPlan)
	}
	if err != nil {
		return fmt.Errorf("prepare package workspace: %w", err)
	}
	entryPath := workspaceResult.EntryPath

	tempDir, err := os.MkdirTemp("", "goxc-package-*")
	if err != nil {
		return fmt.Errorf("create temporary package directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tempWASM := filepath.Join(tempDir, wasmLogicalName)
	fmt.Printf("packaging %s with %s compiler\n", options.appDir, options.compiler)
	if err := compileWASM(options.compiler, entryPath, tempWASM); err != nil {
		return err
	}
	stageDir := filepath.Join(tempDir, "stage")
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return fmt.Errorf("create staging package directory: %w", err)
	}

	assetsDir := filepath.Join(stageDir, assetDirectoryName)
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		return fmt.Errorf("create package assets directory: %w", err)
	}
	assets := map[string]packageAsset{}
	entrypoints := packageEntrypoints{}

	wasmAsset, err := writePackageAsset(tempWASM, assetsDir, wasmLogicalName, options)
	if err != nil {
		return err
	}
	assets[wasmLogicalName] = wasmAsset
	entrypoints.WASM = wasmAsset.Path

	runtimeSource, err := wasmExecPath(options.compiler)
	if err != nil {
		return err
	}
	runtimeAsset, err := writePackageAsset(runtimeSource, assetsDir, runtimeAssetName, options)
	if err != nil {
		return err
	}
	assets[runtimeAssetName] = runtimeAsset
	entrypoints.Runtime = runtimeAsset.Path

	styleRewrites := map[string]string{}
	for _, planned := range assetPlan.Assets {
		if !planned.Exists {
			fmt.Printf("asset %s not found; skipping\n", planned.SourcePath)
			continue
		}
		packaged, err := writePackageAsset(planned.SourcePath, assetsDir, planned.LogicalName, options)
		if err != nil {
			return err
		}
		assets[planned.LogicalName] = packaged
		if strings.EqualFold(path.Ext(planned.LogicalName), ".css") {
			entrypoints.Styles = append(entrypoints.Styles, packaged.Path)
			styleRewrites[planned.LogicalName] = packaged.Path
		}
	}

	sort.Strings(entrypoints.Styles)
	indexOptions := htmlRewriteOptions{
		preload:       options.preload,
		wasmPath:      wasmAsset.Path,
		runtimePath:   runtimeAsset.Path,
		styleRewrites: styleRewrites,
		stylePaths:    entrypoints.Styles,
	}
	if assetPlan.CustomIndexPath != "" {
		if err := writeRewrittenIndex(assetPlan.CustomIndexPath, filepath.Join(stageDir, indexHTMLAssetName), indexOptions); err != nil {
			return err
		}
	} else if assetPlan.GenerateIndex {
		if err := writeGeneratedIndex(filepath.Join(stageDir, indexHTMLAssetName), indexOptions); err != nil {
			return err
		}
	} else {
		return errors.New("package asset plan did not provide an index.html source or generated fallback")
	}

	if err := writeJSONFile(filepath.Join(stageDir, assetManifestName), assetManifest{
		Version:     1,
		Assets:      assets,
		Entrypoints: entrypoints,
	}, "asset manifest"); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(stageDir, packageMetadataName), packageMetadata{
		Version:          1,
		Name:             manifest.Name,
		Compiler:         options.compiler,
		ToolchainVersion: goxcVersion(),
		AssetsDir:        assetDirectoryName,
		HashAssets:       options.assetHash,
		Preload:          options.preload,
		Entrypoints: metadataEntrypoint{
			HTML:    indexHTMLAssetName,
			WASM:    entrypoints.WASM,
			Runtime: entrypoints.Runtime,
		},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}, "package metadata"); err != nil {
		return err
	}

	if explicitOutDir {
		if err := validateExplicitPathRoot(options.outDir, "package output directory", true); err != nil {
			return err
		}
		if err := os.MkdirAll(options.outDir, 0o755); err != nil {
			return fmt.Errorf("create package directory: %w", err)
		}
	} else if err := mkdirAllBelowRoot(layout.WorkspaceRoot, options.outDir, "package output directory"); err != nil {
		return err
	}
	if err := cleanPackageArtifacts(options.outDir, manifest.WASM); err != nil {
		return err
	}
	if err := publishPackageArtifacts(stageDir, options.outDir); err != nil {
		return err
	}
	if err := verifyPublishedPackage(options.outDir); err != nil {
		return err
	}

	fmt.Printf("packaged %s\n", options.outDir)
	return nil
}

func resolvePackageAssetSource(appDir, asset string) (string, string, bool, error) {
	asset, err := cleanPackageAssetName(asset)
	if err != nil {
		return "", "", false, err
	}
	source, exists, err := inspectPackageAssetSource(appDir, asset)
	if err != nil {
		return "", "", false, err
	}
	return asset, source, exists, nil
}

type packageAssetPlanner struct {
	appDir   string
	options  packageOptions
	occupied map[string]string
	plan     packageAssetPlan
}

func planPackageAssets(appDir string, manifest projectManifest, wasmLogicalName string, options packageOptions) (packageAssetPlan, error) {
	planner, err := newPackageAssetPlanner(appDir, wasmLogicalName, options)
	if err != nil {
		return packageAssetPlan{}, err
	}
	switch manifest.Assets.Mode {
	case manifestAssetsAuto:
		if err := planner.addAutoAssets(); err != nil {
			return packageAssetPlan{}, err
		}
	case manifestAssetsDirectory:
		if err := planner.addDirectoryAssets(manifest.Assets.Directory); err != nil {
			return packageAssetPlan{}, err
		}
	case manifestAssetsList:
		if err := planner.addListedAssets(manifest.Assets.List); err != nil {
			return packageAssetPlan{}, err
		}
	default:
		return packageAssetPlan{}, fmt.Errorf("assets in %s has unsupported internal mode", manifestName)
	}
	return planner.finish(), nil
}

func newPackageAssetPlanner(appDir, wasmLogicalName string, options packageOptions) (*packageAssetPlanner, error) {
	planner := &packageAssetPlanner{
		appDir:   appDir,
		options:  options,
		occupied: map[string]string{},
	}
	for _, name := range []string{wasmLogicalName, runtimeAssetName, assetManifestName, packageMetadataName} {
		if err := planner.reserveAsset(name, "generated asset"); err != nil {
			return nil, err
		}
	}
	return planner, nil
}

func (planner *packageAssetPlanner) addAutoAssets() error {
	directory := filepath.Join(planner.appDir, assetDirectoryName)
	info, err := os.Lstat(directory)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("asset directory %s is a symlink; symlink paths are not supported", directory)
		}
		if info.IsDir() {
			return planner.addDirectoryAssets(assetDirectoryName)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect asset directory %s: %w", directory, err)
	}

	source, exists, err := inspectPackageAssetSource(planner.appDir, indexHTMLAssetName)
	if err != nil {
		return err
	}
	if exists {
		return planner.setCustomIndex(source)
	}
	return nil
}

func (planner *packageAssetPlanner) addDirectoryAssets(directory string) error {
	directory, err := cleanManifestAssetDirectory(directory)
	if err != nil {
		return fmt.Errorf("assets %q in %s %s", directory, manifestName, err)
	}
	sourceRoot := filepath.Join(planner.appDir, filepath.FromSlash(directory))
	if err := validatePathBelowRoot(planner.appDir, sourceRoot, "asset directory", true); err != nil {
		return err
	}
	info, err := os.Lstat(sourceRoot)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("asset directory %s does not exist", sourceRoot)
	}
	if err != nil {
		return fmt.Errorf("inspect asset directory %s: %w", sourceRoot, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("asset directory %s is a symlink; symlink paths are not supported", sourceRoot)
	}
	if !info.IsDir() {
		return fmt.Errorf("asset directory %s is not a directory", sourceRoot)
	}
	return filepath.WalkDir(sourceRoot, func(sourcePath string, entry os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("inspect asset path %s: %w", sourcePath, err)
		}
		if sourcePath == sourceRoot {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("asset path %s is a symlink; symlink paths are not supported", sourcePath)
		}
		relative, err := filepath.Rel(sourceRoot, sourcePath)
		if err != nil {
			return fmt.Errorf("resolve asset path %s: %w", sourcePath, err)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := regularFileNoFollow(sourcePath, "asset path")
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("asset path %s is not a regular file", sourcePath)
		}
		logical := filepath.ToSlash(relative)
		if logical == indexHTMLAssetName {
			return planner.setCustomIndex(sourcePath)
		}
		return planner.addAsset(logical, sourcePath, true)
	})
}

func (planner *packageAssetPlanner) addListedAssets(assets []string) error {
	for _, raw := range assets {
		asset, err := cleanPackageAssetName(raw)
		if err != nil {
			return err
		}
		source, exists, err := inspectPackageAssetSource(planner.appDir, asset)
		if err != nil {
			return err
		}
		if asset == indexHTMLAssetName {
			if !exists {
				return fmt.Errorf("custom standalone entrypoint %s was not found", indexHTMLAssetName)
			}
			if err := planner.setCustomIndex(source); err != nil {
				return err
			}
			continue
		}
		if err := planner.addAsset(asset, source, exists); err != nil {
			return err
		}
	}
	return nil
}

func (planner *packageAssetPlanner) addAsset(logicalName, sourcePath string, exists bool) error {
	logicalName, err := cleanPackageAssetName(logicalName)
	if err != nil {
		return err
	}
	if err := planner.reserveAsset(logicalName, "manifest asset"); err != nil {
		return err
	}
	planner.plan.Assets = append(planner.plan.Assets, plannedPackageAsset{
		LogicalName: logicalName,
		SourcePath:  sourcePath,
		Exists:      exists,
	})
	return nil
}

func (planner *packageAssetPlanner) reserveAsset(name, owner string) error {
	name, err := cleanPackageAssetName(name)
	if err != nil {
		return err
	}
	if previous, ok := planner.occupied[name]; ok {
		return fmt.Errorf("package asset %q collides with %s", name, previous)
	}
	planner.occupied[name] = owner
	if isCompressiblePackageAsset(name) {
		if planner.options.compress["gzip"] {
			if err := planner.reserveSidecar(name+".gz", owner+" compressed sidecar"); err != nil {
				return err
			}
		}
		if planner.options.compress["br"] {
			if err := planner.reserveSidecar(name+".br", owner+" compressed sidecar"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (planner *packageAssetPlanner) reserveSidecar(name, owner string) error {
	name, err := cleanPackageAssetName(name)
	if err != nil {
		return err
	}
	if previous, ok := planner.occupied[name]; ok {
		return fmt.Errorf("package asset %q collides with %s", name, previous)
	}
	planner.occupied[name] = owner
	return nil
}

func (planner *packageAssetPlanner) setCustomIndex(sourcePath string) error {
	if planner.plan.CustomIndexPath != "" {
		return fmt.Errorf("package asset %q collides with existing HTML entrypoint", indexHTMLAssetName)
	}
	if _, err := regularFileNoFollow(sourcePath, "index asset"); err != nil {
		return err
	}
	planner.plan.CustomIndexPath = sourcePath
	planner.plan.GenerateIndex = false
	return nil
}

func (planner *packageAssetPlanner) finish() packageAssetPlan {
	sort.Slice(planner.plan.Assets, func(first, second int) bool {
		return planner.plan.Assets[first].LogicalName < planner.plan.Assets[second].LogicalName
	})
	if planner.plan.CustomIndexPath == "" {
		planner.plan.GenerateIndex = true
	}
	return planner.plan
}

func inspectPackageAssetSource(appDir, asset string) (string, bool, error) {
	asset, err := cleanManifestAssetPath(asset)
	if err != nil {
		return "", false, err
	}
	source := filepath.Join(appDir, filepath.FromSlash(asset))
	if err := validatePathBelowRoot(appDir, source, "asset path", true); err != nil {
		return "", false, err
	}
	info, err := os.Lstat(source)
	if errors.Is(err, os.ErrNotExist) {
		return source, false, nil
	} else if err != nil {
		return "", false, fmt.Errorf("inspect asset %s: %w", source, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", false, fmt.Errorf("asset path %s is a symlink; symlink paths are not supported", source)
	}
	if !info.Mode().IsRegular() {
		return "", false, fmt.Errorf("asset path %s is not a regular file", source)
	}
	return source, true, nil
}

func packageOutputDirectory(options packageOptions, layout BuildLayout) string {
	if options.outDir != "" {
		return options.outDir
	}
	return layout.PackageDir
}

func cleanPackageArtifacts(directory, wasmName string) error {
	if err := os.Remove(filepath.Join(directory, packageMetadataName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale package artifact %s: %w", packageMetadataName, err)
	}
	names := []string{
		wasmName,
		wasmName + ".gz",
		wasmName + ".br",
		"bundle.wasm",
		"bundle.wasm.gz",
		"bundle.wasm.br",
		"bundle.wasm.zst",
		"main.wasm",
		"main.wasm.gz",
		"main.wasm.br",
		"main.wasm.zst",
		"main.tiny.wasm",
		"main.tiny.wasm.gz",
		"main.tiny.wasm.br",
		"wasm_exec.js",
		"wasm_exec.tiny.js",
		"service-worker.js",
		indexHTMLAssetName,
		"styles.css",
		legacyPackageManifest,
		assetManifestName,
	}
	for _, name := range names {
		if err := os.Remove(filepath.Join(directory, name)); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return fmt.Errorf("remove stale package artifact %s: %w", name, err)
		}
	}
	assetsDir := filepath.Join(directory, assetDirectoryName)
	if err := os.RemoveAll(assetsDir); err != nil {
		return fmt.Errorf("remove stale package assets directory: %w", err)
	}
	return nil
}

func verifyPublishedPackage(directory string) error {
	ownership := inspectPackageOwnership(directory)
	if ownership.State == packageOwnedCurrent {
		return nil
	}
	if err := os.Remove(filepath.Join(directory, packageMetadataName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("published package failed integrity verification: %s; remove completion marker: %w", ownership.Reason, err)
	}
	return fmt.Errorf("published package failed integrity verification: %s", ownership.Reason)
}

func writeJSONFile(path string, value any, description string) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", description, err)
	}
	content = append(content, '\n')
	if err := writeFileAtomic(path, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

type htmlRewriteOptions struct {
	preload       bool
	wasmPath      string
	runtimePath   string
	stylePaths    []string
	styleRewrites map[string]string
}

func writeRewrittenIndex(sourcePath, destinationPath string, options htmlRewriteOptions) error {
	if _, err := regularFileNoFollow(sourcePath, "index asset"); err != nil {
		return err
	}
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", sourcePath, err)
	}
	rewritten := rewriteIndexHTML(string(content), options)
	if err := writeFileAtomic(destinationPath, []byte(rewritten), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", destinationPath, err)
	}
	return nil
}

func writeGeneratedIndex(destinationPath string, options htmlRewriteOptions) error {
	content := generateIndexHTML(options)
	if err := writeFileAtomic(destinationPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", destinationPath, err)
	}
	return nil
}

func generateIndexHTML(options htmlRewriteOptions) string {
	var builder strings.Builder
	builder.WriteString("<!doctype html>\n")
	builder.WriteString("<html lang=\"en\">\n")
	builder.WriteString("<head>\n")
	builder.WriteString("    <meta charset=\"utf-8\" />\n")
	builder.WriteString("    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\" />\n")
	builder.WriteString("    <title>goframe app</title>\n")
	if preload := preloadHTML(options); preload != "" {
		for _, line := range strings.Split(preload, "\n") {
			builder.WriteString("    ")
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}
	for _, style := range options.stylePaths {
		builder.WriteString(fmt.Sprintf("    <link rel=\"stylesheet\" href=\"%s\" />\n", style))
	}
	builder.WriteString("</head>\n")
	builder.WriteString("<body>\n")
	builder.WriteString("    <div id=\"root\">Loading...</div>\n")
	builder.WriteString("    ")
	builder.WriteString(runtimeHTML(options))
	builder.WriteString("\n")
	for _, line := range strings.Split(bootstrapHTML(options), "\n") {
		builder.WriteString("    ")
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	builder.WriteString("</body>\n")
	builder.WriteString("</html>\n")
	return builder.String()
}

func rewriteIndexHTML(content string, options htmlRewriteOptions) string {
	preload := preloadHTML(options)
	content, replaced := replaceHTMLBlock(content, preloadBlockName, preload)
	if !replaced && preload != "" {
		content = strings.Replace(content, "</head>", preload+"\n</head>", 1)
	}

	runtime := runtimeHTML(options)
	content, replaced = replaceHTMLBlock(content, runtimeBlockName, runtime)
	if !replaced {
		content = strings.ReplaceAll(content, runtimeAssetName, options.runtimePath)
	}

	bootstrap := bootstrapHTML(options)
	content, replaced = replaceHTMLBlock(content, bootstrapBlockName, bootstrap)
	if !replaced {
		content = strings.ReplaceAll(content, "main.wasm", options.wasmPath)
		content = strings.ReplaceAll(content, "bundle.wasm", options.wasmPath)
	}

	for source, destination := range options.styleRewrites {
		content = strings.ReplaceAll(content, `href="`+source+`"`, `href="`+destination+`"`)
		content = strings.ReplaceAll(content, `href="./`+source+`"`, `href="`+destination+`"`)
	}
	return content
}

func preloadHTML(options htmlRewriteOptions) string {
	if !options.preload {
		return ""
	}
	lines := []string{
		fmt.Sprintf(`<link rel="preload" href="%s" as="fetch" type="application/wasm" crossorigin>`, options.wasmPath),
		fmt.Sprintf(`<link rel="preload" href="%s" as="script">`, options.runtimePath),
	}
	for _, style := range options.stylePaths {
		lines = append(lines, fmt.Sprintf(`<link rel="preload" href="%s" as="style">`, style))
	}
	return strings.Join(lines, "\n")
}

func runtimeHTML(options htmlRewriteOptions) string {
	return fmt.Sprintf(`<script src="%s"></script>`, options.runtimePath)
}

func bootstrapHTML(options htmlRewriteOptions) string {
	return fmt.Sprintf(`<script>
    const go = new Go();
    WebAssembly.instantiateStreaming(fetch("%s"), go.importObject)
        .then((result) => go.run(result.instance));
</script>`, options.wasmPath)
}

func replaceHTMLBlock(content, name, replacement string) (string, bool) {
	startMarker := "<!-- goframe:" + name + " -->"
	endMarker := "<!-- /goframe:" + name + " -->"
	start := strings.Index(content, startMarker)
	if start < 0 {
		return content, false
	}
	end := strings.Index(content[start:], endMarker)
	if end < 0 {
		return content, false
	}
	end += start
	blockEnd := end + len(endMarker)
	block := startMarker + "\n" + replacement + "\n" + endMarker
	return content[:start] + block + content[blockEnd:], true
}

func writePackageAsset(sourcePath, assetsDir, logicalName string, options packageOptions) (packageAsset, error) {
	logicalName, err := cleanPackageAssetName(logicalName)
	if err != nil {
		return packageAsset{}, err
	}
	if _, err := regularFileNoFollow(sourcePath, "package asset"); err != nil {
		return packageAsset{}, err
	}
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return packageAsset{}, fmt.Errorf("read %s: %w", sourcePath, err)
	}
	hash := shortContentHash(content)
	outputName := logicalName
	if options.assetHash {
		outputName = hashedAssetName(logicalName, hash)
	}
	destinationPath := filepath.Join(assetsDir, filepath.FromSlash(outputName))
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return packageAsset{}, fmt.Errorf("create package asset directory for %s: %w", outputName, err)
	}
	if err := writeFileAtomic(destinationPath, content, 0o644); err != nil {
		return packageAsset{}, fmt.Errorf("write package asset %s: %w", outputName, err)
	}

	asset := packageAsset{
		Path: path.Join(assetDirectoryName, outputName),
		Type: contentTypeForAsset(logicalName),
	}
	if options.assetHash {
		asset.Hash = hash
	}
	if isCompressiblePackageAsset(logicalName) {
		if options.compress["gzip"] {
			compressedPath := destinationPath + ".gz"
			if err := gzipFile(destinationPath, compressedPath); err != nil {
				return packageAsset{}, err
			}
			if asset.Compressed == nil {
				asset.Compressed = map[string]string{}
			}
			asset.Compressed["gzip"] = asset.Path + ".gz"
			fmt.Printf("compressed %s\n", asset.Compressed["gzip"])
		}
		if options.compress["br"] {
			compressedPath := destinationPath + ".br"
			if err := brotliFile(destinationPath, compressedPath); err != nil {
				return packageAsset{}, err
			}
			if asset.Compressed == nil {
				asset.Compressed = map[string]string{}
			}
			asset.Compressed["br"] = asset.Path + ".br"
			fmt.Printf("compressed %s\n", asset.Compressed["br"])
		}
	}
	return asset, nil
}

func cleanPackageAssetName(name string) (string, error) {
	for _, part := range strings.Split(filepath.ToSlash(name), "/") {
		if part == ".." {
			return "", fmt.Errorf("package asset logical name %q must not contain .. components", name)
		}
	}
	clean := path.Clean(filepath.ToSlash(name))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		return "", fmt.Errorf("package asset logical name %q must be a relative child path", name)
	}
	return clean, nil
}

func shortContentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])[:packageHashLength]
}

func hashedAssetName(name, hash string) string {
	clean := path.Clean(filepath.ToSlash(name))
	directory, base := path.Split(clean)
	extension := path.Ext(base)
	stem := strings.TrimSuffix(base, extension)
	return directory + stem + "." + hash + extension
}

func contentTypeForAsset(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".wasm":
		return "application/wasm"
	case ".js":
		return "text/javascript"
	case ".css":
		return "text/css"
	case ".html":
		return "text/html; charset=utf-8"
	}
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}

func isCompressiblePackageAsset(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".wasm", ".js", ".css":
		return true
	default:
		return false
	}
}

func publishPackageArtifacts(sourceDir, destinationDir string) error {
	var directories []string
	var files []string
	err := filepath.WalkDir(sourceDir, func(sourcePath string, entry os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("inspect package artifact %s: %w", sourcePath, err)
		}
		if sourcePath == sourceDir {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("package artifact %s is a symlink; symlink paths are not supported", sourcePath)
		}
		relative, err := filepath.Rel(sourceDir, sourcePath)
		if err != nil {
			return fmt.Errorf("resolve package artifact %s: %w", sourcePath, err)
		}
		if entry.IsDir() {
			directories = append(directories, relative)
			return nil
		}
		if _, err := regularFileNoFollow(sourcePath, "package artifact"); err != nil {
			return err
		}
		files = append(files, relative)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(files, func(first, second int) bool {
		if files[first] == packageMetadataName {
			return false
		}
		if files[second] == packageMetadataName {
			return true
		}
		return files[first] < files[second]
	})
	for _, relative := range directories {
		destinationPath := filepath.Join(destinationDir, relative)
		if err := mkdirAllBelowRoot(destinationDir, destinationPath, "package artifact directory"); err != nil {
			return err
		}
	}
	for _, relative := range files {
		sourcePath := filepath.Join(sourceDir, relative)
		destinationPath := filepath.Join(destinationDir, relative)
		if err := validatePathBelowRoot(destinationDir, destinationPath, "package artifact", true); err != nil {
			return err
		}
		if err := copyFile(sourcePath, destinationPath); err != nil {
			return err
		}
	}
	return nil
}

func gzipFile(sourcePath, destinationPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", sourcePath, err)
	}
	defer source.Close()
	destination, err := os.Create(destinationPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", destinationPath, err)
	}
	writer, err := gzip.NewWriterLevel(destination, gzip.BestCompression)
	if err != nil {
		destination.Close()
		return fmt.Errorf("create gzip writer: %w", err)
	}
	writer.Header.ModTime = time.Unix(0, 0)
	writer.Header.Name = filepath.Base(sourcePath)

	_, copyErr := io.Copy(writer, source)
	writerErr := writer.Close()
	fileErr := destination.Close()
	if copyErr != nil {
		return fmt.Errorf("compress %s: %w", sourcePath, copyErr)
	}
	if writerErr != nil {
		return fmt.Errorf("finish %s: %w", destinationPath, writerErr)
	}
	if fileErr != nil {
		return fmt.Errorf("close %s: %w", destinationPath, fileErr)
	}
	return nil
}

func brotliFile(sourcePath, destinationPath string) error {
	brotliPath, err := exec.LookPath("brotli")
	if err != nil {
		return errors.New("brotli not found in PATH; install brotli or omit --compress=br")
	}
	command := exec.Command(brotliPath, "-f", "-q", "11", "-o", destinationPath, sourcePath)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("brotli compression failed: %w", err)
	}
	return nil
}
