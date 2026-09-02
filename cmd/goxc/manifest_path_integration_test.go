package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCanonicalAuthoredPathsReachExplicitBoundaries(t *testing.T) {
	appDir := t.TempDir()
	writeTestFile(t, appDir, "cmd/app/main.go", "package main\n")
	writeTestFile(t, appDir, "assets/styles/app.css", "body{}\n")
	writeManifestPathFixture(t, appDir, map[string]any{
		"entry":  `cmd\app`,
		"output": `dist\site`,
		"wasm":   `nested\bundle.wasm`,
		"assets": []string{`assets\styles\app.css`},
	})

	manifest, err := loadManifest(appDir)
	if err != nil {
		t.Fatalf("loadManifest() error: %v", err)
	}
	entryDir, err := resolveEntryPackageDir(appDir, manifest.Entry)
	if err != nil {
		t.Fatalf("resolveEntryPackageDir() error: %v", err)
	}
	if want := filepath.Join(appDir, "cmd", "app"); entryDir != want {
		t.Fatalf("entry directory = %q, want %q", entryDir, want)
	}

	layout, err := newBuildLayout(layoutOptions{appDir: appDir})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := buildOutputPath(buildOptions{}, manifest, layout), filepath.Join(layout.BuildDir, "nested", "bundle.wasm"); got != want {
		t.Fatalf("build output = %q, want %q", got, want)
	}

	plan, err := planPackageAssets(appDir, manifest, "bundle.wasm", packageOptions{compress: map[string]bool{}})
	if err != nil {
		t.Fatalf("planPackageAssets() error: %v", err)
	}
	if len(plan.Assets) != 1 {
		t.Fatalf("planned assets = %#v, want one", plan.Assets)
	}
	asset := plan.Assets[0]
	if asset.LogicalName != "assets/styles/app.css" {
		t.Fatalf("asset logical name = %q", asset.LogicalName)
	}
	if want := filepath.Join(appDir, "assets", "styles", "app.css"); asset.SourcePath != want {
		t.Fatalf("asset source = %q, want %q", asset.SourcePath, want)
	}
}

func TestCanonicalAssetDirectoryReachesFilesystemBoundary(t *testing.T) {
	appDir := t.TempDir()
	writeTestFile(t, appDir, "static/styles/app.css", "body{}\n")
	writeManifestPathFixture(t, appDir, map[string]any{"assets": `static\styles\..\styles`})
	if _, err := loadManifest(appDir); err == nil {
		t.Fatal("loadManifest() accepted a raw parent component")
	}

	writeManifestPathFixture(t, appDir, map[string]any{"assets": `static\styles`})
	manifest, err := loadManifest(appDir)
	if err != nil {
		t.Fatalf("loadManifest() error: %v", err)
	}
	plan, err := planPackageAssets(appDir, manifest, "bundle.wasm", packageOptions{compress: map[string]bool{}})
	if err != nil {
		t.Fatalf("planPackageAssets() error: %v", err)
	}
	if len(plan.Assets) != 1 || plan.Assets[0].LogicalName != "app.css" {
		t.Fatalf("directory asset plan = %#v", plan.Assets)
	}
	if want := filepath.Join(appDir, "static", "styles", "app.css"); plan.Assets[0].SourcePath != want {
		t.Fatalf("directory asset source = %q, want %q", plan.Assets[0].SourcePath, want)
	}
}

