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
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type embedInputPlan struct {
	Files           []embedInputFile
	Watches         []embedWatchSpec
	WatchMembership map[string]string
	Resolved        bool
}

type embedInputFile struct {
	SourcePath  string
	DisplayPath string
	Fingerprint string
}

type embedListPackage struct {
	Dir           string
	ImportPath    string
	EmbedPatterns []string
	EmbedFiles    []string
	Error         *embedListError
	DepsErrors    []embedListError
}

type embedListError struct {
	Err string
}

type embedDiscoveryOverlay struct {
	Replace map[string]string
}

type embedCandidateKind uint8

const (
	embedCandidateRegular embedCandidateKind = iota
	embedCandidateSymlink
	embedCandidateIrregular
)

type embedCandidate struct {
	SourcePath string
	Kind       embedCandidateKind
}

type embedInputKey string

type embedDiscoveryContext struct {
	OverlayPath string
	Candidates  map[embedInputKey]embedCandidate
	Cleanup     func()
}

type embedWatchKind uint8

const (
	embedWatchExact embedWatchKind = iota
	embedWatchTree
)

type embedWatchSpec struct {
	Path     string
	Kind     embedWatchKind
	Identity string
}

type resolvedEmbedInput struct {
	file        embedInputFile
	destination string
	copy        bool
}

func discoverAndMaterializeEmbedInputs(layout BuildLayout, appWorkDir, entryPath string) (embedInputPlan, error) {
	discovery, err := createEmbedDiscoveryOverlay(layout.AppDir, appWorkDir)
	if err != nil {
		return embedInputPlan{}, err
	}
	defer discovery.Cleanup()

	packages, listErr := listEmbedPackages(layout.Compiler, entryPath, discovery.OverlayPath)
	if listErr != nil && layout.Compiler == "tinygo" {
		if partial, partialErr := listGoEmbedPackages(entryPath, discovery.OverlayPath, true); partialErr == nil {
			packages = partial
		}
	}
	inputs, plan, planErr := resolveEmbedInputPlan(layout.AppDir, appWorkDir, discovery.Candidates, packages)
	membershipErr := populateEmbedWatchMembership(layout.AppDir, &plan)
	if planErr != nil {
		return plan, errors.Join(listErr, planErr, membershipErr)
	}
	if metadataErr := embedPackageMetadataError(packages); metadataErr != nil {
		return plan, errors.Join(listErr, metadataErr, membershipErr)
	}
	if listErr != nil {
		return plan, errors.Join(listErr, membershipErr)
	}
	if membershipErr != nil {
		return plan, membershipErr
	}
	for _, input := range inputs {
		if !input.copy {
			continue
		}
		if err := mkdirAllBelowRoot(appWorkDir, filepath.Dir(input.destination), "embed destination directory"); err != nil {
			return plan, err
		}
		if err := copyFile(input.file.SourcePath, input.destination); err != nil {
			return plan, fmt.Errorf("copy embedded input %s: %w", input.file.DisplayPath, err)
		}
	}
	plan.Resolved = true
	return plan, nil
}

