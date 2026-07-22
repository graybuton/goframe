package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
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
	generations := make(chan uint64, 16)
	reloads := make(chan uint64, 16)
	dependencies := devIntegrationDependencies(builds, serverURLs)
	dependencies.hooks.GenerationActivated = func(generation uint64) {
		generations <- generation
	}
	dependencies.hooks.ReloadPublished = func(generation uint64) {
		reloads <- generation
	}
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
	assertDevGenerationEvent(t, generations, 1)
	assertNoDevGenerationEvent(t, reloads)
	serverURL := waitDevServerURL(t, serverURLs)
	assertDevHTTPContains(t, serverURL+"/", "GoFrame dev integration")
	assertDevHTTPHeader(t, serverURL+"/", "Cache-Control", "no-store")
	initialMetadata := readDevHTTPBody(t, serverURL+"/"+packageMetadataName)
	if got := decodeDevPackageMetadata(t, initialMetadata).Name; got != "initial" {
		t.Fatalf("initial package name = %q, want initial", got)
	}

	writeDevIntegrationGOX(t, appDir, "GOX rebuild")
	assertDevEvent(t, waitDevEvent(t, builds), 2, false, false)
	assertDevGenerationEvent(t, generations, 2)
	assertDevGenerationEvent(t, reloads, 2)

	writeTestFile(t, appDir, "message.go", "package main\n\nfunc message() string { return \"Go rebuild\" }\n")
	assertDevEvent(t, waitDevEvent(t, builds), 3, false, false)
	assertDevGenerationEvent(t, generations, 3)
	assertDevGenerationEvent(t, reloads, 3)

	writeTestFile(t, appDir, "assets/message.txt", "asset rebuild")
	assertDevEvent(t, waitDevEvent(t, builds), 4, false, false)
	assertDevGenerationEvent(t, generations, 4)
	assertDevGenerationEvent(t, reloads, 4)
	assertDevHTTPBody(t, serverURL+"/assets/message.txt", "asset rebuild")

	writeTestFile(t, appDir, manifestName, `{"name":"renamed","compiler":"tinygo","assets":"assets"}`)
	assertDevEvent(t, waitDevEvent(t, builds), 5, false, false)
	assertDevGenerationEvent(t, generations, 5)
	assertDevGenerationEvent(t, reloads, 5)
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
	assertNoDevGenerationEvent(t, generations)
	assertNoDevGenerationEvent(t, reloads)
	if got := readDevHTTPBody(t, serverURL+"/"+packageMetadataName); got != metadataBeforeFailure {
		t.Fatalf("failed build replaced package metadata:\nold=%s\nnew=%s", metadataBeforeFailure, got)
	}
	assertDevHTTPBody(t, serverURL+"/assets/message.txt", "asset rebuild")

	writeDevIntegrationGOX(t, appDir, "recovered")
	assertDevEvent(t, waitDevEvent(t, builds), 7, false, false)
	assertDevGenerationEvent(t, generations, 6)
	assertDevGenerationEvent(t, reloads, 6)
	metadataBeforeGoFailure := readDevHTTPBody(t, serverURL+"/"+packageMetadataName)

	writeTestFile(t, appDir, "message.go", "package main\n\nfunc message() string { return missingSymbol }\n")
	assertDevEvent(t, waitDevEvent(t, builds), 8, false, true)
	assertNoDevGenerationEvent(t, generations)
	assertNoDevGenerationEvent(t, reloads)
	if got := readDevHTTPBody(t, serverURL+"/"+packageMetadataName); got != metadataBeforeGoFailure {
		t.Fatalf("Go compiler failure replaced package metadata:\nold=%s\nnew=%s", metadataBeforeGoFailure, got)
	}
	writeTestFile(t, appDir, "message.go", "package main\n\nfunc message() string { return \"Go recovered\" }\n")
	assertDevEvent(t, waitDevEvent(t, builds), 9, false, false)
	assertDevGenerationEvent(t, generations, 7)
	assertDevGenerationEvent(t, reloads, 7)
	metadataBeforeManifestFailure := readDevHTTPBody(t, serverURL+"/"+packageMetadataName)

	writeTestFile(t, appDir, manifestName, `{"name":`)
	assertDevEvent(t, waitDevEvent(t, builds), 10, false, true)
	assertNoDevGenerationEvent(t, generations)
	assertNoDevGenerationEvent(t, reloads)
	if got := readDevHTTPBody(t, serverURL+"/"+packageMetadataName); got != metadataBeforeManifestFailure {
		t.Fatalf("manifest failure replaced package metadata:\nold=%s\nnew=%s", metadataBeforeManifestFailure, got)
	}
	writeTestFile(t, appDir, manifestName, `{"name":"renamed","compiler":"tinygo","assets":"assets"}`)
	assertDevEvent(t, waitDevEvent(t, builds), 11, false, false)
	assertDevGenerationEvent(t, generations, 8)
	assertDevGenerationEvent(t, reloads, 8)

	for _, value := range []string{"burst one", "burst two", "burst final"} {
		writeTestFile(t, appDir, "message.go", fmt.Sprintf("package main\n\nfunc message() string { return %q }\n", value))
		time.Sleep(8 * time.Millisecond)
	}
	burst := waitDevEvent(t, builds)
	assertDevEvent(t, burst, 12, false, false)
	assertDevGenerationEvent(t, generations, 9)
	assertDevGenerationEvent(t, reloads, 9)
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

