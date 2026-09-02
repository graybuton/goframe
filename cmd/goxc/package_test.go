package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
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

func TestGeneratedPackagePathBrowserURLMatrix(t *testing.T) {
	for _, test := range []struct {
		name  string
		path  string
		want  string
		valid bool
	}{
		{name: "ordinary", path: "assets/bundle.wasm", want: "assets/bundle.wasm", valid: true},
		{name: "nested", path: "assets/styles/theme.css", want: "assets/styles/theme.css", valid: true},
		{name: "space", path: "assets/foo bar.css", want: "assets/foo%20bar.css", valid: true},
		{name: "ampersand", path: "assets/foo&bar.css", want: "assets/foo%26bar.css", valid: true},
		{name: "percent", path: "assets/foo%2e.wasm", want: "assets/foo%252e.wasm", valid: true},
		{name: "question", path: "assets/foo?bar.wasm", want: "assets/foo%3Fbar.wasm", valid: true},
		{name: "fragment", path: "assets/foo#bar.wasm", want: "assets/foo%23bar.wasm", valid: true},
		{name: "quotes", path: "assets/foo\"'bar.css", want: "assets/foo%22%27bar.css", valid: true},
		{name: "controls", path: "assets/a\x07\tb\nc\rd.css", want: "assets/a%07%09b%0Ac%0Dd.css", valid: true},
		{name: "Unicode", path: "assets/界.css", want: "assets/%E7%95%8C.css", valid: true},
		{name: "non-BMP", path: "assets/😀.css", want: "assets/%F0%9F%98%80.css", valid: true},
		{name: "NUL", path: "assets/bad\x00.css"},
		{name: "backslash", path: `assets\bad.css`},
		{name: "non-canonical", path: "assets//bad.css"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := encodePackagePathAsBrowserURL(test.path)
			if !test.valid {
				if err == nil || got != "" {
					t.Fatalf("encodePackagePathAsBrowserURL(%q) = %q, %v, want error", test.path, got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("encodePackagePathAsBrowserURL(%q) error: %v", test.path, err)
			}
			if got != test.want {
				t.Fatalf("encodePackagePathAsBrowserURL(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

func TestPackageGeneratedBrowserURLsResolveToExactUnixAssets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows filenames cannot represent this Unix package-path matrix")
	}

	appDir := t.TempDir()
	writeMinimalPackageApp(t, appDir)
	wasmName := "bundle space#query?percent%界\x07.wasm"
	styleName := "style space&query?#percent%\"'\t\n\r界😀.css"
	manifestContent, err := json.Marshal(map[string]any{
		"name":     "browser-url",
		"compiler": "go",
		"wasm":     wasmName,
		"assets":   "assets",
	})
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, appDir, manifestName, string(manifestContent))
	writeTestFile(t, appDir, filepath.Join("assets", styleName), "body { color: green; }\n")

	outDir := filepath.Join(t.TempDir(), "package")
	if err := packageApp(packageOptions{
		appDir: appDir, compiler: "go", outDir: outDir, preload: true, compress: map[string]bool{},
	}); err != nil {
		t.Fatalf("packageApp() error: %v", err)
	}

	var manifest assetManifest
	readInspectJSONFixture(t, outDir, assetManifestName, &manifest)
	wasmAsset := manifest.Assets[wasmName]
	styleAsset := manifest.Assets[styleName]
	if wasmAsset.Path != "assets/"+wasmName || styleAsset.Path != "assets/"+styleName {
		t.Fatalf("manifest paths changed: WASM %q, style %q", wasmAsset.Path, styleAsset.Path)
	}
	for _, asset := range []packageAsset{wasmAsset, styleAsset} {
		if _, err := os.Stat(filepath.Join(outDir, filepath.FromSlash(asset.Path))); err != nil {
			t.Fatalf("packaged asset %q missing: %v", asset.Path, err)
		}
	}

	wasmURL, err := encodePackagePathAsBrowserURL(wasmAsset.Path)
	if err != nil {
		t.Fatal(err)
	}
	styleURL, err := encodePackagePathAsBrowserURL(styleAsset.Path)
	if err != nil {
		t.Fatal(err)
	}
	indexContent, err := os.ReadFile(filepath.Join(outDir, indexHTMLAssetName))
	if err != nil {
		t.Fatal(err)
	}
	index := string(indexContent)
	for _, want := range []string{`fetch("` + wasmURL + `")`, `href="` + wasmURL + `"`, `href="` + styleURL + `"`} {
		if !strings.Contains(index, want) {
			t.Fatalf("generated index missing %q:\n%s", want, index)
		}
	}
	for _, browserURL := range []string{wasmURL, styleURL} {
		response := httptest.NewRecorder()
		staticHandler(outDir).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/"+browserURL, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("serve %q status = %d, want 200", browserURL, response.Code)
		}
	}
	if _, err := inspectPackageGraph(outDir); err != nil {
		t.Fatalf("inspectPackageGraph() rejected generated unusual paths: %v", err)
	}
}

func TestPackageCleansAuthoredNestedWASMArtifacts(t *testing.T) {
	for _, test := range []struct {
		name           string
		authoredWASM   string
		standaloneWASM string
	}{
		{name: "default basename", authoredWASM: "nested/bundle.wasm", standaloneWASM: "bundle.wasm"},
		{name: "custom basename", authoredWASM: "nested/custom.wasm", standaloneWASM: "custom.wasm"},
	} {
		t.Run(test.name, func(t *testing.T) {
			appDir := t.TempDir()
			writeMinimalPackageApp(t, appDir)
			manifestContent, err := json.Marshal(map[string]any{
				"name": "nested-wasm", "compiler": "go", "wasm": test.authoredWASM, "assets": []string{},
			})
			if err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, appDir, manifestName, string(manifestContent))

			outDir := filepath.Join(t.TempDir(), "package")
			options := packageOptions{
				appDir: appDir, compiler: "go", outDir: outDir, compress: map[string]bool{},
			}
			if err := packageApp(options); err != nil {
				t.Fatalf("packageApp(initial) error: %v", err)
			}
			staleNames := []string{
				test.authoredWASM,
				test.authoredWASM + ".gz",
				test.authoredWASM + ".br",
				test.standaloneWASM,
				test.standaloneWASM + ".gz",
				test.standaloneWASM + ".br",
			}
			for _, name := range staleNames {
				writeTestFile(t, outDir, name, "stale "+name+"\n")
			}
			writeTestFile(t, outDir, "nested/notes.txt", "preserve unrelated content\n")
			if _, err := inspectPackageGraph(outDir); err != nil {
				t.Fatalf("inspectPackageGraph(package with undeclared stale WASM) error: %v", err)
			}

			staleBefore := httptest.NewRecorder()
			staticHandler(outDir).ServeHTTP(staleBefore, httptest.NewRequest(http.MethodGet, "/"+test.authoredWASM, nil))
			if staleBefore.Code != http.StatusOK {
				t.Fatalf("stale nested WASM status before repackaging = %d, want 200", staleBefore.Code)
			}

			if err := packageApp(options); err != nil {
				t.Fatalf("packageApp(repackage) error: %v", err)
			}
			if _, err := inspectPackageGraph(outDir); err != nil {
				t.Fatalf("inspectPackageGraph(repackaged) error: %v", err)
			}
			for _, name := range staleNames {
				if _, err := os.Lstat(filepath.Join(outDir, filepath.FromSlash(name))); !os.IsNotExist(err) {
					t.Fatalf("stale package artifact %s still exists after repackaging: %v", name, err)
				}
			}
			assertFileContent(t, filepath.Join(outDir, "nested", "notes.txt"), "preserve unrelated content\n")

			staleAfter := httptest.NewRecorder()
			staticHandler(outDir).ServeHTTP(staleAfter, httptest.NewRequest(http.MethodGet, "/"+test.authoredWASM, nil))
			if staleAfter.Code != http.StatusNotFound {
				t.Fatalf("stale nested WASM status after repackaging = %d, want 404", staleAfter.Code)
			}

			var manifest assetManifest
			readInspectJSONFixture(t, outDir, assetManifestName, &manifest)
			if _, exists := manifest.Assets[test.standaloneWASM]; !exists {
				t.Fatalf("standalone WASM logical name %q missing from manifest", test.standaloneWASM)
			}
			current := httptest.NewRecorder()
			staticHandler(outDir).ServeHTTP(current, httptest.NewRequest(http.MethodGet, "/"+manifest.Entrypoints.WASM, nil))
			if current.Code != http.StatusOK {
				t.Fatalf("current declared WASM status = %d, want 200", current.Code)
			}
		})
	}
}

func TestCleanPackageArtifactsCleansWASMIdentities(t *testing.T) {
	for _, test := range []struct {
		name       string
		wasmNames  []string
		staleNames []string
	}{
		{
			name:       "nested custom basename",
			wasmNames:  []string{"nested/custom.wasm", "custom.wasm"},
			staleNames: []string{"nested/custom.wasm", "nested/custom.wasm.gz", "nested/custom.wasm.br", "custom.wasm", "custom.wasm.gz", "custom.wasm.br"},
		},
		{
			name:       "non-nested duplicate identity",
			wasmNames:  []string{"bundle.wasm", "bundle.wasm"},
			staleNames: []string{"bundle.wasm", "bundle.wasm.gz", "bundle.wasm.br"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			for _, name := range test.staleNames {
				writeTestFile(t, directory, name, "stale "+name+"\n")
			}
			writeTestFile(t, directory, "nested/notes.txt", "preserve unrelated content\n")

			if err := cleanPackageArtifacts(directory, test.wasmNames...); err != nil {
				t.Fatalf("cleanPackageArtifacts() error: %v", err)
			}
			for _, name := range test.staleNames {
				if _, err := os.Lstat(filepath.Join(directory, filepath.FromSlash(name))); !os.IsNotExist(err) {
					t.Fatalf("stale package artifact %s still exists: %v", name, err)
				}
			}
			assertFileContent(t, filepath.Join(directory, "nested", "notes.txt"), "preserve unrelated content\n")
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

func TestGzipSidecarsArePathNeutral(t *testing.T) {
	content := []byte("same package bytes\n")
	var compressed [][]byte
	for _, name := range []string{"bundle.wasm", "renamed.wasm"} {
		directory := t.TempDir()
		source := filepath.Join(directory, name)
		destination := source + ".gz"
		if err := os.WriteFile(source, content, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := gzipFile(source, destination); err != nil {
			t.Fatalf("gzipFile(%q) error: %v", name, err)
		}
		encoded, err := os.ReadFile(destination)
		if err != nil {
			t.Fatal(err)
		}
		compressed = append(compressed, encoded)

		reader, err := gzip.NewReader(bytes.NewReader(encoded))
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if err := reader.Close(); err != nil {
			t.Fatal(err)
		}
		if reader.Header.Name != "" {
			t.Fatalf("gzip header name = %q, want path-neutral empty value", reader.Header.Name)
		}
		if !bytes.Equal(decoded, content) {
			t.Fatalf("decoded content = %q, want %q", decoded, content)
		}
	}
	if !bytes.Equal(compressed[0], compressed[1]) {
		t.Fatal("gzip sidecar bytes changed with the source filename")
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