func createEmbedDiscoveryOverlay(appDir, appWorkDir string) (embedDiscoveryContext, error) {
	empty := embedDiscoveryContext{Cleanup: func() {}}
	appDir, err := filepath.Abs(appDir)
	if err != nil {
		return empty, fmt.Errorf("resolve embed source root: %w", err)
	}
	appWorkDir, err = filepath.Abs(appWorkDir)
	if err != nil {
		return empty, fmt.Errorf("resolve embed workspace root: %w", err)
	}
	replacements := map[string]string{}
	candidates := map[embedInputKey]embedCandidate{}
	unsafeSentinel := ""
	cleanupTemporary := func() {
		if unsafeSentinel != "" {
			_ = os.Remove(unsafeSentinel)
		}
	}
	ensureUnsafeSentinel := func() (string, error) {
		if unsafeSentinel != "" {
			return unsafeSentinel, nil
		}
		temporary, err := os.CreateTemp("", "goxc-embed-unsafe-*")
		if err != nil {
			return "", fmt.Errorf("create embed discovery sentinel: %w", err)
		}
		unsafeSentinel = temporary.Name()
		if err := temporary.Close(); err != nil {
			cleanupTemporary()
			return "", fmt.Errorf("close embed discovery sentinel: %w", err)
		}
		return unsafeSentinel, nil
	}
	err = filepath.WalkDir(appDir, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("inspect embed candidate %s: %w", sourcePath, walkErr)
		}
		if sourcePath == appDir {
			return nil
		}
		relative, err := filepath.Rel(appDir, sourcePath)
		if err != nil {
			return fmt.Errorf("resolve embed candidate %s: %w", sourcePath, err)
		}
		if shouldSkipEmbedCandidate(relative, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if sourcePath != appDir && nestedModuleBoundary(sourcePath) {
				return filepath.SkipDir
			}
			return nil
		}
		key, err := newEmbedInputKey(relative)
		if err != nil {
			return fmt.Errorf("resolve embed candidate %s: %w", sourcePath, err)
		}
		destination := filepath.Join(appWorkDir, relative)
		if err := validatePathBelowRoot(appWorkDir, destination, "embed overlay destination", true); err != nil {
			return err
		}
		destination, err = canonicalPathForComparison(destination)
		if err != nil {
			return fmt.Errorf("resolve embed overlay destination %s: %w", destination, err)
		}

		kind := embedCandidateRegular
		switch {
		case entry.Type()&os.ModeSymlink != 0:
			kind = embedCandidateSymlink
		default:
			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf("inspect embed candidate %s: %w", sourcePath, err)
			}
			if !info.Mode().IsRegular() {
				kind = embedCandidateIrregular
			}
		}
		if kind == embedCandidateRegular {
			if err := validatePathBelowRoot(appDir, sourcePath, "embed candidate", false); err != nil {
				return err
			}
		}
		candidates[key] = embedCandidate{SourcePath: sourcePath, Kind: kind}

		if key == embedInputKey("go.mod") {
			return nil
		}
		if kind != embedCandidateRegular {
			sentinel, err := ensureUnsafeSentinel()
			if err != nil {
				return err
			}
			replacements[destination] = sentinel
			return nil
		}
		if _, err := os.Lstat(destination); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect embed overlay destination %s: %w", destination, err)
		}
		replacements[destination] = sourcePath
		return nil
	})
	if err != nil {
		cleanupTemporary()
		return empty, err
	}

	temporary, err := os.CreateTemp("", "goxc-embed-overlay-*.json")
	if err != nil {
		cleanupTemporary()
		return empty, fmt.Errorf("create embed discovery overlay: %w", err)
	}
	overlayPath := temporary.Name()
	cleanup := func() {
		_ = os.Remove(overlayPath)
		cleanupTemporary()
	}
	encoder := json.NewEncoder(temporary)
	encodeErr := encoder.Encode(embedDiscoveryOverlay{Replace: replacements})
	closeErr := temporary.Close()
	if encodeErr != nil || closeErr != nil {
		cleanup()
		return empty, fmt.Errorf("write embed discovery overlay: %w", errors.Join(encodeErr, closeErr))
	}
	return embedDiscoveryContext{
		OverlayPath: overlayPath,
		Candidates:  candidates,
		Cleanup:     cleanup,
	}, nil
}

func shouldSkipEmbedCandidate(relative string, entry os.DirEntry) bool {
	parts := strings.Split(filepath.ToSlash(relative), "/")
	if len(parts) == 0 {
		return false
	}
	switch parts[0] {
	case defaultWorkspaceName, "build", "dist", "node_modules", ".git", ".goxc-tmp":
		return true
	}
	return false
}

func nestedModuleBoundary(directory string) bool {
	info, err := os.Lstat(filepath.Join(directory, "go.mod"))
	return err == nil && (info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0)
}

