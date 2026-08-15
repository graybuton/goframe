package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

type inspectFixtureOptions struct {
	hashAssets bool
	styles     bool
	compressed bool
	extraAsset bool
}

type inspectFixture struct {
	root       string
	assetPaths map[string]string
}

func TestParseInspectOptions(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		want      inspectOptions
		wantError string
	}{
		{name: "default text", args: []string{"app"}, want: inspectOptions{path: "app", format: inspectFormatText}},
		{name: "explicit text equals", args: []string{"app", "--format=text"}, want: inspectOptions{path: "app", format: inspectFormatText}},
		{name: "JSON equals", args: []string{"app", "--format=json"}, want: inspectOptions{path: "app", format: inspectFormatJSON}},
		{name: "JSON split", args: []string{"--format", "json", "app"}, want: inspectOptions{path: "app", format: inspectFormatJSON}},
		{name: "workspace equals", args: []string{"app", "--workspace=work"}, want: inspectOptions{path: "app", workspace: "work", format: inspectFormatText}},
		{name: "workspace split", args: []string{"--workspace", "work", "app"}, want: inspectOptions{path: "app", workspace: "work", format: inspectFormatText}},
		{name: "directory equals", args: []string{"--dir=package"}, want: inspectOptions{dir: "package", format: inspectFormatText}},
		{name: "directory split", args: []string{"--dir", "package", "--format", "json"}, want: inspectOptions{dir: "package", format: inspectFormatJSON}},
		{name: "missing input", wantError: "usage: goxc inspect"},
		{name: "unknown flag", args: []string{"app", "--verify"}, wantError: "unknown inspect flag"},
		{name: "unsupported format", args: []string{"app", "--format=yaml"}, wantError: "unsupported inspect format"},
		{name: "format missing value", args: []string{"app", "--format"}, wantError: "--format requires a value"},
		{name: "workspace missing value", args: []string{"app", "--workspace"}, wantError: "--workspace requires a value"},
		{name: "directory missing value", args: []string{"--dir"}, wantError: "--dir requires a value"},
		{name: "multiple positional inputs", args: []string{"one", "two"}, wantError: "multiple inspect input paths"},
		{name: "directory and positional", args: []string{"app", "--dir=package"}, wantError: "--dir cannot be combined with a positional path"},
		{name: "directory and workspace", args: []string{"--dir=package", "--workspace=work"}, wantError: "--dir cannot be combined with --workspace"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseInspectOptions(test.args)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("parseInspectOptions(%#v) error = %v, want %q", test.args, err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseInspectOptions(%#v) error: %v", test.args, err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("parseInspectOptions(%#v) = %+v, want %+v", test.args, got, test.want)
			}
		})
	}
}

func TestInspectMinimalCurrentPackage(t *testing.T) {
	fixture := writeInspectFixture(t, t.TempDir(), inspectFixtureOptions{})
	report, err := inspectPackageGraph(fixture.root)
	if err != nil {
		t.Fatalf("inspectPackageGraph() error: %v", err)
	}
	if report.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d, want 1", report.SchemaVersion)
	}
	if report.Package.Name != "demo" || report.Package.Compiler != "tinygo" || report.Package.ToolchainVersion != "devel" {
		t.Fatalf("package = %+v", report.Package)
	}
	if report.Entrypoints.HTML != indexHTMLAssetName || report.Entrypoints.WASM != fixture.assetPaths["bundle.wasm"] || report.Entrypoints.Runtime != fixture.assetPaths[runtimeAssetName] {
		t.Fatalf("entrypoints = %+v", report.Entrypoints)
	}
	if report.Entrypoints.Styles == nil || len(report.Entrypoints.Styles) != 0 {
		t.Fatalf("styles = %#v, want non-nil empty slice", report.Entrypoints.Styles)
	}
	if report.Summary.ArtifactCount != 5 || report.Summary.EdgeCount != 2 {
		t.Fatalf("summary = %+v, want five artifacts and two edges", report.Summary)
	}

	artifacts := inspectArtifactsByPath(t, report)
	for path, roles := range map[string][]string{
		packageMetadataName:               {"package-metadata"},
		assetManifestName:                 {"asset-metadata"},
		indexHTMLAssetName:                {"html-entrypoint"},
		fixture.assetPaths["bundle.wasm"]: {"asset", "wasm-entrypoint"},
		fixture.assetPaths[runtimeAssetName]: {
			"asset", "runtime-entrypoint",
		},
	} {
		artifact, ok := artifacts[path]
		if !ok {
			t.Fatalf("artifact %q missing from %#v", path, report.Artifacts)
		}
		if !reflect.DeepEqual(artifact.Roles, roles) {
			t.Fatalf("artifact %q roles = %#v, want %#v", path, artifact.Roles, roles)
		}
		wantBytes, wantHash := inspectFileEvidence(t, fixture.root, path)
		if artifact.Bytes != wantBytes || artifact.SHA256 != wantHash {
			t.Fatalf("artifact %q evidence = %d %q, want %d %q", path, artifact.Bytes, artifact.SHA256, wantBytes, wantHash)
		}
	}

	var total int64
	for _, artifact := range report.Artifacts {
		total += artifact.Bytes
	}
	if report.Summary.TotalBytes != total {
		t.Fatalf("totalBytes = %d, want %d", report.Summary.TotalBytes, total)
	}

	var output bytes.Buffer
	if err := runInspectCommand([]string{"--dir", fixture.root, "--format=json"}, &output); err != nil {
		t.Fatalf("runInspectCommand(JSON) error: %v", err)
	}
	if !bytes.HasSuffix(output.Bytes(), []byte("\n")) {
		t.Fatalf("JSON does not end in one newline: %q", output.Bytes())
	}
	if strings.Contains(output.String(), "null") || strings.Contains(output.String(), fixture.root) || strings.Contains(output.String(), "generatedAt") {
		t.Fatalf("JSON contains unstable or null data:\n%s", output.String())
	}
	var decoded inspectReport
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("decode inspect JSON: %v", err)
	}
	if !reflect.DeepEqual(decoded, report) {
		t.Fatalf("decoded JSON report differs\ngot:  %+v\nwant: %+v", decoded, report)
	}
	var raw struct {
		Artifacts []map[string]json.RawMessage `json:"artifacts"`
	}
	if err := json.Unmarshal(output.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for index, artifact := range raw.Artifacts {
		for _, field := range []string{"path", "logicalName", "mediaType", "bytes", "sha256", "declaredHash", "encoding", "roles"} {
			if _, ok := artifact[field]; !ok {
				t.Fatalf("artifact %d omits required field %q: %s", index, field, output.String())
			}
		}
	}
}