func TestSeparatorEquivalentStandaloneGraphs(t *testing.T) {
	spellings := []struct {
		name   string
		entry  string
		output string
		wasm   string
		asset  string
	}{
		{name: "slash", entry: "./", output: "./dist/site", wasm: "./nested/bundle.wasm", asset: "./assets/styles/app.css"},
		{name: "backslash", entry: `.\`, output: `.\dist\site`, wasm: `.\nested\bundle.wasm`, asset: `.\assets\styles\app.css`},
		{name: "mixed", entry: ".", output: `dist\site`, wasm: `nested\bundle.wasm`, asset: `assets\styles/app.css`},
	}

	type result struct {
		manifest     projectManifest
		packageFiles map[string][]byte
		exportFiles  map[string][]byte
		inspectText  string
		inspectJSON  string
	}
	results := make(map[string]result, len(spellings))
	for _, spelling := range spellings {
		appDir := filepath.Join(t.TempDir(), "app")
		if err := os.MkdirAll(appDir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeMinimalPackageApp(t, appDir)
		writeTestFile(t, appDir, "assets/styles/app.css", "body { color: navy; }\n")
		writeManifestPathFixture(t, appDir, map[string]any{
			"name":     "separator-equivalent",
			"entry":    spelling.entry,
			"output":   spelling.output,
			"compiler": "go",
			"wasm":     spelling.wasm,
			"assets":   []string{"index.html", spelling.asset},
		})

		manifest, err := loadManifest(appDir)
		if err != nil {
			t.Fatalf("%s loadManifest() error: %v", spelling.name, err)
		}
		if err := packageApp(packageOptions{appDir: appDir, compiler: "go", compress: map[string]bool{}}); err != nil {
			t.Fatalf("%s packageApp() error: %v", spelling.name, err)
		}
		layout, err := newBuildLayout(layoutOptions{appDir: appDir})
		if err != nil {
			t.Fatal(err)
		}
		normalizePackageGeneratedAtForComparison(t, layout.PackageDir)
		if _, err := inspectPackageGraph(layout.PackageDir); err != nil {
			t.Fatalf("%s inspectPackageGraph(package) error: %v", spelling.name, err)
		}
		var inspectText bytes.Buffer
		if err := runInspectCommand([]string{"--dir", layout.PackageDir}, &inspectText); err != nil {
			t.Fatalf("%s text inspect error: %v", spelling.name, err)
		}
		var inspectJSON bytes.Buffer
		if err := runInspectCommand([]string{"--dir", layout.PackageDir, "--format=json"}, &inspectJSON); err != nil {
			t.Fatalf("%s JSON inspect error: %v", spelling.name, err)
		}

		exportDir := filepath.Join(t.TempDir(), "export")
		if err := exportApp(exportOptions{appDir: appDir, outDir: exportDir}); err != nil {
			t.Fatalf("%s exportApp() error: %v", spelling.name, err)
		}
		if _, err := inspectPackageGraph(exportDir); err != nil {
			t.Fatalf("%s inspectPackageGraph(export) error: %v", spelling.name, err)
		}

		results[spelling.name] = result{
			manifest:     manifest,
			packageFiles: semanticPackageFileContents(t, layout.PackageDir),
			exportFiles:  semanticPackageFileContents(t, exportDir),
			inspectText:  inspectText.String(),
			inspectJSON:  inspectJSON.String(),
		}
	}

	want := results["slash"]
	for _, name := range []string{"backslash", "mixed"} {
		got := results[name]
		if !reflect.DeepEqual(got.manifest, want.manifest) {
			t.Fatalf("%s manifest differs from slash form:\ngot:  %+v\nwant: %+v", name, got.manifest, want.manifest)
		}
		if !reflect.DeepEqual(got.packageFiles, want.packageFiles) {
			t.Fatalf("%s package files differ from slash form", name)
		}
		if !reflect.DeepEqual(got.exportFiles, want.exportFiles) {
			t.Fatalf("%s export files differ from slash form", name)
		}
		if got.inspectText != want.inspectText || got.inspectJSON != want.inspectJSON {
			t.Fatalf("%s inspect output differs from slash form", name)
		}
	}
}

func normalizePackageGeneratedAtForComparison(t *testing.T, root string) {
	t.Helper()
	metadataPath := filepath.Join(root, packageMetadataName)
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var metadata packageMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		t.Fatal(err)
	}
	metadata.GeneratedAt = "2000-01-01T00:00:00Z"
	if err := writeJSONFile(metadataPath, metadata, "normalized package metadata fixture"); err != nil {
		t.Fatal(err)
	}
}

func semanticPackageFileContents(t *testing.T, root string) map[string][]byte {
	t.Helper()
	contents := map[string][]byte{}
	if err := filepath.WalkDir(root, func(file string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if filepath.ToSlash(relative) == packageMetadataName {
			var metadata packageMetadata
			if err := json.Unmarshal(content, &metadata); err != nil {
				return err
			}
			metadata.GeneratedAt = ""
			content, err = json.Marshal(metadata)
			if err != nil {
				return err
			}
		}
		contents[filepath.ToSlash(relative)] = content
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return contents
}
