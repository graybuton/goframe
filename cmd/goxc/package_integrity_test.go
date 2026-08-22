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

func TestGraphGateRejectsAdversarialSourcesBeforeDestinationMutation(t *testing.T) {
	_, validPackage := writeRealPackageIntegrityFixture(t)

	tests := []struct {
		name       string
		mutate     func(*testing.T, string)
		stageFence func(*testing.T, string)
	}{
		{
			name: "missing ordinary asset",
			mutate: func(t *testing.T, root string) {
				removeIntegrityAsset(t, root, "images/logo.svg")
			},
		},
		{
			name: "missing style entrypoint",
			mutate: func(t *testing.T, root string) {
				removeIntegrityAsset(t, root, "styles/theme.css")
			},
		},
		{
			name: "missing gzip sidecar",
			mutate: func(t *testing.T, root string) {
				removeIntegritySidecar(t, root, "bundle.wasm", "gzip")
			},
		},
		{
			name: "missing Brotli sidecar",
			mutate: func(t *testing.T, root string) {
				removeIntegritySidecar(t, root, "bundle.wasm", "br")
			},
		},
		{
			name: "corrupt hashed asset",
			mutate: func(t *testing.T, root string) {
				manifest := readIntegrityManifest(t, root)
				asset := manifest.Assets["images/logo.svg"]
				file, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(asset.Path)), os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := file.WriteString("corrupt"); err != nil {
					file.Close()
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "wrong declared hash",
			mutate: func(t *testing.T, root string) {
				manifest := readIntegrityManifest(t, root)
				asset := manifest.Assets["images/logo.svg"]
				oldPath := asset.Path
				asset.Hash = "00000000"
				asset.Path = path.Join(assetDirectoryName, hashedAssetName("images/logo.svg", asset.Hash))
				renameIntegrityArtifact(t, root, oldPath, asset.Path)
				manifest.Assets["images/logo.svg"] = asset
				writeInspectJSONFixture(t, root, assetManifestName, manifest)
			},
		},
		{
			name: "symlinked declared asset",
			mutate: func(t *testing.T, root string) {
				requireSymlinkSupport(t)
				manifest := readIntegrityManifest(t, root)
				assetPath := filepath.Join(root, filepath.FromSlash(manifest.Assets["images/logo.svg"].Path))
				if err := os.Remove(assetPath); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(root, indexHTMLAssetName), assetPath); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "physical artifact alias",
			mutate: func(t *testing.T, root string) {
				manifest := readIntegrityManifest(t, root)
				image := manifest.Assets["images/logo.svg"]
				style := manifest.Assets["styles/theme.css"]
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(image.Path))); err != nil {
					t.Fatal(err)
				}
				image.Hash = style.Hash
				image.Path = path.Join(assetDirectoryName, hashedAssetName("images/logo.svg", image.Hash))
				alias := filepath.Join(root, filepath.FromSlash(image.Path))
				if err := os.MkdirAll(filepath.Dir(alias), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(filepath.Join(root, filepath.FromSlash(style.Path)), alias); err != nil {
					t.Skipf("hard links are unavailable: %v", err)
				}
				manifest.Assets["images/logo.svg"] = image
				writeInspectJSONFixture(t, root, assetManifestName, manifest)
			},
		},
		{
			name: "completion marker changes in staged export",
			stageFence: func(t *testing.T, root string) {
				writeInspectRaw(t, root, packageMetadataName, []byte("replaced completion marker\n"))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appDir, source := copyIntegrityPackageForExport(t, validPackage)
			if test.mutate != nil {
				test.mutate(t, source)
			}
			sourceBefore := snapshotInspectTree(t, source)

			outDir := filepath.Join(t.TempDir(), "export")
			writeInspectRaw(t, outDir, "sentinel.txt", []byte("keep\n"))
			destinationBefore := snapshotInspectTree(t, outDir)
			var output bytes.Buffer
			options := exportOptions{
				appDir: appDir,
				outDir: outDir,
				force:  true,
				stdout: &output,
			}
			if test.stageFence != nil {
				options.beforeStageFinalFence = func(root string) {
					test.stageFence(t, root)
				}
			}
			err := exportApp(options)
			if err == nil || !strings.Contains(err.Error(), "integrity validation") {
				t.Fatalf("exportApp() error = %v, want integrity validation failure", err)
			}
			if output.Len() != 0 {
				t.Fatalf("invalid export emitted success output: %q", output.String())
			}
			if got := snapshotInspectTree(t, outDir); !reflect.DeepEqual(got, destinationBefore) {
				t.Fatalf("invalid export mutated destination\nbefore: %#v\nafter:  %#v", destinationBefore, got)
			}
			if got := snapshotInspectTree(t, source); !reflect.DeepEqual(got, sourceBefore) {
				t.Fatalf("invalid export mutated source\nbefore: %#v\nafter:  %#v", sourceBefore, got)
			}
			if _, err := os.Lstat(filepath.Join(outDir, packageMetadataName)); !os.IsNotExist(err) {
				t.Fatalf("invalid export published a completion marker: %v", err)
			}
		})
	}
}

