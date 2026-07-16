package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	devIntegrationPoll     = 10 * time.Millisecond
	devIntegrationDebounce = 50 * time.Millisecond
)

func TestDevRealWorkflow(t *testing.T) {
	repositoryRoot, ok := findRepositoryRoot(".")
	if !ok {
		t.Fatal("repository root not found")
	}
	appDir := filepath.Join(t.TempDir(), "app")
	workspace := filepath.Join(t.TempDir(), "workspace")
	writeDevIntegrationApp(t, appDir, repositoryRoot, "initial")

	t.Setenv("GOWORK", "off")
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOFLAGS", "-mod=mod")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	builds := make(chan devBuildEvent, 32)
	serverURLs := make(chan string, 2)
	dependencies := devIntegrationDependencies(builds, serverURLs)
	done := make(chan error, 1)
	go func() {
		done <- runDev(ctx, devOptions{
			appDir:    appDir,
			compiler:  "go",
			workspace: workspace,
			port:      0,
		}, dependencies)
	}()

	initial := waitDevEvent(t, builds)
	assertDevEvent(t, initial, 1, true, false)
	serverURL := waitDevServerURL(t, serverURLs)
	assertDevHTTPContains(t, serverURL+"/", "GoFrame dev integration")
	assertDevHTTPHeader(t, serverURL+"/", "Cache-Control", "no-store")
	initialMetadata := readDevHTTPBody(t, serverURL+"/"+packageMetadataName)
	if got := decodeDevPackageMetadata(t, initialMetadata).Name; got != "initial" {
		t.Fatalf("initial package name = %q, want initial", got)
	}

	writeDevIntegrationGOX(t, appDir, "GOX rebuild")
	assertDevEvent(t, waitDevEvent(t, builds), 2, false, false)

	writeTestFile(t, appDir, "message.go", "package main\n\nfunc message() string { return \"Go rebuild\" }\n")
	assertDevEvent(t, waitDevEvent(t, builds), 3, false, false)

	writeTestFile(t, appDir, "assets/message.txt", "asset rebuild")
	assertDevEvent(t, waitDevEvent(t, builds), 4, false, false)
	assertDevHTTPBody(t, serverURL+"/assets/message.txt", "asset rebuild")

	writeTestFile(t, appDir, manifestName, `{"name":"renamed","compiler":"tinygo","assets":"assets"}`)
	assertDevEvent(t, waitDevEvent(t, builds), 5, false, false)
	metadataBeforeFailure := readDevHTTPBody(t, serverURL+"/"+packageMetadataName)
	metadata := decodeDevPackageMetadata(t, metadataBeforeFailure)
	if metadata.Name != "renamed" || metadata.Compiler != "go" {
		t.Fatalf("manifest rebuild metadata = %+v, want renamed package with fixed Go override", metadata)
	}

	writeTestFile(t, appDir, "app.gox", `package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func App() gf.Node {
	return <main><p>broken</main>
}
`)
	failed := waitDevEvent(t, builds)
	assertDevEvent(t, failed, 6, false, true)
	if got := readDevHTTPBody(t, serverURL+"/"+packageMetadataName); got != metadataBeforeFailure {
		t.Fatalf("failed build replaced package metadata:\nold=%s\nnew=%s", metadataBeforeFailure, got)
	}
	assertDevHTTPBody(t, serverURL+"/assets/message.txt", "asset rebuild")

	writeDevIntegrationGOX(t, appDir, "recovered")
	assertDevEvent(t, waitDevEvent(t, builds), 7, false, false)
	metadataBeforeGoFailure := readDevHTTPBody(t, serverURL+"/"+packageMetadataName)

	writeTestFile(t, appDir, "message.go", "package main\n\nfunc message() string { return missingSymbol }\n")
	assertDevEvent(t, waitDevEvent(t, builds), 8, false, true)
	if got := readDevHTTPBody(t, serverURL+"/"+packageMetadataName); got != metadataBeforeGoFailure {
		t.Fatalf("Go compiler failure replaced package metadata:\nold=%s\nnew=%s", metadataBeforeGoFailure, got)
	}
	writeTestFile(t, appDir, "message.go", "package main\n\nfunc message() string { return \"Go recovered\" }\n")
	assertDevEvent(t, waitDevEvent(t, builds), 9, false, false)
	metadataBeforeManifestFailure := readDevHTTPBody(t, serverURL+"/"+packageMetadataName)

	writeTestFile(t, appDir, manifestName, `{"name":`)
	assertDevEvent(t, waitDevEvent(t, builds), 10, false, true)
	if got := readDevHTTPBody(t, serverURL+"/"+packageMetadataName); got != metadataBeforeManifestFailure {
		t.Fatalf("manifest failure replaced package metadata:\nold=%s\nnew=%s", metadataBeforeManifestFailure, got)
	}
	writeTestFile(t, appDir, manifestName, `{"name":"renamed","compiler":"tinygo","assets":"assets"}`)
	assertDevEvent(t, waitDevEvent(t, builds), 11, false, false)

	for _, value := range []string{"burst one", "burst two", "burst final"} {
		writeTestFile(t, appDir, "message.go", fmt.Sprintf("package main\n\nfunc message() string { return %q }\n", value))
		time.Sleep(8 * time.Millisecond)
	}
	burst := waitDevEvent(t, builds)
	assertDevEvent(t, burst, 12, false, false)
	assertNoDevEvent(t, builds, 3*devIntegrationDebounce)

	writeTestFile(t, appDir, ".goframe/ignored.go", "package ignored\n")
	assertNoDevEvent(t, builds, 3*devIntegrationDebounce)
	writeTestFile(t, workspace, "tool-output.txt", "ignored\n")
	assertNoDevEvent(t, builds, 3*devIntegrationDebounce)

	cancel()
	if err := waitDevRun(t, done); err != nil {
		t.Fatalf("runDev() shutdown error: %v", err)
	}
	waitDevListenerClosed(t, serverURL)
}

