package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type exportOptions struct {
	appDir    string
	outDir    string
	workspace string
	force     bool
}

func exportCommand(args []string) error {
	options, err := parseExportOptions(args)
	if err != nil {
		return err
	}
	return exportApp(options)
}

func parseExportOptions(args []string) (exportOptions, error) {
	var options exportOptions
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case strings.HasPrefix(arg, "--out="):
			options.outDir = strings.TrimPrefix(arg, "--out=")
		case arg == "--out":
			index++
			if index >= len(args) {
				return exportOptions{}, errors.New("--out requires a value")
			}
			options.outDir = args[index]
		case strings.HasPrefix(arg, "--workspace="):
			options.workspace = strings.TrimPrefix(arg, "--workspace=")
		case arg == "--workspace":
			index++
			if index >= len(args) {
				return exportOptions{}, errors.New("--workspace requires a value")
			}
			options.workspace = args[index]
		case arg == "--force":
			options.force = true
		case strings.HasPrefix(arg, "-"):
			return exportOptions{}, fmt.Errorf("unknown export flag %q", arg)
		case options.appDir == "":
			options.appDir = arg
		default:
			return exportOptions{}, fmt.Errorf("unexpected export argument %q", arg)
		}
	}
	if options.appDir == "" || options.outDir == "" {
		return exportOptions{}, errors.New("usage: goxc export <app-directory> --out=directory [--workspace=directory] [--force]")
	}
	return options, nil
}

func exportApp(options exportOptions) error {
	if err := ensureAppDirectory(options.appDir); err != nil {
		return err
	}
	layout, err := newBuildLayout(layoutOptions{appDir: options.appDir, workspace: options.workspace})
	if err != nil {
		return err
	}
	if err := validateWorkspaceRoot(layout); err != nil {
		return err
	}
	if err := validatePathBelowRoot(layout.WorkspaceRoot, layout.PackageDir, "standalone package directory", false); err != nil {
		return err
	}
	if info, err := os.Stat(layout.PackageDir); err != nil {
		return fmt.Errorf("no standalone package found; run `goxc package %s` first", options.appDir)
	} else if !info.IsDir() {
		return fmt.Errorf("standalone package path is not a directory: %s", layout.PackageDir)
	}
	if pathsOverlap(layout.PackageDir, options.outDir) {
		return fmt.Errorf("export output directory %s must not overlap standalone package directory %s", options.outDir, layout.PackageDir)
	}
	if err := validateExplicitPathRoot(options.outDir, "export output directory", true); err != nil {
		return err
	}
	if err := validateExportDestination(options.outDir, options.force); err != nil {
		return err
	}
	if err := validateExplicitPathRoot(options.outDir, "export output directory", true); err != nil {
		return err
	}
	if err := os.MkdirAll(options.outDir, 0o755); err != nil {
		return fmt.Errorf("create export directory: %w", err)
	}
	if err := cleanPackageArtifacts(options.outDir, "bundle.wasm"); err != nil {
		return err
	}
	if err := publishPackageArtifacts(layout.PackageDir, options.outDir); err != nil {
		return err
	}
	fmt.Printf("exported %s -> %s\n", layout.PackageDir, options.outDir)
	return nil
}

func validateExportDestination(outDir string, force bool) error {
	if err := validateExplicitPathRoot(outDir, "export output directory", true); err != nil {
		return err
	}
	entries, err := os.ReadDir(outDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect export directory: %w", err)
	}
	if len(entries) == 0 || force || isGoframeOwnedExport(outDir) {
		return nil
	}
	return fmt.Errorf("export output directory %s is not empty and does not look like a previous GoFrame export; pass --force to treat it as tool-owned and overwrite package artifacts", outDir)
}

func validatePackageDestination(outDir string) error {
	if err := validateExplicitPathRoot(outDir, "package output directory", true); err != nil {
		return err
	}
	entries, err := os.ReadDir(outDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect package output directory: %w", err)
	}
	if len(entries) == 0 || isGoframeOwnedExport(outDir) {
		return nil
	}
	return fmt.Errorf("package output directory %s is not empty and does not look like a previous GoFrame package; use the default hidden workspace output, choose an empty directory, or export a package with `goxc export --force` when overwriting is intentional", outDir)
}

func isGoframeOwnedExport(directory string) bool {
	owned, _ := inspectPackageOwnership(directory)
	return owned
}

func inspectPackageOwnership(directory string) (bool, error) {
	if ok, err := validCurrentPackageMetadata(filepath.Join(directory, packageMetadataName)); ok || err != nil {
		return ok, err
	}
	if ok, err := validAssetManifestMetadata(filepath.Join(directory, assetManifestName)); ok || err != nil {
		return ok, err
	}
	if ok, err := validLegacyPackageSignature(directory); ok || err != nil {
		return ok, err
	}
	return false, nil
}

func validCurrentPackageMetadata(path string) (bool, error) {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	var metadata packageMetadata
	if err := readJSONRegular(path, "package metadata", &metadata); err != nil {
		return false, nil
	}
	if metadata.Version != 1 || metadata.Name == "" || metadata.AssetsDir != assetDirectoryName {
		return false, nil
	}
	if metadata.Compiler != "go" && metadata.Compiler != "tinygo" {
		return false, nil
	}
	if !safeChildPath(metadata.Entrypoints.HTML) || !safeChildPath(metadata.Entrypoints.WASM) || !safeChildPath(metadata.Entrypoints.Runtime) {
		return false, nil
	}
	return true, nil
}

func validAssetManifestMetadata(path string) (bool, error) {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	var manifest assetManifest
	if err := readJSONRegular(path, "asset manifest", &manifest); err != nil {
		return false, nil
	}
	if manifest.Version != 1 || len(manifest.Assets) == 0 {
		return false, nil
	}
	if !safeChildPath(manifest.Entrypoints.WASM) || !safeChildPath(manifest.Entrypoints.Runtime) {
		return false, nil
	}
	for _, asset := range manifest.Assets {
		if !safeChildPath(asset.Path) || asset.Type == "" {
			return false, nil
		}
		for _, compressed := range asset.Compressed {
			if !safeChildPath(compressed) {
				return false, nil
			}
		}
	}
	return true, nil
}

func validLegacyPackageSignature(directory string) (bool, error) {
	manifestPath := filepath.Join(directory, legacyPackageManifest)
	if _, err := os.Lstat(manifestPath); errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	var generic map[string]any
	if err := readJSONRegular(manifestPath, "legacy package manifest", &generic); err != nil {
		return false, nil
	}
	wasmRoot := filepath.Join(directory, "main.wasm")
	if _, err := regularFileNoFollow(wasmRoot, "legacy package WASM"); err != nil {
		wasmRoot = filepath.Join(directory, "bundle.wasm")
		if _, err := regularFileNoFollow(wasmRoot, "legacy package WASM"); err != nil {
			return false, nil
		}
	}
	if _, err := regularFileNoFollow(filepath.Join(directory, runtimeAssetName), "legacy runtime asset"); err != nil {
		return false, nil
	}
	return true, nil
}

func readJSONRegular(path string, description string, value any) error {
	if _, err := regularFileNoFollow(path, description); err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(content, value); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
