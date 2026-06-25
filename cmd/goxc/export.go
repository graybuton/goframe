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
	if ownership := inspectPackageOwnership(layout.PackageDir); ownership.State != packageOwnedCurrent {
		return fmt.Errorf("standalone package %s is not a complete current GoFrame package: %s", layout.PackageDir, ownership.Reason)
	}
	if err := ensureNoPhysicalOverlap(options.outDir, layout.PackageDir, "export output directory", "standalone package directory"); err != nil {
		return err
	}
	if err := ensureNoPhysicalOverlap(options.outDir, layout.AppDir, "export output directory", "application directory"); err != nil {
		return err
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
	if err := verifyPublishedPackage(options.outDir); err != nil {
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
	ownership := inspectPackageOwnership(outDir)
	if ownership.State == packageIncompleteOrInvalid {
		return fmt.Errorf("export output directory %s contains incomplete or invalid GoFrame package metadata: %s", outDir, ownership.Reason)
	}
	if len(entries) == 0 || force || ownership.IsOwned() {
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
	ownership := inspectPackageOwnership(outDir)
	if ownership.State == packageIncompleteOrInvalid {
		return fmt.Errorf("package output directory %s contains incomplete or invalid GoFrame package metadata: %s", outDir, ownership.Reason)
	}
	if len(entries) == 0 || ownership.IsOwned() {
		return nil
	}
	return fmt.Errorf("package output directory %s is not empty and does not look like a previous GoFrame package; use the default hidden workspace output, choose an empty directory, or export a package with `goxc export --force` when overwriting is intentional", outDir)
}

func isGoframeOwnedExport(directory string) bool {
	return inspectPackageOwnership(directory).IsOwned()
}

type packageOwnershipState uint8

const (
	packageUnowned packageOwnershipState = iota
	packageOwnedCurrent
	packageOwnedLegacy
	packageIncompleteOrInvalid
)

type packageOwnership struct {
	State  packageOwnershipState
	Reason string
}

func (ownership packageOwnership) IsOwned() bool {
	return ownership.State == packageOwnedCurrent || ownership.State == packageOwnedLegacy
}

func inspectPackageOwnership(directory string) packageOwnership {
	if exists, err := pathExistsNoFollow(filepath.Join(directory, packageMetadataName)); err != nil {
		return packageOwnership{State: packageIncompleteOrInvalid, Reason: err.Error()}
	} else if exists {
		return inspectCurrentPackageOwnership(directory)
	}
	if exists, err := pathExistsNoFollow(filepath.Join(directory, assetManifestName)); err != nil {
		return packageOwnership{State: packageIncompleteOrInvalid, Reason: err.Error()}
	} else if exists {
		if _, err := readAssetManifestMetadata(filepath.Join(directory, assetManifestName)); err != nil {
			return packageOwnership{State: packageIncompleteOrInvalid, Reason: err.Error()}
		}
		return packageOwnership{State: packageUnowned, Reason: "asset manifest is metadata only; goframe-package.json is required"}
	}
	if validLegacyPackageSignature(directory) {
		return packageOwnership{State: packageOwnedLegacy, Reason: "recognized historical GoFrame package manifest"}
	}
	return packageOwnership{State: packageUnowned, Reason: "no complete GoFrame package metadata found"}
}

func inspectCurrentPackageOwnership(directory string) packageOwnership {
	metadata, err := readCurrentPackageMetadata(filepath.Join(directory, packageMetadataName))
	if err != nil {
		return packageOwnership{State: packageIncompleteOrInvalid, Reason: err.Error()}
	}
	manifest, err := readAssetManifestMetadata(filepath.Join(directory, assetManifestName))
	if err != nil {
		return packageOwnership{State: packageIncompleteOrInvalid, Reason: "companion asset manifest is required: " + err.Error()}
	}
	if metadata.Entrypoints.WASM != manifest.Entrypoints.WASM || metadata.Entrypoints.Runtime != manifest.Entrypoints.Runtime {
		return packageOwnership{State: packageIncompleteOrInvalid, Reason: "package metadata and asset manifest entrypoints do not match"}
	}
	for _, entry := range []struct {
		path        string
		description string
	}{
		{metadata.Entrypoints.HTML, "package HTML entrypoint"},
		{metadata.Entrypoints.WASM, "package WASM entrypoint"},
		{metadata.Entrypoints.Runtime, "package runtime entrypoint"},
	} {
		if err := validatePackageOwnedPath(directory, entry.path, entry.description); err != nil {
			return packageOwnership{State: packageIncompleteOrInvalid, Reason: err.Error()}
		}
	}
	return packageOwnership{State: packageOwnedCurrent, Reason: "complete current GoFrame package metadata found"}
}

func readCurrentPackageMetadata(path string) (packageMetadata, error) {
	var metadata packageMetadata
	if err := readJSONRegular(path, "package metadata", &metadata); err != nil {
		return packageMetadata{}, err
	}
	if metadata.Version != 1 || metadata.Name == "" || metadata.AssetsDir != assetDirectoryName {
		return packageMetadata{}, fmt.Errorf("package metadata has unsupported version, empty name, or invalid assetsDir")
	}
	if metadata.Compiler != "go" && metadata.Compiler != "tinygo" {
		return packageMetadata{}, fmt.Errorf("package metadata compiler %q is not supported", metadata.Compiler)
	}
	if !safeChildPath(metadata.Entrypoints.HTML) || !safeChildPath(metadata.Entrypoints.WASM) || !safeChildPath(metadata.Entrypoints.Runtime) {
		return packageMetadata{}, fmt.Errorf("package metadata entrypoints must stay inside the package")
	}
	return metadata, nil
}

func readAssetManifestMetadata(path string) (assetManifest, error) {
	var manifest assetManifest
	if err := readJSONRegular(path, "asset manifest", &manifest); err != nil {
		return assetManifest{}, err
	}
	if manifest.Version != 1 || len(manifest.Assets) == 0 {
		return assetManifest{}, fmt.Errorf("asset manifest has unsupported version or no assets")
	}
	if !safeChildPath(manifest.Entrypoints.WASM) || !safeChildPath(manifest.Entrypoints.Runtime) {
		return assetManifest{}, fmt.Errorf("asset manifest entrypoints must stay inside the package")
	}
	for _, asset := range manifest.Assets {
		if !safeChildPath(asset.Path) || asset.Type == "" {
			return assetManifest{}, fmt.Errorf("asset manifest contains an invalid asset path or type")
		}
		for _, compressed := range asset.Compressed {
			if !safeChildPath(compressed) {
				return assetManifest{}, fmt.Errorf("asset manifest contains an invalid compressed asset path")
			}
		}
	}
	return manifest, nil
}

func validatePackageOwnedPath(directory, relative, description string) error {
	if !safeChildPath(relative) {
		return fmt.Errorf("%s %q must stay inside the package", description, relative)
	}
	path := filepath.Join(directory, filepath.FromSlash(relative))
	if err := validatePathBelowRoot(directory, path, description, false); err != nil {
		return err
	}
	if _, err := regularFileNoFollow(path, description); err != nil {
		return err
	}
	return nil
}

type legacyPackageMetadata struct {
	Name             string   `json:"name"`
	Compiler         string   `json:"compiler"`
	WASM             string   `json:"wasm"`
	Assets           []string `json:"assets"`
	ToolchainVersion string   `json:"toolchainVersion"`
}

func validLegacyPackageSignature(directory string) bool {
	manifestPath := filepath.Join(directory, legacyPackageManifest)
	if exists, err := pathExistsNoFollow(manifestPath); err != nil || !exists {
		return false
	}
	var legacy legacyPackageMetadata
	if err := readJSONRegular(manifestPath, "legacy package manifest", &legacy); err != nil {
		return false
	}
	if legacy.Name == "" || legacy.ToolchainVersion == "" {
		return false
	}
	if legacy.Compiler != "go" && legacy.Compiler != "tinygo" {
		return false
	}
	if !safeChildPath(legacy.WASM) || strings.ToLower(filepath.Ext(legacy.WASM)) != ".wasm" {
		return false
	}
	if err := validatePackageOwnedPath(directory, legacy.WASM, "legacy package WASM"); err != nil {
		return false
	}
	if err := validatePackageOwnedPath(directory, runtimeAssetName, "legacy runtime asset"); err != nil {
		return false
	}
	for _, asset := range legacy.Assets {
		if !safeChildPath(asset) {
			return false
		}
		if _, err := regularFileNoFollow(filepath.Join(directory, filepath.FromSlash(asset)), "legacy package asset"); err != nil {
			return false
		}
	}
	return true
}

func pathExistsNoFollow(path string) (bool, error) {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("inspect %s: %w", path, err)
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