func listEmbedPackages(compiler, entryPath, overlayPath string) ([]embedListPackage, error) {
	if compiler == "tinygo" {
		return listTinyGoEmbedPackages(entryPath, overlayPath)
	}
	return listGoEmbedPackages(entryPath, overlayPath, false)
}

func listGoEmbedPackages(entryPath, overlayPath string, tinyGoTags bool) ([]embedListPackage, error) {
	compilerPath, err := exec.LookPath("go")
	if err != nil {
		return nil, errors.New("Go compiler not found in PATH")
	}
	args := []string{
		"list", "-deps", "-e",
		"-json=Dir,ImportPath,GoFiles,EmbedPatterns,EmbedFiles,Error,DepsErrors",
		"-buildvcs=false", "-overlay=" + overlayPath,
	}
	if tinyGoTags {
		args = append(args, "-tags=tinygo")
	}
	args = append(args, ".")
	command := exec.Command(compilerPath, args...)
	command.Dir = entryPath
	command.Env = append(compilerEnvironment("go"), "GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0")
	return runEmbedListCommand(command, "go list")
}

func listTinyGoEmbedPackages(entryPath, overlayPath string) ([]embedListPackage, error) {
	compilerPath, err := exec.LookPath("tinygo")
	if err != nil {
		return nil, errors.New("TinyGo compiler not found in PATH; install TinyGo or use --compiler=go")
	}
	command := exec.Command(compilerPath, "list", "-target=wasm", "-deps", "-json", ".")
	command.Dir = entryPath
	flags := strings.TrimSpace(os.Getenv("GOFLAGS") + " -buildvcs=false " + strconv.Quote("-overlay="+overlayPath))
	command.Env = setEnvironmentValue(compilerEnvironment("tinygo"), "GOFLAGS", flags)
	return runEmbedListCommand(command, "tinygo list")
}

func runEmbedListCommand(command *exec.Cmd, description string) ([]embedListPackage, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	runErr := command.Run()
	packages, decodeErr := decodeEmbedListPackages(&stdout)
	if decodeErr != nil {
		return packages, fmt.Errorf("decode %s embed metadata: %w", description, decodeErr)
	}
	if runErr != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = runErr.Error()
		}
		return packages, fmt.Errorf("%s embed discovery failed: %s", description, detail)
	}
	return packages, nil
}

func decodeEmbedListPackages(reader io.Reader) ([]embedListPackage, error) {
	decoder := json.NewDecoder(reader)
	var packages []embedListPackage
	for {
		var packageInfo embedListPackage
		err := decoder.Decode(&packageInfo)
		if errors.Is(err, io.EOF) {
			return packages, nil
		}
		if err != nil {
			return packages, err
		}
		packages = append(packages, packageInfo)
	}
}

func setEnvironmentValue(environment []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(environment)+1)
	for _, item := range environment {
		if !strings.HasPrefix(item, prefix) {
			result = append(result, item)
		}
	}
	return append(result, prefix+value)
}

