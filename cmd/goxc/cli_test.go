package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestParseBuildOptions(t *testing.T) {
	options, err := parseBuildOptions([]string{"app", "--compiler=tinygo", "--out=custom"})
	if err != nil {
		t.Fatalf("parseBuildOptions() error: %v", err)
	}
	if options.appDir != "app" || options.compiler != "tinygo" || options.outDir != "custom" {
		t.Fatalf("unexpected options: %+v", options)
	}
}

func TestBuildAndPackageOutputPaths(t *testing.T) {
	manifest := projectManifest{Output: "public", WASM: "app.wasm"}
	if got := buildOutputPath(buildOptions{appDir: "demo"}, manifest); got != filepath.Join("demo", "build", "app.wasm") {
		t.Fatalf("build output = %q", got)
	}
	if got := packageOutputDirectory(packageOptions{appDir: "demo"}, manifest); got != filepath.Join("demo", "public") {
		t.Fatalf("package output = %q", got)
	}
}

func TestParsePackageOptionsCompression(t *testing.T) {
	options, err := parsePackageOptions([]string{"app", "--compress=gzip,br"})
	if err != nil {
		t.Fatalf("parsePackageOptions() error: %v", err)
	}
	if !options.compress["gzip"] || !options.compress["br"] {
		t.Fatalf("compression options = %#v", options.compress)
	}
}

func TestLoadManifestDefaultsAndOverrides(t *testing.T) {
	appDir := t.TempDir()
	defaults, err := loadManifest(appDir)
	if err != nil {
		t.Fatalf("loadManifest(defaults) error: %v", err)
	}
	if defaults.Compiler != "go" || defaults.Output != "dist" || defaults.WASM != "main.wasm" {
		t.Fatalf("unexpected defaults: %+v", defaults)
	}

	content := []byte(`{"name":"demo","compiler":"tinygo","output":"public","assets":["index.html"]}`)
	if err := os.WriteFile(filepath.Join(appDir, manifestName), content, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := loadManifest(appDir)
	if err != nil {
		t.Fatalf("loadManifest(custom) error: %v", err)
	}
	if manifest.Name != "demo" || manifest.Compiler != "tinygo" || manifest.Output != "public" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
}

func TestManifestRejectsApplicationRootAsOutput(t *testing.T) {
	appDir := t.TempDir()
	content := []byte(`{"output":"."}`)
	if err := os.WriteFile(filepath.Join(appDir, manifestName), content, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadManifest(appDir); err == nil {
		t.Fatal("loadManifest() accepted application root as output")
	}
}

func TestManifestRejectsUnknownFields(t *testing.T) {
	appDir := t.TempDir()
	content := []byte(`{"compielr":"tinygo"}`)
	if err := os.WriteFile(filepath.Join(appDir, manifestName), content, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadManifest(appDir); err == nil {
		t.Fatal("loadManifest() accepted an unknown field")
	}
}

func TestManifestRejectsTrailingJSON(t *testing.T) {
	appDir := t.TempDir()
	content := []byte(`{"name":"demo"} {"name":"other"}`)
	if err := os.WriteFile(filepath.Join(appDir, manifestName), content, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadManifest(appDir); err == nil {
		t.Fatal("loadManifest() accepted trailing JSON data")
	}
}

func TestCleanRemovesArtifactsAndGeneratedOnRequest(t *testing.T) {
	appDir := t.TempDir()
	for _, directory := range []string{"build", "dist"} {
		if err := os.Mkdir(filepath.Join(appDir, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.gox"), []byte("<div></div>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.gox.go"), []byte("generated"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cleanApp(cleanOptions{appDir: appDir}); err != nil {
		t.Fatalf("cleanApp() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDir, "app.gox.go")); err != nil {
		t.Fatalf("generated file removed without --generated: %v", err)
	}
	if err := cleanApp(cleanOptions{appDir: appDir, generated: true}); err != nil {
		t.Fatalf("cleanApp(generated) error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDir, "app.gox.go")); !os.IsNotExist(err) {
		t.Fatalf("generated file still exists: %v", err)
	}
}

func TestSizeReportsHelpfulErrorForEmptyDirectory(t *testing.T) {
	err := sizeCommand([]string{t.TempDir()})
	if err == nil {
		t.Fatal("sizeCommand() returned nil error")
	}
}

func TestCleanPackageArtifactsRemovesOldCompression(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"main.wasm", "main.wasm.gz", "main.wasm.br"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := cleanPackageArtifacts(directory, "main.wasm"); err != nil {
		t.Fatalf("cleanPackageArtifacts() error: %v", err)
	}
	for _, name := range []string{"main.wasm", "main.wasm.gz", "main.wasm.br"} {
		if _, err := os.Stat(filepath.Join(directory, name)); !os.IsNotExist(err) {
			t.Fatalf("%s still exists: %v", name, err)
		}
	}
}

func TestPublishPackageArtifactsCopiesStagedTree(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		"main.wasm":        "wasm",
		"manifest.json":    "{}",
		"assets/style.css": "body{}",
	} {
		if err := os.WriteFile(filepath.Join(source, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := publishPackageArtifacts(source, destination); err != nil {
		t.Fatalf("publishPackageArtifacts() error: %v", err)
	}
	for path, want := range map[string]string{
		"main.wasm":        "wasm",
		"manifest.json":    "{}",
		"assets/style.css": "body{}",
	} {
		got, err := os.ReadFile(filepath.Join(destination, path))
		if err != nil {
			t.Fatalf("read published %s: %v", path, err)
		}
		if string(got) != want {
			t.Fatalf("%s = %q, want %q", path, got, want)
		}
	}
}

func TestDoctorDoesNotFail(t *testing.T) {
	if err := doctorCommand(nil); err != nil {
		t.Fatalf("doctorCommand() error: %v", err)
	}
}

func TestStaticHandlerServesWASMContentType(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "main.wasm"), []byte("wasm"), 0o644); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest("GET", "/main.wasm", nil)
	response := httptest.NewRecorder()
	staticHandler(directory).ServeHTTP(response, request)

	if got := response.Header().Get("Content-Type"); got != "application/wasm" {
		t.Fatalf("Content-Type = %q, want application/wasm", got)
	}
}

func TestHumanSize(t *testing.T) {
	if got := humanSize(73_631); got != "71.9 KiB" {
		t.Fatalf("humanSize() = %q, want 71.9 KiB", got)
	}
}