func TestInspectTextReportGolden(t *testing.T) {
	fixture := writeInspectFixture(t, t.TempDir(), inspectFixtureOptions{})
	report, err := inspectPackageGraph(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := writeInspectText(&output, report); err != nil {
		t.Fatalf("writeInspectText() error: %v", err)
	}
	artifacts := inspectArtifactsByPath(t, report)
	bundle := fixture.assetPaths["bundle.wasm"]
	runtime := fixture.assetPaths[runtimeAssetName]
	want := fmt.Sprintf(`Package
  Name: demo
  Compiler: tinygo
  Toolchain version: devel
  Hash assets: false
  Preload: false

Entrypoints
  HTML: index.html
  WASM: %s
  Runtime: %s
  Styles: (none)

Artifacts
  - asset-manifest.json
    Logical name: ""
    Media type: application/json
    Bytes: %d
    SHA-256: %s
    Declared hash: ""
    Encoding: ""
    Roles: asset-metadata
  - %s
    Logical name: "bundle.wasm"
    Media type: application/wasm
    Bytes: %d
    SHA-256: %s
    Declared hash: ""
    Encoding: ""
    Roles: asset, wasm-entrypoint
  - %s
    Logical name: "wasm_exec.js"
    Media type: text/javascript
    Bytes: %d
    SHA-256: %s
    Declared hash: ""
    Encoding: ""
    Roles: asset, runtime-entrypoint
  - goframe-package.json
    Logical name: ""
    Media type: application/json
    Bytes: %d
    SHA-256: %s
    Declared hash: ""
    Encoding: ""
    Roles: package-metadata
  - index.html
    Logical name: ""
    Media type: text/html; charset=utf-8
    Bytes: %d
    SHA-256: %s
    Declared hash: ""
    Encoding: ""
    Roles: html-entrypoint

Edges
  - index.html -> %s
    Kind: runtime-entrypoint
    Encoding: ""
  - index.html -> %s
    Kind: wasm-entrypoint
    Encoding: ""

Summary
  Artifact count: 5
  Edge count: 2
  Total bytes: %d
`,
		bundle,
		runtime,
		artifacts[assetManifestName].Bytes,
		artifacts[assetManifestName].SHA256,
		bundle,
		artifacts[bundle].Bytes,
		artifacts[bundle].SHA256,
		runtime,
		artifacts[runtime].Bytes,
		artifacts[runtime].SHA256,
		artifacts[packageMetadataName].Bytes,
		artifacts[packageMetadataName].SHA256,
		artifacts[indexHTMLAssetName].Bytes,
		artifacts[indexHTMLAssetName].SHA256,
		runtime,
		bundle,
		report.Summary.TotalBytes,
	)
	if output.String() != want {
		t.Fatalf("text report mismatch\n--- got ---\n%s--- want ---\n%s", output.String(), want)
	}
	for _, section := range []string{"Package\n", "Entrypoints\n", "Artifacts\n", "Edges\n", "Summary\n"} {
		if !strings.Contains(output.String(), section) {
			t.Fatalf("text report missing %q", section)
		}
	}
	if strings.Contains(output.String(), fixture.root) {
		t.Fatalf("text report contains absolute root:\n%s", output.String())
	}
}

func TestInspectStylesHashesAndCompression(t *testing.T) {
	fixture := writeInspectFixture(t, t.TempDir(), inspectFixtureOptions{
		hashAssets: true,
		styles:     true,
		compressed: true,
		extraAsset: true,
	})
	report, err := inspectPackageGraph(fixture.root)
	if err != nil {
		t.Fatalf("inspectPackageGraph() error: %v", err)
	}
	wantStyles := []string{fixture.assetPaths["styles/a.css"], fixture.assetPaths["styles/z.css"]}
	if !reflect.DeepEqual(report.Entrypoints.Styles, wantStyles) {
		t.Fatalf("styles = %#v, want %#v", report.Entrypoints.Styles, wantStyles)
	}
	seen := map[string]bool{}
	var total int64
	for _, artifact := range report.Artifacts {
		if seen[artifact.Path] {
			t.Fatalf("duplicate artifact path %q", artifact.Path)
		}
		seen[artifact.Path] = true
		total += artifact.Bytes
		if !sort.StringsAreSorted(artifact.Roles) {
			t.Fatalf("roles for %q are not sorted: %#v", artifact.Path, artifact.Roles)
		}
		if hasInspectRole(artifact.Roles, "asset") {
			if len(artifact.DeclaredHash) != packageHashLength {
				t.Fatalf("asset %q declared hash = %q", artifact.Path, artifact.DeclaredHash)
			}
			if artifact.DeclaredHash != artifact.SHA256[:packageHashLength] {
				t.Fatalf("asset %q declared hash %q does not match %q", artifact.Path, artifact.DeclaredHash, artifact.SHA256)
			}
		}
	}
	if report.Summary.TotalBytes != total || report.Summary.ArtifactCount != len(report.Artifacts) || report.Summary.EdgeCount != len(report.Edges) {
		t.Fatalf("summary = %+v, artifacts=%d edges=%d total=%d", report.Summary, len(report.Artifacts), len(report.Edges), total)
	}

	artifacts := inspectArtifactsByPath(t, report)
	for _, logical := range []string{"bundle.wasm", runtimeAssetName, "styles/a.css", "styles/z.css"} {
		parent := fixture.assetPaths[logical]
		for _, encoding := range []string{"br", "gzip"} {
			sidecar := parent + map[string]string{"br": ".br", "gzip": ".gz"}[encoding]
			artifact, ok := artifacts[sidecar]
			if !ok {
				t.Fatalf("compressed artifact %q missing", sidecar)
			}
			if artifact.LogicalName != logical || artifact.Encoding != encoding || !reflect.DeepEqual(artifact.Roles, []string{"compressed"}) {
				t.Fatalf("compressed artifact %q = %+v", sidecar, artifact)
			}
			if !inspectEdgeExists(report.Edges, parent, sidecar, "compressed", encoding) {
				t.Fatalf("compressed edge %s -> %s (%s) missing", parent, sidecar, encoding)
			}
		}
	}
	for _, style := range wantStyles {
		if !hasInspectRole(artifacts[style].Roles, "style-entrypoint") || !inspectEdgeExists(report.Edges, indexHTMLAssetName, style, "style-entrypoint", "") {
			t.Fatalf("style graph incomplete for %q", style)
		}
	}
	if !sort.SliceIsSorted(report.Artifacts, func(i, j int) bool { return report.Artifacts[i].Path < report.Artifacts[j].Path }) {
		t.Fatalf("artifacts are not sorted: %#v", report.Artifacts)
	}
	if !sort.SliceIsSorted(report.Edges, func(i, j int) bool { return inspectEdgeLess(report.Edges[i], report.Edges[j]) }) {
		t.Fatalf("edges are not sorted: %#v", report.Edges)
	}
}

func TestInspectOutputIsPathIndependentAndRepeatable(t *testing.T) {
	first := writeInspectFixture(t, filepath.Join(t.TempDir(), "one"), inspectFixtureOptions{hashAssets: true, styles: true, compressed: true, extraAsset: true})
	secondRoot := filepath.Join(t.TempDir(), "different", "package")
	copyInspectTree(t, first.root, secondRoot)

	firstJSON := runInspectForTest(t, []string{"--dir", first.root, "--format=json"})
	secondJSON := runInspectForTest(t, []string{"--dir", secondRoot, "--format=json"})
	if firstJSON != secondJSON {
		t.Fatalf("copied-root JSON differs\nfirst:\n%s\nsecond:\n%s", firstJSON, secondJSON)
	}
	firstText := runInspectForTest(t, []string{"--dir", first.root})
	secondText := runInspectForTest(t, []string{"--dir", secondRoot})
	if firstText != secondText {
		t.Fatalf("copied-root text differs\nfirst:\n%s\nsecond:\n%s", firstText, secondText)
	}
	for iteration := 0; iteration < 100; iteration++ {
		if got := runInspectForTest(t, []string{"--dir", first.root, "--format=json"}); got != firstJSON {
			t.Fatalf("JSON iteration %d differs", iteration)
		}
		if got := runInspectForTest(t, []string{"--dir", first.root}); got != firstText {
			t.Fatalf("text iteration %d differs", iteration)
		}
	}
	for _, unstable := range []string{first.root, secondRoot, "generatedAt", "2026-08-15"} {
		if strings.Contains(firstJSON, unstable) || strings.Contains(firstText, unstable) {
			t.Fatalf("output contains unstable value %q", unstable)
		}
	}
}

func TestInspectOrdersUnicodePathsByUTF8Bytes(t *testing.T) {
	fixture := writeInspectFixture(t, filepath.Join(t.TempDir(), "package"), inspectFixtureOptions{})
	const (
		bmpLogical           = "styles/\uE000.css"
		supplementaryLogical = "styles/\U00010000.css"
		bmpPath              = "assets/styles/\uE000.css"
		supplementaryPath    = "assets/styles/\U00010000.css"
	)
	if bytes.Compare([]byte(bmpPath), []byte(supplementaryPath)) >= 0 {
		t.Fatal("test paths do not exercise UTF-8 byte ordering")
	}
	writeInspectRaw(t, fixture.root, bmpPath, []byte(".bmp {}\n"))
	writeInspectRaw(t, fixture.root, supplementaryPath, []byte(".supplementary {}\n"))
	mutateInspectAssetManifest(t, fixture.root, func(manifest *assetManifest) {
		manifest.Assets[bmpLogical] = packageAsset{Path: bmpPath, Type: "text/css"}
		manifest.Assets[supplementaryLogical] = packageAsset{Path: supplementaryPath, Type: "text/css"}
		manifest.Entrypoints.Styles = []string{supplementaryPath, bmpPath}
	})

	report, err := inspectPackageGraph(fixture.root)
	if err != nil {
		t.Fatalf("inspectPackageGraph() error: %v", err)
	}
	want := []string{bmpPath, supplementaryPath}
	if !reflect.DeepEqual(report.Entrypoints.Styles, want) {
		t.Fatalf("styles = %#v, want UTF-8 byte order %#v", report.Entrypoints.Styles, want)
	}
	var artifactPaths []string
	for _, artifact := range report.Artifacts {
		if artifact.Path == bmpPath || artifact.Path == supplementaryPath {
			artifactPaths = append(artifactPaths, artifact.Path)
		}
	}
	if !reflect.DeepEqual(artifactPaths, want) {
		t.Fatalf("Unicode artifacts = %#v, want %#v", artifactPaths, want)
	}
	var styleTargets []string
	for _, edge := range report.Edges {
		if edge.Kind == "style-entrypoint" {
			styleTargets = append(styleTargets, edge.To)
		}
	}
	if !reflect.DeepEqual(styleTargets, want) {
		t.Fatalf("Unicode style edges = %#v, want %#v", styleTargets, want)
	}
	first := runInspectForTest(t, []string{"--dir", fixture.root, "--format=json"})
	second := runInspectForTest(t, []string{"--dir", fixture.root, "--format=json"})
	if first != second {
		t.Fatal("Unicode JSON reports differ")
	}
}

func TestInspectResolvesAppPackageDirectoryAndExternalWorkspace(t *testing.T) {
	appDir := filepath.Join(t.TempDir(), "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	layout, err := newBuildLayout(layoutOptions{appDir: appDir})
	if err != nil {
		t.Fatal(err)
	}
	fixture := writeInspectFixture(t, layout.PackageDir, inspectFixtureOptions{hashAssets: true, styles: true})
	fromApp := runInspectForTest(t, []string{appDir, "--format=json"})
	fromPackage := runInspectForTest(t, []string{fixture.root, "--format=json"})
	fromDir := runInspectForTest(t, []string{"--dir", fixture.root, "--format=json"})
	if fromApp != fromPackage || fromApp != fromDir {
		t.Fatalf("app/package/--dir reports differ\napp: %s\npackage: %s\ndir: %s", fromApp, fromPackage, fromDir)
	}

	externalApp := filepath.Join(t.TempDir(), "external-app")
	externalBase := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(externalApp, 0o755); err != nil {
		t.Fatal(err)
	}
	externalLayout, err := newBuildLayout(layoutOptions{appDir: externalApp, workspace: externalBase})
	if err != nil {
		t.Fatal(err)
	}
	externalFixture := writeInspectFixture(t, externalLayout.PackageDir, inspectFixtureOptions{})
	fromExternalApp := runInspectForTest(t, []string{externalApp, "--workspace", externalBase, "--format=json"})
	fromExternalPackage := runInspectForTest(t, []string{"--dir=" + externalFixture.root, "--format=json"})
	if fromExternalApp != fromExternalPackage {
		t.Fatalf("external workspace report differs\napp: %s\npackage: %s", fromExternalApp, fromExternalPackage)
	}
}

func TestInspectExportedPackageMatchesSourceGraph(t *testing.T) {
	source := writeInspectFixture(t, filepath.Join(t.TempDir(), "source"), inspectFixtureOptions{hashAssets: true, styles: true, compressed: true})
	exportRoot := filepath.Join(t.TempDir(), "export")
	copyInspectTree(t, source.root, exportRoot)
	fromSource := runInspectForTest(t, []string{"--dir", source.root, "--format=json"})
	fromExportDir := runInspectForTest(t, []string{"--dir", exportRoot, "--format=json"})
	fromExportPath := runInspectForTest(t, []string{exportRoot, "--format=json"})
	if fromSource != fromExportDir || fromSource != fromExportPath {
		t.Fatalf("exported package graph differs")
	}
}

func TestInspectIsReadOnly(t *testing.T) {
	fixture := writeInspectFixture(t, filepath.Join(t.TempDir(), "package"), inspectFixtureOptions{hashAssets: true, styles: true, compressed: true, extraAsset: true})
	extra := filepath.Join(fixture.root, "notes.txt")
	if err := os.WriteFile(extra, []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixedTime := time.Unix(1_700_000_000, 123_000_000)
	if err := filepath.WalkDir(fixture.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		return os.Chtimes(path, fixedTime, fixedTime)
	}); err != nil {
		t.Fatal(err)
	}
	before := snapshotInspectTree(t, fixture.root)
	_ = runInspectForTest(t, []string{"--dir", fixture.root})
	_ = runInspectForTest(t, []string{"--dir", fixture.root, "--format=json"})
	after := snapshotInspectTree(t, fixture.root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("package changed during inspection\nbefore: %#v\nafter:  %#v", before, after)
	}
	if strings.Contains(runInspectForTest(t, []string{"--dir", fixture.root, "--format=json"}), "notes.txt") {
		t.Fatal("undeclared extra file appeared in graph")
	}
}

func TestInspectAppWithoutPackageDoesNotCreateWorkspace(t *testing.T) {
	appDir := filepath.Join(t.TempDir(), "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err := runInspectCommand([]string{appDir}, &output)
	if err == nil || !strings.Contains(err.Error(), "no complete current standalone package found") || !strings.Contains(err.Error(), "goxc package") {
		t.Fatalf("runInspectCommand(no package) error = %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("inspect emitted partial output: %q", output.String())
	}
	if _, statErr := os.Lstat(filepath.Join(appDir, defaultWorkspaceName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("inspect created .goframe: %v", statErr)
	}
}

func TestInspectRejectsInvalidGraphsWithoutOutputOrMutation(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, root string)
		want    string
	}{
		{name: "missing package metadata", prepare: func(*testing.T, string) {}, want: packageMetadataName},
		{name: "asset manifest only", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{})
			if err := os.Remove(filepath.Join(fixture.root, packageMetadataName)); err != nil {
				t.Fatal(err)
			}
		}, want: packageMetadataName},
		{name: "legacy package", prepare: writeLegacyPackageSignature, want: "legacy GoFrame packages are not supported"},
		{name: "malformed package metadata", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			writeInspectRaw(t, root, packageMetadataName, []byte("{"))
		}, want: "parse"},
		{name: "malformed asset manifest", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			writeInspectRaw(t, root, assetManifestName, []byte("{"))
		}, want: "parse"},
		{name: "unsupported package metadata version", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			mutateInspectPackageMetadata(t, root, func(metadata *packageMetadata) { metadata.Version = 2 })
		}, want: "unsupported version"},
		{name: "empty toolchain version", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			mutateInspectPackageMetadata(t, root, func(metadata *packageMetadata) { metadata.ToolchainVersion = "" })
		}, want: "toolchainVersion must not be empty"},
		{name: "whitespace toolchain version", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			mutateInspectPackageMetadata(t, root, func(metadata *packageMetadata) { metadata.ToolchainVersion = "   " })
		}, want: "toolchainVersion must not be empty"},
		{name: "unsupported asset manifest version", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) { manifest.Version = 2 })
		}, want: "unsupported version"},
		{name: "metadata and manifest entrypoint mismatch", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{})
			mutateInspectPackageMetadata(t, root, func(metadata *packageMetadata) { metadata.Entrypoints.WASM = fixture.assetPaths[runtimeAssetName] })
		}, want: "entrypoints do not match"},
		{name: "missing HTML", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			if err := os.Remove(filepath.Join(root, indexHTMLAssetName)); err != nil {
				t.Fatal(err)
			}
		}, want: "HTML entrypoint"},
		{name: "missing WASM", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{})
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(fixture.assetPaths["bundle.wasm"]))); err != nil {
				t.Fatal(err)
			}
		}, want: "WASM entrypoint"},
		{name: "missing runtime", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{})
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(fixture.assetPaths[runtimeAssetName]))); err != nil {
				t.Fatal(err)
			}
		}, want: "runtime entrypoint"},
		{name: "WASM entrypoint not declared as asset", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			writeInspectRaw(t, root, "assets/other.wasm", []byte("other wasm"))
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) { manifest.Entrypoints.WASM = "assets/other.wasm" })
			mutateInspectPackageMetadata(t, root, func(metadata *packageMetadata) { metadata.Entrypoints.WASM = "assets/other.wasm" })
		}, want: "not declared as exactly one asset"},
		{name: "runtime entrypoint not declared as asset", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			writeInspectRaw(t, root, "assets/other.js", []byte("other runtime"))
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) { manifest.Entrypoints.Runtime = "assets/other.js" })
			mutateInspectPackageMetadata(t, root, func(metadata *packageMetadata) { metadata.Entrypoints.Runtime = "assets/other.js" })
		}, want: "not declared as exactly one asset"},
		{name: "shared WASM and runtime entrypoint", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{})
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				manifest.Entrypoints.Runtime = fixture.assetPaths["bundle.wasm"]
			})
			mutateInspectPackageMetadata(t, root, func(metadata *packageMetadata) {
				metadata.Entrypoints.Runtime = fixture.assetPaths["bundle.wasm"]
			})
		}, want: "WASM and runtime entrypoints must resolve to distinct assets"},
		{name: "WASM entrypoint is not a WASM file", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			const nonWASMPath = "assets/bundle.txt"
			writeInspectRaw(t, root, nonWASMPath, []byte("not wasm"))
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				asset := manifest.Assets["bundle.wasm"]
				asset.Path = nonWASMPath
				manifest.Assets["bundle.wasm"] = asset
				manifest.Entrypoints.WASM = nonWASMPath
			})
			mutateInspectPackageMetadata(t, root, func(metadata *packageMetadata) {
				metadata.Entrypoints.WASM = nonWASMPath
			})
		}, want: "WASM entrypoint must end in .wasm"},
		{name: "WASM entrypoint has wrong media type", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				asset := manifest.Assets["bundle.wasm"]
				asset.Type = "application/octet-stream"
				manifest.Assets["bundle.wasm"] = asset
			})
		}, want: "must declare media type \"application/wasm\""},
		{name: "runtime entrypoint is CSS", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{})
			const cssRuntimePath = "assets/wasm_exec.css"
			relocateInspectAsset(t, root, fixture.assetPaths[runtimeAssetName], cssRuntimePath)
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				asset := manifest.Assets[runtimeAssetName]
				asset.Path = cssRuntimePath
				asset.Type = "text/css"
				manifest.Assets[runtimeAssetName] = asset
				manifest.Entrypoints.Runtime = cssRuntimePath
			})
			mutateInspectPackageMetadata(t, root, func(metadata *packageMetadata) {
				metadata.Entrypoints.Runtime = cssRuntimePath
			})
		}, want: "runtime entrypoint must end in .js"},
		{name: "runtime entrypoint has wrong media type", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				asset := manifest.Assets[runtimeAssetName]
				asset.Type = "text/css"
				manifest.Assets[runtimeAssetName] = asset
			})
		}, want: "must declare media type \"text/javascript\""},
		{name: "style entrypoint is JavaScript", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{styles: true})
			const scriptStylePath = "assets/styles/a.js"
			relocateInspectAsset(t, root, fixture.assetPaths["styles/a.css"], scriptStylePath)
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				asset := manifest.Assets["styles/a.css"]
				asset.Path = scriptStylePath
				asset.Type = "text/javascript"
				manifest.Assets["styles/a.css"] = asset
				manifest.Entrypoints.Styles = []string{scriptStylePath, fixture.assetPaths["styles/z.css"]}
			})
		}, want: "style entrypoint must end in .css"},
		{name: "style entrypoint has wrong media type", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{styles: true})
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				asset := manifest.Assets["styles/a.css"]
				asset.Type = "text/javascript"
				manifest.Assets["styles/a.css"] = asset
				manifest.Entrypoints.Styles = []string{fixture.assetPaths["styles/a.css"]}
			})
		}, want: "must declare media type \"text/css\""},
		{name: "missing ordinary asset", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{extraAsset: true})
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(fixture.assetPaths["images/logo.svg"]))); err != nil {
				t.Fatal(err)
			}
		}, want: "declared asset"},
		{name: "missing compressed sidecar", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{compressed: true})
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(fixture.assetPaths["bundle.wasm"]+".gz"))); err != nil {
				t.Fatal(err)
			}
		}, want: "compressed asset"},
		{name: "unsafe asset path", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				asset := manifest.Assets["bundle.wasm"]
				asset.Path = "../bundle.wasm"
				manifest.Assets["bundle.wasm"] = asset
			})
		}, want: "invalid asset path"},
		{name: "empty logical asset name", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				manifest.Assets[""] = manifest.Assets["bundle.wasm"]
				delete(manifest.Assets, "bundle.wasm")
			})
		}, want: "logical asset name is empty"},
		{name: "duplicate final asset paths", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{extraAsset: true})
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				logo := manifest.Assets["images/logo.svg"]
				logo.Path = fixture.assetPaths["bundle.wasm"]
				manifest.Assets["images/logo.svg"] = logo
			})
		}, want: "declared by more than one asset"},
		{name: "asset and compressed collision", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{})
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				asset := manifest.Assets["bundle.wasm"]
				asset.Compressed = map[string]string{"gzip": fixture.assetPaths[runtimeAssetName]}
				manifest.Assets["bundle.wasm"] = asset
			})
		}, want: "collides"},
		{name: "style entrypoint not declared", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) { manifest.Entrypoints.Styles = []string{"assets/missing.css"} })
		}, want: "style entrypoint"},
		{name: "duplicate style entrypoint", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{styles: true})
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				manifest.Entrypoints.Styles = []string{fixture.assetPaths["styles/a.css"], fixture.assetPaths["styles/a.css"]}
			})
		}, want: "duplicate style entrypoint"},
		{name: "invalid declared hash", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{hashAssets: true})
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				asset := manifest.Assets["bundle.wasm"]
				asset.Hash = "ABCDEF12"
				manifest.Assets["bundle.wasm"] = asset
			})
		}, want: "eight lowercase hexadecimal"},
		{name: "declared hash mismatch", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{hashAssets: true})
			const mismatchedPath = "assets/bundle.00000000.wasm"
			relocateInspectAsset(t, root, fixture.assetPaths["bundle.wasm"], mismatchedPath)
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				asset := manifest.Assets["bundle.wasm"]
				asset.Hash = "00000000"
				asset.Path = mismatchedPath
				manifest.Assets["bundle.wasm"] = asset
				manifest.Entrypoints.WASM = mismatchedPath
			})
			mutateInspectPackageMetadata(t, root, func(metadata *packageMetadata) {
				metadata.Entrypoints.WASM = mismatchedPath
			})
		}, want: "does not match"},
		{name: "hash assets missing declared hash", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{hashAssets: true})
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				asset := manifest.Assets[runtimeAssetName]
				asset.Hash = ""
				manifest.Assets[runtimeAssetName] = asset
			})
		}, want: "requires a declared hash"},
		{name: "unhashed package declares asset hash", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{})
			_, actualHash := inspectFileEvidence(t, root, fixture.assetPaths["bundle.wasm"])
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				asset := manifest.Assets["bundle.wasm"]
				asset.Hash = actualHash[:packageHashLength]
				manifest.Assets["bundle.wasm"] = asset
			})
		}, want: "hashAssets=false requires an empty declared hash"},
		{name: "hashed package uses unversioned asset path", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{hashAssets: true})
			const unversionedPath = "assets/bundle.wasm"
			relocateInspectAsset(t, root, fixture.assetPaths["bundle.wasm"], unversionedPath)
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				asset := manifest.Assets["bundle.wasm"]
				asset.Path = unversionedPath
				manifest.Assets["bundle.wasm"] = asset
				manifest.Entrypoints.WASM = unversionedPath
			})
			mutateInspectPackageMetadata(t, root, func(metadata *packageMetadata) {
				metadata.Entrypoints.WASM = unversionedPath
			})
		}, want: "content-addressed path"},
		{name: "empty compressed encoding", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{})
			sidecar := fixture.assetPaths["bundle.wasm"] + ".sidecar"
			writeInspectRaw(t, root, sidecar, []byte("compressed"))
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				asset := manifest.Assets["bundle.wasm"]
				asset.Compressed = map[string]string{"": sidecar}
				manifest.Assets["bundle.wasm"] = asset
			})
		}, want: "encoding name is empty"},
		{name: "duplicate sidecar path", prepare: func(t *testing.T, root string) {
			fixture := writeInspectFixture(t, root, inspectFixtureOptions{compressed: true})
			mutateInspectAssetManifest(t, root, func(manifest *assetManifest) {
				bundle := manifest.Assets["bundle.wasm"]
				runtimeAsset := manifest.Assets[runtimeAssetName]
				runtimeAsset.Compressed["gzip"] = bundle.Compressed["gzip"]
				manifest.Assets[runtimeAssetName] = runtimeAsset
			})
			_ = fixture
		}, want: "compressed path"},
		{name: "HTML metadata path collision", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			mutateInspectPackageMetadata(t, root, func(metadata *packageMetadata) { metadata.Entrypoints.HTML = packageMetadataName })
		}, want: "collides"},
		{name: "duplicate logical name", prepare: func(t *testing.T, root string) {
			writeInspectFixture(t, root, inspectFixtureOptions{})
			writeDuplicateInspectLogicalName(t, root)
		}, want: "duplicate logical asset name"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "package")
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			test.prepare(t, root)
			before := snapshotInspectTree(t, root)
			var output bytes.Buffer
			err := runInspectCommand([]string{"--dir", root, "--format=json"}, &output)
			after := snapshotInspectTree(t, root)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("runInspectCommand() error = %v, want %q; output bytes = %d", err, test.want, output.Len())
			}
			if output.Len() != 0 {
				t.Errorf("invalid graph emitted partial output: %q", output.String())
			}
			if !reflect.DeepEqual(after, before) {
				t.Errorf("invalid inspection mutated filesystem\nbefore: %#v\nafter:  %#v", before, after)
			}
		})
	}
}

