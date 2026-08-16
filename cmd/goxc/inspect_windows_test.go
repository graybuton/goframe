//go:build windows

package main

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeInspectPathRejectsWindowsSeparator(t *testing.T) {
	const declared = `assets\bundle.wasm`
	got, err := normalizeInspectPath(declared, "declared path")
	if err == nil {
		t.Fatalf("normalizeInspectPath(%q) = %q, want canonical-form rejection", declared, got)
	}
	if !strings.Contains(err.Error(), `must use canonical form "assets/bundle.wasm"`) {
		t.Fatalf("normalizeInspectPath(%q) error = %v, want slash-separated canonical form", declared, err)
	}
}

func TestInspectRejectsCaseOnlyPhysicalArtifactAlias(t *testing.T) {
	fixture := writeInspectFixture(t, filepath.Join(t.TempDir(), "package"), inspectFixtureOptions{})
	const aliasLogicalName = "WASM_EXEC.js"
	aliasPath := path.Join(assetDirectoryName, aliasLogicalName)
	runtimePath := fixture.assetPaths[runtimeAssetName]
	if aliasLogicalName == runtimeAssetName {
		t.Fatal("case-only alias logical name matches the runtime logical name")
	}
	if aliasPath == runtimePath {
		t.Fatal("case-only alias path matches the reported runtime path")
	}
	if want := path.Join(assetDirectoryName, aliasLogicalName); aliasPath != want {
		t.Fatalf("case-only alias path = %q, want package path %q", aliasPath, want)
	}

	// Both declarations satisfy the package-path formula, while Windows resolves
	// their case-only paths to one physical runtime file.
	mutateInspectAssetManifest(t, fixture.root, func(manifest *assetManifest) {
		manifest.Assets[aliasLogicalName] = packageAsset{
			Path: aliasPath,
			Type: "text/javascript",
		}
	})
	actualRuntimePath := filepath.Join(fixture.root, filepath.FromSlash(runtimePath))
	aliasFilesystemPath := filepath.Join(fixture.root, filepath.FromSlash(aliasPath))
	actualInfo, err := os.Stat(actualRuntimePath)
	if err != nil {
		t.Fatal(err)
	}
	aliasInfo, err := os.Stat(aliasFilesystemPath)
	if os.IsNotExist(err) {
		t.Skip("filesystem resolves case-different paths as distinct")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(actualInfo, aliasInfo) {
		t.Skip("filesystem resolves case-different paths as distinct")
	}

	before := snapshotInspectTree(t, fixture.root)
	var output bytes.Buffer
	err = runInspectCommand([]string{"--dir", fixture.root, "--format=json"}, &output)
	after := snapshotInspectTree(t, fixture.root)
	if err == nil || !strings.Contains(err.Error(), "physical alias") {
		t.Fatalf("runInspectCommand() error = %v, want physical alias rejection", err)
	}
	if output.Len() != 0 {
		t.Fatalf("physical alias emitted partial output: %q", output.String())
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("physical alias inspection mutated filesystem\nbefore: %#v\nafter:  %#v", before, after)
	}
}
