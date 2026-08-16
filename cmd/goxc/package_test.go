package main

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBrowserAssetProducerIntegration(t *testing.T) {
	appDir := t.TempDir()
	writeMinimalPackageApp(t, appDir)
	writeTestFile(t, appDir, manifestName, `{"name":"demo","compiler":"go","assets":"assets"}`)
	writeTestFile(t, appDir, "assets/lower/theme.css", "body { color: blue; }\n")
	writeTestFile(t, appDir, "assets/mixed/theme.CsS", "body { color: purple; }\n")
	writeTestFile(t, appDir, "assets/theme.csſ", "body { color: red; }\n")
	writeTestFile(t, appDir, "assets/upper/THEME.CSS", "body { color: green; }\n")

	outDir := filepath.Join(t.TempDir(), "package")
	if err := packageApp(packageOptions{
		appDir: appDir, compiler: "go", outDir: outDir,
		compress: map[string]bool{"gzip": true},
	}); err != nil {
		t.Fatalf("packageApp() error: %v", err)
	}

	var manifest assetManifest
	readInspectJSONFixture(t, outDir, assetManifestName, &manifest)
	confusable, ok := manifest.Assets["theme.csſ"]
	if !ok {
		t.Fatal("theme.csſ missing from asset manifest")
	}
	t.Logf("theme.csſ type=%q styles=%#v compressed=%#v", confusable.Type, manifest.Entrypoints.Styles, confusable.Compressed)
	if containsPackageString(manifest.Entrypoints.Styles, confusable.Path) {
		t.Errorf("theme.csſ was classified as a style entrypoint: %#v", manifest.Entrypoints.Styles)
	}
	if confusable.Type == "text/css" {
		t.Errorf("theme.csſ media type = %q, want ordinary asset media type", confusable.Type)
	}
	if len(confusable.Compressed) != 0 {
		t.Errorf("theme.csſ compressed as CSS: %#v", confusable.Compressed)
	}

	for _, logicalName := range []string{
		"lower/theme.css",
		"mixed/theme.CsS",
		"upper/THEME.CSS",
	} {
		asset, ok := manifest.Assets[logicalName]
		if !ok {
			t.Errorf("%s missing from asset manifest", logicalName)
			continue
		}
		if !containsPackageString(manifest.Entrypoints.Styles, asset.Path) {
			t.Errorf("%s missing from style entrypoints: %#v", logicalName, manifest.Entrypoints.Styles)
		}
		if asset.Type != "text/css" {
			t.Errorf("%s media type = %q, want text/css", logicalName, asset.Type)
		}
		if asset.Compressed["gzip"] != asset.Path+".gz" {
			t.Errorf("%s gzip sidecar = %#v", logicalName, asset.Compressed)
		}
	}

	before := snapshotInspectTree(t, outDir)
	var output bytes.Buffer
	if err := runInspectCommand([]string{"--dir", outDir, "--format=json"}, &output); err != nil {
		t.Errorf("goxc inspect rejected package produced by goxc package: %v", err)
	}
	after := snapshotInspectTree(t, outDir)
	if !reflect.DeepEqual(before, after) {
		t.Error("inspection mutated the produced package")
	}
	if output.Len() == 0 {
		t.Error("successful producer-to-inspector path emitted no JSON")
	}
}

func containsPackageString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
