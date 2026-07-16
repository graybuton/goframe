package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDevOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want devOptions
	}{
		{
			name: "defaults",
			args: []string{"app"},
			want: devOptions{appDir: "app", port: 8080},
		},
		{
			name: "equals forms",
			args: []string{"--compiler=tinygo", "--port=0", "--workspace=work", "app"},
			want: devOptions{appDir: "app", compiler: "tinygo", port: 0, workspace: "work"},
		},
		{
			name: "separate forms",
			args: []string{"app", "--compiler", "go", "--port", "3210", "--workspace", "work"},
			want: devOptions{appDir: "app", compiler: "go", port: 3210, workspace: "work"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseDevOptions(test.args)
			if err != nil {
				t.Fatalf("parseDevOptions() error: %v", err)
			}
			if got != test.want {
				t.Fatalf("options = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestDevOptionsRejectInvalidArguments(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "missing app", want: "usage: goxc dev"},
		{name: "missing compiler", args: []string{"app", "--compiler"}, want: "--compiler requires a value"},
		{name: "invalid compiler", args: []string{"app", "--compiler=other"}, want: "use go or tinygo"},
		{name: "missing port", args: []string{"app", "--port"}, want: "--port requires a value"},
		{name: "invalid port", args: []string{"app", "--port=nope"}, want: "invalid port"},
		{name: "negative port", args: []string{"app", "--port=-1"}, want: "between 0 and 65535"},
		{name: "large port", args: []string{"app", "--port=65536"}, want: "between 0 and 65535"},
		{name: "missing workspace", args: []string{"app", "--workspace"}, want: "--workspace requires a value"},
		{name: "unknown flag", args: []string{"app", "--reload"}, want: "unknown dev flag"},
		{name: "second app", args: []string{"app", "other"}, want: "unexpected dev argument"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseDevOptions(test.args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("parseDevOptions(%#v) error = %v, want %q", test.args, err, test.want)
			}
		})
	}
}

func TestDevAppearsInTopLevelUsage(t *testing.T) {
	var output bytes.Buffer
	usage(&output)
	if !strings.Contains(output.String(), "dev <app>") {
		t.Fatalf("top-level usage does not list dev:\n%s", output.String())
	}
}

func TestDevHelpUsage(t *testing.T) {
	if !commandHelpRequested([]string{"--help"}) {
		t.Fatal("dev --help was not recognized as a help request")
	}
	var output bytes.Buffer
	commandUsage(&output, "dev")
	if !strings.Contains(output.String(), "goxc dev <app-directory> [--compiler=go|tinygo] [--port=8080] [--workspace=directory]") {
		t.Fatalf("dev help does not contain the command contract:\n%s", output.String())
	}
}

func TestDevSnapshotIncludesAuthoredAssetsAndModuleInputs(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	writeTestFile(t, root, "go.mod", "module example.com/dev\n\ngo 1.22\n")
	writeTestFile(t, root, "go.sum", "example.com/dependency v1.0.0 h1:test\n")
	writeTestFile(t, appDir, manifestName, `{"compiler":"go","assets":"assets"}`)
	writeTestFile(t, appDir, "app.gox", "package main\nfunc App() any { return nil }\n")
	writeTestFile(t, appDir, "main.go", "package main\n")
	writeTestFile(t, appDir, "app.gox.go", "generated\n")
	writeTestFile(t, appDir, "assets/index.html", "<div id=\"root\"></div>")
	writeTestFile(t, appDir, "assets/styles.css", "body{}")
	for _, relative := range []string{
		".goframe/ignored.go",
		"build/ignored.go",
		"dist/ignored.gox",
		"node_modules/ignored.go",
		".git/ignored.go",
		".goxc-tmp/ignored.gox",
	} {
		writeTestFile(t, appDir, relative, "package ignored\n")
	}

	collector := newDevSnapshotCollector(appDir)
	snapshot, err := collector.collect()
	if err != nil {
		t.Fatalf("collect() error: %v", err)
	}
	for _, want := range []string{
		"../go.mod",
		"../go.sum",
		"app.gox",
		"assets/",
		"assets/index.html",
		"assets/styles.css",
		manifestName,
		"main.go",
	} {
		if _, ok := snapshot.files[want]; !ok {
			t.Errorf("snapshot missing %q: %#v", want, snapshot.paths())
		}
	}
	for _, excluded := range []string{
		"app.gox.go",
		".goframe/ignored.go",
		"build/ignored.go",
		"dist/ignored.gox",
		"node_modules/ignored.go",
		".git/ignored.go",
		".goxc-tmp/ignored.gox",
	} {
		if _, ok := snapshot.files[excluded]; ok {
			t.Errorf("snapshot includes excluded %q", excluded)
		}
	}
	paths := snapshot.paths()
	if !sort.StringsAreSorted(paths) {
		t.Fatalf("snapshot paths are not sorted: %#v", paths)
	}
}

func TestDevSnapshotDetectsAssetAndModuleChanges(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	writeTestFile(t, root, "go.mod", "module example.com/dev\n\ngo 1.22\n")
	writeTestFile(t, appDir, manifestName, `{"compiler":"go","assets":"assets"}`)
	writeTestFile(t, appDir, "main.go", "package main\n")
	writeTestFile(t, appDir, "assets/index.html", "first")
	collector := newDevSnapshotCollector(appDir)

	initial, err := collector.collect()
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, appDir, "assets/nested/data.txt", "new")
	created, err := collector.collect()
	if err != nil {
		t.Fatal(err)
	}
	assertDevChangedPath(t, initial, created, "assets/nested/data.txt")

	if err := os.Remove(filepath.Join(appDir, "assets", "nested", "data.txt")); err != nil {
		t.Fatal(err)
	}
	deleted, err := collector.collect()
	if err != nil {
		t.Fatal(err)
	}
	assertDevChangedPath(t, created, deleted, "assets/nested/data.txt")

	writeTestFile(t, root, "go.sum", "module sum\n")
	withSum, err := collector.collect()
	if err != nil {
		t.Fatal(err)
	}
	assertDevChangedPath(t, deleted, withSum, "../go.sum")
	if err := os.Remove(filepath.Join(root, "go.sum")); err != nil {
		t.Fatal(err)
	}
	withoutSum, err := collector.collect()
	if err != nil {
		t.Fatal(err)
	}
	assertDevChangedPath(t, withSum, withoutSum, "../go.sum")

	writeTestFile(t, root, "go.mod", "module example.com/dev\n\ngo 1.23\n")
	withUpdatedModule, err := collector.collect()
	if err != nil {
		t.Fatal(err)
	}
	assertDevChangedPath(t, withoutSum, withUpdatedModule, "../go.mod")
}

func TestDevSnapshotAutoAssetsIgnoresNonDirectoryAndDetectsDirectoryTransitions(t *testing.T) {
	appDir := t.TempDir()
	writeTestFile(t, appDir, manifestName, `{"compiler":"go"}`)
	writeTestFile(t, appDir, "main.go", "package main\n")
	writeTestFile(t, appDir, indexHTMLAssetName, "<main>fallback</main>")
	writeTestFile(t, appDir, assetDirectoryName, "ignored first")
	collector := newDevSnapshotCollector(appDir)

	initial, err := collector.collect()
	if err != nil {
		t.Fatalf("initial collect() error: %v", err)
	}
	if got := initial.files[assetDirectoryName+"/"]; got != "missing" {
		t.Fatalf("non-directory auto assets marker = %q, want missing", got)
	}
	if _, ok := initial.files[assetDirectoryName]; ok {
		t.Fatalf("non-directory auto assets content was fingerprinted: %#v", initial.files)
	}
	if _, ok := initial.files[indexHTMLAssetName]; !ok {
		t.Fatalf("fallback index missing from snapshot: %#v", initial.paths())
	}

	writeTestFile(t, appDir, assetDirectoryName, "ignored second")
	withChangedIgnoredFile, err := collector.collect()
	if err != nil {
		t.Fatalf("collect() after ignored file change: %v", err)
	}
	if !devSnapshotsEqual(initial, withChangedIgnoredFile) {
		t.Fatalf("ignored auto assets content changed snapshot paths: %#v", diffDevSnapshots(initial, withChangedIgnoredFile))
	}

	writeTestFile(t, appDir, "main.go", "package main\n\nvar sourceChanged = true\n")
	withSourceChange, err := collector.collect()
	if err != nil {
		t.Fatalf("collect() after source change: %v", err)
	}
	assertDevChangedPath(t, withChangedIgnoredFile, withSourceChange, "main.go")

	if err := os.Remove(filepath.Join(appDir, assetDirectoryName)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(appDir, assetDirectoryName), 0o755); err != nil {
		t.Fatal(err)
	}
	withDirectory, err := collector.collect()
	if err != nil {
		t.Fatalf("collect() after directory transition: %v", err)
	}
	if got := withDirectory.files[assetDirectoryName+"/"]; got != "directory" {
		t.Fatalf("auto assets directory marker = %q, want directory", got)
	}
	if _, ok := withDirectory.files[indexHTMLAssetName]; ok {
		t.Fatalf("fallback index remained effective with assets directory: %#v", withDirectory.paths())
	}
	assertDevChangedPath(t, withSourceChange, withDirectory, assetDirectoryName+"/")

	writeTestFile(t, appDir, "assets/styles.css", "body{}")
	withDirectoryAsset, err := collector.collect()
	if err != nil {
		t.Fatalf("collect() after directory asset creation: %v", err)
	}
	if _, ok := withDirectoryAsset.files["assets/styles.css"]; !ok {
		t.Fatalf("directory asset missing from snapshot: %#v", withDirectoryAsset.paths())
	}
	assertDevChangedPath(t, withDirectory, withDirectoryAsset, "assets/styles.css")

	if err := os.RemoveAll(filepath.Join(appDir, assetDirectoryName)); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, appDir, assetDirectoryName, "ignored again")
	withFallback, err := collector.collect()
	if err != nil {
		t.Fatalf("collect() after fallback transition: %v", err)
	}
	if got := withFallback.files[assetDirectoryName+"/"]; got != "missing" {
		t.Fatalf("fallback auto assets marker = %q, want missing", got)
	}
	if _, ok := withFallback.files[indexHTMLAssetName]; !ok {
		t.Fatalf("root index missing after fallback transition: %#v", withFallback.paths())
	}
	if _, ok := withFallback.files["assets/styles.css"]; ok {
		t.Fatalf("removed directory asset remained in snapshot: %#v", withFallback.paths())
	}
	assertDevChangedPath(t, withDirectoryAsset, withFallback, assetDirectoryName+"/")

	if err := os.Remove(filepath.Join(appDir, assetDirectoryName)); err != nil {
		t.Fatal(err)
	}
	withMissingPath, err := collector.collect()
	if err != nil {
		t.Fatalf("collect() after removing ignored path: %v", err)
	}
	if !devSnapshotsEqual(withFallback, withMissingPath) {
		t.Fatalf("missing and non-directory auto assets differ: %#v", diffDevSnapshots(withFallback, withMissingPath))
	}
}

