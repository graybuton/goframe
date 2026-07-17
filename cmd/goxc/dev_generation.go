package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type devGenerationManager struct {
	mu          sync.Mutex
	root        string
	nextID      uint64
	active      *devGeneration
	generations map[uint64]*devGeneration
	closed      bool
}

type devGeneration struct {
	id        uint64
	directory string
	leases    int
	retired   bool
}

type devGenerationLease struct {
	manager    *devGenerationManager
	generation *devGeneration
	once       sync.Once
	err        error
}

func newDevGenerationManager() (*devGenerationManager, error) {
	root, err := os.MkdirTemp("", "goxc-dev-generations-*")
	if err != nil {
		return nil, fmt.Errorf("create development generation root: %w", err)
	}
	return &devGenerationManager{
		root:        root,
		generations: map[uint64]*devGeneration{},
	}, nil
}

func (manager *devGenerationManager) activatePackage(packageDir string) (uint64, error) {
	if err := verifyPublishedPackage(packageDir); err != nil {
		return 0, err
	}

	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return 0, errors.New("development generation manager is closed")
	}
	root := manager.root
	manager.mu.Unlock()

	staging, err := os.MkdirTemp(root, ".staging-*")
	if err != nil {
		return 0, fmt.Errorf("create staging development generation: %w", err)
	}
	keepStaging := false
	defer func() {
		if !keepStaging {
			_ = os.RemoveAll(staging)
		}
	}()

	if err := publishPackageArtifacts(packageDir, staging); err != nil {
		return 0, fmt.Errorf("copy development generation: %w", err)
	}
	if err := verifyPublishedPackage(staging); err != nil {
		return 0, fmt.Errorf("verify development generation: %w", err)
	}

	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return 0, errors.New("development generation manager is closed")
	}
	manager.nextID++
	id := manager.nextID
	directory := filepath.Join(manager.root, fmt.Sprintf("generation-%020d", id))
	if err := os.Rename(staging, directory); err != nil {
		manager.mu.Unlock()
		return 0, fmt.Errorf("complete development generation %d: %w", id, err)
	}
	keepStaging = true

	generation := &devGeneration{id: id, directory: directory}
	manager.generations[id] = generation
	retired := manager.active
	manager.active = generation
	var removeDirectory string
	if retired != nil {
		retired.retired = true
		if retired.leases == 0 {
			delete(manager.generations, retired.id)
			removeDirectory = retired.directory
		}
	}
	manager.mu.Unlock()

	if removeDirectory != "" {
		_ = os.RemoveAll(removeDirectory)
	}
	return id, nil
}

func (manager *devGenerationManager) activeID() (uint64, bool) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.active == nil {
		return 0, false
	}
	return manager.active.id, true
}

func (manager *devGenerationManager) acquire() (*devGenerationLease, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.closed {
		return nil, errors.New("development generation manager is closed")
	}
	if manager.active == nil {
		return nil, errors.New("no completed development generation is active")
	}
	manager.active.leases++
	return &devGenerationLease{
		manager:    manager,
		generation: manager.active,
	}, nil
}

func (lease *devGenerationLease) ID() uint64 {
	return lease.generation.id
}

func (lease *devGenerationLease) Directory() string {
	return lease.generation.directory
}

func (lease *devGenerationLease) Release() error {
	lease.once.Do(func() {
		lease.err = lease.manager.release(lease.generation)
	})
	return lease.err
}

func (manager *devGenerationManager) release(generation *devGeneration) error {
	manager.mu.Lock()
	if generation.leases <= 0 {
		manager.mu.Unlock()
		return fmt.Errorf("development generation %d has no active request lease", generation.id)
	}
	generation.leases--
	var removeDirectory string
	if generation.retired && generation.leases == 0 {
		delete(manager.generations, generation.id)
		removeDirectory = generation.directory
	}
	manager.mu.Unlock()

	if removeDirectory != "" {
		if err := os.RemoveAll(removeDirectory); err != nil {
			return fmt.Errorf("remove retired development generation %d: %w", generation.id, err)
		}
	}
	return nil
}

func (manager *devGenerationManager) close() error {
	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return nil
	}
	for _, generation := range manager.generations {
		if generation.leases != 0 {
			manager.mu.Unlock()
			return fmt.Errorf("close development generations with %d active request leases", generation.leases)
		}
	}
	root := manager.root
	manager.closed = true
	manager.active = nil
	manager.generations = nil
	manager.mu.Unlock()

	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("remove development generation root: %w", err)
	}
	return nil
}

func devGenerationHandler(manager *devGenerationManager) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		lease, err := manager.acquire()
		if err != nil {
			http.Error(response, "development package is not ready", http.StatusServiceUnavailable)
			return
		}
		defer lease.Release()
		devStaticHandler(lease.Directory()).ServeHTTP(response, request)
	})
}
