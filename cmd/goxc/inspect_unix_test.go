//go:build !windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const unixLiteralBackslashAssetName = `foo\bar.txt`

func TestNormalizeInspectPathPreservesUnixLiteralBackslash(t *testing.T) {
	declared := path.Join(assetDirectoryName, unixLiteralBackslashAssetName)
	got, err := normalizeInspectPath(declared, "declared path")
	if err != nil {
		t.Fatalf("normalizeInspectPath(%q) error: %v", declared, err)
	}
	if got != declared {
		t.Fatalf("normalizeInspectPath(%q) = %q, want literal backslash preserved", declared, got)
	}
}

func TestInspectRoundTripsPackagedUnixLiteralBackslashAsset(t *testing.T) {
	appDir := filepath.Join(t.TempDir(), "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeMinimalPackageApp(t, appDir)
	writeTestFile(t, appDir, manifestName, `{"name":"demo","compiler":"go","assets":"assets"}`)

	authoredDir := filepath.Join(appDir, assetDirectoryName)
	if err := os.MkdirAll(authoredDir, 0o755); err != nil {
		t.Fatal(err)
	}
	authoredPath := filepath.Join(authoredDir, unixLiteralBackslashAssetName)
	if err := os.WriteFile(authoredPath, []byte("literal Unix backslash\n"), 0o644); err != nil {
		t.Skipf("filesystem cannot create a literal-backslash filename: %v", err)
	}

	packageRoot := filepath.Join(t.TempDir(), "package")
	if err := packageApp(packageOptions{
		appDir:   appDir,
		compiler: "go",
		outDir:   packageRoot,
		compress: map[string]bool{},
	}); err != nil {
		t.Fatalf("packageApp() error: %v", err)
	}

	wantPath := path.Join(assetDirectoryName, unixLiteralBackslashAssetName)
	var manifest assetManifest
	readInspectJSONFixture(t, packageRoot, assetManifestName, &manifest)
	packaged, ok := manifest.Assets[unixLiteralBackslashAssetName]
	if !ok {
		t.Fatalf("packaged manifest omits logical name %q: %#v", unixLiteralBackslashAssetName, manifest.Assets)
	}
	if packaged.Path != wantPath {
		t.Fatalf("packaged path for %q = %q, want %q", unixLiteralBackslashAssetName, packaged.Path, wantPath)
	}
	physicalPath := filepath.Join(packageRoot, filepath.FromSlash(wantPath))
	if _, err := os.Stat(physicalPath); err != nil {
		t.Fatalf("stat packaged literal-backslash asset %q: %v", physicalPath, err)
	}
	t.Logf("packaged logicalName=%q path=%q physicalPath=%q", unixLiteralBackslashAssetName, packaged.Path, physicalPath)

	before := snapshotInspectTree(t, packageRoot)
	var textOutput bytes.Buffer
	if err := runInspectCommand([]string{"--dir", packageRoot}, &textOutput); err != nil {
		t.Fatalf("inspect generated Unix package as text: %v; logicalName=%q path=%q outputBytes=%d", err, unixLiteralBackslashAssetName, wantPath, textOutput.Len())
	}
	if !strings.Contains(textOutput.String(), "  - "+wantPath+"\n") {
		t.Errorf("text report does not preserve path %q: %q", wantPath, textOutput.String())
	}
	if want := fmt.Sprintf("    Logical name: %q\n", unixLiteralBackslashAssetName); !strings.Contains(textOutput.String(), want) {
		t.Errorf("text report does not preserve logical name %q: %q", unixLiteralBackslashAssetName, textOutput.String())
	}

	var jsonOutput bytes.Buffer
	if err := runInspectCommand([]string{"--dir", packageRoot, "--format=json"}, &jsonOutput); err != nil {
		t.Fatalf("inspect generated Unix package as JSON: %v; logicalName=%q path=%q outputBytes=%d", err, unixLiteralBackslashAssetName, wantPath, jsonOutput.Len())
	}
	var report inspectReport
	if err := json.Unmarshal(jsonOutput.Bytes(), &report); err != nil {
		t.Fatalf("decode inspect JSON: %v", err)
	}
	found := false
	for _, artifact := range report.Artifacts {
		if artifact.LogicalName == unixLiteralBackslashAssetName {
			found = true
			if artifact.Path != wantPath {
				t.Errorf("JSON artifact path = %q, want %q", artifact.Path, wantPath)
			}
		}
	}
	if !found {
		t.Errorf("JSON report omits logical asset %q", unixLiteralBackslashAssetName)
	}
	after := snapshotInspectTree(t, packageRoot)
	if !reflect.DeepEqual(after, before) {
		t.Errorf("inspection mutated the produced package\nbefore: %#v\nafter:  %#v", before, after)
	}
	t.Logf("inspect textBytes=%d jsonBytes=%d logicalName=%q path=%q", textOutput.Len(), jsonOutput.Len(), unixLiteralBackslashAssetName, wantPath)
}