func TestDevSnapshotExplicitAssetDirectoryRejectsNonDirectory(t *testing.T) {
	appDir := t.TempDir()
	writeTestFile(t, appDir, manifestName, `{"compiler":"go","assets":"assets"}`)
	writeTestFile(t, appDir, "main.go", "package main\n")
	writeTestFile(t, appDir, assetDirectoryName, "not a directory")

	_, err := newDevSnapshotCollector(appDir).collect()
	if err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("collect() error = %v, want explicit asset-directory rejection", err)
	}
}

func TestDevSnapshotUsesLastAssetsDuringMalformedManifest(t *testing.T) {
	appDir := t.TempDir()
	writeTestFile(t, appDir, manifestName, `{"compiler":"go","assets":"assets"}`)
	writeTestFile(t, appDir, "main.go", "package main\n")
	writeTestFile(t, appDir, "assets/styles.css", "first")
	collector := newDevSnapshotCollector(appDir)

	initial, err := collector.collect()
	if err != nil {
		t.Fatalf("initial collect() error: %v", err)
	}
	writeTestFile(t, appDir, manifestName, `{"compiler":`)
	writeTestFile(t, appDir, "assets/styles.css", "second")
	broken, err := collector.collect()
	if err == nil || !strings.Contains(err.Error(), "parse goframe.json") {
		t.Fatalf("malformed collect error = %v", err)
	}
	if _, ok := broken.files["assets/styles.css"]; !ok {
		t.Fatalf("malformed-manifest snapshot lost last asset set: %#v", broken.paths())
	}
	assertDevChangedPath(t, initial, broken, "assets/styles.css")

	writeTestFile(t, appDir, manifestName, `{"compiler":"go","assets":"assets"}`)
	writeTestFile(t, appDir, "assets/new.txt", "recovered")
	recovered, err := collector.collect()
	if err != nil {
		t.Fatalf("recovered collect() error: %v", err)
	}
	if _, ok := recovered.files["assets/new.txt"]; !ok {
		t.Fatalf("recovered snapshot missing new asset: %#v", recovered.paths())
	}
}