func resolveEmbedInputPlan(appDir, appWorkDir string, candidates map[embedInputKey]embedCandidate, packages []embedListPackage) ([]resolvedEmbedInput, embedInputPlan, error) {
	appDir, err := filepath.Abs(appDir)
	if err != nil {
		return nil, embedInputPlan{}, fmt.Errorf("resolve embed source root: %w", err)
	}
	appWorkDir, err = filepath.Abs(appWorkDir)
	if err != nil {
		return nil, embedInputPlan{}, fmt.Errorf("resolve embed workspace root: %w", err)
	}
	inputsByKey := map[embedInputKey]resolvedEmbedInput{}
	watches := map[string]embedWatchSpec{}
	for _, packageInfo := range packages {
		packageRelative, ok, err := relativePathBelow(appWorkDir, packageInfo.Dir)
		if err != nil {
			return nil, embedInputPlan{}, err
		}
		if !ok {
			continue
		}
		sourcePackageDir := filepath.Join(appDir, packageRelative)
		for _, pattern := range packageInfo.EmbedPatterns {
			watch, err := embedPatternWatchSpec(appDir, sourcePackageDir, pattern)
			if err != nil {
				return nil, embedInputPlan{}, err
			}
			watches[embedWatchKey(watch)] = watch
		}
		for _, embedFile := range packageInfo.EmbedFiles {
			if filepath.IsAbs(filepath.FromSlash(embedFile)) {
				return nil, embedInputPlan{}, fmt.Errorf("embedded input %q for %s must be package-relative", embedFile, packageInfo.ImportPath)
			}
			key, err := newEmbedInputKey(filepath.Join(packageRelative, filepath.FromSlash(embedFile)))
			if err != nil {
				return nil, embedInputPlan{}, fmt.Errorf("resolve embedded input %q for %s: %w", embedFile, packageInfo.ImportPath, err)
			}
			destination := filepath.Join(appWorkDir, filepath.FromSlash(string(key)))
			if err := validatePathBelowRoot(appWorkDir, destination, "embedded workspace input", true); err != nil {
				return nil, embedInputPlan{}, err
			}
			destination, err = filepath.Abs(destination)
			if err != nil {
				return nil, embedInputPlan{}, fmt.Errorf("resolve embedded workspace input %s: %w", destination, err)
			}
			if key == embedInputKey("go.mod") {
				return nil, embedInputPlan{}, fmt.Errorf("embedded input %q for %s collides with generated workspace state at %s; workspace go.mod is not authored embed content", embedFile, packageInfo.ImportPath, destination)
			}
			if strings.HasSuffix(string(key), ".gox.go") {
				return nil, embedInputPlan{}, fmt.Errorf("embedded input %q for %s resolves to workspace-generated GOX source at %s; generated workspace files are not authored embed content", embedFile, packageInfo.ImportPath, destination)
			}
			candidate, ok := candidates[key]
			if !ok {
				return nil, embedInputPlan{}, fmt.Errorf("embedded input %q for %s at workspace destination %s has no valid authored backing file", embedFile, packageInfo.ImportPath, destination)
			}
			source := candidate.SourcePath
			switch candidate.Kind {
			case embedCandidateSymlink:
				return nil, embedInputPlan{}, fmt.Errorf("embedded input %q for %s resolves to symlinked authored input %s; symlink embed inputs are not supported", embedFile, packageInfo.ImportPath, source)
			case embedCandidateIrregular:
				return nil, embedInputPlan{}, fmt.Errorf("embedded input %q for %s resolves to irregular authored input %s; embed inputs must be regular files", embedFile, packageInfo.ImportPath, source)
			case embedCandidateRegular:
			default:
				return nil, embedInputPlan{}, fmt.Errorf("embedded input %q for %s has unknown authored provenance at %s", embedFile, packageInfo.ImportPath, source)
			}
			if err := validatePathBelowRoot(appDir, source, "embedded source input", false); err != nil {
				return nil, embedInputPlan{}, err
			}
			if _, err := regularFileNoFollow(source, "embedded source input"); err != nil {
				return nil, embedInputPlan{}, err
			}
			content, err := os.ReadFile(source)
			if err != nil {
				return nil, embedInputPlan{}, fmt.Errorf("read embedded source input %s: %w", source, err)
			}
			needsCopy := true
			workspaceInfo, workspaceErr := os.Lstat(destination)
			switch {
			case workspaceErr == nil:
				if workspaceInfo.Mode()&os.ModeSymlink != 0 || !workspaceInfo.Mode().IsRegular() {
					return nil, embedInputPlan{}, fmt.Errorf("embedded workspace destination %s for %q is not a regular authored copy", destination, embedFile)
				}
				workspaceContent, err := os.ReadFile(destination)
				if err != nil {
					return nil, embedInputPlan{}, fmt.Errorf("read embedded workspace destination %s: %w", destination, err)
				}
				if !bytes.Equal(workspaceContent, content) {
					return nil, embedInputPlan{}, fmt.Errorf("embedded workspace destination %s for %q does not match authored input %s", destination, embedFile, source)
				}
				needsCopy = false
			case errors.Is(workspaceErr, os.ErrNotExist):
			default:
				return nil, embedInputPlan{}, fmt.Errorf("inspect embedded workspace destination %s: %w", destination, workspaceErr)
			}
			sum := sha256.Sum256(content)
			inputsByKey[key] = resolvedEmbedInput{
				file: embedInputFile{
					SourcePath:  source,
					DisplayPath: string(key),
					Fingerprint: "sha256:" + hex.EncodeToString(sum[:]),
				},
				destination: destination,
				copy:        needsCopy,
			}
		}
	}

	keys := make([]embedInputKey, 0, len(inputsByKey))
	for key := range inputsByKey {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(first, second int) bool {
		return keys[first] < keys[second]
	})
	inputs := make([]resolvedEmbedInput, 0, len(keys))
	plan := embedInputPlan{}
	for _, key := range keys {
		input := inputsByKey[key]
		inputs = append(inputs, input)
		plan.Files = append(plan.Files, input.file)
	}
	for _, watch := range watches {
		plan.Watches = append(plan.Watches, watch)
	}
	sort.Slice(plan.Watches, func(first, second int) bool {
		return embedWatchKey(plan.Watches[first]) < embedWatchKey(plan.Watches[second])
	})
	return inputs, plan, nil
}

