//go:build !windows

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDriveLikeLogicalNameProducerConsumerRoundTrip(t *testing.T) {
	const (
		logicalName = "C:logo.svg"
		assetPath   = "assets/C:logo.svg"
		content     = "drive-like logical name\n"
	)
	appDir := t.TempDir()
	writeMinimalPackageApp(t, appDir)
	writeTestFile(t, appDir, manifestName, `{"name":"demo","compiler":"go","assets":"assets"}`)
	writeTestFile(t, appDir, "assets/index.html", "<!doctype html><div id=\"root\"></div>")
	writeTestFile(t, appDir, "assets/"+logicalName, content)

	outDir := filepath.Join(t.TempDir(), "package")
	if err := packageApp(packageOptions{
		appDir: appDir, compiler: "go", outDir: outDir, compress: map[string]bool{},
	}); err != nil {
		t.Fatalf("packageApp() error: %v", err)
	}
	if ownership := inspectPackageOwnership(outDir); ownership.State != packageOwnedCurrent {
		t.Fatalf("inspectPackageOwnership() = %+v, want current package", ownership)
	}

	before := snapshotInspectTree(t, outDir)
	var output bytes.Buffer
	if err := runInspectCommand([]string{"--dir", outDir, "--format=json"}, &output); err != nil {
		t.Fatalf("runInspectCommand() error: %v", err)
	}
	after := snapshotInspectTree(t, outDir)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("inspection mutated drive-like package\nbefore: %#v\nafter:  %#v", before, after)
	}
	var report inspectReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode inspect report: %v", err)
	}
	artifact, ok := inspectArtifactsByPath(t, report)[assetPath]
	if !ok {
		t.Fatalf("drive-like artifact %q missing from inspect report", assetPath)
	}
	if artifact.LogicalName != logicalName {
		t.Fatalf("artifact logicalName = %q, want %q", artifact.LogicalName, logicalName)
	}

	response := httptest.NewRecorder()
	staticHandler(outDir).ServeHTTP(response, httptest.NewRequest("GET", "/"+assetPath, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("serve status = %d, want 200", response.Code)
	}
	if got := response.Body.String(); got != content {
		t.Fatalf("served body = %q, want %q", got, content)
	}
}

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
