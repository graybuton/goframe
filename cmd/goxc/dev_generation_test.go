package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDevGenerationFirstCompletedActivation(t *testing.T) {
	packageDir := t.TempDir()
	writeDevGenerationPackage(t, packageDir, "generation one")
	manager := newTestDevGenerationManager(t)

	id, err := manager.activatePackage(packageDir)
	if err != nil {
		t.Fatalf("activatePackage() error: %v", err)
	}
	if id != 1 {
		t.Fatalf("generation ID = %d, want 1", id)
	}
	if active, ok := manager.activeID(); !ok || active != id {
		t.Fatalf("active generation = %d, %v, want 1, true", active, ok)
	}

	lease, err := manager.acquire()
	if err != nil {
		t.Fatal(err)
	}
	if samePath(lease.Directory(), packageDir) {
		t.Fatal("active development generation aliases the canonical package directory")
	}
	assertFileContent(t, filepath.Join(lease.Directory(), indexHTMLAssetName), "generation one")
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestDevGenerationHandlerNeverServesCanonicalPackage(t *testing.T) {
	packageDir := t.TempDir()
	writeDevGenerationPackage(t, packageDir, "completed generation")
	manager := newTestDevGenerationManager(t)
	if _, err := manager.activatePackage(packageDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageDir, indexHTMLAssetName), []byte("mutable canonical package"), 0o644); err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	devGenerationHandler(manager).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || response.Body.String() != "completed generation" {
		t.Fatalf("response = %d %q, want completed generation", response.Code, response.Body.String())
	}
}

func TestDevGenerationCopyFailurePreservesActiveGeneration(t *testing.T) {
	requireSymlinkSupport(t)
	packageDir := t.TempDir()
	writeDevGenerationPackage(t, packageDir, "generation one")
	manager := newTestDevGenerationManager(t)
	if _, err := manager.activatePackage(packageDir); err != nil {
		t.Fatal(err)
	}

	external := filepath.Join(t.TempDir(), "external.txt")
	if err := os.WriteFile(external, []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(packageDir, "assets", "linked.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.activatePackage(packageDir); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("activatePackage() error = %v, want symlink rejection", err)
	}
	if active, ok := manager.activeID(); !ok || active != 1 {
		t.Fatalf("active generation = %d, %v, want preserved generation 1", active, ok)
	}
	assertActiveDevGenerationContent(t, manager, "generation one")
}

func TestDevGenerationVerificationFailurePreservesActiveGeneration(t *testing.T) {
	packageDir := t.TempDir()
	writeDevGenerationPackage(t, packageDir, "generation one")
	manager := newTestDevGenerationManager(t)
	if _, err := manager.activatePackage(packageDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(packageDir, "assets", "bundle.12345678.wasm")); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.activatePackage(packageDir); err == nil || !strings.Contains(err.Error(), "integrity verification") {
		t.Fatalf("activatePackage() error = %v, want verification failure", err)
	}
	if active, ok := manager.activeID(); !ok || active != 1 {
		t.Fatalf("active generation = %d, %v, want preserved generation 1", active, ok)
	}
	assertActiveDevGenerationContent(t, manager, "generation one")
}

func TestDevGenerationActivationRetainsInflightGeneration(t *testing.T) {
	packageDir := t.TempDir()
	writeDevGenerationPackage(t, packageDir, "generation one")
	manager := newTestDevGenerationManager(t)
	if _, err := manager.activatePackage(packageDir); err != nil {
		t.Fatal(err)
	}
	oldLease, err := manager.acquire()
	if err != nil {
		t.Fatal(err)
	}
	oldDirectory := oldLease.Directory()

	writeDevGenerationPackage(t, packageDir, "generation two")
	if id, err := manager.activatePackage(packageDir); err != nil || id != 2 {
		t.Fatalf("second activation = %d, %v, want 2, nil", id, err)
	}
	assertFileContent(t, filepath.Join(oldDirectory, indexHTMLAssetName), "generation one")
	assertActiveDevGenerationContent(t, manager, "generation two")
	if _, err := os.Stat(oldDirectory); err != nil {
		t.Fatalf("leased retired generation was removed early: %v", err)
	}

	if err := oldLease.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldDirectory); !os.IsNotExist(err) {
		t.Fatalf("retired generation remained after final lease: %v", err)
	}
}

func TestDevGenerationCloseWithNoLeasesRemovesPrivateRoot(t *testing.T) {
	packageDir := t.TempDir()
	writeDevGenerationPackage(t, packageDir, "generation one")
	manager, err := newDevGenerationManager()
	if err != nil {
		t.Fatal(err)
	}
	root := manager.root
	if _, err := manager.activatePackage(packageDir); err != nil {
		t.Fatal(err)
	}
	if err := manager.close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("generation root remained after shutdown: %v", err)
	}
}

func TestDevGenerationCloseWaitsForActiveLease(t *testing.T) {
	packageDir := t.TempDir()
	writeDevGenerationPackage(t, packageDir, "generation one")
	manager := newTestDevGenerationManager(t)
	if _, err := manager.activatePackage(packageDir); err != nil {
		t.Fatal(err)
	}
	lease, err := manager.acquire()
	if err != nil {
		t.Fatal(err)
	}
	root := manager.root

	baseCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	closeCtx := newObservedDoneContext(baseCtx)
	closed := make(chan error, 1)
	go func() {
		closed <- manager.close(closeCtx)
	}()
	waitForObservedDone(t, closeCtx)

	if _, err := manager.acquire(); err == nil || !strings.Contains(err.Error(), "closing") {
		t.Fatalf("acquire() after close began error = %v, want closing error", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("generation root removed with an active lease: %v", err)
	}
	select {
	case err := <-closed:
		t.Fatalf("close returned before lease release: %v", err)
	default:
	}

	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("close() error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not finish after the active lease was released")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("generation root remained after lease drain: %v", err)
	}
	if err := manager.close(context.Background()); err != nil {
		t.Fatalf("repeated close() error: %v", err)
	}
}

