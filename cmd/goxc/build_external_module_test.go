package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalModuleEnvironmentIsolation(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	uiDir := filepath.Join(root, "ui")

	writeExternalCardModule(t, root, "ui", "example.com/ui", "ui")
	writeTestFile(t, uiDir, "card.gox", `package ui

import gf "github.com/graybuton/goframe/pkg/goframe"

func ExternalGOX() gf.Node {
	return <Card Title="external gox" />
}
`)
	writeTestFile(t, appDir, "go.mod", `module example.com/app

go 1.22

require example.com/ui v0.0.0

replace example.com/ui => ../ui
`)
	writeTestFile(t, appDir, "main.go", `package main

func main() {}
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

	workPath, workContent := writeHostileParentWorkspace(t, root)
	setHostileCompilerWorkflowEnvironment(t, workPath, "-mod=vendor")

	workspaceBase := filepath.Join(t.TempDir(), "workspace")
	outputPath, err := buildApp(buildOptions{
		appDir:    appDir,
		compiler:  "go",
		workspace: workspaceBase,
	})
	if err != nil {
		t.Fatalf("buildApp() error: %v", err)
	}
	outputInfo, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("build output missing: %v", err)
	}
	if !outputInfo.Mode().IsRegular() || outputInfo.Size() <= 0 {
		t.Fatalf("build output is not a non-empty regular file: mode=%s size=%d", outputInfo.Mode(), outputInfo.Size())
	}

	layout, err := newBuildLayout(layoutOptions{
		appDir:    appDir,
		compiler:  "go",
		workspace: workspaceBase,
	})
	if err != nil {
		t.Fatal(err)
	}
	config := workspaceModuleConfigForApp(appDir)
	appWorkDir := filepath.Join(layout.WorkDir, filepath.FromSlash(config.AppRel))
	assertGeneratedFileContains(t, filepath.Join(appWorkDir, "app.gox.go"),
		`gf.NewComponentType("example.com/ui.Card", "ui.Card")`,
	)

	workspaceGoMod, err := os.ReadFile(filepath.Join(layout.WorkDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	workspaceGoModText := string(workspaceGoMod)
	replaceTarget := filepath.ToSlash(uiDir)
	for _, want := range []string{
		"example.com/ui v0.0.0",
		"example.com/ui => " + replaceTarget,
	} {
		if !strings.Contains(workspaceGoModText, want) {
			t.Fatalf("workspace go.mod missing %q:\n%s", want, workspaceGoModText)
		}
	}
	if strings.Contains(workspaceGoModText, "=> ../ui") {
		t.Fatalf("workspace go.mod preserved stale relative replace target:\n%s", workspaceGoModText)
	}

	if _, err := os.Stat(filepath.Join(uiDir, "card.gox.go")); !os.IsNotExist(err) {
		t.Fatalf("external dependency GOX source was generated next to dependency source: %v", err)
	}
	assertWorkspaceDoesNotContainGeneratedExternalGOX(t, layout.WorkDir)
	assertTestFileUnchanged(t, workPath, workContent)
}

func assertWorkspaceDoesNotContainGeneratedExternalGOX(t *testing.T, workDir string) {
	t.Helper()
	err := filepath.WalkDir(workDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Base(path) == "card.gox.go" {
			t.Fatalf("external dependency GOX source was materialized inside workspace at %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk workspace for external GOX output: %v", err)
	}
}
