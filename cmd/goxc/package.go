package main

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type packageOptions struct {
	appDir   string
	compiler string
	outDir   string
	compress map[string]bool
}

type packageManifest struct {
	Name             string   `json:"name"`
	Compiler         string   `json:"compiler"`
	WASM             string   `json:"wasm"`
	Assets           []string `json:"assets"`
	ToolchainVersion string   `json:"toolchainVersion"`
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
		case strings.HasPrefix(arg, "-"):
			return packageOptions{}, fmt.Errorf("unknown package flag %q", arg)
		case options.appDir == "":
			options.appDir = arg
		default:
			return packageOptions{}, fmt.Errorf("unexpected package argument %q", arg)
		}
	}
	if options.appDir == "" {
		return packageOptions{}, errors.New("usage: goxc package <app-directory> [--compiler=go|tinygo] [--out=directory] [--compress=gzip,br]")
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
	options.outDir = packageOutputDirectory(options, manifest)
	if err := ensureAppDirectory(options.appDir); err != nil {
		return err
	}
	if err := generateForBuild(options.appDir); err != nil {
		return fmt.Errorf("generate GOX: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "goxc-package-*")
	if err != nil {
		return fmt.Errorf("create temporary package directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tempWASM := filepath.Join(tempDir, manifest.WASM)
	entryPath := filepath.Join(options.appDir, manifest.Entry)
	fmt.Printf("packaging %s with %s compiler\n", options.appDir, options.compiler)
	if err := compileWASM(options.compiler, entryPath, tempWASM); err != nil {
		return err
	}
	stageDir := filepath.Join(tempDir, "stage")
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return fmt.Errorf("create staging package directory: %w", err)
	}
	wasmDestination := filepath.Join(stageDir, manifest.WASM)
	if err := copyFile(tempWASM, wasmDestination); err != nil {
		return err
	}
	runtimeSource, err := wasmExecPath(options.compiler)
	if err != nil {
		return err
	}
	if err := copyFile(runtimeSource, filepath.Join(stageDir, "wasm_exec.js")); err != nil {
		return err
	}

	copiedAssets := make([]string, 0, len(manifest.Assets))
	for _, asset := range manifest.Assets {
		source := filepath.Join(options.appDir, asset)
		if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
			fmt.Printf("asset %s not found; skipping\n", source)
			continue
		} else if err != nil {
			return fmt.Errorf("inspect asset %s: %w", source, err)
		}
		if err := copyFile(source, filepath.Join(stageDir, asset)); err != nil {
			return err
		}
		copiedAssets = append(copiedAssets, asset)
	}

	if err := writePackageManifest(stageDir, packageManifest{
		Name:             manifest.Name,
		Compiler:         options.compiler,
		WASM:             manifest.WASM,
		Assets:           copiedAssets,
		ToolchainVersion: version,
	}); err != nil {
		return err
	}
	if options.compress["gzip"] {
		if err := gzipFile(wasmDestination, wasmDestination+".gz"); err != nil {
			return err
		}
		fmt.Printf("compressed %s\n", filepath.Join(options.outDir, manifest.WASM+".gz"))
	}
	if options.compress["br"] {
		if err := brotliFile(wasmDestination, wasmDestination+".br"); err != nil {
			return err
		}
		fmt.Printf("compressed %s\n", filepath.Join(options.outDir, manifest.WASM+".br"))
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

func packageOutputDirectory(options packageOptions, manifest projectManifest) string {
	if options.outDir != "" {
		return options.outDir
	}
	return filepath.Join(options.appDir, manifest.Output)
}

func cleanPackageArtifacts(directory, wasmName string) error {
	names := []string{
		wasmName,
		wasmName + ".gz",
		wasmName + ".br",
		"main.tiny.wasm",
		"main.tiny.wasm.gz",
		"main.tiny.wasm.br",
		"wasm_exec.js",
		"wasm_exec.tiny.js",
		"manifest.json",
	}
	for _, name := range names {
		if err := os.Remove(filepath.Join(directory, name)); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return fmt.Errorf("remove stale package artifact %s: %w", name, err)
		}
	}
	return nil
}

func writePackageManifest(directory string, manifest packageManifest) error {
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode package manifest: %w", err)
	}
	content = append(content, '\n')
	path := filepath.Join(directory, "manifest.json")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
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