func TestDevEnvironmentIsolationRealWorkflow(t *testing.T) {
	repositoryRoot, ok := findRepositoryRoot(".")
	if !ok {
		t.Fatal("repository root not found")
	}
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	workspace := filepath.Join(t.TempDir(), "workspace")
	writeDevIntegrationApp(t, appDir, repositoryRoot, "environment-isolation")
	workPath, workContent := writeHostileParentWorkspace(t, root)
	setHostileCompilerWorkflowEnvironment(t, workPath, "-mod=vendor")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	builds := make(chan devBuildEvent, 8)
	serverURLs := make(chan string, 1)
	generations := make(chan uint64, 4)
	reloads := make(chan uint64, 4)
	dependencies := devIntegrationDependencies(builds, serverURLs)
	dependencies.hooks.GenerationActivated = func(generation uint64) {
		generations <- generation
	}
	dependencies.hooks.ReloadPublished = func(generation uint64) {
		reloads <- generation
	}
	done := make(chan error, 1)
	go func() {
		done <- runDev(ctx, devOptions{
			appDir: appDir, compiler: "go", workspace: workspace, port: 0,
		}, dependencies)
	}()

	assertDevEvent(t, waitDevEvent(t, builds), 1, true, false)
	assertDevGenerationEvent(t, generations, 1)
	assertNoDevGenerationEvent(t, reloads)
	serverURL := waitDevServerURL(t, serverURLs)
	assertDevHTTPContains(t, serverURL+"/", "GoFrame dev integration")

	writeTestFile(t, appDir, "message.go", "package main\n\nfunc message() string { return \"isolated rebuild\" }\n")
	assertDevEvent(t, waitDevEvent(t, builds), 2, false, false)
	assertDevGenerationEvent(t, generations, 2)
	assertDevGenerationEvent(t, reloads, 2)
	assertNoDevEvent(t, builds, 3*devIntegrationDebounce)
	assertTestFileUnchanged(t, workPath, workContent)

	cancel()
	if err := waitDevRun(t, done); err != nil {
		t.Fatalf("runDev() shutdown error: %v", err)
	}
	waitDevListenerClosed(t, serverURL)
}

