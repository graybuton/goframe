//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnixAuthoredBackslashSelectsSeparatorPath(t *testing.T) {
	appDir := t.TempDir()
	nested := filepath.Join(appDir, "assets", "styles.css")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte("nested\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	literal := filepath.Join(appDir, `assets\styles.css`)
	if err := os.WriteFile(literal, []byte("literal backslash\n"), 0o644); err != nil {
		t.Skipf("filesystem cannot create literal-backslash filename: %v", err)
	}
	writeManifestPathFixture(t, appDir, map[string]any{"assets": []string{`assets\styles.css`}})

	manifest, err := loadManifest(appDir)
	if err != nil {
		t.Fatalf("loadManifest() error: %v", err)
	}
	if len(manifest.Assets.List) != 1 || manifest.Assets.List[0] != "assets/styles.css" {
		t.Fatalf("canonical asset list = %#v", manifest.Assets.List)
	}
	plan, err := planPackageAssets(appDir, manifest, "bundle.wasm", packageOptions{compress: map[string]bool{}})
	if err != nil {
		t.Fatalf("planPackageAssets() error: %v", err)
	}
	if len(plan.Assets) != 1 || plan.Assets[0].SourcePath != nested {
		t.Fatalf("planned source = %#v, want nested separator path %q", plan.Assets, nested)
	}
	content, err := os.ReadFile(plan.Assets[0].SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "nested\n" {
		t.Fatalf("selected content = %q, want nested path content", content)
	}
}
