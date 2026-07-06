package main

import (
	"bytes"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandUsage(t *testing.T) {
	for _, command := range []string{"generate", "build", "package", "export", "serve", "size", "doctor", "clean", "version"} {
		t.Run(command, func(t *testing.T) {
			var output bytes.Buffer
			commandUsage(&output, command)
			got := output.String()
			if !strings.Contains(got, "usage: goxc "+command) {
				t.Fatalf("commandUsage(%q) = %q", command, got)
			}
		})
	}
}

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
	layout := BuildLayout{
		BuildDir:   filepath.Join("demo", defaultWorkspaceName, "build", "tinygo", "dev"),
		PackageDir: filepath.Join("demo", defaultWorkspaceName, "package", "standalone"),
	}
	if got := buildOutputPath(buildOptions{appDir: "demo"}, manifest, layout); got != filepath.Join("demo", defaultWorkspaceName, "build", "tinygo", "dev", "app.wasm") {
		t.Fatalf("build output = %q", got)
	}
	if got := packageOutputDirectory(packageOptions{appDir: "demo"}, layout); got != filepath.Join("demo", defaultWorkspaceName, "package", "standalone") {
		t.Fatalf("package output = %q", got)
	}
}

func TestBuildLayoutDefaultsAndExternalWorkspace(t *testing.T) {
	appDir := filepath.Join(t.TempDir(), "dashboard")
	layout, err := newBuildLayout(layoutOptions{appDir: appDir, compiler: "tinygo"})
	if err != nil {
		t.Fatalf("newBuildLayout() error: %v", err)
	}
	if layout.WorkspaceRoot != filepath.Join(appDir, defaultWorkspaceName) {
		t.Fatalf("workspace root = %q", layout.WorkspaceRoot)
	}
	if layout.BuildDir != filepath.Join(appDir, defaultWorkspaceName, "build", "tinygo", "dev") {
		t.Fatalf("build dir = %q", layout.BuildDir)
	}

	external := filepath.Join(t.TempDir(), "workspace")
	layout, err = newBuildLayout(layoutOptions{appDir: appDir, compiler: "tinygo", workspace: external})
	if err != nil {
		t.Fatalf("newBuildLayout(external) error: %v", err)
	}
	if !strings.HasPrefix(layout.WorkspaceRoot, external+string(filepath.Separator)) {
		t.Fatalf("external workspace root = %q, want below %q", layout.WorkspaceRoot, external)
	}
}

func TestBuildLayoutExternalWorkspaceAvoidsAppCollisions(t *testing.T) {
	root := t.TempDir()
	firstApp := filepath.Join(root, "one", "dashboard")
	secondApp := filepath.Join(root, "two", "dashboard")
	external := filepath.Join(t.TempDir(), "workspace")

	first, err := newBuildLayout(layoutOptions{appDir: firstApp, workspace: external})
	if err != nil {
		t.Fatalf("newBuildLayout(first) error: %v", err)
	}
	second, err := newBuildLayout(layoutOptions{appDir: secondApp, workspace: external})
	if err != nil {
		t.Fatalf("newBuildLayout(second) error: %v", err)
	}
	if first.WorkspaceRoot == second.WorkspaceRoot {
		t.Fatalf("external workspace roots collide: %q", first.WorkspaceRoot)
	}
	for _, layout := range []BuildLayout{first, second} {
		if !strings.HasPrefix(layout.WorkspaceRoot, external+string(filepath.Separator)) {
			t.Fatalf("workspace root %q is not below %q", layout.WorkspaceRoot, external)
		}
	}
}

func TestParsePackageOptionsCompression(t *testing.T) {
	options, err := parsePackageOptions([]string{"app", "--asset-hash", "--preload", "--compress=gzip,br"})
	if err != nil {
		t.Fatalf("parsePackageOptions() error: %v", err)
	}
	if !options.compress["gzip"] || !options.compress["br"] {
		t.Fatalf("compression options = %#v", options.compress)
	}
	if !options.assetHash || !options.preload {
		t.Fatalf("asset flags = assetHash:%v preload:%v", options.assetHash, options.preload)
	}
}