func TestDevEmbedIntegrationRealWorkflow(t *testing.T) {
	repositoryRoot, ok := findRepositoryRoot(".")
	if !ok {
		t.Fatal("repository root not found")
	}
	appDir := filepath.Join(t.TempDir(), "app")
	workspace := filepath.Join(t.TempDir(), "workspace")
	writeDevEmbedIntegrationApp(t, appDir, repositoryRoot)

	t.Setenv("GOWORK", "off")
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOFLAGS", "-mod=mod")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	builds := make(chan devBuildEvent, 16)
	serverURLs := make(chan string, 2)
	generations := make(chan uint64, 8)
	reloads := make(chan uint64, 8)
	dependencies := devIntegrationDependencies(builds, serverURLs)
	dependencies.hooks.GenerationActivated = func(generation uint64) { generations <- generation }
	dependencies.hooks.ReloadPublished = func(generation uint64) { reloads <- generation }
	done := make(chan error, 1)
	go func() {
		done <- runDev(ctx, devOptions{
			appDir:    appDir,
			compiler:  "go",
			workspace: workspace,
			port:      0,
		}, dependencies)
	}()

	assertDevEvent(t, waitDevEvent(t, builds), 1, true, false)
	assertDevGenerationEvent(t, generations, 1)
	assertNoDevGenerationEvent(t, reloads)
	serverURL := waitDevServerURL(t, serverURLs)
	assertDevWASMContains(t, serverURL, "alpha")

	writeTestFile(t, appDir, "unrelated/deep/value.txt", "unrelated")
	assertNoDevEvent(t, builds, 3*devIntegrationDebounce)
	assertNoDevGenerationEvent(t, generations)
	assertNoDevGenerationEvent(t, reloads)

	writeTestFile(t, appDir, "message.txt", "beta")
	assertDevEvent(t, waitDevEvent(t, builds), 2, false, false)
	assertDevGenerationEvent(t, generations, 2)
	assertDevGenerationEvent(t, reloads, 2)
	assertDevWASMContains(t, serverURL, "beta")

	writeTestFile(t, appDir, "extras/second.txt", "second")
	assertDevEvent(t, waitDevEvent(t, builds), 3, false, false)
	assertDevGenerationEvent(t, generations, 3)
	assertDevGenerationEvent(t, reloads, 3)

	writeTestFile(t, appDir, "extras/ignored.bin", "ignored")
	assertNoDevEvent(t, builds, 3*devIntegrationDebounce)
	assertNoDevGenerationEvent(t, generations)
	assertNoDevGenerationEvent(t, reloads)

	if err := os.Remove(filepath.Join(appDir, "message.txt")); err != nil {
		t.Fatal(err)
	}
	assertDevEvent(t, waitDevEvent(t, builds), 4, false, true)
	assertNoDevGenerationEvent(t, generations)
	assertNoDevGenerationEvent(t, reloads)
	assertDevWASMContains(t, serverURL, "beta")

	writeTestFile(t, appDir, "message.txt", "gamma")
	assertDevEvent(t, waitDevEvent(t, builds), 5, false, false)
	assertDevGenerationEvent(t, generations, 4)
	assertDevGenerationEvent(t, reloads, 4)
	assertDevWASMContains(t, serverURL, "gamma")
	assertNoDevEvent(t, builds, 3*devIntegrationDebounce)

	cancel()
	if err := waitDevRun(t, done); err != nil {
		t.Fatalf("runDev() shutdown error: %v", err)
	}
	waitDevListenerClosed(t, serverURL)
}