func TestDevGenerationCloseTimeoutCanRecoverAfterRelease(t *testing.T) {
	packageDir := t.TempDir()
	writeDevGenerationPackage(t, packageDir, "generation one")
	manager := newTestDevGenerationManager(t)
	if _, err := manager.activatePackage(packageDir); err != nil {
		t.Fatal(err)
	}
	lease, err := manager.acquire()
	if err != nil {
		t.Fatal(err)
	}
	root := manager.root

	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	err = manager.close(expired)
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "leases to drain") {
		t.Fatalf("close() error = %v, want generation lease drain deadline", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("generation root removed after drain timeout: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if err := manager.close(context.Background()); err != nil {
		t.Fatalf("close() after release error: %v", err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("generation root remained after recovered close: %v", err)
	}
}

func TestDevGenerationConcurrentAcquireReleaseClose(t *testing.T) {
	packageDir := t.TempDir()
	writeDevGenerationPackage(t, packageDir, "generation one")
	manager := newTestDevGenerationManager(t)
	if _, err := manager.activatePackage(packageDir); err != nil {
		t.Fatal(err)
	}

	const workers = 16
	acquired := make(chan struct{}, workers)
	release := make(chan struct{})
	errorCh := make(chan error, workers)
	var group sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			lease, err := manager.acquire()
			if err != nil {
				errorCh <- err
				return
			}
			acquired <- struct{}{}
			<-release
			if _, err := manager.acquire(); err == nil {
				errorCh <- errors.New("acquire succeeded after close began")
			}
			if err := lease.Release(); err != nil {
				errorCh <- err
			}
		}()
	}
	for worker := 0; worker < workers; worker++ {
		<-acquired
	}

	baseCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	closeCtx := newObservedDoneContext(baseCtx)
	closed := make(chan error, 1)
	go func() {
		closed <- manager.close(closeCtx)
	}()
	waitForObservedDone(t, closeCtx)
	close(release)
	group.Wait()
	close(errorCh)
	for err := range errorCh {
		t.Errorf("concurrent close operation: %v", err)
	}
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("close() error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not finish after concurrent releases")
	}
	manager.mu.Lock()
	leaseCount := manager.leaseCount
	manager.mu.Unlock()
	if leaseCount != 0 {
		t.Fatalf("active lease count = %d, want 0", leaseCount)
	}
}

func TestDevGenerationConcurrentAcquireActivateRelease(t *testing.T) {
	manager := newTestDevGenerationManager(t)
	packages := make([]string, 8)
	for index := range packages {
		packages[index] = t.TempDir()
		writeDevGenerationPackage(t, packages[index], fmt.Sprintf("generation %d", index+1))
	}
	if _, err := manager.activatePackage(packages[0]); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errors := make(chan error, 32)
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for iteration := 0; iteration < 100; iteration++ {
				lease, err := manager.acquire()
				if err != nil {
					errors <- err
					return
				}
				if _, err := os.ReadFile(filepath.Join(lease.Directory(), indexHTMLAssetName)); err != nil {
					errors <- err
					_ = lease.Release()
					return
				}
				if err := lease.Release(); err != nil {
					errors <- err
					return
				}
			}
		}()
	}
	close(start)
	for index := 1; index < len(packages); index++ {
		if _, err := manager.activatePackage(packages[index]); err != nil {
			t.Fatal(err)
		}
	}
	workers.Wait()
	close(errors)
	for err := range errors {
		t.Errorf("concurrent generation operation: %v", err)
	}
}

func newTestDevGenerationManager(t *testing.T) *devGenerationManager {
	t.Helper()
	manager, err := newDevGenerationManager()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := manager.close(ctx); err != nil {
			t.Errorf("close development generations: %v", err)
		}
	})
	return manager
}

type observedDoneContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func newObservedDoneContext(ctx context.Context) *observedDoneContext {
	return &observedDoneContext{Context: ctx, observed: make(chan struct{})}
}

func (ctx *observedDoneContext) Done() <-chan struct{} {
	ctx.once.Do(func() {
		close(ctx.observed)
	})
	return ctx.Context.Done()
}

func waitForObservedDone(t *testing.T, ctx *observedDoneContext) {
	t.Helper()
	select {
	case <-ctx.observed:
	case <-time.After(time.Second):
		t.Fatal("close did not begin waiting for generation leases")
	}
}

func writeDevGenerationPackage(t *testing.T, directory, index string) {
	t.Helper()
	writeCompleteCurrentPackage(t, directory)
	if err := os.WriteFile(filepath.Join(directory, indexHTMLAssetName), []byte(index), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertActiveDevGenerationContent(t *testing.T, manager *devGenerationManager, want string) {
	t.Helper()
	lease, err := manager.acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	assertFileContent(t, filepath.Join(lease.Directory(), indexHTMLAssetName), want)
}
