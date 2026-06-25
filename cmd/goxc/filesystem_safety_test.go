package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeValidPackageMetadata(t *testing.T, directory string) {
	t.Helper()
	content := `{
  "version": 1,
  "name": "demo",
  "compiler": "tinygo",
  "toolchainVersion": "test",
  "assetsDir": "assets",
  "hashAssets": true,
  "preload": true,
  "entrypoints": {
    "html": "index.html",
    "wasm": "assets/bundle.12345678.wasm",
    "runtime": "assets/wasm_exec.12345678.js"
  },
  "generatedAt": "2026-01-01T00:00:00Z"
}`
	if err := os.WriteFile(filepath.Join(directory, packageMetadataName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeValidAssetManifest(t *testing.T, directory string) {
	t.Helper()
	content := `{
  "version": 1,
  "assets": {
    "bundle.wasm": {
      "path": "assets/bundle.12345678.wasm",
      "hash": "12345678",
      "type": "application/wasm"
    },
    "wasm_exec.js": {
      "path": "assets/wasm_exec.12345678.js",
      "hash": "12345678",
      "type": "text/javascript"
    }
  },
  "entrypoints": {
    "wasm": "assets/bundle.12345678.wasm",
    "runtime": "assets/wasm_exec.12345678.js"
  }
}`
	if err := os.WriteFile(filepath.Join(directory, assetManifestName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeLegacyPackageSignature(t *testing.T, directory string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, legacyPackageManifest), []byte(`{"goframe":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "main.wasm"), []byte("wasm"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, runtimeAssetName), []byte("js"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeMinimalGOXApp(t *testing.T, appDir string) {
	t.Helper()
	source := `package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func App() gf.Node {
	return <main>demo</main>
}
`
	if err := os.WriteFile(filepath.Join(appDir, "app.gox"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}

func TestEnsureAppDirectoryRejectsSymlinkedAppRoot(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	target := filepath.Join(root, "target-app")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "app-link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := ensureAppDirectory(link); err == nil {
		t.Fatal("ensureAppDirectory() accepted symlinked app root")
	}
}

func TestWorkspaceRootSymlinkRejectedBeforeMutation(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.Mkdir(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeMinimalGOXApp(t, appDir)
	external := filepath.Join(root, "external-workspace")
	if err := os.Mkdir(external, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(external, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(appDir, defaultWorkspaceName)); err != nil {
		t.Fatal(err)
	}

	for name, run := range map[string]func() error{
		"generate": func() error { return generatePath(generateOptions{path: appDir}, true) },
		"build":    func() error { _, err := buildApp(buildOptions{appDir: appDir, compiler: "go"}); return err },
		"package": func() error {
			return packageApp(packageOptions{appDir: appDir, compiler: "go", compress: map[string]bool{}})
		},
		"clean": func() error { return cleanApp(cleanOptions{appDir: appDir}) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(); err == nil {
				t.Fatalf("%s accepted symlinked workspace root", name)
			}
			assertFileContent(t, sentinel, "keep")
		})
	}
}

func TestWorkspaceIntermediateSymlinkRejectedForBuildPreparation(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(filepath.Join(appDir, defaultWorkspaceName), 0o755); err != nil {
		t.Fatal(err)
	}
	writeMinimalGOXApp(t, appDir)
	external := filepath.Join(root, "external-work")
	if err := os.Mkdir(external, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(appDir, defaultWorkspaceName, "work")); err != nil {
		t.Fatal(err)
	}
	layout, err := newBuildLayout(layoutOptions{appDir: appDir})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepareBuildWorkspace(layout, projectManifest{Entry: "."}); err == nil {
		t.Fatal("prepareBuildWorkspace() accepted intermediate workspace symlink")
	}
}

func TestExternalWorkspaceOverlapRejected(t *testing.T) {
	appDir := t.TempDir()
	for _, workspace := range []string{
		appDir,
		filepath.Join(appDir, "workspace"),
	} {
		t.Run(workspace, func(t *testing.T) {
			if _, err := newBuildLayout(layoutOptions{appDir: appDir, workspace: workspace}); err == nil {
				t.Fatalf("newBuildLayout() accepted overlapping workspace %s", workspace)
			}
		})
	}
	if _, err := newBuildLayout(layoutOptions{appDir: appDir, workspace: filepath.Join(t.TempDir(), "workspace")}); err != nil {
		t.Fatalf("newBuildLayout(sibling workspace) error: %v", err)
	}
}

func TestCopyAuthoredGoFilesAllowsDefaultHiddenWorkspaceOnly(t *testing.T) {
	appDir := t.TempDir()
	writeTestFile(t, appDir, "main.go", "package main\n")
	if err := copyAuthoredGoFiles(appDir, filepath.Join(appDir, defaultWorkspaceName, "work", "dev", "app")); err != nil {
		t.Fatalf("copyAuthoredGoFiles(default hidden workspace) error: %v", err)
	}
	if err := copyAuthoredGoFiles(appDir, filepath.Join(appDir, "nested-workspace")); err == nil {
		t.Fatal("copyAuthoredGoFiles() accepted non-tool destination inside source root")
	}
}

func TestGenerateRejectsDirectGOXFileSymlink(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	external := filepath.Join(root, "external.gox")
	if err := os.WriteFile(external, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "app.gox")
	if err := os.Symlink(external, link); err != nil {
		t.Fatal(err)
	}
	if err := generatePath(generateOptions{path: link, outDir: filepath.Join(root, "out")}, true); err == nil {
		t.Fatal("generatePath() accepted direct symlink .gox file")
	}
}

func TestGenerateRejectsDestinationSymlink(t *testing.T) {
	requireSymlinkSupport(t)

	appDir := t.TempDir()
	writeMinimalGOXApp(t, appDir)
	external := filepath.Join(t.TempDir(), "external.go")
	if err := os.WriteFile(external, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "out")
	if err := os.Mkdir(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(outDir, "app.gox.go")); err != nil {
		t.Fatal(err)
	}
	if err := generatePath(generateOptions{path: appDir, outDir: outDir}, true); err == nil {
		t.Fatal("generatePath() accepted symlinked destination")
	}
	assertFileContent(t, external, "keep")
}

func TestCleanGeneratedRejectsSymlinkedGOXSourceBeforeMutation(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.Mkdir(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(root, "external.gox")
	if err := os.WriteFile(external, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(appDir, "app.gox")); err != nil {
		t.Fatal(err)
	}
	generated := filepath.Join(appDir, "app.gox.go")
	if err := os.WriteFile(generated, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cleanApp(cleanOptions{appDir: appDir, generated: true}); err == nil {
		t.Fatal("cleanApp(--generated) accepted symlinked GOX source")
	}
	assertFileContent(t, generated, "keep")
}

func TestWriteFileAtomicDoesNotTruncateHardlinkPeer(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.txt")
	second := filepath.Join(root, "second.txt")
	if err := os.WriteFile(first, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(first, second); err != nil {
		t.Skipf("hardlinks unavailable: %v", err)
	}
	if err := writeFileAtomic(second, []byte("new"), 0o644); err != nil {
		t.Fatalf("writeFileAtomic() error: %v", err)
	}
	assertFileContent(t, first, "keep")
	assertFileContent(t, second, "new")
}

func TestOwnershipMarkersAreStructured(t *testing.T) {
	for _, test := range []struct {
		name    string
		write   func(t *testing.T, dir string)
		owned   bool
		message string
	}{
		{name: "empty current marker", write: func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, packageMetadataName), []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "malformed current marker", write: func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, packageMetadataName), []byte("{"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "empty asset manifest", write: func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, assetManifestName), []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "generic web manifest", write: func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, legacyPackageManifest), []byte(`{"name":"web app"}`), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "valid current marker", write: writeValidPackageMetadata, owned: true},
		{name: "valid asset manifest", write: writeValidAssetManifest, owned: true},
		{name: "valid legacy signature", write: writeLegacyPackageSignature, owned: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			test.write(t, dir)
			if got := isGoframeOwnedExport(dir); got != test.owned {
				t.Fatalf("isGoframeOwnedExport() = %v, want %v", got, test.owned)
			}
		})
	}
}

func TestPackageDestinationFalseMarkerPreservesFiles(t *testing.T) {
	outDir := t.TempDir()
	userFile := filepath.Join(outDir, "user.txt")
	if err := os.WriteFile(filepath.Join(outDir, packageMetadataName), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userFile, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validatePackageDestination(outDir); err == nil {
		t.Fatal("validatePackageDestination() accepted false marker")
	}
	assertFileContent(t, userFile, "keep")
}

func TestExportRejectsOverlappingDestination(t *testing.T) {
	appDir := t.TempDir()
	layout, err := newBuildLayout(layoutOptions{appDir: appDir})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(layout.PackageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeValidPackageMetadata(t, layout.PackageDir)
	for _, outDir := range []string{
		layout.PackageDir,
		filepath.Join(layout.PackageDir, "nested"),
		filepath.Dir(layout.PackageDir),
	} {
		t.Run(outDir, func(t *testing.T) {
			if err := exportApp(exportOptions{appDir: appDir, outDir: outDir}); err == nil {
				t.Fatalf("exportApp() accepted overlapping outDir %s", outDir)
			}
		})
	}
}

func TestPackageRejectsExplicitOutputInsideApp(t *testing.T) {
	appDir := t.TempDir()
	writeMinimalGOXApp(t, appDir)
	if err := packageApp(packageOptions{
		appDir:   appDir,
		compiler: "go",
		outDir:   filepath.Join(appDir, "dist"),
		compress: map[string]bool{},
	}); err == nil {
		t.Fatal("packageApp() accepted explicit output inside app tree")
	}
}

func TestPackageAssetPlanRejectsReservedAndDuplicateNames(t *testing.T) {
	tests := []projectManifest{
		{WASM: "bundle.wasm", Assets: []string{"bundle.wasm"}},
		{WASM: "bundle.wasm", Assets: []string{"wasm_exec.js"}},
		{WASM: "bundle.wasm", Assets: []string{"bundle.wasm.gz"}},
		{WASM: "bundle.wasm", Assets: []string{"styles.css", "./styles.css"}},
		{WASM: "bundle.wasm", Assets: []string{"a/../styles.css", "styles.css"}},
		{WASM: "wasm_exec.js", Assets: []string{"index.html"}},
	}
	for _, manifest := range tests {
		t.Run(strings.Join(manifest.Assets, ","), func(t *testing.T) {
			if _, err := validatePackageAssetPlan(manifest, filepath.Base(manifest.WASM), packageOptions{compress: map[string]bool{"gzip": true}}); err == nil {
				t.Fatalf("validatePackageAssetPlan() accepted manifest %+v", manifest)
			}
		})
	}
}

func TestPackageAssetPlanAllowsDistinctNestedAssets(t *testing.T) {
	assets, err := validatePackageAssetPlan(projectManifest{
		WASM:   "bundle.wasm",
		Assets: []string{"index.html", "styles/app.css", "images/logo.svg"},
	}, "bundle.wasm", packageOptions{compress: map[string]bool{"gzip": true, "br": true}})
	if err != nil {
		t.Fatalf("validatePackageAssetPlan() rejected distinct nested assets: %v", err)
	}
	want := []string{"index.html", "styles/app.css", "images/logo.svg"}
	if strings.Join(assets, ",") != strings.Join(want, ",") {
		t.Fatalf("assets = %#v, want %#v", assets, want)
	}
}

func TestPublishRejectsSymlinkedSourceEntries(t *testing.T) {
	requireSymlinkSupport(t)

	source := t.TempDir()
	destination := t.TempDir()
	external := filepath.Join(t.TempDir(), "external.html")
	if err := os.WriteFile(external, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(source, "index.html")); err != nil {
		t.Fatal(err)
	}
	if err := publishPackageArtifacts(source, destination); err == nil {
		t.Fatal("publishPackageArtifacts() accepted symlinked source file")
	}
	if _, err := os.Stat(filepath.Join(destination, "index.html")); !os.IsNotExist(err) {
		t.Fatalf("symlinked source was published: %v", err)
	}
}

func TestPublishWritesMetadataLastOnFailure(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "assets", "bundle.wasm"), []byte("wasm"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeValidPackageMetadata(t, source)
	if err := os.WriteFile(filepath.Join(destination, "assets"), []byte("not-a-directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := publishPackageArtifacts(source, destination); err == nil {
		t.Fatal("publishPackageArtifacts() unexpectedly succeeded")
	}
	if _, err := os.Stat(filepath.Join(destination, packageMetadataName)); !os.IsNotExist(err) {
		t.Fatalf("metadata published after failure: %v", err)
	}
}

func TestCleanRemovesFinalSymlinkOnly(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(filepath.Join(appDir, defaultWorkspaceName), 0o755); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(root, "external")
	if err := os.Mkdir(external, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(external, "keep.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(appDir, defaultWorkspaceName, "build")
	if err := os.Symlink(external, link); err != nil {
		t.Fatal(err)
	}
	if err := cleanApp(cleanOptions{appDir: appDir}); err != nil {
		t.Fatalf("cleanApp() error: %v", err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("final cleanup symlink still exists: %v", err)
	}
	assertFileContent(t, keep, "keep")
}

func TestServeRejectsSymlinkRootAndEntries(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	target := filepath.Join(root, "serve-root")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	linkRoot := filepath.Join(root, "serve-link")
	if err := os.Symlink(target, linkRoot); err != nil {
		t.Fatal(err)
	}
	if err := serve(serveOptions{dir: linkRoot, port: 0}); err == nil {
		t.Fatal("serve() accepted symlinked root")
	}

	external := filepath.Join(root, "secret.txt")
	if err := os.WriteFile(external, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(target, "secret.txt")); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/secret.txt", nil)
	response := httptest.NewRecorder()
	staticHandler(target).ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("symlinked serve entry status = %d, want 404", response.Code)
	}
}