func TestDevRealWorkflowServesCompletedGenerationDuringPublication(t *testing.T) {
	appDir := filepath.Join(t.TempDir(), "app")
	workspace := filepath.Join(t.TempDir(), "workspace")
	writeTestFile(t, appDir, manifestName, `{"compiler":"go"}`)
	writeTestFile(t, appDir, "main.go", "package main\n")
	oldPackage := t.TempDir()
	newPackage := t.TempDir()
	writeDevGenerationPackage(t, oldPackage, "old completed index")
	writeDevGenerationPackage(t, newPackage, "new completed index")
	if err := os.WriteFile(filepath.Join(oldPackage, "assets", "bundle.12345678.wasm"), []byte("old completed wasm"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newPackage, "assets", "bundle.12345678.wasm"), []byte("new completed wasm"), 0o644); err != nil {
		t.Fatal(err)
	}

	builds := make(chan devBuildEvent, 4)
	serverURLs := make(chan string, 1)
	generations := make(chan uint64, 4)
	reloads := make(chan uint64, 4)
	publicationOpen := make(chan struct{})
	releasePublication := make(chan struct{})
	packageCalls := 0
	dependencies := devIntegrationDependencies(builds, serverURLs)
	dependencies.hooks.GenerationActivated = func(generation uint64) {
		generations <- generation
	}
	dependencies.hooks.ReloadPublished = func(generation uint64) {
		reloads <- generation
	}
	dependencies.packageApp = func(options packageOptions) error {
		packageCalls++
		layout, err := newBuildLayout(layoutOptions{appDir: options.appDir, compiler: "go", workspace: options.workspace})
		if err != nil {
			return err
		}
		if packageCalls == 1 {
			return replaceDevCanonicalPackage(oldPackage, layout.PackageDir)
		}
		if err := os.RemoveAll(layout.PackageDir); err != nil {
			return err
		}
		if err := os.MkdirAll(layout.PackageDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(layout.PackageDir, indexHTMLAssetName), []byte("partial canonical index"), 0o644); err != nil {
			return err
		}
		close(publicationOpen)
		<-releasePublication
		return replaceDevCanonicalPackage(newPackage, layout.PackageDir)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runDev(ctx, devOptions{appDir: appDir, compiler: "go", workspace: workspace, port: 0}, dependencies)
	}()

	assertDevEvent(t, waitDevEvent(t, builds), 1, true, false)
	assertDevGenerationEvent(t, generations, 1)
	assertNoDevGenerationEvent(t, reloads)
	serverURL := waitDevServerURL(t, serverURLs)
	oldMetadata := readDevHTTPBody(t, serverURL+"/"+packageMetadataName)
	assertDevInjectedHTTPBody(t, serverURL+"/", "old completed index", 1)
	assertDevHTTPBody(t, serverURL+"/assets/bundle.12345678.wasm", "old completed wasm")

	writeTestFile(t, appDir, "main.go", "package main\n\nvar rebuild = true\n")
	waitDevSignal(t, publicationOpen, "mutable canonical publication")
	for request := 0; request < 12; request++ {
		assertDevInjectedHTTPBody(t, serverURL+"/", "old completed index", 1)
		assertDevHTTPBody(t, serverURL+"/"+packageMetadataName, oldMetadata)
		assertDevHTTPBody(t, serverURL+"/assets/bundle.12345678.wasm", "old completed wasm")
	}
	close(releasePublication)
	assertDevEvent(t, waitDevEvent(t, builds), 2, false, false)
	assertDevGenerationEvent(t, generations, 2)
	assertDevGenerationEvent(t, reloads, 2)
	assertDevInjectedHTTPBody(t, serverURL+"/", "new completed index", 2)
	assertDevHTTPBody(t, serverURL+"/assets/bundle.12345678.wasm", "new completed wasm")

	cancel()
	if err := waitDevRun(t, done); err != nil {
		t.Fatal(err)
	}
	waitDevListenerClosed(t, serverURL)
}