func TestLoadManifestDefaultsAndOverrides(t *testing.T) {
	appDir := t.TempDir()
	defaults, err := loadManifest(appDir)
	if err != nil {
		t.Fatalf("loadManifest(defaults) error: %v", err)
	}
	if defaults.Compiler != "go" || defaults.Output != "dist" || defaults.WASM != "bundle.wasm" {
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

func TestLoadManifestAssetsUnionForms(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		mode    manifestAssetMode
		dir     string
		list    []string
	}{
		{
			name:    "omitted",
			content: `{}`,
			mode:    manifestAssetsAuto,
		},
		{
			name:    "null",
			content: `{"assets":null}`,
			mode:    manifestAssetsAuto,
		},
		{
			name:    "directory",
			content: `{"assets":"./assets"}`,
			mode:    manifestAssetsDirectory,
			dir:     "assets",
		},
		{
			name:    "directory without dot",
			content: `{"assets":"assets"}`,
			mode:    manifestAssetsDirectory,
			dir:     "assets",
		},
		{
			name:    "list",
			content: `{"assets":["index.html","styles.css"]}`,
			mode:    manifestAssetsList,
			list:    []string{"index.html", "styles.css"},
		},
		{
			name:    "empty list",
			content: `{"assets":[]}`,
			mode:    manifestAssetsList,
			list:    []string{},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			appDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(appDir, manifestName), []byte(test.content), 0o644); err != nil {
				t.Fatal(err)
			}
			manifest, err := loadManifest(appDir)
			if err != nil {
				t.Fatalf("loadManifest() error: %v", err)
			}
			if manifest.Assets.Mode != test.mode || manifest.Assets.Directory != test.dir || strings.Join(manifest.Assets.List, ",") != strings.Join(test.list, ",") {
				t.Fatalf("assets = %+v, want mode:%v dir:%q list:%#v", manifest.Assets, test.mode, test.dir, test.list)
			}
		})
	}
}

func TestLoadManifestRejectsInvalidAssetsType(t *testing.T) {
	for _, content := range []string{
		`{"assets":42}`,
		`{"assets":true}`,
		`{"assets":{"dir":"assets"}}`,
	} {
		t.Run(content, func(t *testing.T) {
			appDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(appDir, manifestName), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := loadManifest(appDir)
			if err == nil || !strings.Contains(err.Error(), "assets must be a string directory, array of paths, null, or omitted") {
				t.Fatalf("loadManifest() error = %v, want assets type guidance", err)
			}
		})
	}
}