func TestInspectAcceptsReviewedInvariantControls(t *testing.T) {
	tests := []struct {
		name    string
		options inspectFixtureOptions
		prepare func(t *testing.T, fixture inspectFixture)
	}{
		{name: "development toolchain"},
		{name: "tagged toolchain", prepare: func(t *testing.T, fixture inspectFixture) {
			mutateInspectPackageMetadata(t, fixture.root, func(metadata *packageMetadata) {
				metadata.ToolchainVersion = "v0.3.0-preview.1"
			})
		}},
		{name: "unhashed package without declared hashes"},
		{name: "hashed package", options: inspectFixtureOptions{hashAssets: true}},
		{name: "hashed WASM filename", options: inspectFixtureOptions{hashAssets: true}},
		{name: "nested hashed CSS path", options: inspectFixtureOptions{hashAssets: true, styles: true}},
		{name: "case-insensitive entrypoint extensions", options: inspectFixtureOptions{styles: true}, prepare: func(t *testing.T, fixture inspectFixture) {
			const (
				wasmPath    = "assets/APP.WASM"
				runtimePath = "assets/RUNTIME.JS"
				stylePath   = "assets/styles/STYLE.CSS"
			)
			relocateInspectAsset(t, fixture.root, fixture.assetPaths["bundle.wasm"], wasmPath)
			relocateInspectAsset(t, fixture.root, fixture.assetPaths[runtimeAssetName], runtimePath)
			relocateInspectAsset(t, fixture.root, fixture.assetPaths["styles/a.css"], stylePath)
			mutateInspectAssetManifest(t, fixture.root, func(manifest *assetManifest) {
				wasm := manifest.Assets["bundle.wasm"]
				wasm.Path = wasmPath
				manifest.Assets["bundle.wasm"] = wasm
				runtimeAsset := manifest.Assets[runtimeAssetName]
				runtimeAsset.Path = runtimePath
				manifest.Assets[runtimeAssetName] = runtimeAsset
				style := manifest.Assets["styles/a.css"]
				style.Path = stylePath
				manifest.Assets["styles/a.css"] = style
				manifest.Entrypoints.WASM = wasmPath
				manifest.Entrypoints.Runtime = runtimePath
				manifest.Entrypoints.Styles = []string{stylePath, fixture.assetPaths["styles/z.css"]}
			})
			mutateInspectPackageMetadata(t, fixture.root, func(metadata *packageMetadata) {
				metadata.Entrypoints.WASM = wasmPath
				metadata.Entrypoints.Runtime = runtimePath
			})
		}},
		{name: "future compression encoding", prepare: func(t *testing.T, fixture inspectFixture) {
			const sidecarPath = "assets/bundle.wasm.future"
			writeInspectRaw(t, fixture.root, sidecarPath, []byte("future encoding"))
			mutateInspectAssetManifest(t, fixture.root, func(manifest *assetManifest) {
				asset := manifest.Assets["bundle.wasm"]
				asset.Compressed = map[string]string{"future": sidecarPath}
				manifest.Assets["bundle.wasm"] = asset
			})
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := writeInspectFixture(t, t.TempDir(), test.options)
			if test.prepare != nil {
				test.prepare(t, fixture)
			}
			if _, err := inspectPackageGraph(fixture.root); err != nil {
				t.Fatalf("inspectPackageGraph() error: %v", err)
			}
		})
	}
}

