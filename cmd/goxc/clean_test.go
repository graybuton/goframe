package main

import (
	"os"
	"path/filepath"
	"strings"
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
	if err := os.WriteFile(filepath.Join(appDir, "app.gox.go"), []byte(generatedGOXFileHeader+"package main\n"), 0o644); err != nil {
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

func TestCleanGeneratedRejectsUnmanagedAdjacentOutput(t *testing.T) {
	appDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(appDir, "app.gox"), []byte("<div></div>"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(appDir, "app.gox.go")
	want := "package main\n\nvar authored = true\n"
	if err := os.WriteFile(output, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}
	workspaceFile := filepath.Join(appDir, defaultWorkspaceName, "work", "sentinel")
	if err := os.MkdirAll(filepath.Dir(workspaceFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workspaceFile, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := cleanApp(cleanOptions{appDir: appDir, generated: true})
	if err == nil || !strings.Contains(err.Error(), "file is not managed by goxc") {
		t.Fatalf("cleanApp(generated) error = %v, want unmanaged-output refusal", err)
	}
	assertFileContent(t, output, want)
	assertFileContent(t, workspaceFile, "keep")
}

func TestCleanGeneratedPreflightsAllAdjacentOutputs(t *testing.T) {
	appDir := t.TempDir()
	for _, name := range []string{"a.gox", "z.gox"} {
		if err := os.WriteFile(filepath.Join(appDir, name), []byte("<div></div>"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	managedPath := filepath.Join(appDir, "a.gox.go")
	managed := generatedGOXFileHeader + "package main\n"
	if err := os.WriteFile(managedPath, []byte(managed), 0o644); err != nil {
		t.Fatal(err)
	}
	unmanagedPath := filepath.Join(appDir, "z.gox.go")
	unmanaged := "package main\n\nvar authored = true\n"
	if err := os.WriteFile(unmanagedPath, []byte(unmanaged), 0o644); err != nil {
		t.Fatal(err)
	}

	err := cleanApp(cleanOptions{appDir: appDir, generated: true})
	if err == nil || !strings.Contains(err.Error(), unmanagedPath) {
		t.Fatalf("cleanApp(generated) error = %v, want path %s", err, unmanagedPath)
	}
	assertFileContent(t, managedPath, managed)
	assertFileContent(t, unmanagedPath, unmanaged)
}

func TestCleanGeneratedRejectsUnsafeAdjacentOutput(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		requireSymlinkSupport(t)

		appDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(appDir, "app.gox"), []byte("<div></div>"), 0o644); err != nil {
			t.Fatal(err)
		}
		external := filepath.Join(t.TempDir(), "external.go")
		if err := os.WriteFile(external, []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(appDir, "app.gox.go")
		if err := os.Symlink(external, output); err != nil {
			t.Fatal(err)
		}

		err := cleanApp(cleanOptions{appDir: appDir, generated: true})
		if err == nil || !strings.Contains(err.Error(), "symlink paths are not supported") {
			t.Fatalf("cleanApp(generated) error = %v, want symlink refusal", err)
		}
		assertFileContent(t, external, "keep")
		if info, err := os.Lstat(output); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("adjacent output symlink changed: info=%v err=%v", info, err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		appDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(appDir, "app.gox"), []byte("<div></div>"), 0o644); err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(appDir, "app.gox.go")
		if err := os.Mkdir(output, 0o755); err != nil {
			t.Fatal(err)
		}

		err := cleanApp(cleanOptions{appDir: appDir, generated: true})
		if err == nil || !strings.Contains(err.Error(), "is not a regular file") {
			t.Fatalf("cleanApp(generated) error = %v, want directory refusal", err)
		}
		if info, err := os.Stat(output); err != nil || !info.IsDir() {
			t.Fatalf("adjacent output directory changed: info=%v err=%v", info, err)
		}
	})
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
	writeCompleteCurrentPackage(t, dist)
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
	writeLegacyPackageSignature(t, dist)
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

func TestCleanLegacyPreservesGenericWebManifestDist(t *testing.T) {
	appDir := t.TempDir()
	dist := filepath.Join(appDir, "dist")
	if err := os.Mkdir(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	userFile := filepath.Join(dist, "user.txt")
	if err := os.WriteFile(filepath.Join(dist, legacyPackageManifest), []byte(`{"name":"web app"}`), 0o644); err != nil {
		t.Fatal(err)
	}
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
		t.Fatalf("generic web manifest dist changed: content=%q err=%v", content, err)
	}
}

func TestCleanLegacyPreservesGenericGoWASMDist(t *testing.T) {
	appDir := t.TempDir()
	dist := filepath.Join(appDir, "dist")
	if err := os.Mkdir(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		legacyPackageManifest: `{"name":"generic wasm app"}`,
		"main.wasm":           "wasm",
		runtimeAssetName:      "js",
		"user.txt":            "keep",
	} {
		if err := os.WriteFile(filepath.Join(dist, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.gox"), []byte("<div></div>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cleanApp(cleanOptions{appDir: appDir, legacy: true}); err != nil {
		t.Fatalf("cleanApp(legacy) error: %v", err)
	}
	assertFileContent(t, filepath.Join(dist, "user.txt"), "keep")
}
