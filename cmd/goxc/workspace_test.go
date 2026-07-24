package main

import (
	"go/ast"
	"go/build"
	"go/parser"
	gotoken "go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
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

func TestGenerateIntoDirectoryCharacterizesExternalModuleComponentIdentity(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	writeExternalCardModule(t, root, "ui", "example.com/ui", "ui")
	writeExternalCardModule(t, root, "otherui", "example.com/otherui", "otherui")
	writeExternalCardModule(t, root, "ui-v2", "example.com/ui/v2", "ui")
	writeTestFile(t, appDir, "go.mod", `module example.com/app

go 1.22

require (
	example.com/otherui v0.0.0
	example.com/ui v0.0.0
	example.com/ui/v2 v2.0.0
)

replace example.com/ui => ../ui
replace example.com/otherui => ../otherui
replace example.com/ui/v2 => ../ui-v2
`)
	writeTestFile(t, appDir, "app.gox", `package main

import (
	ui "example.com/ui"
	gf "github.com/graybuton/goframe/pkg/goframe"
)

func App() gf.Node {
	return <ui.Card Title="App" />
}
`)
	writeTestFile(t, appDir, "alias.gox", `package main

import (
	widgets "example.com/ui"
	gf "github.com/graybuton/goframe/pkg/goframe"
)

func AliasView() gf.Node {
	return <widgets.Card Title="Alias" />
}
`)
	writeTestFile(t, appDir, "external_cards.gox", `package main

import (
	other "example.com/otherui"
	ui "example.com/ui"
	uiv2 "example.com/ui/v2"
	gf "github.com/graybuton/goframe/pkg/goframe"
)

func Cards() gf.Node {
	return (
		<section>
			<ui.Card Title="UI" />
			<other.Card Title="Other" />
			<uiv2.Card Title="V2" />
		</section>
	)
}
`)

	destination := t.TempDir()
	if err := generateIntoDirectory(appDir, destination, true); err != nil {
		t.Fatalf("generateIntoDirectory() error: %v", err)
	}

	assertGeneratedFileContains(t, filepath.Join(destination, "app.gox.go"),
		`gf.NewComponentType("example.com/ui.Card", "ui.Card")`,
		`gf.ComponentT(_goxComponent_app_ui_Card, ui.CardProps{`,
	)
	assertGeneratedFileContains(t, filepath.Join(destination, "alias.gox.go"),
		`gf.NewComponentType("example.com/ui.Card", "widgets.Card")`,
		`gf.ComponentT(_goxComponent_alias_widgets_Card, widgets.CardProps{`,
	)
	assertGeneratedFileContains(t, filepath.Join(destination, "external_cards.gox.go"),
		`gf.NewComponentType("example.com/ui.Card", "ui.Card")`,
		`gf.NewComponentType("example.com/otherui.Card", "other.Card")`,
		`gf.NewComponentType("example.com/ui/v2.Card", "uiv2.Card")`,
	)
	aliasContent, err := os.ReadFile(filepath.Join(destination, "alias.gox.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(aliasContent), `gf.NewComponentType("widgets.Card"`) {
		t.Fatalf("alias-generated output used the import alias as identity:\n%s", aliasContent)
	}
}

func TestPrepareBuildWorkspacePreservesExternalModuleDependencies(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	writeExternalCardModule(t, root, "ui", "example.com/ui", "ui")
	writeTestFile(t, filepath.Join(root, "ui"), "card.gox", `package ui

import gf "github.com/graybuton/goframe/pkg/goframe"

func ExternalGOX() gf.Node {
	return <Card Title="external gox" />
}
`)
	writeTestFile(t, appDir, "go.mod", `module example.com/app

go 1.22

require (
	example.com/ui v0.0.0
)

replace (
	example.com/ui => ../ui
)
`)
	writeTestFile(t, appDir, "main.go", "package main\n")
	writeTestFile(t, appDir, "app.gox", `package main

import (
	ui "example.com/ui"
	gf "github.com/graybuton/goframe/pkg/goframe"
)

func App() gf.Node {
	return <ui.Card Title="App" />
}
`)

	layout, err := newBuildLayout(layoutOptions{
		appDir:    appDir,
		workspace: filepath.Join(t.TempDir(), "workspace"),
	})
	if err != nil {
		t.Fatal(err)
	}
	entryDir, err := prepareBuildWorkspace(layout, projectManifest{Entry: "."})
	if err != nil {
		t.Fatalf("prepareBuildWorkspace() error: %v", err)
	}

	workspaceGoMod, err := os.ReadFile(filepath.Join(layout.WorkDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	workspaceGoModText := string(workspaceGoMod)
	replaceTarget := filepath.ToSlash(filepath.Join(root, "ui"))
	for _, want := range []string{
		"require (\n\texample.com/ui v0.0.0\n)",
		"replace (\n\texample.com/ui => " + replaceTarget + "\n)",
	} {
		if !strings.Contains(workspaceGoModText, want) {
			t.Fatalf("workspace go.mod missing %q:\n%s", want, workspaceGoModText)
		}
	}
	if strings.Contains(workspaceGoModText, "=> ../ui") {
		t.Fatalf("workspace go.mod preserved stale relative replace target:\n%s", workspaceGoModText)
	}
	if _, err := os.Stat(filepath.Join(root, "ui", "card.gox.go")); !os.IsNotExist(err) {
		t.Fatalf("external dependency GOX source was generated next to dependency source: %v", err)
	}
	config := workspaceModuleConfigForApp(appDir)
	appWorkDir := filepath.Join(layout.WorkDir, filepath.FromSlash(config.AppRel))
	assertGeneratedFileContains(t, filepath.Join(appWorkDir, "app.gox.go"),
		`gf.NewComponentType("example.com/ui.Card", "ui.Card")`,
	)
	if _, err := os.Stat(filepath.Join(layout.WorkDir, "ui", "card.gox.go")); !os.IsNotExist(err) {
		t.Fatalf("external dependency GOX source was materialized inside workspace: %v", err)
	}
	runGoListInWorkspace(t, entryDir)
}

func TestWriteWorkspaceGoModPreservesSingleLineExternalModuleDirectives(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	writeTestFile(t, appDir, "go.mod", `module example.com/app

go 1.22

require example.com/ui v0.0.0

replace example.com/ui => ../ui
`)
	writeTestFile(t, appDir, "go.sum", "example.com/ui v0.0.0 h1:local\n")

	workDir := t.TempDir()
	if err := writeWorkspaceGoMod(workDir, appDir); err != nil {
		t.Fatalf("writeWorkspaceGoMod() error: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(workDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{
		"require (\n\texample.com/ui v0.0.0\n)",
		"replace (\n\texample.com/ui => " + filepath.ToSlash(filepath.Join(root, "ui")) + "\n)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("workspace go.mod missing %q:\n%s", want, text)
		}
	}
	goSum, err := os.ReadFile(filepath.Join(workDir, "go.sum"))
	if err != nil {
		t.Fatalf("workspace go.sum missing: %v", err)
	}
	if string(goSum) != "example.com/ui v0.0.0 h1:local\n" {
		t.Fatalf("workspace go.sum = %q", goSum)
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

func TestGenerateIntoDirectoryAvoidsCrossFileGeneratedIdentifierCollision(t *testing.T) {
	root := newPackageIdentifierFixture(t)
	writeTestFile(t, root, "view.gox", packageIdentifierGOXSource("View", "A_B"))
	writeTestFile(t, root, "view_A.gox", packageIdentifierGOXSource("Other", "B"))

	generatePackageIdentifierFixture(t, root)
	view := generatedIdentifierForComponent(t, filepath.Join(root, "view.gox.go"), "A_B")
	other := generatedIdentifierForComponent(t, filepath.Join(root, "view_A.gox.go"), "B")
	if view == other {
		t.Fatalf("cross-file components use duplicate generated identifier %q", view)
	}
	goTestPackageIdentifierFixture(t, root)
}

func TestGenerateIntoDirectoryAvoidsSanitizedFilenameCollision(t *testing.T) {
	root := newPackageIdentifierFixture(t)
	writeTestFile(t, root, "view-a.gox", packageIdentifierGOXSource("ViewDash", "Button"))
	writeTestFile(t, root, "view_a.gox", packageIdentifierGOXSource("ViewUnderscore", "Button"))

	generatePackageIdentifierFixture(t, root)
	dash := generatedIdentifierForComponent(t, filepath.Join(root, "view-a.gox.go"), "Button")
	underscore := generatedIdentifierForComponent(t, filepath.Join(root, "view_a.gox.go"), "Button")
	if dash == underscore {
		t.Fatalf("sanitized filenames use duplicate generated identifier %q", dash)
	}
	goTestPackageIdentifierFixture(t, root)
}

func TestGenerateIntoDirectoryAvoidsCrossFileAuthoredIdentifierCollisions(t *testing.T) {
	const authoredName = "_goxComponent_view_Button"
	tests := []struct {
		name        string
		declaration string
	}{
		{name: "variable", declaration: "var " + authoredName + " = 1"},
		{name: "function", declaration: "func " + authoredName + "() {}"},
		{name: "type", declaration: "type " + authoredName + " struct{}"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newPackageIdentifierFixture(t)
			writeTestFile(t, root, "collision.go", "package app\n\n"+test.declaration+"\n")
			writeTestFile(t, root, "view.gox", packageIdentifierGOXSource("View", "Button"))

			generatePackageIdentifierFixture(t, root)
			if got := generatedIdentifierForComponent(t, filepath.Join(root, "view.gox.go"), "Button"); got == authoredName {
				t.Fatalf("generated identifier collides with authored %s %q", test.name, authoredName)
			}
			goTestPackageIdentifierFixture(t, root)
		})
	}
}

func TestGenerateIntoDirectoryAllowsSameComponentAcrossGOXFiles(t *testing.T) {
	root := newPackageIdentifierFixture(t)
	writeTestFile(t, root, "first.gox", packageIdentifierGOXSource("First", "Button"))
	writeTestFile(t, root, "second.gox", packageIdentifierGOXSource("Second", "Button"))

	generatePackageIdentifierFixture(t, root)
	first := generatedIdentifierForComponent(t, filepath.Join(root, "first.gox.go"), "Button")
	second := generatedIdentifierForComponent(t, filepath.Join(root, "second.gox.go"), "Button")
	if first == second {
		t.Fatalf("separate GOX files use duplicate generated identifier %q", first)
	}
	goTestPackageIdentifierFixture(t, root)
}

func TestGenerateIntoDirectoryPackageIdentifiersAreDeterministic(t *testing.T) {
	root := newPackageIdentifierFixture(t)
	writeTestFile(t, root, "view.gox", packageIdentifierGOXSource("View", "A_B"))
	writeTestFile(t, root, "view_A.gox", packageIdentifierGOXSource("Other", "B"))
	files, err := findGOXFiles(root)
	if err != nil {
		t.Fatal(err)
	}

	first := generatePackageIdentifierFilesInOrder(t, root, files)
	for left, right := 0, len(files)-1; left < right; left, right = left+1, right-1 {
		files[left], files[right] = files[right], files[left]
	}
	second := generatePackageIdentifierFilesInOrder(t, root, files)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("generated package changed with file order:\nfirst: %#v\nsecond: %#v", first, second)
	}
}

func TestGeneratePathRemovesManagedOutputWhenGOXBecomesInactive(t *testing.T) {
	root := newPackageIdentifierFixture(t)
	sourcePath := filepath.Join(root, "view.gox")
	writeTestFile(t, root, "view.gox", packageIdentifierGOXSource("View", "Button"))

	if err := generatePath(generateOptions{path: root}, true); err != nil {
		t.Fatalf("initial generatePath() error: %v", err)
	}
	layout, err := newBuildLayout(layoutOptions{appDir: root})
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(layout.GenDir, "view.gox.go")
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("managed output missing after active generation: %v", err)
	}
	unrelated := filepath.Join(layout.GenDir, "keep.txt")
	if err := os.WriteFile(unrelated, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	inactive := "//go:build windows && linux\n\n" +
		packageIdentifierGOXSource("View", "Button")
	if err := os.WriteFile(sourcePath, []byte(inactive), 0o644); err != nil {
		t.Fatal(err)
	}
	var generationErr error
	outputText := captureStdout(t, func() {
		generationErr = generatePath(generateOptions{path: root}, true)
	})
	err = generationErr
	if err == nil {
		t.Fatal("generatePath() succeeded with no active GOX files")
	}
	for _, want := range []string{
		"no active .gox files found below",
		build.Default.GOOS + "/" + build.Default.GOARCH,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
	if strings.Contains(outputText, "generated ") {
		t.Fatalf("inactive generation printed a success line: %q", outputText)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("inactive managed output remains: %v", err)
	}
	content, err := os.ReadFile(unrelated)
	if err != nil {
		t.Fatalf("unrelated output missing: %v", err)
	}
	if string(content) != "keep" {
		t.Fatalf("unrelated output = %q", content)
	}
}

func TestGeneratePathDoesNotRemoveUnmanagedInactiveOutput(t *testing.T) {
	root := newPackageIdentifierFixture(t)
	sourcePath := filepath.Join(root, "view.gox")
	writeTestFile(t, root, "view.gox", packageIdentifierGOXSource("View", "Button"))
	outputRoot := t.TempDir()
	options := generateOptions{path: root, outDir: outputRoot}
	if err := generatePath(options, true); err != nil {
		t.Fatalf("initial generatePath() error: %v", err)
	}
	output := filepath.Join(outputRoot, "view.gox.go")
	const authored = "package app\n\nvar userOwned = true\n"
	if err := os.WriteFile(output, []byte(authored), 0o644); err != nil {
		t.Fatal(err)
	}
	inactive := "//go:build windows && linux\n\n" +
		packageIdentifierGOXSource("View", "Button")
	if err := os.WriteFile(sourcePath, []byte(inactive), 0o644); err != nil {
		t.Fatal(err)
	}

	err := generatePath(options, true)
	if err == nil {
		t.Fatal("generatePath() removed an unmanaged inactive output")
	}
	if !strings.Contains(err.Error(), "file is not managed by goxc") {
		t.Fatalf("error %q does not identify unmanaged output", err)
	}
	content, readErr := os.ReadFile(output)
	if readErr != nil {
		t.Fatalf("unmanaged output missing: %v", readErr)
	}
	if string(content) != authored {
		t.Fatalf("unmanaged output changed:\n%s", content)
	}
}

func TestGeneratePathRemovesManagedInactiveExplicitOutput(t *testing.T) {
	root := newPackageIdentifierFixture(t)
	sourcePath := filepath.Join(root, "view.gox")
	writeTestFile(t, root, "view.gox", packageIdentifierGOXSource("View", "Button"))
	outputRoot := t.TempDir()
	options := generateOptions{path: root, outDir: outputRoot}
	if err := generatePath(options, true); err != nil {
		t.Fatalf("initial generatePath() error: %v", err)
	}
	output := filepath.Join(outputRoot, "view.gox.go")
	unrelated := filepath.Join(outputRoot, "keep.txt")
	if err := os.WriteFile(unrelated, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	inactive := "//go:build windows && linux\n\n" +
		packageIdentifierGOXSource("View", "Button")
	if err := os.WriteFile(sourcePath, []byte(inactive), 0o644); err != nil {
		t.Fatal(err)
	}

	err := generatePath(options, true)
	if err == nil || !strings.Contains(err.Error(), "no active .gox files found below") {
		t.Fatalf("generatePath() error = %v", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("inactive managed explicit output remains: %v", err)
	}
	content, err := os.ReadFile(unrelated)
	if err != nil {
		t.Fatalf("unrelated explicit output missing: %v", err)
	}
	if string(content) != "keep" {
		t.Fatalf("unrelated explicit output = %q", content)
	}
}

func newPackageIdentifierFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	writeTestFile(t, root, "go.mod", "module example.com/collision\n\ngo 1.22\n\n"+
		"require github.com/graybuton/goframe v0.0.0\n\n"+
		"replace github.com/graybuton/goframe => "+strconv.Quote(filepath.ToSlash(repositoryRoot))+"\n")
	writeTestFile(t, root, "components.go", `package app

import gf "github.com/graybuton/goframe/pkg/goframe"

type A_BProps struct{}
type BProps struct{}
type ButtonProps struct{}

func A_B(A_BProps) gf.Node { return gf.Text("A_B") }
func B(BProps) gf.Node { return gf.Text("B") }
func Button(ButtonProps) gf.Node { return gf.Text("Button") }
`)
	return root
}

func packageIdentifierGOXSource(function, component string) string {
	return "package app\n\n" +
		"import gf \"github.com/graybuton/goframe/pkg/goframe\"\n\n" +
		"func " + function + "() gf.Node {\n\treturn <" + component + " />\n}\n"
}

func generatePackageIdentifierFixture(t *testing.T, root string) {
	t.Helper()
	if err := generateIntoDirectory(root, root, true); err != nil {
		t.Fatalf("generateIntoDirectory() error: %v", err)
	}
}

func generatePackageIdentifierFilesInOrder(t *testing.T, sourceRoot string, files []string) map[string]string {
	t.Helper()
	destination := t.TempDir()
	if err := generateFilesIntoDirectory(sourceRoot, destination, files); err != nil {
		t.Fatalf("generate files: %v", err)
	}
	paths := make([]string, 0, len(files))
	for _, file := range files {
		relative, err := filepath.Rel(sourceRoot, file)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, filepath.ToSlash(relative)+".go")
	}
	sort.Strings(paths)
	outputs := make(map[string]string, len(paths))
	for _, relative := range paths {
		content, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		outputs[relative] = string(content)
	}
	return outputs
}

func generatedIdentifierForComponent(t *testing.T, path, component string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated file %s: %v", path, err)
	}
	file, err := parser.ParseFile(gotoken.NewFileSet(), path, content, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse generated file %s: %v", path, err)
	}
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok || gen.Tok != gotoken.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || len(value.Values) != 1 {
				continue
			}
			call, ok := value.Values[0].(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				continue
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "NewComponentType" {
				continue
			}
			debugName, ok := call.Args[1].(*ast.BasicLit)
			if !ok || debugName.Kind != gotoken.STRING {
				continue
			}
			name, err := strconv.Unquote(debugName.Value)
			if err != nil {
				t.Fatal(err)
			}
			if name == component {
				return value.Names[0].Name
			}
		}
	}
	t.Fatalf("generated file %s has no component declaration for %q", path, component)
	return ""
}

func goTestPackageIdentifierFixture(t *testing.T, root string) {
	t.Helper()
	command := exec.Command("go", "test", "./...")
	command.Dir = root
	command.Env = append(os.Environ(),
		"GOWORK=off",
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOFLAGS=-mod=mod -buildvcs=false",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go test package fixture: %v\n%s", err, output)
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
		"C:/abs/path",
		`C:\abs\path`,
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

	layout, err := newBuildLayout(layoutOptions{
		appDir:    appDir,
		workspace: filepath.Join(t.TempDir(), "workspace"),
	})
	if err != nil {
		t.Fatal(err)
	}
	entryDir, err := prepareBuildWorkspace(layout, projectManifest{Entry: "cmd/app"})
	if err != nil {
		t.Fatalf("prepareBuildWorkspace(child entry) error: %v", err)
	}
	config := workspaceModuleConfigForApp(appDir)
	appWorkDir := filepath.Join(layout.WorkDir, filepath.FromSlash(config.AppRel))
	wantEntry := filepath.Join(appWorkDir, "cmd", "app")
	if entryDir != wantEntry {
		t.Fatalf("entry dir = %q, want %q", entryDir, wantEntry)
	}
	for _, path := range []string{
		"cmd/app/app.gox.go",
		"internal/ui/layout.gox.go",
	} {
		if _, err := os.Stat(filepath.Join(appWorkDir, path)); err != nil {
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

	layout, err := newBuildLayout(layoutOptions{
		appDir:    appDir,
		workspace: filepath.Join(t.TempDir(), "workspace"),
	})
	if err != nil {
		t.Fatal(err)
	}
	entryDir, err := prepareBuildWorkspace(layout, projectManifest{Entry: "./src/app"})
	if err != nil {
		t.Fatalf("prepareBuildWorkspace(src entry) error: %v", err)
	}
	config := workspaceModuleConfigForApp(appDir)
	appWorkDir := filepath.Join(layout.WorkDir, filepath.FromSlash(config.AppRel))
	wantEntry := filepath.Join(appWorkDir, "src", "app")
	if entryDir != wantEntry {
		t.Fatalf("entry dir = %q, want %q", entryDir, wantEntry)
	}
	if _, err := os.Stat(filepath.Join(appWorkDir, "src/components/button.gox.go")); err != nil {
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

func writeExternalCardModule(t *testing.T, root, directory, modulePath, packageName string) {
	t.Helper()
	moduleDir := filepath.Join(root, directory)
	writeTestFile(t, moduleDir, "go.mod", "module "+modulePath+"\n\ngo 1.22\n")
	writeTestFile(t, moduleDir, "card.go", `package `+packageName+`

import gf "github.com/graybuton/goframe/pkg/goframe"

type CardProps struct {
	Title string
}

func Card(props CardProps) gf.Node {
	return gf.Text(props.Title)
}
`)
}

func assertGeneratedFileContains(t *testing.T, path string, wants ...string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated file %s: %v", path, err)
	}
	for _, want := range wants {
		if !strings.Contains(string(content), want) {
			t.Fatalf("generated file %s missing %q:\n%s", path, want, content)
		}
	}
}

func runGoListInWorkspace(t *testing.T, dir string) {
	t.Helper()
	command := exec.Command("go", "list", "-deps", ".")
	command.Dir = dir
	goFlags := strings.TrimSpace(os.Getenv("GOFLAGS") + " -mod=mod -buildvcs=false")
	command.Env = append(os.Environ(),
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOWORK=off",
		"GOFLAGS="+goFlags,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go list in workspace failed: %v\n%s", err, output)
	}
}
