//go:build !windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPackageRejectsUnixLiteralBackslashAssetNames(t *testing.T) {
	for _, logicalName := range []string{
		`foo\bar.txt`,
		`foo\..\bar.txt`,
		`styles\theme.css`,
	} {
		t.Run(logicalName, func(t *testing.T) {
			appDir := filepath.Join(t.TempDir(), "app")
			assetRoot := filepath.Join(appDir, assetDirectoryName)
			if err := os.MkdirAll(assetRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			writeMinimalPackageApp(t, appDir)
			writeTestFile(t, appDir, manifestName, `{"name":"demo","compiler":"go","assets":"assets"}`)
			if err := os.WriteFile(filepath.Join(assetRoot, logicalName), []byte("literal backslash\n"), 0o644); err != nil {
				t.Skipf("filesystem cannot create literal-backslash filename %q: %v", logicalName, err)
			}

			outDir := filepath.Join(t.TempDir(), "package")
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, outDir, "sentinel.txt", "keep\n")
			before := snapshotInspectTree(t, outDir)

			err := packageApp(packageOptions{
				appDir: appDir, compiler: "go", outDir: outDir, compress: map[string]bool{},
			})
			if err == nil || !strings.Contains(err.Error(), "must not contain backslashes") {
				t.Fatalf("packageApp(%q) error = %v, want slash-only logical-name rejection", logicalName, err)
			}
			if _, err := os.Lstat(filepath.Join(outDir, packageMetadataName)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("completion marker state = %v, want absent", err)
			}
			if _, err := os.Lstat(filepath.Join(outDir, assetManifestName)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("asset manifest state = %v, want absent", err)
			}
			if ownership := inspectPackageOwnership(outDir); ownership.State == packageOwnedCurrent {
				t.Fatalf("rejected package reported complete ownership: %+v", ownership)
			}
			after := snapshotInspectTree(t, outDir)
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("rejected package changed explicit output\nbefore: %#v\nafter:  %#v", before, after)
			}
		})
	}
}