func TestLoadManifestAcceptsLegacyMainWASM(t *testing.T) {
	appDir := t.TempDir()
	content := []byte(`{"wasm":"main.wasm"}`)
	if err := os.WriteFile(filepath.Join(appDir, manifestName), content, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := loadManifest(appDir)
	if err != nil {
		t.Fatalf("loadManifest(main.wasm) error: %v", err)
	}
	if manifest.WASM != "main.wasm" {
		t.Fatalf("WASM = %q, want main.wasm", manifest.WASM)
	}
}

func TestManifestRejectsNonWASMOutputNames(t *testing.T) {
	tests := []string{
		`{"wasm":"main.go"}`,
		`{"wasm":"go.mod"}`,
		`{"wasm":"bundle"}`,
		`{"wasm":"bundle.wasm.gz"}`,
		`{"wasm":"wasm_exec.js"}`,
	}
	for _, content := range tests {
		t.Run(content, func(t *testing.T) {
			appDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(appDir, manifestName), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := loadManifest(appDir); err == nil {
				t.Fatalf("loadManifest() accepted unsafe wasm path from %s", content)
			}
		})
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

func TestManifestRejectsEscapingPaths(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "entry", content: `{"entry":"../cmd"}`},
		{name: "entry slash root", content: `{"entry":"/abs/path"}`},
		{name: "entry drive root", content: `{"entry":"C:/abs/path"}`},
		{name: "output", content: `{"output":"../dist"}`},
		{name: "output slash root", content: `{"output":"/dist"}`},
		{name: "wasm", content: `{"wasm":"../bundle.wasm"}`},
		{name: "wasm slash root", content: `{"wasm":"/bundle.wasm"}`},
		{name: "asset", content: `{"assets":["../secret.css"]}`},
		{name: "asset slash root", content: `{"assets":["/secret.css"]}`},
		{name: "asset directory parent", content: `{"assets":"../assets"}`},
		{name: "asset directory slash root", content: `{"assets":"/assets"}`},
		{name: "asset directory drive root", content: `{"assets":"C:/assets"}`},
		{name: "asset directory tool root", content: `{"assets":".goframe/assets"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(appDir, manifestName), []byte(test.content), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := loadManifest(appDir); err == nil {
				t.Fatalf("loadManifest() accepted escaping %s path", test.name)
			}
		})
	}
}

func TestManifestRejectsUnsafeEntryPaths(t *testing.T) {
	tests := []string{
		`{"entry":""}`,
		`{"entry":"/abs/path"}`,
		`{"entry":"cmd/../outside"}`,
		`{"entry":".goframe/work"}`,
		`{"entry":"build"}`,
		`{"entry":"dist"}`,
		`{"entry":"node_modules"}`,
		`{"entry":".git"}`,
	}
	for _, content := range tests {
		t.Run(content, func(t *testing.T) {
			appDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(appDir, manifestName), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := loadManifest(appDir); err == nil {
				t.Fatalf("loadManifest() accepted unsafe entry path from %s", content)
			}
		})
	}
}

func TestLoadManifestNormalizesChildEntry(t *testing.T) {
	appDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(appDir, manifestName), []byte(`{"entry":"./cmd/app"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := loadManifest(appDir)
	if err != nil {
		t.Fatalf("loadManifest(child entry) error: %v", err)
	}
	if manifest.Entry != "cmd/app" {
		t.Fatalf("manifest entry = %q, want cmd/app", manifest.Entry)
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
	for _, directory := range []string{
		filepath.Join(defaultWorkspaceName, "work"),
		filepath.Join(defaultWorkspaceName, "build"),
		filepath.Join(defaultWorkspaceName, "package"),
		filepath.Join(defaultWorkspaceName, "gen"),
	} {
		if err := os.MkdirAll(filepath.Join(appDir, directory), 0o755); err != nil {
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
	for _, directory := range []string{"work", "build", "package"} {
		if _, err := os.Stat(filepath.Join(appDir, defaultWorkspaceName, directory)); !os.IsNotExist(err) {
			t.Fatalf("%s still exists: %v", directory, err)
		}
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
	if _, err := os.Stat(filepath.Join(appDir, defaultWorkspaceName, "gen")); !os.IsNotExist(err) {
		t.Fatalf("generated directory still exists: %v", err)
	}
}

func TestGeneratePathWritesToHiddenWorkspaceByDefault(t *testing.T) {
	appDir := t.TempDir()
	source := `package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func App() gf.Node {
	return <div>Hello</div>
}
`
	if err := os.WriteFile(filepath.Join(appDir, "app.gox"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := generatePath(generateOptions{path: appDir}, true); err != nil {
		t.Fatalf("generatePath() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDir, "app.gox.go")); !os.IsNotExist(err) {
		t.Fatalf("adjacent generated file exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDir, defaultWorkspaceName, "gen", "app.gox.go")); err != nil {
		t.Fatalf("hidden generated file missing: %v", err)
	}
}

func TestGenerateInPlaceWritesAdjacentFileAndWarns(t *testing.T) {
	appDir := t.TempDir()
	source := `package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func App() gf.Node {
	return <div>Hello</div>
}
`
	if err := os.WriteFile(filepath.Join(appDir, "app.gox"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	stderr := captureStderr(t, func() {
		if err := generatePath(generateOptions{path: appDir, inPlace: true}, true); err != nil {
			t.Fatalf("generatePath(inPlace) error: %v", err)
		}
	})
	if !strings.Contains(stderr, "--in-place writes generated compiler output into the source tree") {
		t.Fatalf("missing --in-place warning in stderr: %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(appDir, "app.gox.go")); err != nil {
		t.Fatalf("adjacent generated file missing: %v", err)
	}
}

func TestSizeReportsHelpfulErrorForEmptyDirectory(t *testing.T) {
	err := sizeCommand([]string{t.TempDir()})
	if err == nil {
		t.Fatal("sizeCommand() returned nil error")
	}
}

func TestParseServeOptionsUsesWorkspacePackageDir(t *testing.T) {
	appDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(appDir, manifestName), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	options, err := parseServeOptions([]string{appDir})
	if err != nil {
		t.Fatalf("parseServeOptions() error: %v", err)
	}
	layout, err := newBuildLayout(layoutOptions{appDir: appDir})
	if err != nil {
		t.Fatal(err)
	}
	if options.dir != layout.PackageDir {
		t.Fatalf("serve dir = %q, want %q", options.dir, layout.PackageDir)
	}
}

func TestArtifactDirectoryPrefersWorkspacePackage(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(layout.PackageDir, "assets", "bundle.wasm"), []byte("wasm"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := artifactDirectory(sizeOptions{path: appDir})
	if err != nil {
		t.Fatalf("artifactDirectory() error: %v", err)
	}
	if got != layout.PackageDir {
		t.Fatalf("artifact dir = %q, want %q", got, layout.PackageDir)
	}
}

func TestCleanPackageArtifactsRemovesOldCompression(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"main.wasm",
		"main.wasm.gz",
		"main.wasm.br",
		"bundle.wasm",
		"index.html",
		"asset-manifest.json",
		"goframe-package.json",
		filepath.Join("assets", "bundle.oldhash.wasm"),
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := cleanPackageArtifacts(directory, "bundle.wasm"); err != nil {
		t.Fatalf("cleanPackageArtifacts() error: %v", err)
	}
	for _, name := range []string{"main.wasm", "main.wasm.gz", "main.wasm.br", "bundle.wasm", "index.html", "asset-manifest.json", "goframe-package.json", "assets"} {
		if _, err := os.Stat(filepath.Join(directory, name)); !os.IsNotExist(err) {
			t.Fatalf("%s still exists: %v", name, err)
		}
	}
}

func TestValidatePackageDestinationRejectsUnownedNonEmptyDirectory(t *testing.T) {
	outDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outDir, "notes.txt"), []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validatePackageDestination(outDir); err == nil {
		t.Fatal("validatePackageDestination() accepted non-empty unowned directory")
	}
}

func TestValidatePackageDestinationAllowsPreviousGoFramePackage(t *testing.T) {
	outDir := t.TempDir()
	writeCompleteCurrentPackage(t, outDir)
	if err := validatePackageDestination(outDir); err != nil {
		t.Fatalf("validatePackageDestination(previous package) error: %v", err)
	}
}

func TestPublishPackageArtifactsCopiesStagedTree(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		"index.html":                 "<html></html>",
		"asset-manifest.json":        "{}",
		"goframe-package.json":       "{}",
		"assets/bundle.wasm":         "wasm",
		"assets/wasm_exec.js":        "js",
		"assets/styles.12345678.css": "body{}",
	} {
		if err := os.WriteFile(filepath.Join(source, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := publishPackageArtifacts(source, destination); err != nil {
		t.Fatalf("publishPackageArtifacts() error: %v", err)
	}
	for path, want := range map[string]string{
		"index.html":                 "<html></html>",
		"asset-manifest.json":        "{}",
		"goframe-package.json":       "{}",
		"assets/bundle.wasm":         "wasm",
		"assets/wasm_exec.js":        "js",
		"assets/styles.12345678.css": "body{}",
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

func TestWritePackageAssetRejectsEscapingLogicalName(t *testing.T) {
	source := filepath.Join(t.TempDir(), "styles.css")
	if err := os.WriteFile(source, []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	assetsDir := filepath.Join(t.TempDir(), "assets")
	if _, err := writePackageAsset(source, assetsDir, "../styles.css", packageOptions{compress: map[string]bool{}}); err == nil {
		t.Fatal("writePackageAsset() accepted escaping logical name")
	}
	if _, err := os.Stat(filepath.Join(assetsDir, "..", "styles.css")); !os.IsNotExist(err) {
		t.Fatalf("escaping asset was written: %v", err)
	}
}

func TestDoctorDoesNotFail(t *testing.T) {
	if err := doctorCommand(nil); err != nil {
		t.Fatalf("doctorCommand() error: %v", err)
	}
}

func TestDoctorReturnsErrorWhenRequiredChecksFail(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var err error
	output := captureStdout(t, func() {
		err = doctorCommand(nil)
	})
	if err == nil {
		t.Fatal("doctorCommand() returned nil error for required check failure")
	}
	for _, want := range []string{
		"Go:           not found",
		"Status: errors found",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, output)
		}
	}
}

func TestStaticHandlerServesWASMContentType(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "assets", "bundle.wasm"), []byte("wasm"), 0o644); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest("GET", "/assets/bundle.wasm", nil)
	response := httptest.NewRecorder()
	staticHandler(directory).ServeHTTP(response, request)

	if got := response.Header().Get("Content-Type"); got != "application/wasm" {
		t.Fatalf("Content-Type = %q, want application/wasm", got)
	}
}

func TestHashedAssetName(t *testing.T) {
	if got := hashedAssetName("bundle.wasm", "a83f19c4"); got != "bundle.a83f19c4.wasm" {
		t.Fatalf("hashedAssetName(bundle) = %q", got)
	}
	if got := hashedAssetName("css/styles.css", "77a1de20"); got != "css/styles.77a1de20.css" {
		t.Fatalf("hashedAssetName(styles) = %q", got)
	}
}

func TestRewriteIndexHTMLPlaceholders(t *testing.T) {
	source := `<!doctype html>
<head>
<!-- goframe:preload -->
<!-- /goframe:preload -->
<link rel="stylesheet" href="styles.css" />
</head>
<body>
<!-- goframe:runtime -->
<script src="wasm_exec.js"></script>
<!-- /goframe:runtime -->
<!-- goframe:bootstrap -->
<script>WebAssembly.instantiateStreaming(fetch("main.wasm"), go.importObject)</script>
<!-- /goframe:bootstrap -->
</body>`
	got := rewriteIndexHTML(source, htmlRewriteOptions{
		preload:     true,
		wasmPath:    "assets/bundle.a83f19c4.wasm",
		runtimePath: "assets/wasm_exec.91b2cc10.js",
		stylePaths:  []string{"assets/styles.77a1de20.css"},
		styleRewrites: map[string]string{
			"styles.css": "assets/styles.77a1de20.css",
		},
	})
	for _, want := range []string{
		`href="assets/bundle.a83f19c4.wasm" as="fetch" type="application/wasm" crossorigin`,
		`<script src="assets/wasm_exec.91b2cc10.js"></script>`,
		`fetch("assets/bundle.a83f19c4.wasm")`,
		`href="assets/styles.77a1de20.css"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rewritten HTML missing %q:\n%s", want, got)
		}
	}
}

func TestWritePackageAssetUsesHashAndManifestPath(t *testing.T) {
	source := filepath.Join(t.TempDir(), "bundle.wasm")
	if err := os.WriteFile(source, []byte("wasm"), 0o644); err != nil {
		t.Fatal(err)
	}
	assetsDir := filepath.Join(t.TempDir(), "assets")
	asset, err := writePackageAsset(source, assetsDir, "bundle.wasm", packageOptions{assetHash: true, compress: map[string]bool{}})
	if err != nil {
		t.Fatalf("writePackageAsset() error: %v", err)
	}
	if asset.Hash == "" || asset.Path != "assets/bundle."+asset.Hash+".wasm" || asset.Type != "application/wasm" {
		t.Fatalf("asset entry = %+v", asset)
	}
	if _, err := os.Stat(filepath.Join(assetsDir, "bundle."+asset.Hash+".wasm")); err != nil {
		t.Fatalf("hashed asset not written: %v", err)
	}
}

func TestWritePackageAssetCreatesGzipSidecar(t *testing.T) {
	source := filepath.Join(t.TempDir(), "bundle.wasm")
	if err := os.WriteFile(source, []byte("wasm"), 0o644); err != nil {
		t.Fatal(err)
	}
	assetsDir := filepath.Join(t.TempDir(), "assets")
	asset, err := writePackageAsset(source, assetsDir, "bundle.wasm", packageOptions{compress: map[string]bool{"gzip": true}})
	if err != nil {
		t.Fatalf("writePackageAsset() error: %v", err)
	}
	if asset.Compressed["gzip"] != "assets/bundle.wasm.gz" {
		t.Fatalf("gzip manifest entry = %#v", asset.Compressed)
	}
	if _, err := os.Stat(filepath.Join(assetsDir, "bundle.wasm.gz")); err != nil {
		t.Fatalf("gzip sidecar not written: %v", err)
	}
}

func TestHumanSize(t *testing.T) {
	if got := humanSize(73_631); got != "71.9 KiB" {
		t.Fatalf("humanSize() = %q, want 71.9 KiB", got)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	defer func() {
		os.Stderr = old
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = old
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return string(content)
}
