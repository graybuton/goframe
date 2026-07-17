package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultDevPollInterval = 200 * time.Millisecond
	defaultDevDebounce     = 150 * time.Millisecond
	devShutdownTimeout     = 5 * time.Second
)

type devOptions struct {
	appDir    string
	compiler  string
	workspace string
	port      int
}

type devBuildRequest struct {
	Number  int
	Initial bool
	Changed []string
}

type devBuildEvent struct {
	Request  devBuildRequest
	Err      error
	Duration time.Duration
}

type devHooks struct {
	BuildStarted        func(devBuildRequest)
	BuildFinished       func(devBuildEvent)
	GenerationActivated func(uint64)
	ReloadPublished     func(uint64)
	ServerStarted       func(string)
}

type devDependencies struct {
	packageApp   func(packageOptions) error
	listen       func(string, string) (net.Listener, error)
	pollInterval time.Duration
	debounce     time.Duration
	stdout       io.Writer
	stderr       io.Writer
	hooks        devHooks
}

func defaultDevDependencies() devDependencies {
	return devDependencies{
		packageApp:   packageApp,
		listen:       net.Listen,
		pollInterval: defaultDevPollInterval,
		debounce:     defaultDevDebounce,
		stdout:       os.Stdout,
		stderr:       os.Stderr,
	}
}

func devCommand(args []string) error {
	options, err := parseDevOptions(args)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return runDev(ctx, options, defaultDevDependencies())
}

func parseDevOptions(args []string) (devOptions, error) {
	options := devOptions{port: 8080}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case strings.HasPrefix(arg, "--compiler="):
			options.compiler = strings.TrimPrefix(arg, "--compiler=")
		case arg == "--compiler":
			index++
			if index >= len(args) {
				return devOptions{}, errors.New("--compiler requires a value")
			}
			options.compiler = args[index]
		case strings.HasPrefix(arg, "--port="):
			port, err := strconv.Atoi(strings.TrimPrefix(arg, "--port="))
			if err != nil {
				return devOptions{}, fmt.Errorf("invalid port: %w", err)
			}
			options.port = port
		case arg == "--port":
			index++
			if index >= len(args) {
				return devOptions{}, errors.New("--port requires a value")
			}
			port, err := strconv.Atoi(args[index])
			if err != nil {
				return devOptions{}, fmt.Errorf("invalid port: %w", err)
			}
			options.port = port
		case strings.HasPrefix(arg, "--workspace="):
			options.workspace = strings.TrimPrefix(arg, "--workspace=")
		case arg == "--workspace":
			index++
			if index >= len(args) {
				return devOptions{}, errors.New("--workspace requires a value")
			}
			options.workspace = args[index]
		case strings.HasPrefix(arg, "-"):
			return devOptions{}, fmt.Errorf("unknown dev flag %q", arg)
		case options.appDir == "":
			options.appDir = arg
		default:
			return devOptions{}, fmt.Errorf("unexpected dev argument %q", arg)
		}
	}
	if options.appDir == "" {
		return devOptions{}, errors.New("usage: goxc dev <app-directory> [--compiler=go|tinygo] [--port=8080] [--workspace=directory]")
	}
	if options.compiler != "" {
		if err := validateCompiler(options.compiler); err != nil {
			return devOptions{}, err
		}
	}
	if options.port < 0 || options.port > 65535 {
		return devOptions{}, errors.New("port must be between 0 and 65535")
	}
	return options, nil
}