func TestInspectRejectsPhysicalArtifactAliases(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, fixture inspectFixture)
	}{
		{name: "ordinary assets", prepare: func(t *testing.T, fixture inspectFixture) {
			aliasInspectFile(t, fixture.root, fixture.assetPaths["bundle.wasm"], fixture.assetPaths["images/logo.svg"])
		}},
		{name: "HTML and ordinary asset", prepare: func(t *testing.T, fixture inspectFixture) {
			aliasInspectFile(t, fixture.root, indexHTMLAssetName, fixture.assetPaths["images/logo.svg"])
		}},
		{name: "ordinary asset and compressed sidecar", prepare: func(t *testing.T, fixture inspectFixture) {
			aliasInspectFile(t, fixture.root, fixture.assetPaths["bundle.wasm"], fixture.assetPaths["bundle.wasm"]+".gz")
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := writeInspectFixture(t, filepath.Join(t.TempDir(), "package"), inspectFixtureOptions{
				compressed: true,
				extraAsset: true,
			})
			test.prepare(t, fixture)
			before := snapshotInspectTree(t, fixture.root)
			var output bytes.Buffer
			err := runInspectCommand([]string{"--dir", fixture.root, "--format=json"}, &output)
			after := snapshotInspectTree(t, fixture.root)
			if err == nil || !strings.Contains(err.Error(), "physical alias") {
				t.Errorf("runInspectCommand() error = %v, want physical alias rejection; output bytes = %d", err, output.Len())
			}
			if output.Len() != 0 {
				t.Errorf("physical alias emitted partial output: %q", output.String())
			}
			if !reflect.DeepEqual(after, before) {
				t.Errorf("physical alias inspection mutated filesystem\nbefore: %#v\nafter:  %#v", before, after)
			}
		})
	}
}

