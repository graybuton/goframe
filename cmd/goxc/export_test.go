package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestExportCopiesStandalonePackage(t *testing.T) {
	appDir := createPackagedTestApp(t)
	outDir := filepath.Join(t.TempDir(), "dist")
	if err := exportApp(exportOptions{appDir: appDir, outDir: outDir}); err != nil {
		t.Fatalf("exportApp() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "assets", "bundle.wasm")); err != nil {
		t.Fatalf("exported bundle missing: %v", err)
	}
}

func TestExportRejectsNonEmptyUnownedDirectory(t *testing.T) {
	appDir := createPackagedTestApp(t)
	outDir := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(filepath.Join(outDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	userAsset := filepath.Join(outDir, "assets", "user.txt")
	if err := os.WriteFile(userAsset, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := exportApp(exportOptions{appDir: appDir, outDir: outDir})
	if err == nil {
		t.Fatal("exportApp() accepted non-empty unowned directory")
	}
	if content, readErr := os.ReadFile(userAsset); readErr != nil || string(content) != "keep" {
		t.Fatalf("user asset changed after rejected export: content=%q err=%v", content, readErr)
	}
}

func TestExportAllowsPreviousGoframeExport(t *testing.T) {
	appDir := createPackagedTestApp(t)
	outDir := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCompleteCurrentPackage(t, outDir)
	if err := os.MkdirAll(filepath.Join(outDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "assets", "stale.wasm"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exportApp(exportOptions{appDir: appDir, outDir: outDir}); err != nil {
		t.Fatalf("exportApp(previous export) error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "assets", "stale.wasm")); !os.IsNotExist(err) {
		t.Fatalf("stale package asset still exists: %v", err)
	}
}

func TestExportForceAllowsNonEmptyUnownedDirectory(t *testing.T) {
	appDir := createPackagedTestApp(t)
	outDir := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(filepath.Join(outDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "assets", "user.txt"), []byte("delete"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exportApp(exportOptions{appDir: appDir, outDir: outDir, force: true}); err != nil {
		t.Fatalf("exportApp(force) error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "assets", "bundle.wasm")); err != nil {
		t.Fatalf("forced export bundle missing: %v", err)
	}
}

func TestExportRejectsOverlappingTemporaryRoot(t *testing.T) {
	tests := []struct {
		name     string
		tempRoot func(t *testing.T, source, destination string) string
		reject   bool
	}{
		{
			name: "source package root",
			tempRoot: func(_ *testing.T, source, _ string) string {
				return source
			},
			reject: true,
		},
		{
			name: "source package descendant",
			tempRoot: func(t *testing.T, source, _ string) string {
				root := filepath.Join(source, ".export-temp")
				if err := os.MkdirAll(root, 0o755); err != nil {
					t.Fatal(err)
				}
				return root
			},
			reject: true,
		},
		{
			name: "symlink resolving inside source package",
			tempRoot: func(t *testing.T, source, _ string) string {
				requireSymlinkSupport(t)
				alias := filepath.Join(t.TempDir(), "source-alias")
				if err := os.Symlink(source, alias); err != nil {
					t.Fatal(err)
				}
				return alias
			},
			reject: true,
		},
		{
			name: "existing destination root",
			tempRoot: func(_ *testing.T, _, destination string) string {
				return destination
			},
			reject: true,
		},
		{
			name: "destination descendant",
			tempRoot: func(t *testing.T, _, destination string) string {
				root := filepath.Join(destination, ".export-temp")
				if err := os.MkdirAll(root, 0o755); err != nil {
					t.Fatal(err)
				}
				return root
			},
			reject: true,
		},
		{
			name: "physically separate root",
			tempRoot: func(t *testing.T, _, _ string) string {
				return t.TempDir()
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appDir := createPackagedTestApp(t)
			layout, err := newBuildLayout(layoutOptions{appDir: appDir})
			if err != nil {
				t.Fatal(err)
			}
			source := layout.PackageDir
			destination := filepath.Join(t.TempDir(), "export")
			if err := os.MkdirAll(destination, 0o755); err != nil {
				t.Fatal(err)
			}
			if test.reject {
				writeInspectRaw(t, destination, "sentinel.txt", []byte("keep\n"))
			}
			tempRoot := test.tempRoot(t, source, destination)
			setExportTemporaryRoot(t, tempRoot)

			sourceBytes := packageFileBytes(t, source)
			sourceEntries := exportTreeEntries(t, source)
			destinationBytes := packageFileBytes(t, destination)
			destinationEntries := exportTreeEntries(t, destination)
			var output bytes.Buffer
			err = exportApp(exportOptions{
				appDir: appDir,
				outDir: destination,
				force:  true,
				stdout: &output,
			})

			if !test.reject {
				if err != nil {
					t.Fatalf("exportApp() error: %v", err)
				}
				if got := packageFileBytes(t, destination); !reflect.DeepEqual(got, sourceBytes) {
					t.Fatalf("exported bytes differ from source\nsource: %#v\nexport: %#v", sourceBytes, got)
				}
				if paths := exportTemporaryPaths(t, destination); len(paths) != 0 {
					t.Fatalf("disjoint export published temporary paths: %#v", paths)
				}
				if strings.Count(output.String(), "exported ") != 1 {
					t.Fatalf("success output = %q, want exactly one export line", output.String())
				}
				return
			}

			if err == nil {
				t.Fatalf("exportApp() accepted overlapping temporary root %q; published temporary paths: %#v", tempRoot, exportTemporaryPaths(t, destination))
			}
			if !strings.Contains(err.Error(), "temporary export directory") || !strings.Contains(err.Error(), "must not overlap") {
				t.Fatalf("exportApp() error = %v, want temporary-root overlap rejection", err)
			}
			if output.Len() != 0 {
				t.Fatalf("rejected export emitted success output: %q", output.String())
			}
			if got := packageFileBytes(t, source); !reflect.DeepEqual(got, sourceBytes) {
				t.Fatalf("rejected export changed source bytes\nbefore: %#v\nafter:  %#v", sourceBytes, got)
			}
			if got := exportTreeEntries(t, source); !reflect.DeepEqual(got, sourceEntries) {
				t.Fatalf("rejected export changed source paths\nbefore: %#v\nafter:  %#v", sourceEntries, got)
			}
			if got := packageFileBytes(t, destination); !reflect.DeepEqual(got, destinationBytes) {
				t.Fatalf("rejected export changed destination bytes\nbefore: %#v\nafter:  %#v", destinationBytes, got)
			}
			if got := exportTreeEntries(t, destination); !reflect.DeepEqual(got, destinationEntries) {
				t.Fatalf("rejected export changed destination paths\nbefore: %#v\nafter:  %#v", destinationEntries, got)
			}
			if _, statErr := os.Lstat(filepath.Join(destination, packageMetadataName)); !os.IsNotExist(statErr) {
				t.Fatalf("rejected export published a completion marker: %v", statErr)
			}
			if paths := exportTemporaryPaths(t, source); len(paths) != 0 {
				t.Fatalf("rejected export retained source temporary paths: %#v", paths)
			}
			if paths := exportTemporaryPaths(t, destination); len(paths) != 0 {
				t.Fatalf("rejected export retained destination temporary paths: %#v", paths)
			}
		})
	}
}

func TestExportRejectsDestinationOverlapBeforeStaging(t *testing.T) {
	appDir := createPackagedTestApp(t)
	layout, err := newBuildLayout(layoutOptions{appDir: appDir})
	if err != nil {
		t.Fatal(err)
	}
	setExportTemporaryRoot(t, filepath.Join(t.TempDir(), "missing"))

	err = exportApp(exportOptions{
		appDir: appDir,
		outDir: filepath.Join(layout.PackageDir, "nested"),
	})
	if err == nil || !strings.Contains(err.Error(), "must not overlap") {
		t.Fatalf("exportApp() error = %v, want destination overlap before temporary staging", err)
	}
}

func TestLegacyWASMExtensionUsesASCIIOnlyCaseFolding(t *testing.T) {
	for _, test := range []struct {
		name   string
		wasm   string
		accept bool
	}{
		{name: "uppercase", wasm: "APP.WASM", accept: true},
		{name: "mixed ASCII case", wasm: "App.WaSm", accept: true},
		{name: "Unicode long s", wasm: "app.waſm", accept: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			manifest := fmt.Sprintf(`{
  "name": "demo",
  "compiler": "go",
  "wasm": %q,
  "toolchainVersion": "test"
}`, test.wasm)
			if err := os.WriteFile(filepath.Join(directory, legacyPackageManifest), []byte(manifest), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, test.wasm), []byte("wasm"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, runtimeAssetName), []byte("js"), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := validLegacyPackageSignature(directory); got != test.accept {
				t.Fatalf("validLegacyPackageSignature(%q) = %v, want %v", test.wasm, got, test.accept)
			}
		})
	}
}

func createPackagedTestApp(t *testing.T) string {
	t.Helper()
	appDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(appDir, manifestName), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	layout, err := newBuildLayout(layoutOptions{appDir: appDir})
	if err != nil {
		t.Fatal(err)
	}
	writeInspectFixture(t, layout.PackageDir, inspectFixtureOptions{
		styles:     true,
		compressed: true,
		extraAsset: true,
	})
	return appDir
}

func setExportTemporaryRoot(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, root)
	}
	if !samePath(os.TempDir(), root) {
		t.Fatalf("os.TempDir() = %q, want physical root %q", os.TempDir(), root)
	}
}

func exportTreeEntries(t *testing.T, root string) []string {
	t.Helper()
	var result []string
	if err := filepath.WalkDir(root, func(current string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			relative += "/"
		}
		result = append(result, relative)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(result)
	return result
}

func exportTemporaryPaths(t *testing.T, root string) []string {
	t.Helper()
	var result []string
	for _, relative := range exportTreeEntries(t, root) {
		for _, component := range strings.Split(relative, "/") {
			if strings.HasPrefix(component, "goxc-export-") {
				result = append(result, relative)
				break
			}
		}
	}
	return result
}
