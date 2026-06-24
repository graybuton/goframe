package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteWorkspaceGoModFailsWithoutRepoRootOrVersion(t *testing.T) {
	oldFind := findRepositoryRootForWorkspace
	oldVersion := goframeModuleVersionForBuild
	findRepositoryRootForWorkspace = func(string) (string, bool) { return "", false }
	goframeModuleVersionForBuild = func() string { return "v0.0.0" }
	defer func() {
		findRepositoryRootForWorkspace = oldFind
		goframeModuleVersionForBuild = oldVersion
	}()

	err := writeWorkspaceGoMod(t.TempDir(), t.TempDir())
	if err == nil {
		t.Fatal("writeWorkspaceGoMod() returned nil error")
	}
	for _, want := range []string{"repository root was not found", "install a released goxc", "GOFRAME_WORKSPACE"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err, want)
		}
	}
}

func TestWorkspaceModulePathUsesNearestModuleAndAppRelativePath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/example/root\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(root, "apps", "demo")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := workspaceModulePath(appDir); got != "github.com/example/root/apps/demo" {
		t.Fatalf("workspaceModulePath() = %q", got)
	}
}

func TestPackageIdentityForFileUsesPackageDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/example/root\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(root, "apps", "demo")
	packageDir := filepath.Join(appDir, "internal", "ui")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(packageDir, "header.gox")
	if got := packageIdentityForFile(appDir, file); got != "github.com/example/root/apps/demo/internal/ui" {
		t.Fatalf("packageIdentityForFile() = %q", got)
	}
}

