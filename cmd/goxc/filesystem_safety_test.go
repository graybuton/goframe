package main

import (
	"encoding/json"
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

func writeCompleteCurrentPackage(t *testing.T, directory string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(directory, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeValidPackageMetadata(t, directory)
	writeValidAssetManifest(t, directory)
	for path, content := range map[string]string{
		"index.html":                        "<html></html>",
		"assets/bundle.12345678.wasm":       "wasm",
		"assets/wasm_exec.12345678.js":      "js",
		"assets/bundle.wasm":                "wasm",
		"assets/wasm_exec.js":               "js",
		"assets/styles.12345678.css":        "body{}",
		"assets/bundle.12345678.wasm.gz":    "gzip",
		"assets/bundle.12345678.wasm.br":    "br",
		"assets/wasm_exec.12345678.js.gz":   "js gzip",
		"assets/wasm_exec.12345678.js.br":   "js br",
		"assets/styles.12345678.css.gz":     "css gzip",
		"assets/styles.12345678.css.br":     "css br",
		"assets/bundle.12345678.wasm.zstd":  "zstd",
		"assets/wasm_exec.12345678.js.zstd": "js zstd",
		"assets/styles.12345678.css.zstd":   "css zstd",
	} {
		if err := os.WriteFile(filepath.Join(directory, filepath.FromSlash(path)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
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
	if err := os.WriteFile(filepath.Join(directory, legacyPackageManifest), []byte(`{
  "name": "demo",
  "compiler": "tinygo",
  "wasm": "main.wasm",
  "assets": ["index.html"],
  "toolchainVersion": "v0.0.0-test"
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "main.wasm"), []byte("wasm"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, runtimeAssetName), []byte("js"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("<html></html>"), 0o644); err != nil {
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
	if err := os.WriteFile(filepath.Join(appDir, indexHTMLAssetName), []byte("<!doctype html><div id=\"root\"></div>"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeMinimalPackageApp(t *testing.T, appDir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(appDir, manifestName), []byte(`{"name":"demo","compiler":"go","assets":["index.html"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	mainSource := `//go:build js && wasm

package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func main() {
	done := make(chan struct{})
	gf.Mount("root", App)
	<-done
}
`
	if err := os.WriteFile(filepath.Join(appDir, "main.go"), []byte(mainSource), 0o644); err != nil {
		t.Fatal(err)
	}
	writeMinimalGOXApp(t, appDir)
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

func TestPhysicalPathRelationDetectsSymlinkAliasOverlap(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.Mkdir(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "app-alias")
	if err := os.Symlink(appDir, alias); err != nil {
		t.Fatal(err)
	}
	overlap, err := physicalPathsOverlap(appDir, filepath.Join(alias, "out"))
	if err != nil {
		t.Fatalf("physicalPathsOverlap() error: %v", err)
	}
	if !overlap {
		t.Fatal("physicalPathsOverlap() missed symlink alias overlap")
	}
}

func TestExternalWorkspacePhysicalAliasOverlapRejected(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.Mkdir(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "app-alias")
	if err := os.Symlink(appDir, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := newBuildLayout(layoutOptions{appDir: appDir, workspace: filepath.Join(alias, "workspace")}); err == nil {
		t.Fatal("newBuildLayout() accepted external workspace alias into app tree")
	}
}

func TestBuildRejectsOutputAliasIntoAppSource(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	writeTestFile(t, appDir, "go.mod", "module example.com/app\n\ngo 1.22\n")
	writeTestFile(t, appDir, "goframe.json", `{"name":"demo","entry":"."}`)
	writeTestFile(t, appDir, "main.go", "package main\nfunc main() {}\n")
	alias := filepath.Join(root, "app-alias")
	if err := os.Symlink(appDir, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := buildApp(buildOptions{appDir: appDir, compiler: "go", outDir: filepath.Join(alias, "out")}); err == nil {
		t.Fatal("buildApp() accepted output alias into app source")
	}
	assertFileContent(t, filepath.Join(appDir, "main.go"), "package main\nfunc main() {}\n")
	assertFileContent(t, filepath.Join(appDir, "go.mod"), "module example.com/app\n\ngo 1.22\n")
	assertFileContent(t, filepath.Join(appDir, "goframe.json"), `{"name":"demo","entry":"."}`)
}

func TestGenerateRejectsOutputAliasIntoAppSource(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.Mkdir(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeMinimalGOXApp(t, appDir)
	alias := filepath.Join(root, "app-alias")
	if err := os.Symlink(appDir, alias); err != nil {
		t.Fatal(err)
	}
	if err := generatePath(generateOptions{path: appDir, outDir: filepath.Join(alias, "gen")}, true); err == nil {
		t.Fatal("generatePath() accepted output alias into app source")
	}
	if _, err := os.Stat(filepath.Join(appDir, "gen", "app.gox.go")); !os.IsNotExist(err) {
		t.Fatalf("generated file appeared through alias: %v", err)
	}
}

func TestPackageRejectsOutputAliasIntoAppSource(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.Mkdir(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeMinimalGOXApp(t, appDir)
	alias := filepath.Join(root, "app-alias")
	if err := os.Symlink(appDir, alias); err != nil {
		t.Fatal(err)
	}
	if err := packageApp(packageOptions{appDir: appDir, compiler: "go", outDir: filepath.Join(alias, "package"), compress: map[string]bool{}}); err == nil {
		t.Fatal("packageApp() accepted output alias into app source")
	}
	if _, err := os.Stat(filepath.Join(appDir, "package")); !os.IsNotExist(err) {
		t.Fatalf("package output appeared through alias: %v", err)
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
		{name: "valid current package", write: writeCompleteCurrentPackage, owned: true},
		{name: "valid asset manifest without completion marker", write: writeValidAssetManifest, owned: false},
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

func TestCurrentPackageOwnershipRequiresCompanionMetadataAndArtifacts(t *testing.T) {
	tests := []struct {
		name  string
		write func(t *testing.T, dir string)
	}{
		{name: "metadata without asset manifest", write: writeValidPackageMetadata},
		{name: "metadata with mismatched asset manifest", write: func(t *testing.T, dir string) {
			writeValidPackageMetadata(t, dir)
			writeValidAssetManifest(t, dir)
			content, err := os.ReadFile(filepath.Join(dir, assetManifestName))
			if err != nil {
				t.Fatal(err)
			}
			content = []byte(strings.Replace(string(content), "assets/bundle.12345678.wasm", "assets/other.wasm", 1))
			if err := os.WriteFile(filepath.Join(dir, assetManifestName), content, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "missing referenced files", write: func(t *testing.T, dir string) {
			writeValidPackageMetadata(t, dir)
			writeValidAssetManifest(t, dir)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			test.write(t, dir)
			ownership := inspectPackageOwnership(dir)
			if ownership.State != packageIncompleteOrInvalid {
				t.Fatalf("ownership state = %v (%s), want incomplete/invalid", ownership.State, ownership.Reason)
			}
			if isGoframeOwnedExport(dir) {
				t.Fatal("incomplete package was treated as owned")
			}
		})
	}
}

func TestAssetManifestAloneDoesNotGrantOwnership(t *testing.T) {
	dir := t.TempDir()
	writeValidAssetManifest(t, dir)
	ownership := inspectPackageOwnership(dir)
	if ownership.State != packageUnowned {
		t.Fatalf("ownership state = %v (%s), want unowned", ownership.State, ownership.Reason)
	}
	if isGoframeOwnedExport(dir) {
		t.Fatal("asset manifest alone granted ownership")
	}
}

func TestLegacyOwnershipIsFailClosedForGenericGoWASM(t *testing.T) {
	dir := t.TempDir()
	for path, content := range map[string]string{
		legacyPackageManifest: `{"name":"generic wasm app"}`,
		"main.wasm":           "wasm",
		runtimeAssetName:      "js",
	} {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if isGoframeOwnedExport(dir) {
		t.Fatal("generic Go/WASM dist was treated as GoFrame-owned legacy package")
	}
}

func TestLegacyOwnershipRejectsMalformedOrSymlinkedSignature(t *testing.T) {
	requireSymlinkSupport(t)

	for _, test := range []struct {
		name  string
		write func(t *testing.T, dir string)
	}{
		{name: "empty manifest", write: func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, legacyPackageManifest), []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "main.wasm"), []byte("wasm"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, runtimeAssetName), []byte("js"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "malformed manifest", write: func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, legacyPackageManifest), []byte("{"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "main.wasm"), []byte("wasm"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, runtimeAssetName), []byte("js"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "manifest symlink", write: func(t *testing.T, dir string) {
			target := filepath.Join(t.TempDir(), "manifest.json")
			if err := os.WriteFile(target, []byte(`{"name":"demo","compiler":"tinygo","wasm":"main.wasm","toolchainVersion":"test"}`), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(dir, legacyPackageManifest)); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "main.wasm"), []byte("wasm"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, runtimeAssetName), []byte("js"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "runtime symlink", write: func(t *testing.T, dir string) {
			writeLegacyPackageSignature(t, dir)
			target := filepath.Join(t.TempDir(), "wasm_exec.js")
			if err := os.WriteFile(target, []byte("js"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(dir, runtimeAssetName)); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(dir, runtimeAssetName)); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			test.write(t, dir)
			if isGoframeOwnedExport(dir) {
				t.Fatal("invalid legacy signature was treated as owned")
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
	writeCompleteCurrentPackage(t, layout.PackageDir)
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

func TestExportRejectsPhysicalAliasOverlap(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	layout, err := newBuildLayout(layoutOptions{appDir: appDir})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(layout.PackageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCompleteCurrentPackage(t, layout.PackageDir)
	alias := filepath.Join(root, "package-alias")
	if err := os.Symlink(layout.PackageDir, alias); err != nil {
		t.Fatal(err)
	}
	if err := exportApp(exportOptions{appDir: appDir, outDir: filepath.Join(alias, "nested")}); err == nil {
		t.Fatal("exportApp() accepted output alias into package source")
	}
	if _, err := os.Stat(filepath.Join(layout.PackageDir, "nested")); !os.IsNotExist(err) {
		t.Fatalf("export output appeared through alias: %v", err)
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
		{WASM: "bundle.wasm", Assets: listManifestAssets([]string{"bundle.wasm"})},
		{WASM: "bundle.wasm", Assets: listManifestAssets([]string{"wasm_exec.js"})},
		{WASM: "bundle.wasm", Assets: listManifestAssets([]string{"bundle.wasm.gz"})},
		{WASM: "bundle.wasm", Assets: listManifestAssets([]string{"asset-manifest.json"})},
		{WASM: "bundle.wasm", Assets: listManifestAssets([]string{"goframe-package.json"})},
		{WASM: "bundle.wasm", Assets: listManifestAssets([]string{"styles.css", "./styles.css"})},
		{WASM: "wasm_exec.js", Assets: listManifestAssets([]string{})},
	}
	for _, manifest := range tests {
		t.Run(strings.Join(manifest.Assets.List, ","), func(t *testing.T) {
			appDir := t.TempDir()
			if _, err := planPackageAssets(appDir, manifest, filepath.Base(manifest.WASM), packageOptions{compress: map[string]bool{"gzip": true}}); err == nil {
				t.Fatalf("planPackageAssets() accepted manifest %+v", manifest)
			}
		})
	}
}

func TestPackageAssetPlanAllowsDistinctNestedAssets(t *testing.T) {
	appDir := t.TempDir()
	writeTestFile(t, appDir, indexHTMLAssetName, "<html></html>")
	plan, err := planPackageAssets(appDir, projectManifest{
		WASM:   "bundle.wasm",
		Assets: listManifestAssets([]string{"index.html", "styles/app.css", "images/logo.svg"}),
	}, "bundle.wasm", packageOptions{compress: map[string]bool{"gzip": true, "br": true}})
	if err != nil {
		t.Fatalf("planPackageAssets() rejected distinct nested assets: %v", err)
	}
	if plan.CustomIndexPath == "" || plan.GenerateIndex {
		t.Fatalf("index plan = custom:%q generated:%v, want custom index", plan.CustomIndexPath, plan.GenerateIndex)
	}
	got := []string{}
	for _, asset := range plan.Assets {
		got = append(got, asset.LogicalName)
	}
	want := []string{"images/logo.svg", "styles/app.css"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("assets = %#v, want %#v", got, want)
	}
}

func TestPackageAssetPlanRejectsSymlinkIndex(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.Mkdir(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(root, "external-index.html")
	if err := os.WriteFile(external, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(appDir, indexHTMLAssetName)); err != nil {
		t.Fatal(err)
	}
	_, err := planPackageAssets(appDir, projectManifest{
		WASM:   "bundle.wasm",
		Assets: listManifestAssets([]string{indexHTMLAssetName}),
	}, "bundle.wasm", packageOptions{compress: map[string]bool{}})
	if err == nil || !strings.Contains(err.Error(), "is a symlink") {
		t.Fatalf("error = %v, want symlink index source", err)
	}
	assertFileContent(t, external, "keep")
}

func TestPackageAssetPlanDirectoryMode(t *testing.T) {
	appDir := t.TempDir()
	writeTestFile(t, appDir, "assets/index.html", "<html></html>")
	writeTestFile(t, appDir, "assets/styles.css", "body{}")
	writeTestFile(t, appDir, "assets/data/issues.txt", "RD-1")
	plan, err := planPackageAssets(appDir, projectManifest{
		WASM:   "bundle.wasm",
		Assets: directoryManifestAssets("./assets"),
	}, "bundle.wasm", packageOptions{compress: map[string]bool{}})
	if err != nil {
		t.Fatalf("planPackageAssets(directory) error: %v", err)
	}
	if plan.CustomIndexPath != filepath.Join(appDir, "assets", "index.html") || plan.GenerateIndex {
		t.Fatalf("index plan = custom:%q generated:%v", plan.CustomIndexPath, plan.GenerateIndex)
	}
	got := []string{}
	for _, asset := range plan.Assets {
		got = append(got, asset.LogicalName)
	}
	want := []string{"data/issues.txt", "styles.css"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("assets = %#v, want %#v", got, want)
	}
}

func TestPackageAssetPlanRejectsMissingAssetDirectory(t *testing.T) {
	appDir := t.TempDir()
	_, err := planPackageAssets(appDir, projectManifest{
		WASM:   "bundle.wasm",
		Assets: directoryManifestAssets("./assets"),
	}, "bundle.wasm", packageOptions{compress: map[string]bool{}})
	if err == nil || !strings.Contains(err.Error(), "asset directory") || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("planPackageAssets() error = %v, want missing asset directory guidance", err)
	}
}

func TestPackageAssetPlanRejectsNonRegularExplicitAsset(t *testing.T) {
	appDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(appDir, "styles.css"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := planPackageAssets(appDir, projectManifest{
		WASM:   "bundle.wasm",
		Assets: listManifestAssets([]string{"styles.css"}),
	}, "bundle.wasm", packageOptions{compress: map[string]bool{}})
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("planPackageAssets() error = %v, want non-regular asset rejection", err)
	}
}

func TestPackageAssetPlanAutoAndGeneratedIndex(t *testing.T) {
	t.Run("auto assets directory", func(t *testing.T) {
		appDir := t.TempDir()
		writeTestFile(t, appDir, "assets/styles.css", "body{}")
		plan, err := planPackageAssets(appDir, projectManifest{WASM: "bundle.wasm", Assets: autoManifestAssets()}, "bundle.wasm", packageOptions{compress: map[string]bool{}})
		if err != nil {
			t.Fatalf("planPackageAssets(auto assets dir) error: %v", err)
		}
		if !plan.GenerateIndex || len(plan.Assets) != 1 || plan.Assets[0].LogicalName != "styles.css" {
			t.Fatalf("plan = %+v, want generated index with styles.css", plan)
		}
	})

	t.Run("auto root index fallback", func(t *testing.T) {
		appDir := t.TempDir()
		writeTestFile(t, appDir, indexHTMLAssetName, "<html></html>")
		plan, err := planPackageAssets(appDir, projectManifest{WASM: "bundle.wasm", Assets: autoManifestAssets()}, "bundle.wasm", packageOptions{compress: map[string]bool{}})
		if err != nil {
			t.Fatalf("planPackageAssets(auto root index) error: %v", err)
		}
		if plan.CustomIndexPath != filepath.Join(appDir, indexHTMLAssetName) || plan.GenerateIndex {
			t.Fatalf("plan = %+v, want custom root index", plan)
		}
	})

	t.Run("auto generated index fallback", func(t *testing.T) {
		appDir := t.TempDir()
		plan, err := planPackageAssets(appDir, projectManifest{WASM: "bundle.wasm", Assets: autoManifestAssets()}, "bundle.wasm", packageOptions{compress: map[string]bool{}})
		if err != nil {
			t.Fatalf("planPackageAssets(auto generated index) error: %v", err)
		}
		if !plan.GenerateIndex || plan.CustomIndexPath != "" || len(plan.Assets) != 0 {
			t.Fatalf("plan = %+v, want generated index with no user assets", plan)
		}
	})

	t.Run("empty list generates index", func(t *testing.T) {
		appDir := t.TempDir()
		plan, err := planPackageAssets(appDir, projectManifest{WASM: "bundle.wasm", Assets: listManifestAssets([]string{})}, "bundle.wasm", packageOptions{compress: map[string]bool{}})
		if err != nil {
			t.Fatalf("planPackageAssets(empty list) error: %v", err)
		}
		if !plan.GenerateIndex || plan.CustomIndexPath != "" || len(plan.Assets) != 0 {
			t.Fatalf("plan = %+v, want generated index only", plan)
		}
	})
}

func TestPackageAssetPlanRejectsSymlinkedAssetDirectory(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	external := filepath.Join(root, "external-assets")
	if err := os.MkdirAll(external, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(appDir, assetDirectoryName)); err != nil {
		t.Fatal(err)
	}
	_, err := planPackageAssets(appDir, projectManifest{WASM: "bundle.wasm", Assets: directoryManifestAssets("assets")}, "bundle.wasm", packageOptions{compress: map[string]bool{}})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("planPackageAssets() error = %v, want symlink rejection", err)
	}
	_, err = planPackageAssets(appDir, projectManifest{WASM: "bundle.wasm", Assets: autoManifestAssets()}, "bundle.wasm", packageOptions{compress: map[string]bool{}})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("planPackageAssets(auto) error = %v, want symlink rejection", err)
	}
}

func TestPackageAssetPlanRejectsSymlinkedDirectoryAsset(t *testing.T) {
	requireSymlinkSupport(t)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(filepath.Join(appDir, assetDirectoryName), 0o755); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(root, "external.css")
	if err := os.WriteFile(external, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(appDir, assetDirectoryName, "styles.css")); err != nil {
		t.Fatal(err)
	}
	_, err := planPackageAssets(appDir, projectManifest{WASM: "bundle.wasm", Assets: directoryManifestAssets("assets")}, "bundle.wasm", packageOptions{compress: map[string]bool{}})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("planPackageAssets() error = %v, want symlink rejection", err)
	}
	assertFileContent(t, external, "keep")
}

func TestGeneratedIndexHTMLIncludesRuntimeWASMStylesAndPreload(t *testing.T) {
	html := generateIndexHTML(htmlRewriteOptions{
		preload:     true,
		wasmPath:    "assets/bundle.12345678.wasm",
		runtimePath: "assets/wasm_exec.87654321.js",
		stylePaths:  []string{"assets/app.11111111.css", "assets/theme.22222222.css"},
	})
	for _, want := range []string{
		`<div id="root">Loading...</div>`,
		`<link rel="preload" href="assets/bundle.12345678.wasm" as="fetch" type="application/wasm" crossorigin>`,
		`<link rel="preload" href="assets/wasm_exec.87654321.js" as="script">`,
		`<link rel="preload" href="assets/app.11111111.css" as="style">`,
		`<link rel="stylesheet" href="assets/app.11111111.css" />`,
		`<script src="assets/wasm_exec.87654321.js"></script>`,
		`fetch("assets/bundle.12345678.wasm")`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("generated index missing %q:\n%s", want, html)
		}
	}
}

func TestPackageGeneratesDefaultIndexForEmptyAssets(t *testing.T) {
	appDir := t.TempDir()
	writeMinimalPackageApp(t, appDir)
	if err := os.Remove(filepath.Join(appDir, indexHTMLAssetName)); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, appDir, manifestName, `{"name":"demo","compiler":"go","assets":[]}`)
	outDir := filepath.Join(t.TempDir(), "package")
	if err := packageApp(packageOptions{appDir: appDir, compiler: "go", outDir: outDir, compress: map[string]bool{}}); err != nil {
		t.Fatalf("packageApp() error: %v", err)
	}
	index, err := os.ReadFile(filepath.Join(outDir, indexHTMLAssetName))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<div id="root">Loading...</div>`, `assets/wasm_exec.js`, `fetch("assets/bundle.wasm")`} {
		if !strings.Contains(string(index), want) {
			t.Fatalf("generated index missing %q:\n%s", want, index)
		}
	}
}

func TestPackageNullAssetsUsesAutoGeneratedIndex(t *testing.T) {
	appDir := t.TempDir()
	writeMinimalPackageApp(t, appDir)
	if err := os.Remove(filepath.Join(appDir, indexHTMLAssetName)); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, appDir, manifestName, `{"name":"demo","compiler":"go","assets":null}`)
	outDir := filepath.Join(t.TempDir(), "package")
	if err := packageApp(packageOptions{appDir: appDir, compiler: "go", outDir: outDir, compress: map[string]bool{}}); err != nil {
		t.Fatalf("packageApp() error: %v", err)
	}
	index, err := os.ReadFile(filepath.Join(outDir, indexHTMLAssetName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), `<div id="root">Loading...</div>`) {
		t.Fatalf("generated index missing root mount:\n%s", index)
	}
	if ownership := inspectPackageOwnership(outDir); ownership.State != packageOwnedCurrent {
		t.Fatalf("package ownership = %v (%s), want current", ownership.State, ownership.Reason)
	}
}

func TestPackageDirectoryModePublishesAssetsRelativeToDirectory(t *testing.T) {
	appDir := t.TempDir()
	writeMinimalPackageApp(t, appDir)
	if err := os.Remove(filepath.Join(appDir, indexHTMLAssetName)); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, appDir, manifestName, `{"name":"demo","compiler":"go","assets":"./assets"}`)
	writeTestFile(t, appDir, "assets/styles.css", "body{}")
	writeTestFile(t, appDir, "assets/data/issues.txt", "RD-1")
	outDir := filepath.Join(t.TempDir(), "package")
	if err := packageApp(packageOptions{appDir: appDir, compiler: "go", outDir: outDir, preload: true, compress: map[string]bool{}}); err != nil {
		t.Fatalf("packageApp() error: %v", err)
	}
	for _, relative := range []string{
		indexHTMLAssetName,
		assetManifestName,
		packageMetadataName,
		"assets/bundle.wasm",
		"assets/wasm_exec.js",
		"assets/styles.css",
		"assets/data/issues.txt",
	} {
		if _, err := os.Stat(filepath.Join(outDir, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("packaged artifact %s missing: %v", relative, err)
		}
	}
	index, err := os.ReadFile(filepath.Join(outDir, indexHTMLAssetName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), `href="assets/styles.css"`) || !strings.Contains(string(index), `as="style"`) {
		t.Fatalf("generated index missing CSS link/preload path:\n%s", index)
	}
	var manifest assetManifest
	assetManifestContent, err := os.ReadFile(filepath.Join(outDir, assetManifestName))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(assetManifestContent, &manifest); err != nil {
		t.Fatalf("decode asset manifest: %v", err)
	}
	if manifest.Version != 1 || manifest.Entrypoints.WASM != "assets/bundle.wasm" || manifest.Entrypoints.Runtime != "assets/wasm_exec.js" {
		t.Fatalf("asset manifest = %+v, want versioned runtime/wasm entrypoints", manifest)
	}
	if strings.Join(manifest.Entrypoints.Styles, ",") != "assets/styles.css" {
		t.Fatalf("asset manifest styles = %#v, want styles.css entrypoint", manifest.Entrypoints.Styles)
	}
	var metadata packageMetadata
	metadataContent, err := os.ReadFile(filepath.Join(outDir, packageMetadataName))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(metadataContent, &metadata); err != nil {
		t.Fatalf("decode package metadata: %v", err)
	}
	if metadata.Version != 1 || metadata.Entrypoints.HTML != indexHTMLAssetName || metadata.Entrypoints.WASM != manifest.Entrypoints.WASM || metadata.Entrypoints.Runtime != manifest.Entrypoints.Runtime {
		t.Fatalf("package metadata = %+v, want versioned matching entrypoints", metadata)
	}
	if ownership := inspectPackageOwnership(outDir); ownership.State != packageOwnedCurrent {
		t.Fatalf("package ownership = %v (%s), want current", ownership.State, ownership.Reason)
	}
}

func TestPackageDirectoryModePublishesCustomIndexAtRoot(t *testing.T) {
	appDir := t.TempDir()
	writeMinimalPackageApp(t, appDir)
	if err := os.Remove(filepath.Join(appDir, indexHTMLAssetName)); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, appDir, manifestName, `{"name":"demo","compiler":"go","assets":"assets"}`)
	writeTestFile(t, appDir, "assets/index.html", `<!doctype html><title>custom</title><link rel="stylesheet" href="styles.css"><div id="root"></div><script src="wasm_exec.js"></script><script>fetch("bundle.wasm")</script>`)
	writeTestFile(t, appDir, "assets/styles.css", "body{}")
	outDir := filepath.Join(t.TempDir(), "package")
	if err := packageApp(packageOptions{appDir: appDir, compiler: "go", outDir: outDir, assetHash: true, compress: map[string]bool{}}); err != nil {
		t.Fatalf("packageApp() error: %v", err)
	}
	index, err := os.ReadFile(filepath.Join(outDir, indexHTMLAssetName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "<title>custom</title>") || !strings.Contains(string(index), `href="assets/styles.`) {
		t.Fatalf("custom index was not rewritten at package root:\n%s", index)
	}
	if _, err := os.Stat(filepath.Join(outDir, assetDirectoryName, indexHTMLAssetName)); !os.IsNotExist(err) {
		t.Fatalf("assets/index.html was copied into package assets: %v", err)
	}
}

func TestPackageRejectsInvalidCustomIndexBeforeOutputMutation(t *testing.T) {
	for _, test := range []struct {
		name     string
		manifest string
		write    func(t *testing.T, appDir string)
		want     string
	}{
		{
			name:     "declared but missing",
			manifest: `{"name":"demo","compiler":"go","assets":["index.html"]}`,
			write:    func(*testing.T, string) {},
			want:     "was not found",
		},
		{
			name:     "duplicate normalized index",
			manifest: `{"name":"demo","compiler":"go","assets":["index.html","./index.html"]}`,
			write: func(t *testing.T, appDir string) {
				writeTestFile(t, appDir, indexHTMLAssetName, "<html></html>")
			},
			want: "collides",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			appDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(appDir, manifestName), []byte(test.manifest), 0o644); err != nil {
				t.Fatal(err)
			}
			test.write(t, appDir)
			outDir := filepath.Join(t.TempDir(), "package")
			err := packageApp(packageOptions{appDir: appDir, compiler: "go", outDir: outDir, compress: map[string]bool{}})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("packageApp() error = %v, want %q", err, test.want)
			}
			if _, statErr := os.Stat(filepath.Join(appDir, defaultWorkspaceName)); !os.IsNotExist(statErr) {
				t.Fatalf("workspace was created before custom index validation: %v", statErr)
			}
			if _, statErr := os.Stat(outDir); !os.IsNotExist(statErr) {
				t.Fatalf("output was mutated before custom index validation: %v", statErr)
			}
		})
	}
}

func TestPackageInvalidIndexPreservesPreviousCompletePackage(t *testing.T) {
	appDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(appDir, manifestName), []byte(`{"name":"demo","compiler":"go","assets":["index.html"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "package")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCompleteCurrentPackage(t, outDir)
	before := map[string]string{}
	for _, relative := range []string{
		indexHTMLAssetName,
		assetManifestName,
		packageMetadataName,
		"assets/bundle.12345678.wasm",
		"assets/wasm_exec.12345678.js",
	} {
		content, err := os.ReadFile(filepath.Join(outDir, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		before[relative] = string(content)
	}

	err := packageApp(packageOptions{appDir: appDir, compiler: "go", outDir: outDir, compress: map[string]bool{}})
	if err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("packageApp() error = %v, want missing index validation", err)
	}
	for relative, content := range before {
		assertFileContent(t, filepath.Join(outDir, filepath.FromSlash(relative)), content)
	}
	if ownership := inspectPackageOwnership(outDir); ownership.State != packageOwnedCurrent {
		t.Fatalf("previous package ownership = %v (%s), want current", ownership.State, ownership.Reason)
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
	writeCompleteCurrentPackage(t, source)
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

func TestPartialPublicationDoesNotGrantOwnership(t *testing.T) {
	requireSymlinkSupport(t)

	source := t.TempDir()
	destination := t.TempDir()
	writeCompleteCurrentPackage(t, source)
	if err := os.MkdirAll(filepath.Join(destination, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external.wasm")
	if err := os.WriteFile(external, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(destination, "assets", "bundle.12345678.wasm")); err != nil {
		t.Fatal(err)
	}
	if err := publishPackageArtifacts(source, destination); err == nil {
		t.Fatal("publishPackageArtifacts() unexpectedly succeeded")
	}
	if _, err := os.Stat(filepath.Join(destination, assetManifestName)); err != nil {
		t.Fatalf("asset manifest was not copied before injected failure: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, packageMetadataName)); !os.IsNotExist(err) {
		t.Fatalf("completion metadata exists after partial publish: %v", err)
	}
	ownership := inspectPackageOwnership(destination)
	if ownership.IsOwned() {
		t.Fatalf("partial package considered owned: %v %s", ownership.State, ownership.Reason)
	}
	if err := validatePackageDestination(destination); err == nil {
		t.Fatal("validatePackageDestination() accepted partial package as reusable output")
	}
	assertFileContent(t, external, "keep")
}

func TestCleanPackageArtifactsRemovesCompletionMarkerFirst(t *testing.T) {
	destination := t.TempDir()
	writeCompleteCurrentPackage(t, destination)
	stale := filepath.Join(destination, "bundle.wasm")
	if err := os.Mkdir(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "child"), []byte("block remove"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cleanPackageArtifacts(destination, "bundle.wasm"); err == nil {
		t.Fatal("cleanPackageArtifacts() unexpectedly succeeded")
	}
	if _, err := os.Stat(filepath.Join(destination, packageMetadataName)); !os.IsNotExist(err) {
		t.Fatalf("completion marker still exists after failed cleanup: %v", err)
	}
	if ownership := inspectPackageOwnership(destination); ownership.IsOwned() {
		t.Fatalf("failed cleanup left directory owned: %v %s", ownership.State, ownership.Reason)
	}
}

func TestCleanPackageArtifactsRemovesManagedIndex(t *testing.T) {
	destination := t.TempDir()
	writeCompleteCurrentPackage(t, destination)
	if err := os.WriteFile(filepath.Join(destination, "user.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cleanPackageArtifacts(destination, "bundle.wasm"); err != nil {
		t.Fatalf("cleanPackageArtifacts() error: %v", err)
	}
	for _, name := range []string{packageMetadataName, indexHTMLAssetName, assetManifestName, assetDirectoryName} {
		if _, err := os.Stat(filepath.Join(destination, name)); !os.IsNotExist(err) {
			t.Fatalf("%s still exists after cleanup: %v", name, err)
		}
	}
	assertFileContent(t, filepath.Join(destination, "user.txt"), "keep")
}

func TestVerifyPublishedPackageInvalidatesIncompleteMarker(t *testing.T) {
	directory := t.TempDir()
	if err := os.MkdirAll(filepath.Join(directory, assetDirectoryName), 0o755); err != nil {
		t.Fatal(err)
	}
	writeValidPackageMetadata(t, directory)
	writeValidAssetManifest(t, directory)
	if err := os.WriteFile(filepath.Join(directory, "assets", "bundle.12345678.wasm"), []byte("wasm"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "assets", "wasm_exec.12345678.js"), []byte("js"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := verifyPublishedPackage(directory)
	if err == nil || !strings.Contains(err.Error(), "failed integrity verification") {
		t.Fatalf("verifyPublishedPackage() error = %v, want integrity failure", err)
	}
	if _, statErr := os.Stat(filepath.Join(directory, packageMetadataName)); !os.IsNotExist(statErr) {
		t.Fatalf("completion marker remained after failed verification: %v", statErr)
	}
	if ownership := inspectPackageOwnership(directory); ownership.IsOwned() {
		t.Fatalf("incomplete package considered owned after failed verification: %v %s", ownership.State, ownership.Reason)
	}
}

func TestVerifyPublishedPackageAcceptsCompletePackage(t *testing.T) {
	directory := t.TempDir()
	writeCompleteCurrentPackage(t, directory)
	if err := verifyPublishedPackage(directory); err != nil {
		t.Fatalf("verifyPublishedPackage() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, packageMetadataName)); err != nil {
		t.Fatalf("completion marker removed from complete package: %v", err)
	}
}

func TestPackageOutputIsImmediatelyOwnedAndExportable(t *testing.T) {
	appDir := t.TempDir()
	writeMinimalPackageApp(t, appDir)
	if err := packageApp(packageOptions{appDir: appDir, compiler: "go", compress: map[string]bool{}}); err != nil {
		t.Fatalf("packageApp() error: %v", err)
	}
	layout, err := newBuildLayout(layoutOptions{appDir: appDir, compiler: "go"})
	if err != nil {
		t.Fatal(err)
	}
	if ownership := inspectPackageOwnership(layout.PackageDir); ownership.State != packageOwnedCurrent {
		t.Fatalf("package ownership = %v (%s), want current", ownership.State, ownership.Reason)
	}
	for _, name := range []string{packageMetadataName, assetManifestName, indexHTMLAssetName} {
		if _, err := os.Stat(filepath.Join(layout.PackageDir, name)); err != nil {
			t.Fatalf("published package missing %s: %v", name, err)
		}
	}
	outDir := filepath.Join(t.TempDir(), "export")
	if err := exportApp(exportOptions{appDir: appDir, outDir: outDir}); err != nil {
		t.Fatalf("exportApp() error: %v", err)
	}
	if ownership := inspectPackageOwnership(outDir); ownership.State != packageOwnedCurrent {
		t.Fatalf("export ownership = %v (%s), want current", ownership.State, ownership.Reason)
	}
	if _, err := os.Stat(filepath.Join(outDir, indexHTMLAssetName)); err != nil {
		t.Fatalf("exported package missing index.html: %v", err)
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