func TestDevCancellationDuringBuildSkipsGenerationAndReload(t *testing.T) {
	t.Run("later rebuild", func(t *testing.T) {
		appDir := filepath.Join(t.TempDir(), "app")
		workspace := filepath.Join(t.TempDir(), "workspace")
		writeTestFile(t, appDir, manifestName, `{"compiler":"go"}`)
		writeTestFile(t, appDir, "main.go", "package main\n")
		completedPackage := t.TempDir()
		writeDevGenerationPackage(t, completedPackage, "initial completed index")

		builds := make(chan devBuildEvent, 4)
		serverURLs := make(chan string, 1)
		generations := make(chan uint64, 4)
		reloads := make(chan uint64, 4)
		packageCalls := make(chan int, 4)
		releaseSecond := make(chan struct{})
		callNumber := 0
		dependencies := devIntegrationDependencies(builds, serverURLs)
		dependencies.hooks.GenerationActivated = func(generation uint64) {
			generations <- generation
		}
		dependencies.hooks.ReloadPublished = func(generation uint64) {
			reloads <- generation
		}
		dependencies.packageApp = func(options packageOptions) error {
			callNumber++
			packageCalls <- callNumber
			if callNumber == 2 {
				<-releaseSecond
				return nil
			}
			if callNumber != 1 {
				return fmt.Errorf("unexpected package call %d", callNumber)
			}
			layout, err := newBuildLayout(layoutOptions{appDir: options.appDir, compiler: "go", workspace: options.workspace})
			if err != nil {
				return err
			}
			return replaceDevCanonicalPackage(completedPackage, layout.PackageDir)
		}

		rootsBefore := devGenerationRootSet(t)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- runDev(ctx, devOptions{appDir: appDir, compiler: "go", workspace: workspace, port: 0}, dependencies)
		}()

		assertDevPackageCall(t, packageCalls, 1)
		assertDevEvent(t, waitDevEvent(t, builds), 1, true, false)
		assertDevGenerationEvent(t, generations, 1)
		assertNoDevGenerationEvent(t, reloads)
		serverURL := waitDevServerURL(t, serverURLs)
		generationRoot := assertOneNewDevGenerationRoot(t, rootsBefore)

		writeTestFile(t, appDir, "main.go", "package main\n\nvar rebuild = true\n")
		assertDevPackageCall(t, packageCalls, 2)
		writeTestFile(t, appDir, "main.go", "package main\n\nvar followUp = true\n")
		cancel()
		close(releaseSecond)

		assertDevEvent(t, waitDevEvent(t, builds), 2, false, false)
		if err := waitDevRun(t, done); err != nil {
			t.Fatalf("runDev() shutdown error: %v", err)
		}
		waitDevListenerClosed(t, serverURL)
		assertNoDevGenerationEvent(t, generations)
		assertNoDevGenerationEvent(t, reloads)
		assertNoDevPackageCall(t, packageCalls)
		if _, err := os.Stat(generationRoot); !os.IsNotExist(err) {
			t.Fatalf("generation root remained after canceled rebuild: %v", err)
		}
	})

	t.Run("initial build", func(t *testing.T) {
		appDir := filepath.Join(t.TempDir(), "app")
		workspace := filepath.Join(t.TempDir(), "workspace")
		writeTestFile(t, appDir, manifestName, `{"compiler":"go"}`)
		writeTestFile(t, appDir, "main.go", "package main\n")

		builds := make(chan devBuildEvent, 2)
		serverURLs := make(chan string, 1)
		generations := make(chan uint64, 2)
		reloads := make(chan uint64, 2)
		packageCalls := make(chan int, 2)
		releasePackage := make(chan struct{})
		dependencies := devIntegrationDependencies(builds, serverURLs)
		dependencies.hooks.GenerationActivated = func(generation uint64) {
			generations <- generation
		}
		dependencies.hooks.ReloadPublished = func(generation uint64) {
			reloads <- generation
		}
		dependencies.packageApp = func(packageOptions) error {
			packageCalls <- 1
			<-releasePackage
			return nil
		}

		rootsBefore := devGenerationRootSet(t)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- runDev(ctx, devOptions{appDir: appDir, compiler: "go", workspace: workspace, port: 0}, dependencies)
		}()

		assertDevPackageCall(t, packageCalls, 1)
		generationRoot := assertOneNewDevGenerationRoot(t, rootsBefore)
		cancel()
		close(releasePackage)
		assertDevEvent(t, waitDevEvent(t, builds), 1, true, false)
		if err := waitDevRun(t, done); err != nil {
			t.Fatalf("runDev() shutdown error: %v", err)
		}
		assertNoDevGenerationEvent(t, generations)
		assertNoDevGenerationEvent(t, reloads)
		assertNoDevPackageCall(t, packageCalls)
		select {
		case url := <-serverURLs:
			t.Fatalf("server started after canceled initial package: %s", url)
		default:
		}
		if _, err := os.Stat(generationRoot); !os.IsNotExist(err) {
			t.Fatalf("generation root remained after canceled initial build: %v", err)
		}
	})
}