func TestInspectAcceptsDistinctCaseDifferentFiles(t *testing.T) {
	fixture := writeInspectFixture(t, filepath.Join(t.TempDir(), "package"), inspectFixtureOptions{})
	const upperPath = "assets/App.js"
	const lowerPath = "assets/app.js"
	writeInspectRaw(t, fixture.root, upperPath, []byte("upper"))
	writeInspectRaw(t, fixture.root, lowerPath, []byte("lower"))
	upperInfo, err := os.Stat(filepath.Join(fixture.root, filepath.FromSlash(upperPath)))
	if err != nil {
		t.Fatal(err)
	}
	lowerInfo, err := os.Stat(filepath.Join(fixture.root, filepath.FromSlash(lowerPath)))
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(upperInfo, lowerInfo) {
		t.Skip("filesystem does not support distinct case-different files")
	}
	mutateInspectAssetManifest(t, fixture.root, func(manifest *assetManifest) {
		manifest.Assets["App.js"] = packageAsset{Path: upperPath, Type: "text/javascript"}
		manifest.Assets["app.js"] = packageAsset{Path: lowerPath, Type: "text/javascript"}
	})

	report, err := inspectPackageGraph(fixture.root)
	if err != nil {
		t.Fatalf("inspectPackageGraph() error: %v", err)
	}
	artifacts := inspectArtifactsByPath(t, report)
	if _, ok := artifacts[upperPath]; !ok {
		t.Fatalf("artifact %q missing", upperPath)
	}
	if _, ok := artifacts[lowerPath]; !ok {
		t.Fatalf("artifact %q missing", lowerPath)
	}
}