func TestDevSnapshotRetainedAssetFileRejectsIntermediateSymlink(t *testing.T) {
	requireSymlinkSupport(t)
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	assetPath := ".site/static/styles.css"
	manifest := `{"compiler":"go","assets":[".site/static/styles.css"]}`
	writeTestFile(t, appDir, manifestName, manifest)
	writeTestFile(t, appDir, "main.go", "package main\n")
	writeTestFile(t, appDir, assetPath, "local first")
	collector := newDevSnapshotCollector(appDir)

	healthy, err := collector.collect()
	if err != nil {
		t.Fatalf("initial collect() error: %v", err)
	}
	if _, ok := healthy.files[assetPath]; !ok {
		t.Fatalf("initial snapshot missing retained asset file: %#v", healthy.paths())
	}

	writeTestFile(t, appDir, manifestName, `{"compiler":`)
	staticPath := filepath.Join(appDir, ".site", "static")
	if err := os.RemoveAll(staticPath); err != nil {
		t.Fatal(err)
	}
	externalDir := filepath.Join(root, "external-file-assets")
	writeTestFile(t, externalDir, "styles.css", "external first")
	if err := os.Symlink(externalDir, staticPath); err != nil {
		t.Fatal(err)
	}

	unsafeSnapshot, err := collector.collect()
	assertDevBuildableScanError(t, err, "parse goframe.json", "symlink")
	if _, ok := unsafeSnapshot.files[assetPath]; ok {
		t.Fatalf("external retained asset file was fingerprinted: %#v", unsafeSnapshot.files)
	}
	unsafeError := err.Error()

	writeTestFile(t, externalDir, "styles.css", "external second")
	changedExternal, err := collector.collect()
	assertDevBuildableScanError(t, err, "parse goframe.json", "symlink")
	if err.Error() != unsafeError {
		t.Fatalf("repeated retained-file error changed:\nfirst:  %s\nsecond: %s", unsafeError, err)
	}
	if !devSnapshotsEqual(unsafeSnapshot, changedExternal) {
		t.Fatalf("external file content changed safe snapshot: %#v", diffDevSnapshots(unsafeSnapshot, changedExternal))
	}

	if err := os.Remove(staticPath); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, appDir, assetPath, "local restored")
	retained, err := collector.collect()
	assertDevBuildableScanError(t, err, "parse goframe.json")
	if strings.Contains(err.Error(), "symlink") {
		t.Fatalf("restored retained-file error still reports symlink: %v", err)
	}
	if _, ok := retained.files[assetPath]; !ok {
		t.Fatalf("restored retained asset file missing: %#v", retained.paths())
	}

	writeTestFile(t, appDir, manifestName, manifest)
	recovered, err := collector.collect()
	if err != nil {
		t.Fatalf("recovered collect() error: %v", err)
	}
	if _, ok := recovered.files[assetPath]; !ok {
		t.Fatalf("recovered snapshot missing local asset file: %#v", recovered.paths())
	}
}

