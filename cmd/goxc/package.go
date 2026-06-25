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
	appDir    string
	compiler  string
	outDir    string
	workspace string
	compress  map[string]bool
	assetHash bool
	preload   bool
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
	layout, err := newBuildLayout(layoutOptions{
		appDir:    options.appDir,
		compiler:  options.compiler,
		profile:   packageProfile(options.assetHash, options.preload, options.compress),
		workspace: options.workspace,
	})
	if err != nil {
		return err
	}
	explicitOutDir := options.outDir != ""
	options.outDir = packageOutputDirectory(options, layout)
	if err := rejectSymlinkPath(options.outDir, "package output directory"); err != nil {
		return err
	}
	if explicitOutDir {
		if err := validatePackageDestination(options.outDir); err != nil {
			return err
		}
	}
	entryPath, err := prepareBuildWorkspace(layout, manifest)
	if err != nil {
		return fmt.Errorf("prepare package workspace: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "goxc-package-*")
	if err != nil {
		return fmt.Errorf("create temporary package directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	wasmLogicalName := path.Base(filepath.ToSlash(filepath.Clean(manifest.WASM)))
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

	copiedAssets := make([]string, 0, len(manifest.Assets))
	styleRewrites := map[string]string{}
	for _, asset := range manifest.Assets {
		asset, source, exists, err := resolvePackageAssetSource(options.appDir, asset)
		if err != nil {
			return err
		}
		if !exists {
			fmt.Printf("asset %s not found; skipping\n", source)
			continue
		}
		if asset == indexHTMLAssetName {
			copiedAssets = append(copiedAssets, asset)
			continue
		}
		packaged, err := writePackageAsset(source, assetsDir, asset, options)
		if err != nil {
			return err
		}
		assets[asset] = packaged
		if strings.EqualFold(path.Ext(asset), ".css") {
			entrypoints.Styles = append(entrypoints.Styles, packaged.Path)
			styleRewrites[asset] = packaged.Path
		}
		copiedAssets = append(copiedAssets, asset)
	}

	sort.Strings(entrypoints.Styles)
	if containsString(copiedAssets, indexHTMLAssetName) {
		if err := writeRewrittenIndex(filepath.Join(options.appDir, indexHTMLAssetName), filepath.Join(stageDir, indexHTMLAssetName), htmlRewriteOptions{
			preload:       options.preload,
			wasmPath:      wasmAsset.Path,
			runtimePath:   runtimeAsset.Path,
			styleRewrites: styleRewrites,
			stylePaths:    entrypoints.Styles,
		}); err != nil {
			return err
		}
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
		ToolchainVersion: version,
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

	if err := os.MkdirAll(options.outDir, 0o755); err != nil {
		return fmt.Errorf("create package directory: %w", err)
	}
	if err := cleanPackageArtifacts(options.outDir, manifest.WASM); err != nil {
		return err
	}
	if err := publishPackageArtifacts(stageDir, options.outDir); err != nil {
		return err
	}

	fmt.Printf("packaged %s\n", options.outDir)
	return nil
}

func resolvePackageAssetSource(appDir, asset string) (string, string, bool, error) {
	asset = path.Clean(filepath.ToSlash(asset))
	source := filepath.Join(appDir, asset)
	if err := rejectSymlinkPath(source, "asset path"); err != nil {
		return "", "", false, err
	}
	if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
		return asset, source, false, nil
	} else if err != nil {
		return "", "", false, fmt.Errorf("inspect asset %s: %w", source, err)
	}
	return asset, source, true, nil
}

func packageOutputDirectory(options packageOptions, layout BuildLayout) string {
	if options.outDir != "" {
		return options.outDir
	}
	return layout.PackageDir
}

func cleanPackageArtifacts(directory, wasmName string) error {
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
		"styles.css",
		legacyPackageManifest,
		assetManifestName,
		packageMetadataName,
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

func writeJSONFile(path string, value any, description string) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", description, err)
	}
	content = append(content, '\n')
	if err := os.WriteFile(path, content, 0o644); err != nil {
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
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", sourcePath, err)
	}
	rewritten := rewriteIndexHTML(string(content), options)
	if err := os.WriteFile(destinationPath, []byte(rewritten), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", destinationPath, err)
	}
	return nil
}

func rewriteIndexHTML(content string, options htmlRewriteOptions) string {
	preload := ""
	if options.preload {
		lines := []string{
			fmt.Sprintf(`<link rel="preload" href="%s" as="fetch" type="application/wasm" crossorigin>`, options.wasmPath),
			fmt.Sprintf(`<link rel="preload" href="%s" as="script">`, options.runtimePath),
		}
		for _, style := range options.stylePaths {
			lines = append(lines, fmt.Sprintf(`<link rel="preload" href="%s" as="style">`, style))
		}
		preload = strings.Join(lines, "\n")
	}
	content, replaced := replaceHTMLBlock(content, preloadBlockName, preload)
	if !replaced && preload != "" {
		content = strings.Replace(content, "</head>", preload+"\n</head>", 1)
	}

	runtime := fmt.Sprintf(`<script src="%s"></script>`, options.runtimePath)
	content, replaced = replaceHTMLBlock(content, runtimeBlockName, runtime)
	if !replaced {
		content = strings.ReplaceAll(content, runtimeAssetName, options.runtimePath)
	}

	bootstrap := fmt.Sprintf(`<script>
    const go = new Go();
    WebAssembly.instantiateStreaming(fetch("%s"), go.importObject)
        .then((result) => go.run(result.instance));
</script>`, options.wasmPath)
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
	if err := os.WriteFile(destinationPath, content, 0o644); err != nil {
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func publishPackageArtifacts(sourceDir, destinationDir string) error {
	return filepath.WalkDir(sourceDir, func(sourcePath string, entry os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("inspect package artifact %s: %w", sourcePath, err)
		}
		if sourcePath == sourceDir {
			return nil
		}
		relative, err := filepath.Rel(sourceDir, sourcePath)
		if err != nil {
			return fmt.Errorf("resolve package artifact %s: %w", sourcePath, err)
		}
		destinationPath := filepath.Join(destinationDir, relative)
		if entry.IsDir() {
			if err := os.MkdirAll(destinationPath, 0o755); err != nil {
				return fmt.Errorf("create package artifact directory %s: %w", destinationPath, err)
			}
			return nil
		}
		if err := copyFile(sourcePath, destinationPath); err != nil {
			return err
		}
		return nil
	})
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