func TestGraphGateRejectsInvalidStageBeforePublication(t *testing.T) {
	appDir, outDir := writeRealPackageIntegrityFixture(t)
	before := snapshotInspectTree(t, outDir)

	err := packageApp(packageOptions{
		appDir:    appDir,
		compiler:  "go",
		assetHash: true,
		compress:  map[string]bool{"gzip": true, "br": true},
		beforeStageValidation: func(root string) {
			removeIntegrityAsset(t, root, "images/logo.svg")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "staged package failed integrity validation") {
		t.Fatalf("packageApp() error = %v, want staged integrity failure", err)
	}
	if got := snapshotInspectTree(t, outDir); !reflect.DeepEqual(got, before) {
		t.Fatalf("failed staged package replaced previous complete package\nbefore: %#v\nafter:  %#v", before, got)
	}
	if _, err := inspectPackageGraph(outDir); err != nil {
		t.Fatalf("previous package is no longer complete: %v", err)
	}
}

func TestGraphGatePreservesValidBytes(t *testing.T) {
	appDir, source := writeRealPackageIntegrityFixture(t)
	sourceBefore := packageFileBytes(t, source)
	var output bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "export")
	if err := exportApp(exportOptions{appDir: appDir, outDir: outDir, stdout: &output}); err != nil {
		t.Fatalf("exportApp() error: %v", err)
	}
	if got := packageFileBytes(t, source); !reflect.DeepEqual(got, sourceBefore) {
		t.Fatal("successful export mutated its source package")
	}
	if got := packageFileBytes(t, outDir); !reflect.DeepEqual(got, sourceBefore) {
		t.Fatalf("exported package bytes differ from source\nsource: %#v\nexport: %#v", sourceBefore, got)
	}
	if output.Len() == 0 {
		t.Fatal("successful export emitted no success output")
	}
	fromSource := runInspectForTest(t, []string{"--dir", source, "--format=json"})
	fromExport := runInspectForTest(t, []string{"--dir", outDir, "--format=json"})
	if fromSource != fromExport {
		t.Fatal("exported package graph differs from source")
	}
}

func TestDamagedPackageRemainsOwnedForCleanup(t *testing.T) {
	fixture := writeInspectFixture(t, filepath.Join(t.TempDir(), "package"), inspectFixtureOptions{
		hashAssets: true,
		extraAsset: true,
	})
	removeIntegrityAsset(t, fixture.root, "images/logo.svg")
	if _, err := inspectPackageGraph(fixture.root); err == nil {
		t.Fatal("strong graph validation accepted a missing ordinary asset")
	}
	ownership := inspectPackageOwnership(fixture.root)
	if ownership.State != packageOwnedCurrent {
		t.Fatalf("damaged package ownership = %v (%s), want current ownership for cleanup", ownership.State, ownership.Reason)
	}
	if err := cleanPackageArtifacts(fixture.root, "bundle.wasm"); err != nil {
		t.Fatalf("cleanPackageArtifacts() error: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(fixture.root, packageMetadataName)); !os.IsNotExist(err) {
		t.Fatalf("cleanup retained completion marker: %v", err)
	}
}

func writeRealPackageIntegrityFixture(t *testing.T) (string, string) {
	t.Helper()
	appDir := t.TempDir()
	writeMinimalPackageApp(t, appDir)
	writeTestFile(t, appDir, manifestName, `{"name":"integrity","compiler":"go","assets":["index.html","styles/theme.css","images/logo.svg"]}`)
	writeTestFile(t, appDir, "styles/theme.css", "body { color: navy; }\n")
	writeTestFile(t, appDir, "images/logo.svg", "<svg></svg>\n")
	if err := packageApp(packageOptions{
		appDir:    appDir,
		compiler:  "go",
		assetHash: true,
		compress:  map[string]bool{"gzip": true, "br": true},
	}); err != nil {
		t.Fatalf("packageApp() error: %v", err)
	}
	layout, err := newBuildLayout(layoutOptions{appDir: appDir})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inspectPackageGraph(layout.PackageDir); err != nil {
		t.Fatalf("real package failed graph validation: %v", err)
	}
	return appDir, layout.PackageDir
}

func copyIntegrityPackageForExport(t *testing.T, source string) (string, string) {
	t.Helper()
	appDir := t.TempDir()
	writeTestFile(t, appDir, manifestName, `{"name":"integrity"}`)
	layout, err := newBuildLayout(layoutOptions{appDir: appDir})
	if err != nil {
		t.Fatal(err)
	}
	copyInspectTree(t, source, layout.PackageDir)
	return appDir, layout.PackageDir
}

func readIntegrityManifest(t *testing.T, root string) assetManifest {
	t.Helper()
	var manifest assetManifest
	readInspectJSONFixture(t, root, assetManifestName, &manifest)
	return manifest
}

func removeIntegrityAsset(t *testing.T, root, logicalName string) {
	t.Helper()
	manifest := readIntegrityManifest(t, root)
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(manifest.Assets[logicalName].Path))); err != nil {
		t.Fatal(err)
	}
}

func removeIntegritySidecar(t *testing.T, root, logicalName, encoding string) {
	t.Helper()
	manifest := readIntegrityManifest(t, root)
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(manifest.Assets[logicalName].Compressed[encoding]))); err != nil {
		t.Fatal(err)
	}
}

func renameIntegrityArtifact(t *testing.T, root, oldPath, newPath string) {
	t.Helper()
	destination := filepath.Join(root, filepath.FromSlash(newPath))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, filepath.FromSlash(oldPath)), destination); err != nil {
		t.Fatal(err)
	}
}

func packageFileBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	if err := filepath.WalkDir(root, func(file string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = string(content)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result
}
