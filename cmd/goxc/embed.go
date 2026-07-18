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
	Files      []embedInputFile
	WatchRoots []string
	Resolved   bool
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

type resolvedEmbedInput struct {
	file        embedInputFile
	destination string
	copy        bool
}

func discoverAndMaterializeEmbedInputs(layout BuildLayout, appWorkDir, entryPath string) (embedInputPlan, error) {
	overlayPath, replacements, cleanup, err := createEmbedDiscoveryOverlay(layout.AppDir, appWorkDir)
	if err != nil {
		return embedInputPlan{}, err
	}
	defer cleanup()

	packages, listErr := listEmbedPackages(layout.Compiler, entryPath, overlayPath)
	if listErr != nil && layout.Compiler == "tinygo" {
		if partial, partialErr := listGoEmbedPackages(entryPath, overlayPath, true); partialErr == nil {
			packages = partial
		}
	}
	inputs, plan, planErr := resolveEmbedInputPlan(layout.AppDir, appWorkDir, replacements, packages)
	if planErr != nil {
		return plan, errors.Join(listErr, planErr)
	}
	if metadataErr := embedPackageMetadataError(packages); metadataErr != nil {
		return plan, errors.Join(listErr, metadataErr)
	}
	if listErr != nil {
		return plan, listErr
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

func createEmbedDiscoveryOverlay(appDir, appWorkDir string) (string, map[string]string, func(), error) {
	appDir, err := filepath.Abs(appDir)
	if err != nil {
		return "", nil, func() {}, fmt.Errorf("resolve embed source root: %w", err)
	}
	appWorkDir, err = filepath.Abs(appWorkDir)
	if err != nil {
		return "", nil, func() {}, fmt.Errorf("resolve embed workspace root: %w", err)
	}
	replacements := map[string]string{}
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
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			if sourcePath != appDir && nestedModuleBoundary(sourcePath) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || strings.HasSuffix(sourcePath, ".gox.go") {
			return nil
		}
		if err := validatePathBelowRoot(appDir, sourcePath, "embed candidate", false); err != nil {
			return err
		}
		destination := filepath.Join(appWorkDir, relative)
		if err := validatePathBelowRoot(appWorkDir, destination, "embed overlay destination", true); err != nil {
			return err
		}
		if samePath(destination, filepath.Join(appWorkDir, "go.mod")) {
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
		return "", nil, func() {}, err
	}

	temporary, err := os.CreateTemp("", "goxc-embed-overlay-*.json")
	if err != nil {
		return "", nil, func() {}, fmt.Errorf("create embed discovery overlay: %w", err)
	}
	overlayPath := temporary.Name()
	cleanup := func() { _ = os.Remove(overlayPath) }
	encoder := json.NewEncoder(temporary)
	encodeErr := encoder.Encode(embedDiscoveryOverlay{Replace: replacements})
	closeErr := temporary.Close()
	if encodeErr != nil || closeErr != nil {
		cleanup()
		return "", nil, func() {}, fmt.Errorf("write embed discovery overlay: %w", errors.Join(encodeErr, closeErr))
	}
	return overlayPath, replacements, cleanup, nil
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

func resolveEmbedInputPlan(appDir, appWorkDir string, replacements map[string]string, packages []embedListPackage) ([]resolvedEmbedInput, embedInputPlan, error) {
	inputsByDestination := map[string]resolvedEmbedInput{}
	watchRoots := map[string]struct{}{}
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
			root, err := embedPatternWatchRoot(appDir, sourcePackageDir, pattern)
			if err != nil {
				return nil, embedInputPlan{}, err
			}
			watchRoots[root] = struct{}{}
		}
		for _, embedFile := range packageInfo.EmbedFiles {
			if filepath.IsAbs(filepath.FromSlash(embedFile)) {
				return nil, embedInputPlan{}, fmt.Errorf("embedded input %q for %s must be package-relative", embedFile, packageInfo.ImportPath)
			}
			destination := filepath.Join(packageInfo.Dir, filepath.FromSlash(embedFile))
			if err := validatePathBelowRoot(appWorkDir, destination, "embedded workspace input", true); err != nil {
				return nil, embedInputPlan{}, err
			}
			source := replacements[destination]
			needsCopy := source != ""
			if source == "" {
				source = filepath.Join(sourcePackageDir, filepath.FromSlash(embedFile))
				if _, err := regularFileNoFollow(source, "embedded source input"); err != nil {
					continue
				}
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
			displayPath, err := filepath.Rel(appDir, source)
			if err != nil {
				return nil, embedInputPlan{}, fmt.Errorf("resolve embedded source input %s: %w", source, err)
			}
			sum := sha256.Sum256(content)
			inputsByDestination[destination] = resolvedEmbedInput{
				file: embedInputFile{
					SourcePath:  source,
					DisplayPath: filepath.ToSlash(displayPath),
					Fingerprint: "sha256:" + hex.EncodeToString(sum[:]),
				},
				destination: destination,
				copy:        needsCopy,
			}
		}
	}

	destinations := make([]string, 0, len(inputsByDestination))
	for destination := range inputsByDestination {
		destinations = append(destinations, destination)
	}
	sort.Strings(destinations)
	inputs := make([]resolvedEmbedInput, 0, len(destinations))
	plan := embedInputPlan{}
	for _, destination := range destinations {
		input := inputsByDestination[destination]
		inputs = append(inputs, input)
		plan.Files = append(plan.Files, input.file)
	}
	for root := range watchRoots {
		plan.WatchRoots = append(plan.WatchRoots, root)
	}
	sort.Strings(plan.WatchRoots)
	return inputs, plan, nil
}

func relativePathBelow(root, target string) (string, bool, error) {
	root, target, err := cleanRootAndTarget(root, target)
	if err != nil {
		return "", false, err
	}
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return "", false, fmt.Errorf("resolve package path %s below %s: %w", target, root, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false, nil
	}
	return relative, true, nil
}

func embedPatternWatchRoot(appDir, sourcePackageDir, pattern string) (string, error) {
	pattern = strings.TrimPrefix(pattern, "all:")
	firstMeta := strings.IndexAny(pattern, "*?[")
	rootRelative := ""
	if firstMeta >= 0 {
		prefix := strings.TrimSuffix(pattern[:firstMeta], "/")
		rootRelative = path.Dir(prefix)
		if strings.HasSuffix(pattern[:firstMeta], "/") {
			rootRelative = prefix
		}
	} else {
		candidate := filepath.Join(sourcePackageDir, filepath.FromSlash(pattern))
		if info, err := os.Lstat(candidate); err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			rootRelative = pattern
		} else {
			rootRelative = path.Dir(pattern)
		}
	}
	if rootRelative == "" || rootRelative == "." {
		rootRelative = "."
	}
	root := filepath.Join(sourcePackageDir, filepath.FromSlash(rootRelative))
	if err := validatePathBelowRoot(appDir, root, "embed pattern watch root", true); err != nil {
		return "", err
	}
	return root, nil
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