func TestInspectRejectsSymlinkedRootsMetadataAndAssets(t *testing.T) {
	requireSymlinkSupport(t)
	t.Run("package root", func(t *testing.T) {
		target := writeInspectFixture(t, filepath.Join(t.TempDir(), "target"), inspectFixtureOptions{})
		alias := filepath.Join(t.TempDir(), "alias")
		if err := os.Symlink(target.root, alias); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		err := runInspectCommand([]string{"--dir", alias}, &output)
		if err == nil || !strings.Contains(err.Error(), "symlink") || output.Len() != 0 {
			t.Fatalf("symlink root error=%v output=%q", err, output.String())
		}
	})
	t.Run("package metadata", func(t *testing.T) {
		fixture := writeInspectFixture(t, filepath.Join(t.TempDir(), "package"), inspectFixtureOptions{})
		metadata := filepath.Join(fixture.root, packageMetadataName)
		content, err := os.ReadFile(metadata)
		if err != nil {
			t.Fatal(err)
		}
		external := filepath.Join(t.TempDir(), packageMetadataName)
		if err := os.WriteFile(external, content, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(metadata); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(external, metadata); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		err = runInspectCommand([]string{"--dir", fixture.root}, &output)
		if err == nil || !strings.Contains(err.Error(), "symlink") || output.Len() != 0 {
			t.Fatalf("symlink metadata error=%v output=%q", err, output.String())
		}
	})
	t.Run("declared asset", func(t *testing.T) {
		fixture := writeInspectFixture(t, filepath.Join(t.TempDir(), "package"), inspectFixtureOptions{})
		assetPath := filepath.Join(fixture.root, filepath.FromSlash(fixture.assetPaths["bundle.wasm"]))
		external := filepath.Join(t.TempDir(), "bundle.wasm")
		if err := os.WriteFile(external, []byte("wasm"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(assetPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(external, assetPath); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		err := runInspectCommand([]string{"--dir", fixture.root}, &output)
		if err == nil || !strings.Contains(err.Error(), "symlink") || output.Len() != 0 {
			t.Fatalf("symlink asset error=%v output=%q", err, output.String())
		}
	})
}

func TestInspectWriterFailures(t *testing.T) {
	fixture := writeInspectFixture(t, t.TempDir(), inspectFixtureOptions{})
	for _, format := range []string{"text", "json"} {
		t.Run(format, func(t *testing.T) {
			writer := &inspectFailWriter{err: errors.New("injected writer failure")}
			err := runInspectCommand([]string{"--dir", fixture.root, "--format=" + format}, writer)
			if err == nil || !strings.Contains(err.Error(), "write inspect "+format+" report") || !strings.Contains(err.Error(), "injected writer failure") {
				t.Fatalf("runInspectCommand(%s) error = %v", format, err)
			}
		})
	}
}

type inspectFailWriter struct {
	err error
}

func (writer *inspectFailWriter) Write([]byte) (int, error) {
	return 0, writer.err
}

type inspectSnapshotEntry struct {
	mode    os.FileMode
	mtime   int64
	content string
}

func writeInspectFixture(t *testing.T, root string, options inspectFixtureOptions) inspectFixture {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	type fixtureAsset struct {
		content []byte
		kind    string
	}
	assets := map[string]fixtureAsset{
		"bundle.wasm":    {content: []byte("\x00asm-inspect-fixture"), kind: "application/wasm"},
		runtimeAssetName: {content: []byte("globalThis.Go = class Go {};\n"), kind: "text/javascript"},
	}
	if options.styles {
		assets["styles/z.css"] = fixtureAsset{content: []byte("body { color: navy; }\n"), kind: "text/css"}
		assets["styles/a.css"] = fixtureAsset{content: []byte("main { display: block; }\n"), kind: "text/css"}
	}
	if options.extraAsset {
		assets["images/logo.svg"] = fixtureAsset{content: []byte("<svg></svg>\n"), kind: "image/svg+xml"}
	}

	assetPaths := make(map[string]string, len(assets))
	manifestAssets := make(map[string]packageAsset, len(assets))
	logicalNames := make([]string, 0, len(assets))
	for logical := range assets {
		logicalNames = append(logicalNames, logical)
	}
	sort.Strings(logicalNames)
	for _, logical := range logicalNames {
		asset := assets[logical]
		shortHash := inspectContentHash(asset.content)[:packageHashLength]
		outputName := logical
		if options.hashAssets {
			outputName = hashedAssetName(logical, shortHash)
		}
		finalPath := filepath.ToSlash(filepath.Join(assetDirectoryName, filepath.FromSlash(outputName)))
		assetPaths[logical] = finalPath
		writeInspectRaw(t, root, finalPath, asset.content)
		entry := packageAsset{Path: finalPath, Type: asset.kind}
		if options.hashAssets {
			entry.Hash = shortHash
		}
		if options.compressed && isCompressiblePackageAsset(logical) {
			entry.Compressed = map[string]string{
				"gzip": finalPath + ".gz",
				"br":   finalPath + ".br",
			}
			writeInspectRaw(t, root, entry.Compressed["gzip"], []byte("gzip:"+logical))
			writeInspectRaw(t, root, entry.Compressed["br"], []byte("br:"+logical))
		}
		manifestAssets[logical] = entry
	}

	styles := []string{}
	if options.styles {
		styles = []string{assetPaths["styles/z.css"], assetPaths["styles/a.css"]}
	}
	manifest := assetManifest{
		Version: 1,
		Assets:  manifestAssets,
		Entrypoints: packageEntrypoints{
			WASM:    assetPaths["bundle.wasm"],
			Runtime: assetPaths[runtimeAssetName],
			Styles:  styles,
		},
	}
	metadata := packageMetadata{
		Version:          1,
		Name:             "demo",
		Compiler:         "tinygo",
		ToolchainVersion: "devel",
		AssetsDir:        assetDirectoryName,
		HashAssets:       options.hashAssets,
		Preload:          options.hashAssets,
		Entrypoints: metadataEntrypoint{
			HTML:    indexHTMLAssetName,
			WASM:    assetPaths["bundle.wasm"],
			Runtime: assetPaths[runtimeAssetName],
		},
		GeneratedAt: "2026-08-15T00:00:00Z",
	}
	writeInspectRaw(t, root, indexHTMLAssetName, []byte("<!doctype html><title>inspect</title>\n"))
	writeInspectJSONFixture(t, root, assetManifestName, manifest)
	writeInspectJSONFixture(t, root, packageMetadataName, metadata)
	return inspectFixture{root: root, assetPaths: assetPaths}
}

func writeInspectRaw(t *testing.T, root, relative string, content []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func relocateInspectAsset(t *testing.T, root, fromRelative, toRelative string) {
	t.Helper()
	from := filepath.Join(root, filepath.FromSlash(fromRelative))
	to := filepath.Join(root, filepath.FromSlash(toRelative))
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
}

func aliasInspectFile(t *testing.T, root, targetRelative, aliasRelative string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(targetRelative))
	alias := filepath.Join(root, filepath.FromSlash(aliasRelative))
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(target, alias); err != nil {
		t.Skipf("hard links are unavailable: %v", err)
	}
}

func writeInspectJSONFixture(t *testing.T, root, relative string, value any) {
	t.Helper()
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeInspectRaw(t, root, relative, append(content, '\n'))
}

func mutateInspectPackageMetadata(t *testing.T, root string, mutate func(*packageMetadata)) {
	t.Helper()
	var metadata packageMetadata
	readInspectJSONFixture(t, root, packageMetadataName, &metadata)
	mutate(&metadata)
	writeInspectJSONFixture(t, root, packageMetadataName, metadata)
}

func mutateInspectAssetManifest(t *testing.T, root string, mutate func(*assetManifest)) {
	t.Helper()
	var manifest assetManifest
	readInspectJSONFixture(t, root, assetManifestName, &manifest)
	mutate(&manifest)
	writeInspectJSONFixture(t, root, assetManifestName, manifest)
}

func readInspectJSONFixture(t *testing.T, root, relative string, value any) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, value); err != nil {
		t.Fatal(err)
	}
}

func writeDuplicateInspectLogicalName(t *testing.T, root string) {
	t.Helper()
	var manifest assetManifest
	readInspectJSONFixture(t, root, assetManifestName, &manifest)
	bundle, err := json.Marshal(manifest.Assets["bundle.wasm"])
	if err != nil {
		t.Fatal(err)
	}
	runtimeAsset, err := json.Marshal(manifest.Assets[runtimeAssetName])
	if err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf(`{
  "version": 1,
  "assets": {
    "bundle.wasm": %s,
    "bundle.wasm": %s,
    "wasm_exec.js": %s
  },
  "entrypoints": {
    "wasm": %q,
    "runtime": %q,
    "styles": []
  }
}
`, bundle, bundle, runtimeAsset, manifest.Entrypoints.WASM, manifest.Entrypoints.Runtime)
	writeInspectRaw(t, root, assetManifestName, []byte(content))
}

func inspectContentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func inspectFileEvidence(t *testing.T, root, relative string) (int64, string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	return int64(len(content)), inspectContentHash(content)
}

func inspectArtifactsByPath(t *testing.T, report inspectReport) map[string]inspectArtifact {
	t.Helper()
	artifacts := make(map[string]inspectArtifact, len(report.Artifacts))
	for _, artifact := range report.Artifacts {
		if _, exists := artifacts[artifact.Path]; exists {
			t.Fatalf("duplicate artifact %q", artifact.Path)
		}
		artifacts[artifact.Path] = artifact
	}
	return artifacts
}

func hasInspectRole(roles []string, want string) bool {
	for _, role := range roles {
		if role == want {
			return true
		}
	}
	return false
}

func inspectEdgeExists(edges []inspectEdge, from, to, kind, encoding string) bool {
	for _, edge := range edges {
		if edge.From == from && edge.To == to && edge.Kind == kind && edge.Encoding == encoding {
			return true
		}
	}
	return false
}

func inspectEdgeLess(first, second inspectEdge) bool {
	if first.From != second.From {
		return first.From < second.From
	}
	if first.Kind != second.Kind {
		return first.Kind < second.Kind
	}
	if first.Encoding != second.Encoding {
		return first.Encoding < second.Encoding
	}
	return first.To < second.To
}

func runInspectForTest(t *testing.T, args []string) string {
	t.Helper()
	var output bytes.Buffer
	if err := runInspectCommand(args, &output); err != nil {
		t.Fatalf("runInspectCommand(%#v) error: %v", args, err)
	}
	return output.String()
}

func copyInspectTree(t *testing.T, source, destination string) {
	t.Helper()
	if err := filepath.WalkDir(source, func(sourcePath string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, sourcePath)
		if err != nil {
			return err
		}
		destinationPath := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(destinationPath, 0o755)
		}
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		return os.WriteFile(destinationPath, content, 0o644)
	}); err != nil {
		t.Fatal(err)
	}
}

func snapshotInspectTree(t *testing.T, root string) map[string]inspectSnapshotEntry {
	t.Helper()
	snapshot := map[string]inspectSnapshotEntry{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		item := inspectSnapshotEntry{mode: info.Mode(), mtime: info.ModTime().UnixNano()}
		if info.Mode()&os.ModeSymlink != 0 {
			item.content, err = os.Readlink(path)
		} else if info.Mode().IsRegular() {
			var content []byte
			content, err = os.ReadFile(path)
			item.content = string(content)
		}
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(relative)] = item
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

var _ io.Writer = (*inspectFailWriter)(nil)