func relativePathBelow(root, target string) (string, bool, error) {
	originalRoot, originalTarget := root, target
	root, err := canonicalPathForComparison(root)
	if err != nil {
		return "", false, fmt.Errorf("resolve package root %s: %w", originalRoot, err)
	}
	target, err = canonicalPathForComparison(target)
	if err != nil {
		return "", false, fmt.Errorf("resolve package path %s: %w", originalTarget, err)
	}
	return relativeCanonicalPathBelow(root, target, originalRoot, originalTarget)
}

func newEmbedInputKey(relative string) (embedInputKey, error) {
	if relative == "" || filepath.IsAbs(relative) || filepath.VolumeName(relative) != "" {
		return "", fmt.Errorf("embed input path %q must be relative", relative)
	}
	cleaned := filepath.ToSlash(filepath.Clean(relative))
	if cleaned == "" || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) {
		return "", fmt.Errorf("embed input path %q must stay inside the application workspace", relative)
	}
	return embedInputKey(cleaned), nil
}

func embedPatternWatchSpec(appDir, sourcePackageDir, pattern string) (embedWatchSpec, error) {
	pattern = strings.TrimPrefix(pattern, "all:")
	firstMeta := strings.IndexAny(pattern, "*?[")
	if firstMeta < 0 {
		candidate := filepath.Join(sourcePackageDir, filepath.FromSlash(pattern))
		if err := validateEmbedWatchExactPath(appDir, candidate); err != nil {
			return embedWatchSpec{}, err
		}
		candidate, err := filepath.Abs(candidate)
		if err != nil {
			return embedWatchSpec{}, fmt.Errorf("resolve embed pattern watch path %s: %w", candidate, err)
		}
		kind := embedWatchExact
		if info, err := os.Lstat(candidate); err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			kind = embedWatchTree
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return embedWatchSpec{}, fmt.Errorf("inspect embed pattern watch path %s: %w", candidate, err)
		}
		return newEmbedWatchSpec(appDir, candidate, kind)
	}

	rootRelative := ""
	if firstMeta >= 0 {
		prefix := strings.TrimSuffix(pattern[:firstMeta], "/")
		rootRelative = path.Dir(prefix)
		if strings.HasSuffix(pattern[:firstMeta], "/") {
			rootRelative = prefix
		}
	}
	if rootRelative == "" || rootRelative == "." {
		rootRelative = "."
	}
	root := filepath.Join(sourcePackageDir, filepath.FromSlash(rootRelative))
	if err := validatePathBelowRoot(appDir, root, "embed pattern watch root", true); err != nil {
		return embedWatchSpec{}, err
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return embedWatchSpec{}, fmt.Errorf("resolve embed pattern watch root %s: %w", root, err)
	}
	return newEmbedWatchSpec(appDir, root, embedWatchTree)
}

