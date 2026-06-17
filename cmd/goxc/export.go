package main

import (
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
	if info, err := os.Stat(layout.PackageDir); err != nil {
		return fmt.Errorf("no standalone package found; run `goxc package %s` first", options.appDir)
	} else if !info.IsDir() {
		return fmt.Errorf("standalone package path is not a directory: %s", layout.PackageDir)
	}
	if err := validateExportDestination(options.outDir, options.force); err != nil {
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
	if fileExists(filepath.Join(directory, packageMetadataName)) {
		return true
	}
	if fileExists(filepath.Join(directory, assetManifestName)) {
		return true
	}
	if fileExists(filepath.Join(directory, legacyPackageManifest)) {
		return true
	}
	return false
}