func runDev(ctx context.Context, options devOptions, dependencies devDependencies) error {
	dependencies = normalizeDevDependencies(dependencies)
	if err := ensureAppDirectory(options.appDir); err != nil {
		return err
	}
	layout, err := newBuildLayout(layoutOptions{
		appDir:    options.appDir,
		compiler:  options.compiler,
		workspace: options.workspace,
	})
	if err != nil {
		return err
	}
	if err := validateWorkspaceRoot(layout); err != nil {
		return err
	}

	collector := newDevSnapshotCollector(layout.AppDir)
	server, err := newDevServer(layout.PackageDir, options.port, dependencies)
	if err != nil {
		return err
	}

	build := func(request devBuildRequest) error {
		err := dependencies.packageApp(packageOptions{
			appDir:    layout.AppDir,
			compiler:  options.compiler,
			workspace: options.workspace,
			compress:  map[string]bool{},
		})
		if err != nil {
			return err
		}
		if _, err := server.activatePackage(); err != nil {
			return err
		}
		if !server.started() {
			if err := server.start(); err != nil {
				return devFatalError{err: err}
			}
		}
		return nil
	}

	runErr := runDevCoordinator(ctx, devCoordinatorConfig{
		scan:         collector.collect,
		build:        build,
		serverErrors: server.errors(),
		pollInterval: dependencies.pollInterval,
		debounce:     dependencies.debounce,
		stdout:       dependencies.stdout,
		stderr:       dependencies.stderr,
		hooks:        dependencies.hooks,
	})
	shutdownErr := server.shutdown()
	if runErr != nil {
		return runErr
	}
	return shutdownErr
}

func normalizeDevDependencies(dependencies devDependencies) devDependencies {
	defaults := defaultDevDependencies()
	if dependencies.packageApp == nil {
		dependencies.packageApp = defaults.packageApp
	}
	if dependencies.listen == nil {
		dependencies.listen = defaults.listen
	}
	if dependencies.pollInterval <= 0 {
		dependencies.pollInterval = defaults.pollInterval
	}
	if dependencies.debounce <= 0 {
		dependencies.debounce = defaults.debounce
	}
	if dependencies.stdout == nil {
		dependencies.stdout = defaults.stdout
	}
	if dependencies.stderr == nil {
		dependencies.stderr = defaults.stderr
	}
	return dependencies
}

type devServer struct {
	packageDir  string
	port        int
	listen      func(string, string) (net.Listener, error)
	stdout      io.Writer
	hooks       devHooks
	generations *devGenerationManager
	reload      *devReloadBroker

	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
	errCh    chan error
}

func newDevServer(packageDir string, port int, dependencies devDependencies) (*devServer, error) {
	generations, err := newDevGenerationManager()
	if err != nil {
		return nil, err
	}
	return &devServer{
		packageDir:  packageDir,
		port:        port,
		listen:      dependencies.listen,
		stdout:      dependencies.stdout,
		hooks:       dependencies.hooks,
		generations: generations,
		reload:      newDevReloadBroker(),
		errCh:       make(chan error, 1),
	}, nil
}

func (server *devServer) activatePackage() (uint64, error) {
	notify := server.started()
	generation, err := server.generations.activatePackage(server.packageDir)
	if err != nil {
		return 0, err
	}
	server.reload.activate(generation, notify)
	if server.hooks.GenerationActivated != nil {
		server.hooks.GenerationActivated(generation)
	}
	if notify && server.hooks.ReloadPublished != nil {
		server.hooks.ReloadPublished(generation)
	}
	return generation, nil
}

func (server *devServer) start() error {
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.server != nil {
		return nil
	}
	listener, err := server.listen("tcp", fmt.Sprintf("127.0.0.1:%d", server.port))
	if err != nil {
		return fmt.Errorf("start development server: %w", err)
	}
	httpServer := &http.Server{Handler: devReloadHandler(server.generations, server.reload)}
	server.listener = listener
	server.server = httpServer
	url := "http://" + listener.Addr().String()
	fmt.Fprintf(server.stdout, "development server ready at %s\n", url)
	if server.hooks.ServerStarted != nil {
		server.hooks.ServerStarted(url)
	}
	go func() {
		err := httpServer.Serve(listener)
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return
		}
		select {
		case server.errCh <- fmt.Errorf("development server failed: %w", err):
		default:
		}
	}()
	return nil
}

func (server *devServer) started() bool {
	server.mu.Lock()
	defer server.mu.Unlock()
	return server.server != nil
}

func (server *devServer) errors() <-chan error {
	return server.errCh
}

func (server *devServer) shutdown() error {
	server.mu.Lock()
	httpServer := server.server
	server.mu.Unlock()
	server.reload.close()
	var shutdownErr error
	if httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), devShutdownTimeout)
		if err := httpServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			shutdownErr = fmt.Errorf("shut down development server: %w", err)
		}
		cancel()
	}
	return errors.Join(shutdownErr, server.generations.close())
}