func TestDevSnapshotRetainedAssetDirectoryRejectsIntermediateSymlink(t *testing.T) {
	requireSymlinkSupport(t)
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	assetDirectory := ".site/static/public"
	assetPath := assetDirectory + "/data.txt"
	manifest := `{"compiler":"go","assets":".site/static/public"}`
	writeTestFile(t, appDir, manifestName, manifest)
	writeTestFile(t, appDir, "main.go", "package main\n")
	writeTestFile(t, appDir, assetPath, "local first")
	collector := newDevSnapshotCollector(appDir)

	healthy, err := collector.collect()
	if err != nil {
		t.Fatalf("initial collect() error: %v", err)
	}
	if _, ok := healthy.files[assetPath]; !ok {
		t.Fatalf("initial snapshot missing retained directory asset: %#v", healthy.paths())
	}

	writeTestFile(t, appDir, manifestName, `{"compiler":`)
	staticPath := filepath.Join(appDir, ".site", "static")
	if err := os.RemoveAll(staticPath); err != nil {
		t.Fatal(err)
	}
	externalDir := filepath.Join(root, "external-directory-assets")
	writeTestFile(t, externalDir, "public/data.txt", "external first")
	if err := os.Symlink(externalDir, staticPath); err != nil {
		t.Fatal(err)
	}

	unsafeSnapshot, err := collector.collect()
	assertDevBuildableScanError(t, err, "parse goframe.json", "symlink")
	if _, ok := unsafeSnapshot.files[assetDirectory+"/"]; ok {
		t.Fatalf("external retained asset directory was traversed: %#v", unsafeSnapshot.files)
	}
	if _, ok := unsafeSnapshot.files[assetPath]; ok {
		t.Fatalf("external retained directory asset was fingerprinted: %#v", unsafeSnapshot.files)
	}
	unsafeError := err.Error()

	writeTestFile(t, externalDir, "public/data.txt", "external second")
	changedExternal, err := collector.collect()
	assertDevBuildableScanError(t, err, "parse goframe.json", "symlink")
	if err.Error() != unsafeError {
		t.Fatalf("repeated retained-directory error changed:\nfirst:  %s\nsecond: %s", unsafeError, err)
	}
	if !devSnapshotsEqual(unsafeSnapshot, changedExternal) {
		t.Fatalf("external directory content changed safe snapshot: %#v", diffDevSnapshots(unsafeSnapshot, changedExternal))
	}

	if err := os.Remove(staticPath); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, appDir, assetPath, "local restored")
	retained, err := collector.collect()
	assertDevBuildableScanError(t, err, "parse goframe.json")
	if strings.Contains(err.Error(), "symlink") {
		t.Fatalf("restored retained-directory error still reports symlink: %v", err)
	}
	if _, ok := retained.files[assetPath]; !ok {
		t.Fatalf("restored retained directory asset missing: %#v", retained.paths())
	}

	writeTestFile(t, appDir, manifestName, manifest)
	recovered, err := collector.collect()
	if err != nil {
		t.Fatalf("recovered collect() error: %v", err)
	}
	if _, ok := recovered.files[assetPath]; !ok {
		t.Fatalf("recovered snapshot missing local directory asset: %#v", recovered.paths())
	}
}

