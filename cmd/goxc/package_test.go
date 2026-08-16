package main

import (
	"bytes"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestPackageAssetLogicalNameNormalization(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantError bool
	}{
		{name: "base", input: "logo.svg", want: "logo.svg"},
		{name: "nested", input: "images/logo.svg", want: "images/logo.svg"},
		{name: "space", input: "my logo.svg", want: "my logo.svg"},
		{name: "graphic Unicode", input: "界.svg", want: "界.svg"},
		{name: "leading dot child", input: ".well-known/config.json", want: ".well-known/config.json"},
		{name: "drive-like base", input: "C:logo.svg", want: "C:logo.svg"},
		{name: "drive-like segment", input: "C:/icons/logo.svg", want: "C:/icons/logo.svg"},
		{name: "colon data", input: "a:b:c.svg", want: "a:b:c.svg"},
		{name: "repeated separator", input: "images//logo.svg", want: "images/logo.svg"},
		{name: "dot component", input: "images/./logo.svg", want: "images/logo.svg"},
		{name: "absolute", input: "/logo.svg", wantError: true},
		{name: "dot", input: ".", wantError: true},
		{name: "parent", input: "..", wantError: true},
		{name: "parent prefix", input: "../logo.svg", wantError: true},
		{name: "nested parent", input: "images/../logo.svg", wantError: true},
		{name: "empty", input: "", wantError: true},
	}
	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name      string
			input     string
			want      string
			wantError bool
		}{name: "native separator", input: `foo\bar.svg`, want: "foo/bar.svg"})
	} else {
		tests = append(tests, struct {
			name      string
			input     string
			want      string
			wantError bool
		}{name: "literal backslash", input: `foo\bar.svg`, wantError: true})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizePackageAssetLogicalName(test.input)
			if test.wantError {
				if err == nil {
					t.Fatalf("normalizePackageAssetLogicalName(%q) = %q, want error", test.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizePackageAssetLogicalName(%q) error: %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("normalizePackageAssetLogicalName(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

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