func devStaticHandler(directory string) http.Handler {
	handler := staticHandler(directory)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		handler.ServeHTTP(response, request)
	})
}

type devFatalError struct {
	err error
}

func (failure devFatalError) Error() string {
	return failure.err.Error()
}

func (failure devFatalError) Unwrap() error {
	return failure.err
}

type devSnapshot struct {
	files map[string]string
}

func newDevSnapshot() devSnapshot {
	return devSnapshot{files: map[string]string{}}
}

func (snapshot devSnapshot) paths() []string {
	paths := make([]string, 0, len(snapshot.files))
	for path := range snapshot.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func devSnapshotsEqual(first, second devSnapshot) bool {
	if len(first.files) != len(second.files) {
		return false
	}
	for path, fingerprint := range first.files {
		if second.files[path] != fingerprint {
			return false
		}
	}
	return true
}

func diffDevSnapshots(previous, current devSnapshot) []string {
	changed := map[string]struct{}{}
	for path, fingerprint := range previous.files {
		if current.files[path] != fingerprint {
			changed[path] = struct{}{}
		}
	}
	for path, fingerprint := range current.files {
		if previous.files[path] != fingerprint {
			changed[path] = struct{}{}
		}
	}
	paths := make([]string, 0, len(changed))
	for path := range changed {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

type devSnapshotCollector struct {
	appDir     string
	lastAssets devAssetWatchSpec
	haveAssets bool
}

type devAssetWatchSpec struct {
	directories []devAssetDirectory
	files       []string
}

type devAssetDirectory struct {
	path     string
	required bool
}

func newDevSnapshotCollector(appDir string) *devSnapshotCollector {
	return &devSnapshotCollector{appDir: appDir}
}

func (collector *devSnapshotCollector) collect() (devSnapshot, error) {
	snapshot := newDevSnapshot()
	if err := collector.collectAuthoredSource(&snapshot); err != nil {
		return snapshot, err
	}
	if err := collector.collectFile(&snapshot, filepath.Join(collector.appDir, manifestName), true); err != nil {
		return snapshot, err
	}
	if err := collector.collectModuleInputs(&snapshot); err != nil {
		return snapshot, err
	}

	manifest, err := loadManifest(collector.appDir)
	if err != nil {
		return snapshot, collector.collectRetainedAssets(&snapshot, err)
	}
	wasmLogicalName := path.Base(filepath.ToSlash(filepath.Clean(manifest.WASM)))
	_, planErr := planPackageAssets(collector.appDir, manifest, wasmLogicalName, packageOptions{compress: map[string]bool{}})
	if planErr != nil {
		return snapshot, collector.collectRetainedAssets(&snapshot, planErr)
	}

	spec, err := collector.assetWatchSpec(manifest)
	if err != nil {
		return snapshot, err
	}
	collector.lastAssets = spec
	collector.haveAssets = true
	if err := collector.collectAssets(&snapshot, spec); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func (collector *devSnapshotCollector) collectRetainedAssets(snapshot *devSnapshot, scanErr error) error {
	if collector.haveAssets {
		if err := collector.collectAssets(snapshot, collector.lastAssets); err != nil {
			scanErr = errors.Join(scanErr, err)
		}
	}
	return devBuildableScanError{err: scanErr}
}

func (collector *devSnapshotCollector) collectAuthoredSource(snapshot *devSnapshot) error {
	return filepath.WalkDir(collector.appDir, func(current string, entry os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("inspect watched source %s: %w", current, err)
		}
		if current == collector.appDir {
			return nil
		}
		relative, err := filepath.Rel(collector.appDir, current)
		if err != nil {
			return fmt.Errorf("resolve watched source %s: %w", current, err)
		}
		if shouldSkipWorkspaceSource(relative, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("watched source %s is a symlink; symlink paths are not supported", current)
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(current, ".gox.go") {
			return nil
		}
		extension := strings.ToLower(filepath.Ext(current))
		if extension != ".go" && extension != ".gox" {
			return nil
		}
		return collector.collectFile(snapshot, current, false)
	})
}

func (collector *devSnapshotCollector) collectModuleInputs(snapshot *devSnapshot) error {
	paths, err := devModuleInputPaths(collector.appDir)
	if err != nil {
		return err
	}
	for _, modulePath := range paths {
		if err := collector.collectFile(snapshot, modulePath, true); err != nil {
			return err
		}
	}
	return nil
}

func devModuleInputPaths(appDir string) ([]string, error) {
	current, err := filepath.Abs(appDir)
	if err != nil {
		return nil, fmt.Errorf("resolve application directory: %w", err)
	}
	for {
		goMod := filepath.Join(current, "go.mod")
		info, err := os.Lstat(goMod)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("watched module file %s is a symlink; symlink paths are not supported", goMod)
			}
			return []string{goMod, filepath.Join(current, "go.sum")}, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("inspect watched module file %s: %w", goMod, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil, nil
		}
		current = parent
	}
}

func (collector *devSnapshotCollector) assetWatchSpec(manifest projectManifest) (devAssetWatchSpec, error) {
	var spec devAssetWatchSpec
	switch manifest.Assets.Mode {
	case manifestAssetsAuto:
		assetsPath := filepath.Join(collector.appDir, assetDirectoryName)
		info, err := os.Lstat(assetsPath)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return devAssetWatchSpec{}, fmt.Errorf("asset directory %s is a symlink; symlink paths are not supported", assetsPath)
			}
			if info.IsDir() {
				spec.directories = append(spec.directories, devAssetDirectory{path: assetsPath})
				return spec, nil
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return devAssetWatchSpec{}, fmt.Errorf("inspect asset directory %s: %w", assetsPath, err)
		}
		spec.directories = append(spec.directories, devAssetDirectory{path: assetsPath})
		spec.files = append(spec.files, filepath.Join(collector.appDir, indexHTMLAssetName))
	case manifestAssetsDirectory:
		spec.directories = append(spec.directories, devAssetDirectory{
			path:     filepath.Join(collector.appDir, filepath.FromSlash(manifest.Assets.Directory)),
			required: true,
		})
	case manifestAssetsList:
		for _, asset := range manifest.Assets.List {
			spec.files = append(spec.files, filepath.Join(collector.appDir, filepath.FromSlash(asset)))
		}
	default:
		return devAssetWatchSpec{}, fmt.Errorf("assets in %s has unsupported internal mode", manifestName)
	}
	sort.Slice(spec.directories, func(first, second int) bool {
		return spec.directories[first].path < spec.directories[second].path
	})
	sort.Strings(spec.files)
	return spec, nil
}

func (collector *devSnapshotCollector) collectAssets(snapshot *devSnapshot, spec devAssetWatchSpec) error {
	for _, directory := range spec.directories {
		if err := validatePathBelowRoot(collector.appDir, directory.path, "watched asset directory", true); err != nil {
			return err
		}
		if err := collector.collectAssetDirectory(snapshot, directory); err != nil {
			return err
		}
	}
	for _, file := range spec.files {
		if err := validatePathBelowRoot(collector.appDir, file, "watched asset path", true); err != nil {
			return err
		}
		if err := collector.collectFile(snapshot, file, true); err != nil {
			return err
		}
	}
	return nil
}

func (collector *devSnapshotCollector) collectAssetDirectory(snapshot *devSnapshot, directory devAssetDirectory) error {
	info, err := os.Lstat(directory.path)
	if errors.Is(err, os.ErrNotExist) {
		snapshot.files[collector.displayPath(directory.path)+"/"] = "missing"
		if directory.required {
			return fmt.Errorf("watched asset directory %s does not exist", directory.path)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect watched asset directory %s: %w", directory.path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("watched asset directory %s is a symlink; symlink paths are not supported", directory.path)
	}
	if !info.IsDir() {
		if !directory.required {
			snapshot.files[collector.displayPath(directory.path)+"/"] = "missing"
			return nil
		}
		return fmt.Errorf("watched asset directory %s is not a directory", directory.path)
	}
	snapshot.files[collector.displayPath(directory.path)+"/"] = "directory"
	return filepath.WalkDir(directory.path, func(current string, entry os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("inspect watched asset %s: %w", current, err)
		}
		if current == directory.path {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("watched asset %s is a symlink; symlink paths are not supported", current)
		}
		if entry.IsDir() {
			snapshot.files[collector.displayPath(current)+"/"] = "directory"
			return nil
		}
		return collector.collectFile(snapshot, current, false)
	})
}

func (collector *devSnapshotCollector) collectFile(snapshot *devSnapshot, file string, allowMissing bool) error {
	info, err := os.Lstat(file)
	if errors.Is(err, os.ErrNotExist) && allowMissing {
		snapshot.files[collector.displayPath(file)] = "missing"
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect watched input %s: %w", file, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("watched input %s is a symlink; symlink paths are not supported", file)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("watched input %s is not a regular file", file)
	}
	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read watched input %s: %w", file, err)
	}
	sum := sha256.Sum256(content)
	snapshot.files[collector.displayPath(file)] = "sha256:" + hex.EncodeToString(sum[:])
	return nil
}

func (collector *devSnapshotCollector) displayPath(file string) string {
	relative, err := filepath.Rel(collector.appDir, file)
	if err != nil {
		return filepath.ToSlash(file)
	}
	return filepath.ToSlash(relative)
}

type devCoordinatorConfig struct {
	scan         func() (devSnapshot, error)
	build        func(devBuildRequest) error
	serverErrors <-chan error
	pollInterval time.Duration
	debounce     time.Duration
	stdout       io.Writer
	stderr       io.Writer
	hooks        devHooks
}

type devBuildableScanError struct {
	err error
}

func (failure devBuildableScanError) Error() string {
	return failure.err.Error()
}

func (failure devBuildableScanError) Unwrap() error {
	return failure.err
}

func devScanAllowsBuild(err error) bool {
	if err == nil {
		return true
	}
	var buildable devBuildableScanError
	return errors.As(err, &buildable)
}

func runDevCoordinator(ctx context.Context, config devCoordinatorConfig) error {
	if config.scan == nil || config.build == nil {
		return errors.New("development coordinator requires scan and build functions")
	}
	if config.pollInterval <= 0 || config.debounce <= 0 {
		return errors.New("development coordinator intervals must be positive")
	}
	if config.stdout == nil {
		config.stdout = io.Discard
	}
	if config.stderr == nil {
		config.stderr = io.Discard
	}
	if ctx.Err() != nil {
		return nil
	}

	initialSnapshot, scanErr := config.scan()
	haveHealthySnapshot := scanErr == nil
	lastScanError := reportDevScanState(config.stderr, "", scanErr)
	if ctx.Err() != nil {
		return nil
	}

	lastAttempted := newDevSnapshot()
	buildNumber := 0
	attemptBuild := func(snapshot devSnapshot, changed []string) error {
		initial := buildNumber == 0
		buildNumber++
		lastAttempted = snapshot
		if initial {
			changed = nil
		}
		return runDevBuild(config, devBuildRequest{
			Number:  buildNumber,
			Initial: initial,
			Changed: changed,
		})
	}
	if devScanAllowsBuild(scanErr) {
		if err := attemptBuild(initialSnapshot, nil); err != nil {
			var fatal devFatalError
			if errors.As(err, &fatal) {
				return fatal.err
			}
		}
	}
	if ctx.Err() != nil {
		return nil
	}

	pollTicker := time.NewTicker(config.pollInterval)
	defer pollTicker.Stop()
	var debounceTimer *time.Timer
	var debounceC <-chan time.Time
	var pending devSnapshot
	havePending := false
	forcePending := false

	stopDebounce := func() {
		if debounceTimer == nil {
			return
		}
		if !debounceTimer.Stop() {
			select {
			case <-debounceTimer.C:
			default:
			}
		}
		debounceC = nil
	}
	resetDebounce := func() {
		if debounceTimer == nil {
			debounceTimer = time.NewTimer(config.debounce)
		} else {
			stopDebounce()
			debounceTimer.Reset(config.debounce)
		}
		debounceC = debounceTimer.C
	}
	defer func() {
		if debounceTimer != nil {
			stopDebounce()
		}
	}()

	handleScan := func(snapshot devSnapshot, err error) {
		previousError := lastScanError
		lastScanError = reportDevScanState(config.stderr, lastScanError, err)
		if !devScanAllowsBuild(err) {
			havePending = false
			forcePending = false
			stopDebounce()
			return
		}
		if buildNumber == 0 {
			if err == nil {
				haveHealthySnapshot = true
			}
			if !havePending || !devSnapshotsEqual(pending, snapshot) {
				pending = snapshot
				havePending = true
				forcePending = true
				resetDebounce()
			}
			return
		}
		if err != nil {
			if !devSnapshotsEqual(lastAttempted, snapshot) {
				if !havePending || !devSnapshotsEqual(pending, snapshot) {
					pending = snapshot
					havePending = true
					forcePending = false
					resetDebounce()
				}
				return
			}
			havePending = false
			forcePending = false
			stopDebounce()
			return
		}
		recovered := previousError != ""
		if !haveHealthySnapshot {
			recovered = true
			haveHealthySnapshot = true
		}
		if recovered || !devSnapshotsEqual(lastAttempted, snapshot) {
			if !havePending || !devSnapshotsEqual(pending, snapshot) || recovered {
				pending = snapshot
				havePending = true
				forcePending = recovered
				resetDebounce()
			}
			return
		}
		if havePending && forcePending {
			return
		}
		havePending = false
		forcePending = false
		stopDebounce()
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-config.serverErrors:
			if err != nil {
				return err
			}
		case <-pollTicker.C:
			snapshot, err := config.scan()
			handleScan(snapshot, err)
		case <-debounceC:
			debounceC = nil
			havePending = false
			if ctx.Err() != nil {
				return nil
			}
			changed := diffDevSnapshots(lastAttempted, pending)
			if buildNumber > 0 && len(changed) == 0 && !forcePending {
				continue
			}
			if buildNumber > 0 && len(changed) == 0 {
				changed = []string{"watch inputs recovered"}
			}
			forcePending = false
			err := attemptBuild(pending, changed)
			var fatal devFatalError
			if errors.As(err, &fatal) {
				return fatal.err
			}
			if ctx.Err() != nil {
				return nil
			}
			snapshot, scanErr := config.scan()
			handleScan(snapshot, scanErr)
		}
	}
}