func TestDevServerShutdownForceClosesActiveGenerationLease(t *testing.T) {
	packageDir := t.TempDir()
	writeDevGenerationPackage(t, packageDir, "completed generation")
	manager := newTestDevGenerationManager(t)
	if _, err := manager.activatePackage(packageDir); err != nil {
		t.Fatal(err)
	}
	root := manager.root

	leaseHeld := make(chan struct{})
	requestCanceled := make(chan struct{})
	handlerDone := make(chan error, 1)
	handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		lease, err := manager.acquire()
		if err != nil {
			handlerDone <- err
			return
		}
		close(leaseHeld)
		<-request.Context().Done()
		close(requestCanceled)
		handlerDone <- lease.Release()
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	httpServer := &http.Server{Handler: handler}
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- httpServer.Serve(listener)
	}()
	server := &devServer{
		generations:   manager,
		reload:        newDevReloadBroker(testDevReloadInstance),
		server:        httpServer,
		listener:      listener,
		shutdownGrace: 20 * time.Millisecond,
		shutdownDrain: time.Second,
	}

	requestDone := make(chan error, 1)
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	go func() {
		response, err := client.Get("http://" + listener.Addr().String() + "/blocked")
		if response != nil {
			response.Body.Close()
		}
		requestDone <- err
	}()
	waitDevSignal(t, leaseHeld, "active HTTP generation lease")

	shutdownErr := server.shutdown()
	if !errors.Is(shutdownErr, context.DeadlineExceeded) || !strings.Contains(shutdownErr.Error(), "shut down development server") {
		t.Fatalf("shutdown() error = %v, want precise graceful deadline", shutdownErr)
	}
	waitDevSignal(t, requestCanceled, "forced request cancellation")
	select {
	case err := <-handlerDone:
		if err != nil {
			t.Fatalf("release generation lease: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("HTTP handler did not return after force close")
	}
	select {
	case <-requestDone:
	case <-time.After(time.Second):
		t.Fatal("HTTP request did not terminate after force close")
	}
	select {
	case err := <-serveDone:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Serve() error = %v, want http.ErrServerClosed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("HTTP server goroutine remained active after shutdown")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("generation root remained after forced shutdown: %v", err)
	}
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
	generations := make(chan uint64, 2)
	reloads := make(chan uint64, 2)
	dependencies := devIntegrationDependencies(builds, serverURLs)
	dependencies.hooks.GenerationActivated = func(generation uint64) {
		generations <- generation
	}
	dependencies.hooks.ReloadPublished = func(generation uint64) {
		reloads <- generation
	}
	done := make(chan error, 1)
	go func() {
		done <- runDev(ctx, devOptions{
			appDir:    appDir,
			workspace: workspace,
			port:      0,
		}, dependencies)
	}()

	failed := waitDevEvent(t, builds)
	assertDevEvent(t, failed, 1, true, true)
	assertNoDevGenerationEvent(t, generations)
	assertNoDevGenerationEvent(t, reloads)
	select {
	case url := <-serverURLs:
		t.Fatalf("server started after failed initial package: %s", url)
	case <-time.After(100 * time.Millisecond):
	}

	writeDevIntegrationGOX(t, appDir, "fixed initial source")
	recovered := waitDevEvent(t, builds)
	assertDevEvent(t, recovered, 2, false, false)
	assertDevGenerationEvent(t, generations, 1)
	assertNoDevGenerationEvent(t, reloads)
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

func writeDevEmbedIntegrationApp(t *testing.T, appDir, repositoryRoot string) {
	t.Helper()
	writeTestFile(t, appDir, "go.mod", fmt.Sprintf(`module example.com/devembed

go 1.22

require %s v0.0.0

replace %s => %s
`, canonicalModulePath, canonicalModulePath, filepath.ToSlash(repositoryRoot)))
	writeTestFile(t, appDir, manifestName, `{"name":"dev-embed","compiler":"go","assets":"assets"}`)
	writeTestFile(t, appDir, "main.go", `//go:build js && wasm

package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func main() {
	done := make(chan struct{})
	gf.Mount("root", App)
	<-done
}
`)
	writeTestFile(t, appDir, "embedded.go", `package main

import "embed"

//go:embed message.txt
var embeddedMessage string

//go:embed extras/*.txt
var embeddedExtras embed.FS
`)
	writeTestFile(t, appDir, "app.gox", `package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func App() gf.Node {
	return <main><span id="embedded-value">{embeddedMessage}</span></main>
}
`)
	writeTestFile(t, appDir, "message.txt", "alpha")
	writeTestFile(t, appDir, "extras/base.txt", "base")
	writeTestFile(t, appDir, "assets/index.html", `<!doctype html>
<html><body><div id="root"></div><script src="wasm_exec.js"></script><script>fetch("bundle.wasm")</script></body></html>
`)
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

func assertDevGenerationEvent(t *testing.T, events <-chan uint64, want uint64) {
	t.Helper()
	select {
	case got := <-events:
		if got != want {
			t.Fatalf("development generation event = %d, want %d", got, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for development generation %d", want)
	}
}

func assertNoDevGenerationEvent(t *testing.T, events <-chan uint64) {
	t.Helper()
	select {
	case generation := <-events:
		t.Fatalf("unexpected development generation event %d", generation)
	default:
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

func assertDevWASMContains(t *testing.T, serverURL, want string) {
	t.Helper()
	body := []byte(readDevHTTPBody(t, serverURL+"/assets/bundle.wasm"))
	if !bytes.Contains(body, []byte(want)) {
		t.Fatalf("served WASM does not contain embedded value %q", want)
	}
}

func assertDevInjectedHTTPBody(t *testing.T, url, canonical string, generation uint64) {
	t.Helper()
	got := readDevHTTPBody(t, url)
	if !strings.HasPrefix(got, canonical+"\n<script ") {
		t.Fatalf("GET %s body does not preserve canonical index %q: %q", url, canonical, got)
	}
	if strings.Count(got, devReloadMarker) != 1 {
		t.Fatalf("GET %s reload marker count = %d, want 1", url, strings.Count(got, devReloadMarker))
	}
	if !strings.Contains(got, fmt.Sprintf(`data-goframe-generation="%d"`, generation)) {
		t.Fatalf("GET %s body does not contain generation %d: %q", url, generation, got)
	}
	instancePrefix := `data-goframe-instance="`
	instanceStart := strings.Index(got, instancePrefix)
	if instanceStart < 0 {
		t.Fatalf("GET %s body does not contain a reload instance: %q", url, got)
	}
	instanceValue := got[instanceStart+len(instancePrefix):]
	if end := strings.IndexByte(instanceValue, '"'); end <= 0 {
		t.Fatalf("GET %s body has an empty reload instance: %q", url, got)
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

func replaceDevCanonicalPackage(source, destination string) error {
	if err := os.RemoveAll(destination); err != nil {
		return err
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	if err := publishPackageArtifacts(source, destination); err != nil {
		return err
	}
	return verifyPublishedPackage(destination)
}

func waitDevSignal(t *testing.T, signal <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", label)
	}
}

func assertDevPackageCall(t *testing.T, calls <-chan int, want int) {
	t.Helper()
	select {
	case got := <-calls:
		if got != want {
			t.Fatalf("package call = %d, want %d", got, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for package call %d", want)
	}
}

func assertNoDevPackageCall(t *testing.T, calls <-chan int) {
	t.Helper()
	select {
	case call := <-calls:
		t.Fatalf("unexpected package call %d", call)
	default:
	}
}

func devGenerationRootSet(t *testing.T) map[string]struct{} {
	t.Helper()
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	roots := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "goxc-dev-generations-") {
			roots[filepath.Join(os.TempDir(), entry.Name())] = struct{}{}
		}
	}
	return roots
}

func assertOneNewDevGenerationRoot(t *testing.T, before map[string]struct{}) string {
	t.Helper()
	var added []string
	for root := range devGenerationRootSet(t) {
		if _, ok := before[root]; !ok {
			added = append(added, root)
		}
	}
	if len(added) != 1 {
		t.Fatalf("new development generation roots = %#v, want one", added)
	}
	return added[0]
}