func embedWatchKey(watch embedWatchSpec) string {
	identity := watch.Identity
	if identity == "" {
		identity = filepath.ToSlash(filepath.Clean(watch.Path))
	}
	return strconv.Itoa(int(watch.Kind)) + ":" + identity
}

func newEmbedWatchSpec(appDir, watchPath string, kind embedWatchKind) (embedWatchSpec, error) {
	relative, inside, err := relativeEmbedWatchPathBelow(appDir, watchPath)
	if err != nil {
		return embedWatchSpec{}, err
	}
	if !inside {
		return embedWatchSpec{}, fmt.Errorf("embed watch path %s must stay inside %s", watchPath, appDir)
	}
	identity := filepath.ToSlash(filepath.Clean(relative))
	if identity == "" || identity == ".." || strings.HasPrefix(identity, "../") || path.IsAbs(identity) {
		return embedWatchSpec{}, fmt.Errorf("embed watch path %s has invalid application-relative identity %q", watchPath, identity)
	}
	return embedWatchSpec{Path: watchPath, Kind: kind, Identity: identity}, nil
}

func relativeEmbedWatchPathBelow(root, target string) (string, bool, error) {
	originalRoot, originalTarget := root, target
	root, err := canonicalPathForComparison(root)
	if err != nil {
		return "", false, fmt.Errorf("resolve embed watch root %s: %w", originalRoot, err)
	}
	info, lstatErr := os.Lstat(target)
	if lstatErr == nil && info.Mode()&os.ModeSymlink != 0 {
		parent, err := canonicalPathForComparison(filepath.Dir(target))
		if err != nil {
			return "", false, fmt.Errorf("resolve embed watch parent %s: %w", filepath.Dir(originalTarget), err)
		}
		target = filepath.Join(parent, filepath.Base(target))
	} else {
		if lstatErr != nil && !errors.Is(lstatErr, os.ErrNotExist) {
			return "", false, fmt.Errorf("inspect embed watch path %s: %w", originalTarget, lstatErr)
		}
		target, err = canonicalPathForComparison(target)
		if err != nil {
			return "", false, fmt.Errorf("resolve embed watch path %s: %w", originalTarget, err)
		}
	}
	return relativeCanonicalPathBelow(root, target, originalRoot, originalTarget)
}

func relativeCanonicalPathBelow(root, target, originalRoot, originalTarget string) (string, bool, error) {
	if !strings.EqualFold(filepath.VolumeName(root), filepath.VolumeName(target)) {
		return "", false, nil
	}
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return "", false, fmt.Errorf("resolve package path %s below %s: %w", originalTarget, originalRoot, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false, nil
	}
	return relative, true, nil
}