func runDevBuild(config devCoordinatorConfig, request devBuildRequest) error {
	classification := "rebuild"
	changeSummary := summarizeDevChanges(request.Changed)
	if request.Initial {
		classification = "initial"
		changeSummary = "initial inputs"
	}
	fmt.Fprintf(config.stdout, "dev build %d (%s): %s\n", request.Number, classification, changeSummary)
	if config.hooks.BuildStarted != nil {
		config.hooks.BuildStarted(request)
	}
	started := time.Now()
	err := config.build(request)
	duration := time.Since(started)
	event := devBuildEvent{Request: request, Err: err, Duration: duration}
	if config.hooks.BuildFinished != nil {
		config.hooks.BuildFinished(event)
	}
	if err != nil {
		fmt.Fprintf(config.stderr, "dev build %d failed after %s: %v\n", request.Number, duration.Round(time.Millisecond), err)
		return err
	}
	fmt.Fprintf(config.stdout, "dev build %d succeeded after %s\n", request.Number, duration.Round(time.Millisecond))
	return nil
}

func summarizeDevChanges(changed []string) string {
	const limit = 6
	if len(changed) <= limit {
		return strings.Join(changed, ", ")
	}
	return fmt.Sprintf("%s, and %d more", strings.Join(changed[:limit], ", "), len(changed)-limit)
}

func reportDevScanState(output io.Writer, previous string, err error) string {
	if err == nil {
		if previous != "" {
			fmt.Fprintln(output, "dev watch recovered")
		}
		return ""
	}
	current := err.Error()
	if current != previous {
		fmt.Fprintf(output, "dev watch error: %v\n", err)
	}
	return current
}
