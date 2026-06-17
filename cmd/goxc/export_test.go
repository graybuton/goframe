package main

import (
	"os"
	"path/filepath"
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
	if err := os.WriteFile(filepath.Join(outDir, packageMetadataName), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
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
	if err := os.MkdirAll(filepath.Join(layout.PackageDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		"index.html":             "<html></html>",
		packageMetadataName:      "{}",
		assetManifestName:        "{}",
		"assets/bundle.wasm":     "wasm",
		"assets/wasm_exec.js":    "js",
		"assets/styles.app.css":  "body{}",
		"assets/bundle.wasm.gz":  "gzip",
		"assets/bundle.wasm.br":  "br",
		"assets/wasm_exec.js.gz": "js gzip",
	} {
		if err := os.WriteFile(filepath.Join(layout.PackageDir, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return appDir
}