func embedPackageMetadataError(packages []embedListPackage) error {
	var failures []error
	seen := map[string]struct{}{}
	for _, packageInfo := range packages {
		if packageInfo.Error != nil && packageInfo.Error.Err != "" {
			message := packageInfo.Error.Err
			if packageInfo.ImportPath != "" {
				message = packageInfo.ImportPath + ": " + message
			}
			if _, ok := seen[message]; !ok {
				seen[message] = struct{}{}
				failures = append(failures, errors.New(message))
			}
		}
		for _, dependencyError := range packageInfo.DepsErrors {
			if dependencyError.Err == "" {
				continue
			}
			if _, ok := seen[dependencyError.Err]; !ok {
				seen[dependencyError.Err] = struct{}{}
				failures = append(failures, errors.New(dependencyError.Err))
			}
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("resolve go:embed inputs: %w", errors.Join(failures...))
}

func populateEmbedWatchMembership(appDir string, plan *embedInputPlan) error {
	membership, err := embedWatchMembership(appDir, plan.Watches)
	plan.WatchMembership = membership
	return err
}

func embedWatchMembership(appDir string, watches []embedWatchSpec) (map[string]string, error) {
	membership := make(map[string]string, len(watches))
	for _, watch := range watches {
		var fingerprint string
		var err error
		switch watch.Kind {
		case embedWatchExact:
			fingerprint, err = embedWatchExactFingerprint(appDir, watch.Path)
		case embedWatchTree:
			fingerprint, err = embedWatchTreeFingerprint(appDir, watch.Path)
		default:
			err = fmt.Errorf("embed watch path %s has unknown kind %d", watch.Path, watch.Kind)
		}
		if err != nil {
			return membership, err
		}
		membership[embedWatchKey(watch)] = fingerprint
	}
	return membership, nil
}

func embedWatchExactFingerprint(appDir, target string) (string, error) {
	if err := validateEmbedWatchExactPath(appDir, target); err != nil {
		return "", err
	}
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return "missing", nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect exact embed watch path %s: %w", target, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "symlink", nil
	}
	if info.Mode().IsRegular() {
		return "regular", nil
	}
	if info.IsDir() {
		return "directory", nil
	}
	return "other", nil
}

func validateEmbedWatchExactPath(appDir, target string) error {
	root, target, err := cleanRootAndTarget(appDir, target)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return fmt.Errorf("resolve exact embed watch path %s below %s: %w", target, root, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("exact embed watch path %s must stay inside %s", target, appDir)
	}
	return validatePathBelowRoot(appDir, filepath.Dir(target), "exact embed watch parent", true)
}

func embedWatchTreeFingerprint(appDir, root string) (string, error) {
	if err := validatePathBelowRoot(appDir, root, "embed watch root", true); err != nil {
		return "", err
	}
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		return "missing", nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect embed watch root %s: %w", root, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("embed watch root %s is a symlink; symlink paths are not supported", root)
	}
	if !info.IsDir() {
		return "not-directory:" + info.Mode().Type().String(), nil
	}
	hash := sha256.New()
	err = filepath.WalkDir(root, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("inspect embed watch input %s: %w", current, walkErr)
		}
		if current != root {
			relativeToApp, err := filepath.Rel(appDir, current)
			if err != nil {
				return fmt.Errorf("resolve embed watch input %s: %w", current, err)
			}
			if shouldSkipEmbedCandidate(relativeToApp, entry) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return fmt.Errorf("resolve embed watch input %s: %w", current, err)
		}
		kind := entry.Type().String()
		if entry.IsDir() {
			kind = "directory"
		} else if entry.Type().IsRegular() {
			kind = "file"
		} else if entry.Type()&os.ModeSymlink != 0 {
			kind = "symlink"
		}
		_, _ = fmt.Fprintf(hash, "%s\x00%s\n", filepath.ToSlash(relative), kind)
		if current != root && entry.IsDir() && nestedModuleBoundary(current) {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func embedInputPlansEqual(first, second embedInputPlan) bool {
	if len(first.Files) != len(second.Files) {
		return false
	}
	for index := range first.Files {
		if first.Files[index].DisplayPath != second.Files[index].DisplayPath ||
			first.Files[index].Fingerprint != second.Files[index].Fingerprint {
			return false
		}
	}
	if len(first.Watches) != len(second.Watches) {
		return false
	}
	for index := range first.Watches {
		if embedWatchKey(first.Watches[index]) != embedWatchKey(second.Watches[index]) {
			return false
		}
	}
	return true
}

func resolveCurrentEmbedInputPlan(options packageOptions) (embedInputPlan, error) {
	manifest, err := loadManifest(options.appDir)
	if err != nil {
		return embedInputPlan{}, err
	}
	compiler := options.compiler
	if compiler == "" {
		compiler = manifest.Compiler
	}
	if err := validateCompiler(compiler); err != nil {
		return embedInputPlan{}, err
	}
	layout, err := newBuildLayout(layoutOptions{
		appDir:    options.appDir,
		compiler:  compiler,
		profile:   packageProfile(options.assetHash, options.preload, options.compress),
		workspace: options.workspace,
	})
	if err != nil {
		return embedInputPlan{}, err
	}
	result, err := prepareBuildWorkspaceResult(layout, manifest)
	return result.EmbedPlan, err
}