func TestDevInitialFailureRecoversBeforeServerStarts(t *testing.T) {
	repositoryRoot, ok := findRepositoryRoot(".")
	if !ok {
		t.Fatal("repository root not found")
	}
	appDir := filepath.Join(t.TempDir(), "app")
	workspace := filepath.Join(t.TempDir(), "workspace")
	writeDevIntegrationApp(t, appDir, repositoryRoot, "initial-failure")
	writeTestFile(t, appDir, "app.gox", `package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func App() gf.Node {
	return <main><p>broken</main>
}
`)
	t.Setenv("GOWORK", "off")
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOFLAGS", "-mod=mod")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	builds := make(chan devBuildEvent, 8)
	serverURLs := make(chan string, 2)
	done := make(chan error, 1)
	go func() {
		done <- runDev(ctx, devOptions{
			appDir:    appDir,
			workspace: workspace,
			port:      0,
		}, devIntegrationDependencies(builds, serverURLs))
	}()

	failed := waitDevEvent(t, builds)
	assertDevEvent(t, failed, 1, true, true)
	select {
	case url := <-serverURLs:
		t.Fatalf("server started after failed initial package: %s", url)
	case <-time.After(100 * time.Millisecond):
	}

	writeDevIntegrationGOX(t, appDir, "fixed initial source")
	recovered := waitDevEvent(t, builds)
	assertDevEvent(t, recovered, 2, false, false)
	serverURL := waitDevServerURL(t, serverURLs)
	assertDevHTTPContains(t, serverURL+"/", "GoFrame dev integration")
	cancel()
	if err := waitDevRun(t, done); err != nil {
		t.Fatal(err)
	}
	waitDevListenerClosed(t, serverURL)
}

func devIntegrationDependencies(builds chan<- devBuildEvent, serverURLs chan<- string) devDependencies {
	dependencies := defaultDevDependencies()
	dependencies.pollInterval = devIntegrationPoll
	dependencies.debounce = devIntegrationDebounce
	dependencies.stdout = io.Discard
	dependencies.stderr = io.Discard
	dependencies.hooks.BuildFinished = func(event devBuildEvent) {
		builds <- event
	}
	dependencies.hooks.ServerStarted = func(url string) {
		serverURLs <- url
	}
	return dependencies
}