func TestDevSnapshotRejectsSymlinksWithoutTraversal(t *testing.T) {
	requireSymlinkSupport(t)
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	writeTestFile(t, appDir, manifestName, `{"compiler":"go","assets":"assets"}`)
	writeTestFile(t, appDir, "main.go", "package main\n")
	external := filepath.Join(root, "external.css")
	writeTestFile(t, root, "external.css", "secret")
	if err := os.MkdirAll(filepath.Join(appDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(appDir, "assets", "styles.css")); err != nil {
		t.Fatal(err)
	}

	_, err := newDevSnapshotCollector(appDir).collect()
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("collect() error = %v, want symlink rejection", err)
	}
	assertFileContent(t, external, "secret")
}

func TestDevStaticHandlerAddsNoStore(t *testing.T) {
	directory := t.TempDir()
	writeServeFixture(t, directory, "index.html", "dev")
	response := httptest.NewRecorder()
	devStaticHandler(directory).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || response.Body.String() != "dev" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

func TestDevCoordinatorDebouncesAndCoalescesBurst(t *testing.T) {
	scanner := newFakeDevScanner(devTestSnapshot("app.gox", "one"))
	builds := make(chan devBuildRequest, 8)
	ctx, cancel := context.WithCancel(context.Background())
	done := startDevCoordinator(t, ctx, scanner.scan, func(request devBuildRequest) error {
		builds <- request
		return nil
	}, 3*time.Millisecond, 40*time.Millisecond, nil)
	initial := waitDevBuild(t, builds)
	if !initial.Initial || initial.Number != 1 {
		t.Fatalf("initial request = %+v", initial)
	}

	scanner.set(devTestSnapshot("app.gox", "two"), nil)
	time.Sleep(8 * time.Millisecond)
	scanner.set(devTestSnapshot("app.gox", "three"), nil)
	time.Sleep(8 * time.Millisecond)
	scanner.set(devTestSnapshot("app.gox", "four"), nil)
	rebuild := waitDevBuild(t, builds)
	if rebuild.Initial || rebuild.Number != 2 || strings.Join(rebuild.Changed, ",") != "app.gox" {
		t.Fatalf("rebuild request = %+v", rebuild)
	}
	assertNoDevBuild(t, builds, 100*time.Millisecond)
	cancel()
	if err := waitDevDone(t, done); err != nil {
		t.Fatalf("coordinator error: %v", err)
	}
}

func TestDevCoordinatorSerializesAndQueuesOneFollowUp(t *testing.T) {
	scanner := newFakeDevScanner(devTestSnapshot("app.gox", "one"))
	builds := make(chan devBuildRequest, 8)
	releaseSecond := make(chan struct{})
	var active atomic.Int32
	var maximum atomic.Int32
	build := func(request devBuildRequest) error {
		current := active.Add(1)
		for {
			observed := maximum.Load()
			if current <= observed || maximum.CompareAndSwap(observed, current) {
				break
			}
		}
		builds <- request
		if request.Number == 2 {
			<-releaseSecond
		}
		active.Add(-1)
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := startDevCoordinator(t, ctx, scanner.scan, build, 3*time.Millisecond, 20*time.Millisecond, nil)
	waitDevBuild(t, builds)
	scanner.set(devTestSnapshot("app.gox", "two"), nil)
	second := waitDevBuild(t, builds)
	if second.Number != 2 {
		t.Fatalf("second request = %+v", second)
	}
	scanner.set(devTestSnapshot("app.gox", "three"), nil)
	time.Sleep(10 * time.Millisecond)
	scanner.set(devTestSnapshot("app.gox", "four"), nil)
	close(releaseSecond)
	third := waitDevBuild(t, builds)
	if third.Number != 3 || strings.Join(third.Changed, ",") != "app.gox" {
		t.Fatalf("follow-up request = %+v", third)
	}
	assertNoDevBuild(t, builds, 80*time.Millisecond)
	if got := maximum.Load(); got != 1 {
		t.Fatalf("maximum concurrent builds = %d, want 1", got)
	}
	cancel()
	if err := waitDevDone(t, done); err != nil {
		t.Fatal(err)
	}
}

func TestDevCoordinatorFailureWaitsForAnotherChange(t *testing.T) {
	scanner := newFakeDevScanner(devTestSnapshot("app.gox", "one"))
	builds := make(chan devBuildRequest, 8)
	ctx, cancel := context.WithCancel(context.Background())
	done := startDevCoordinator(t, ctx, scanner.scan, func(request devBuildRequest) error {
		builds <- request
		if request.Number == 2 {
			return errors.New("authored source failed")
		}
		return nil
	}, 3*time.Millisecond, 20*time.Millisecond, nil)
	waitDevBuild(t, builds)
	scanner.set(devTestSnapshot("app.gox", "broken"), nil)
	failed := waitDevBuild(t, builds)
	if failed.Number != 2 {
		t.Fatalf("failed request = %+v", failed)
	}
	assertNoDevBuild(t, builds, 80*time.Millisecond)
	scanner.set(devTestSnapshot("app.gox", "fixed"), nil)
	recovered := waitDevBuild(t, builds)
	if recovered.Number != 3 {
		t.Fatalf("recovery request = %+v", recovered)
	}
	cancel()
	if err := waitDevDone(t, done); err != nil {
		t.Fatal(err)
	}
}

func TestDevCoordinatorCancellationDuringBuildStartsNoFollowUp(t *testing.T) {
	scanner := newFakeDevScanner(devTestSnapshot("app.gox", "one"))
	builds := make(chan devBuildRequest, 8)
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	done := startDevCoordinator(t, ctx, scanner.scan, func(request devBuildRequest) error {
		builds <- request
		if request.Number == 2 {
			<-release
		}
		return nil
	}, 3*time.Millisecond, 20*time.Millisecond, nil)
	waitDevBuild(t, builds)
	scanner.set(devTestSnapshot("app.gox", "two"), nil)
	waitDevBuild(t, builds)
	scanner.set(devTestSnapshot("app.gox", "three"), nil)
	cancel()
	close(release)
	if err := waitDevDone(t, done); err != nil {
		t.Fatal(err)
	}
	assertNoDevBuild(t, builds, 40*time.Millisecond)
}

func TestDevCoordinatorCancellationDuringIdleStops(t *testing.T) {
	scanner := newFakeDevScanner(devTestSnapshot("app.gox", "one"))
	builds := make(chan devBuildRequest, 2)
	ctx, cancel := context.WithCancel(context.Background())
	done := startDevCoordinator(t, ctx, scanner.scan, func(request devBuildRequest) error {
		builds <- request
		return nil
	}, 3*time.Millisecond, 20*time.Millisecond, nil)
	waitDevBuild(t, builds)
	cancel()
	if err := waitDevDone(t, done); err != nil {
		t.Fatal(err)
	}
	assertNoDevBuild(t, builds, 40*time.Millisecond)
}

func TestDevCoordinatorSuppressesRepeatedScanErrors(t *testing.T) {
	scanner := newFakeDevScanner(devTestSnapshot("app.gox", "one"))
	scanner.set(devTestSnapshot("app.gox", "one"), errors.New("unreadable watched input"))
	builds := make(chan devBuildRequest, 8)
	var stderr bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	done := startDevCoordinator(t, ctx, scanner.scan, func(request devBuildRequest) error {
		builds <- request
		return nil
	}, 3*time.Millisecond, 20*time.Millisecond, &stderr)
	waitDevBuild(t, builds)
	time.Sleep(40 * time.Millisecond)
	if got := strings.Count(stderr.String(), "dev watch error:"); got != 1 {
		t.Fatalf("watch error count = %d, output:\n%s", got, stderr.String())
	}
	scanner.set(devTestSnapshot("app.gox", "one"), nil)
	waitDevBuild(t, builds)
	cancel()
	if err := waitDevDone(t, done); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(stderr.String(), "dev watch recovered"); got != 1 {
		t.Fatalf("watch recovery count = %d, output:\n%s", got, stderr.String())
	}
}

func TestDevRunTreatsListenerFailureAsFatal(t *testing.T) {
	appDir := t.TempDir()
	writeTestFile(t, appDir, manifestName, `{"compiler":"go"}`)
	writeTestFile(t, appDir, "main.go", "package main\n")
	packageCalls := 0
	dependencies := defaultDevDependencies()
	dependencies.stdout = &bytes.Buffer{}
	dependencies.stderr = &bytes.Buffer{}
	dependencies.packageApp = func(options packageOptions) error {
		packageCalls++
		layout, err := newBuildLayout(layoutOptions{appDir: options.appDir, compiler: "go", workspace: options.workspace})
		if err != nil {
			return err
		}
		if err := os.MkdirAll(layout.PackageDir, 0o755); err != nil {
			return err
		}
		writeCompleteCurrentPackage(t, layout.PackageDir)
		return nil
	}
	dependencies.listen = func(string, string) (net.Listener, error) {
		return nil, errors.New("address unavailable")
	}
	err := runDev(context.Background(), devOptions{appDir: appDir, port: 0}, dependencies)
	if err == nil || !strings.Contains(err.Error(), "start development server") {
		t.Fatalf("runDev() error = %v, want listener failure", err)
	}
	if packageCalls != 1 {
		t.Fatalf("package calls = %d, want 1", packageCalls)
	}
}

func assertDevChangedPath(t *testing.T, previous, current devSnapshot, want string) {
	t.Helper()
	for _, path := range diffDevSnapshots(previous, current) {
		if path == want {
			return
		}
	}
	t.Fatalf("changed paths = %#v, want %q", diffDevSnapshots(previous, current), want)
}

func assertDevBuildableScanError(t *testing.T, err error, contains ...string) {
	t.Helper()
	var buildable devBuildableScanError
	if !errors.As(err, &buildable) {
		t.Fatalf("collect() error = %v, want devBuildableScanError", err)
	}
	for _, want := range contains {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("collect() error = %v, want %q", err, want)
		}
	}
}

type fakeDevScanner struct {
	mu       sync.Mutex
	snapshot devSnapshot
	err      error
}

func newFakeDevScanner(snapshot devSnapshot) *fakeDevScanner {
	return &fakeDevScanner{snapshot: snapshot}
}

func (scanner *fakeDevScanner) set(snapshot devSnapshot, err error) {
	scanner.mu.Lock()
	defer scanner.mu.Unlock()
	scanner.snapshot = snapshot
	scanner.err = err
}

func (scanner *fakeDevScanner) scan() (devSnapshot, error) {
	scanner.mu.Lock()
	defer scanner.mu.Unlock()
	copy := newDevSnapshot()
	for path, fingerprint := range scanner.snapshot.files {
		copy.files[path] = fingerprint
	}
	return copy, scanner.err
}

func devTestSnapshot(values ...string) devSnapshot {
	if len(values)%2 != 0 {
		panic("devTestSnapshot requires path/fingerprint pairs")
	}
	snapshot := newDevSnapshot()
	for index := 0; index < len(values); index += 2 {
		snapshot.files[values[index]] = values[index+1]
	}
	return snapshot
}

func startDevCoordinator(
	t *testing.T,
	ctx context.Context,
	scan func() (devSnapshot, error),
	build func(devBuildRequest) error,
	pollInterval time.Duration,
	debounce time.Duration,
	stderr *bytes.Buffer,
) <-chan error {
	t.Helper()
	if stderr == nil {
		stderr = &bytes.Buffer{}
	}
	done := make(chan error, 1)
	go func() {
		done <- runDevCoordinator(ctx, devCoordinatorConfig{
			scan:         scan,
			build:        build,
			pollInterval: pollInterval,
			debounce:     debounce,
			stdout:       &bytes.Buffer{},
			stderr:       stderr,
		})
	}()
	return done
}

func waitDevBuild(t *testing.T, builds <-chan devBuildRequest) devBuildRequest {
	t.Helper()
	select {
	case request := <-builds:
		return request
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for development build")
		return devBuildRequest{}
	}
}

func assertNoDevBuild(t *testing.T, builds <-chan devBuildRequest, duration time.Duration) {
	t.Helper()
	select {
	case request := <-builds:
		t.Fatalf("unexpected development build: %+v", request)
	case <-time.After(duration):
	}
}

func waitDevDone(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for development coordinator shutdown")
		return nil
	}
}

func Example_parseDevOptions() {
	options, _ := parseDevOptions([]string{"app", "--port=0"})
	fmt.Println(options.appDir, options.port)
	// Output: app 0
}
