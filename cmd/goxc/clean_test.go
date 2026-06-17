package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanLegacyRemovesBuildAndAdjacentGenerated(t *testing.T) {
	appDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(appDir, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.gox"), []byte("<div></div>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.gox.go"), []byte("generated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cleanApp(cleanOptions{appDir: appDir, legacy: true}); err != nil {
		t.Fatalf("cleanApp(legacy) error: %v", err)
	}
	for _, path := range []string{"build", "app.gox.go"} {
		if _, err := os.Stat(filepath.Join(appDir, path)); !os.IsNotExist(err) {
			t.Fatalf("legacy artifact %s still exists: %v", path, err)
		}
	}
}

func TestCleanLegacyPreservesUnownedDist(t *testing.T) {
	appDir := t.TempDir()
	dist := filepath.Join(appDir, "dist")
	if err := os.Mkdir(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	userFile := filepath.Join(dist, "user.txt")
	if err := os.WriteFile(userFile, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.gox"), []byte("<div></div>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cleanApp(cleanOptions{appDir: appDir, legacy: true}); err != nil {
		t.Fatalf("cleanApp(legacy) error: %v", err)
	}
	if content, err := os.ReadFile(userFile); err != nil || string(content) != "keep" {
		t.Fatalf("unowned dist file changed: content=%q err=%v", content, err)
	}
}

func TestCleanLegacyRemovesGoframeOwnedDist(t *testing.T) {
	appDir := t.TempDir()
	dist := filepath.Join(appDir, "dist")
	if err := os.Mkdir(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, packageMetadataName), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.gox"), []byte("<div></div>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cleanApp(cleanOptions{appDir: appDir, legacy: true}); err != nil {
		t.Fatalf("cleanApp(legacy) error: %v", err)
	}
	if _, err := os.Stat(dist); !os.IsNotExist(err) {
		t.Fatalf("goframe-owned dist still exists: %v", err)
	}
}

func TestCleanLegacyRemovesLegacyManifestDist(t *testing.T) {
	appDir := t.TempDir()
	dist := filepath.Join(appDir, "dist")
	if err := os.Mkdir(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, legacyPackageManifest), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.gox"), []byte("<div></div>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cleanApp(cleanOptions{appDir: appDir, legacy: true}); err != nil {
		t.Fatalf("cleanApp(legacy) error: %v", err)
	}
	if _, err := os.Stat(dist); !os.IsNotExist(err) {
		t.Fatalf("legacy manifest dist still exists: %v", err)
	}
}