func TestFindGOXFilesSkipsToolOwnedDirectories(t *testing.T) {
	appDir := t.TempDir()
	for _, path := range []string{
		"app.gox",
		"internal/ui/layout.gox",
		".goframe/gen/stale.gox",
		"dist/old.gox",
		"build/old.gox",
		"node_modules/pkg/old.gox",
	} {
		full := filepath.Join(appDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("package main\nfunc App() any { return <div></div> }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := findGOXFiles(appDir)
	if err != nil {
		t.Fatalf("findGOXFiles() error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("findGOXFiles() found %d files, want 2: %#v", len(files), files)
	}
	for _, file := range files {
		relative, err := filepath.Rel(appDir, file)
		if err != nil {
			t.Fatal(err)
		}
		relative = filepath.ToSlash(relative)
		if relative != "app.gox" && relative != "internal/ui/layout.gox" {
			t.Fatalf("unexpected GOX file %q", relative)
		}
	}
}

func TestGenerateIntoDirectoryPreservesPackageStructureAndIdentity(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/example/root\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(root, "apps", "demo")
	sourceDir := filepath.Join(appDir, "internal", "ui")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := `package ui

import gf "github.com/graybuton/goframe/pkg/goframe"

func View() gf.Node {
	return <Header />
}
`
	if err := os.WriteFile(filepath.Join(sourceDir, "layout.gox"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	if err := generateIntoDirectory(appDir, destination, true); err != nil {
		t.Fatalf("generateIntoDirectory() error: %v", err)
	}
	output := filepath.Join(destination, "internal", "ui", "layout.gox.go")
	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("generated file missing: %v", err)
	}
	if !strings.Contains(string(content), `gf.NewComponentType("github.com/example/root/apps/demo/internal/ui.Header", "Header")`) {
		t.Fatalf("generated file missing import-aware identity:\n%s", content)
	}
}

func TestGenerateIntoDirectorySupportsQualifiedComponentIdentity(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/example/root\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(root, "apps", "demo")
	writeTestFile(t, appDir, "app.gox", `package main

import (
	ui "github.com/example/root/apps/demo/internal/ui"
	gf "github.com/graybuton/goframe/pkg/goframe"
)

func App() gf.Node {
	return <ui.Layout Title="Demo" />
}
`)
	writeTestFile(t, appDir, "internal/ui/layout.gox", `package ui

import gf "github.com/graybuton/goframe/pkg/goframe"

type LayoutProps struct {
	Title string
}

func Layout(props LayoutProps) gf.Node {
	return <section>{props.Title}</section>
}
`)

	destination := t.TempDir()
	if err := generateIntoDirectory(appDir, destination, true); err != nil {
		t.Fatalf("generateIntoDirectory() error: %v", err)
	}
	output := filepath.Join(destination, "app.gox.go")
	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("generated file missing: %v", err)
	}
	for _, want := range []string{
		`gf.NewComponentType("github.com/example/root/apps/demo/internal/ui.Layout", "ui.Layout")`,
		`gf.ComponentT(_goxComponent_app_ui_Layout, ui.LayoutProps{`,
		`}, ui.Layout)`,
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("generated app.gox.go missing %q:\n%s", want, content)
		}
	}
	if _, err := os.Stat(filepath.Join(appDir, "app.gox.go")); !os.IsNotExist(err) {
		t.Fatalf("source tree generated file exists after generateIntoDirectory: %v", err)
	}
}

func TestGenerateIntoDirectoryReportsOriginalGOXSource(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/example/root\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(root, "apps", "demo")
	sourceDir := filepath.Join(appDir, "internal", "ui")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(sourceDir, "layout.gox")
	source := `package ui

import gf "github.com/graybuton/goframe/pkg/goframe"

func View() gf.Node {
	return <main><p>Broken</main>
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	err := generateIntoDirectory(appDir, t.TempDir(), true)
	if err == nil {
		t.Fatal("generateIntoDirectory() returned nil error")
	}
	for _, want := range []string{
		"generate failed for " + sourcePath,
		sourcePath + ":6:",
		"expected closing tag </p>, got </main>",
		"<p>Broken</main>",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
}

func TestCleanManifestEntryAcceptsChildEntries(t *testing.T) {
	tests := map[string]string{
		".":         ".",
		"./cmd/app": "cmd/app",
		"cmd/app":   "cmd/app",
		"./src/app": "src/app",
		"src/app":   "src/app",
		"./app":     "app",
		"app":       "app",
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			got, err := cleanManifestEntry(input)
			if err != nil {
				t.Fatalf("cleanManifestEntry(%q) error: %v", input, err)
			}
			if got != want {
				t.Fatalf("cleanManifestEntry(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestCleanManifestEntryRejectsUnsafeEntries(t *testing.T) {
	tests := []string{
		"",
		"/abs/path",
		"../outside",
		"./../outside",
		"cmd/../outside",
		".goframe/work",
		"./.goframe/work",
		"build",
		"dist",
		"node_modules",
		".git",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := cleanManifestEntry(input); err == nil {
				t.Fatalf("cleanManifestEntry(%q) returned nil error", input)
			}
		})
	}
}

func TestResolveEntryPackageDirRejectsFile(t *testing.T) {
	appDir := t.TempDir()
	writeTestFile(t, appDir, "cmd/app/main.go", "package main\n")
	if _, err := resolveEntryPackageDir(appDir, "cmd/app/main.go"); err == nil {
		t.Fatal("resolveEntryPackageDir() accepted a file entry")
	}
}

func TestPrepareBuildWorkspaceAcceptsChildEntry(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module github.com/example/root\n\ngo 1.22\n")
	appDir := filepath.Join(root, "apps", "demo")
	writeTestFile(t, appDir, "cmd/app/main.go", "package main\n")
	writeTestFile(t, appDir, "cmd/app/app.gox", `package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func AppShell() gf.Node {
	return <main>Demo</main>
}
`)
	writeTestFile(t, appDir, "internal/ui/layout.gox", `package ui

import gf "github.com/graybuton/goframe/pkg/goframe"

func Layout() gf.Node {
	return <section>UI</section>
}
`)

	layout := BuildLayout{
		AppDir:  appDir,
		WorkDir: filepath.Join(t.TempDir(), "work"),
	}
	entryDir, err := prepareBuildWorkspace(layout, projectManifest{Entry: "cmd/app"})
	if err != nil {
		t.Fatalf("prepareBuildWorkspace(child entry) error: %v", err)
	}
	wantEntry := filepath.Join(layout.WorkDir, "apps", "demo", "cmd", "app")
	if entryDir != wantEntry {
		t.Fatalf("entry dir = %q, want %q", entryDir, wantEntry)
	}
	for _, path := range []string{
		"apps/demo/cmd/app/app.gox.go",
		"apps/demo/internal/ui/layout.gox.go",
	} {
		if _, err := os.Stat(filepath.Join(layout.WorkDir, path)); err != nil {
			t.Fatalf("workspace generated file %s missing: %v", path, err)
		}
	}
}

func TestPrepareBuildWorkspaceAcceptsGenericSrcEntry(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module github.com/example/root\n\ngo 1.22\n")
	appDir := filepath.Join(root, "apps", "demo")
	writeTestFile(t, appDir, "src/app/main.go", "package main\n")
	writeTestFile(t, appDir, "src/app/app.gox", `package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func App() gf.Node {
	return <main>src app</main>
}
`)
	writeTestFile(t, appDir, "src/components/button.gox", `package components

import gf "github.com/graybuton/goframe/pkg/goframe"

func Button() gf.Node {
	return <button>Button</button>
}
`)

	layout := BuildLayout{
		AppDir:  appDir,
		WorkDir: filepath.Join(t.TempDir(), "work"),
	}
	entryDir, err := prepareBuildWorkspace(layout, projectManifest{Entry: "./src/app"})
	if err != nil {
		t.Fatalf("prepareBuildWorkspace(src entry) error: %v", err)
	}
	wantEntry := filepath.Join(layout.WorkDir, "apps", "demo", "src", "app")
	if entryDir != wantEntry {
		t.Fatalf("entry dir = %q, want %q", entryDir, wantEntry)
	}
	if _, err := os.Stat(filepath.Join(layout.WorkDir, "apps/demo/src/components/button.gox.go")); err != nil {
		t.Fatalf("workspace did not generate non-entry package GOX file: %v", err)
	}
}

func TestWriteWorkspaceGoModUsesRootModulePath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/example/root\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(root, "apps", "demo")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldFind := findRepositoryRootForWorkspace
	findRepositoryRootForWorkspace = func(string) (string, bool) { return root, true }
	defer func() { findRepositoryRootForWorkspace = oldFind }()

	workDir := t.TempDir()
	if err := writeWorkspaceGoMod(workDir, appDir); err != nil {
		t.Fatalf("writeWorkspaceGoMod() error: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(workDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"module github.com/example/root",
		"require " + canonicalModulePath + " v0.0.0",
		"replace " + canonicalModulePath + " => " + filepath.ToSlash(root),
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("workspace go.mod missing %q:\n%s", want, content)
		}
	}
}

func writeTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
