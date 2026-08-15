//go:build windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInspectRejectsCaseOnlyPhysicalArtifactAlias(t *testing.T) {
	fixture := writeInspectFixture(t, filepath.Join(t.TempDir(), "package"), inspectFixtureOptions{})
	mutateInspectAssetManifest(t, fixture.root, func(manifest *assetManifest) {
		manifest.Assets["runtime-alias.js"] = packageAsset{
			Path: "assets/WASM_EXEC.js",
			Type: "text/javascript",
		}
	})
	actualInfo, err := os.Stat(filepath.Join(fixture.root, filepath.FromSlash(fixture.assetPaths[runtimeAssetName])))
	if err != nil {
		t.Fatal(err)
	}
	aliasInfo, err := os.Stat(filepath.Join(fixture.root, filepath.FromSlash("assets/WASM_EXEC.js")))
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
