package main

import (
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