func writeDevIntegrationApp(t *testing.T, appDir, repositoryRoot, name string) {
	t.Helper()
	writeTestFile(t, appDir, "go.mod", fmt.Sprintf(`module example.com/devapp

go 1.22

require %s v0.0.0

replace %s => %s
`, canonicalModulePath, canonicalModulePath, filepath.ToSlash(repositoryRoot)))
	writeTestFile(t, appDir, manifestName, fmt.Sprintf(`{"name":%q,"compiler":"go","assets":"assets"}`, name))
	writeTestFile(t, appDir, "main.go", `//go:build js && wasm

package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func main() {
	done := make(chan struct{})
	gf.Mount("root", App)
	<-done
}
`)
	writeTestFile(t, appDir, "message.go", "package main\n\nfunc message() string { return \"initial\" }\n")
	writeDevIntegrationGOX(t, appDir, "initial")
	writeTestFile(t, appDir, "assets/index.html", `<!doctype html>
<html><body><div id="root">GoFrame dev integration</div><script src="wasm_exec.js"></script><script>fetch("bundle.wasm")</script></body></html>
`)
	writeTestFile(t, appDir, "assets/message.txt", "initial asset")
}

func writeDevIntegrationGOX(t *testing.T, appDir, label string) {
	t.Helper()
	writeTestFile(t, appDir, "app.gox", fmt.Sprintf(`package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func App() gf.Node {
	return <main><h1>%s</h1><p>{message()}</p></main>
}
`, label))
}

func waitDevEvent(t *testing.T, builds <-chan devBuildEvent) devBuildEvent {
	t.Helper()
	select {
	case event := <-builds:
		return event
	case <-time.After(20 * time.Second):
		t.Fatal("timed out waiting for real development build")
		return devBuildEvent{}
	}
}

func assertDevEvent(t *testing.T, event devBuildEvent, number int, initial, wantFailure bool) {
	t.Helper()
	if event.Request.Number != number || event.Request.Initial != initial {
		t.Fatalf("build event = %+v, want number=%d initial=%v", event.Request, number, initial)
	}
	if wantFailure && event.Err == nil {
		t.Fatalf("build %d succeeded, want failure", number)
	}
	if !wantFailure && event.Err != nil {
		t.Fatalf("build %d failed: %v", number, event.Err)
	}
}

func assertNoDevEvent(t *testing.T, builds <-chan devBuildEvent, duration time.Duration) {
	t.Helper()
	select {
	case event := <-builds:
		t.Fatalf("unexpected development build event: %+v", event)
	case <-time.After(duration):
	}
}

func waitDevServerURL(t *testing.T, urls <-chan string) string {
	t.Helper()
	select {
	case url := <-urls:
		return url
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for development server URL")
		return ""
	}
}

func readDevHTTPBody(t *testing.T, url string) string {
	t.Helper()
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer response.Body.Close()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s: %v", url, err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, body=%q", url, response.StatusCode, content)
	}
	return string(content)
}

func assertDevHTTPBody(t *testing.T, url, want string) {
	t.Helper()
	if got := readDevHTTPBody(t, url); got != want {
		t.Fatalf("GET %s body = %q, want %q", url, got, want)
	}
}

func assertDevHTTPContains(t *testing.T, url, want string) {
	t.Helper()
	if got := readDevHTTPBody(t, url); !strings.Contains(got, want) {
		t.Fatalf("GET %s body does not contain %q: %q", url, want, got)
	}
}

func assertDevHTTPHeader(t *testing.T, url, name, want string) {
	t.Helper()
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if got := response.Header.Get(name); got != want {
		t.Fatalf("GET %s %s = %q, want %q", url, name, got, want)
	}
}

func decodeDevPackageMetadata(t *testing.T, content string) packageMetadata {
	t.Helper()
	var metadata packageMetadata
	if err := json.Unmarshal([]byte(content), &metadata); err != nil {
		t.Fatalf("decode package metadata: %v\n%s", err, content)
	}
	return metadata
}

func waitDevRun(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for development command shutdown")
		return nil
	}
}

func waitDevListenerClosed(t *testing.T, serverURL string) {
	t.Helper()
	client := &http.Client{Timeout: 100 * time.Millisecond}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(serverURL + "/")
		if err != nil {
			return
		}
		response.Body.Close()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("development listener at %s remained open after shutdown", serverURL)
}
