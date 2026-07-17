package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
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

func TestDevGenerationShutdownRemovesPrivateRoot(t *testing.T) {
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
	if err := manager.close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("generation root remained after shutdown: %v", err)
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
		if err := manager.close(); err != nil {
			t.Errorf("close development generations: %v", err)
		}
	})
	return manager
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
