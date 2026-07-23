package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/graybuton/goframe/pkg/gox"
)

var (
	findRepositoryRootForWorkspace = findRepositoryRoot
	goframeModuleVersionForBuild   = goframeModuleVersion
)

func prepareBuildWorkspace(layout BuildLayout, manifest projectManifest) (string, error) {
	result, err := prepareBuildWorkspaceResult(layout, manifest)
	return result.EntryPath, err
}

type buildWorkspaceResult struct {
	EntryPath string
	EmbedPlan embedInputPlan
}

func prepareBuildWorkspaceResult(layout BuildLayout, manifest projectManifest) (buildWorkspaceResult, error) {
	var result buildWorkspaceResult
	if err := validateWorkspaceRoot(layout); err != nil {
		return result, err
	}
	entry, err := resolveEntryPackageDir(layout.AppDir, manifest.Entry)
	if err != nil {
		return result, err
	}
	if err := validatePathBelowRoot(layout.WorkspaceRoot, layout.WorkDir, "workspace work directory", true); err != nil {
		return result, err
	}
	if err := refreshDirectory(layout.WorkDir); err != nil {
		return result, err
	}
	config := workspaceModuleConfigForApp(layout.AppDir)
	appWorkDir := filepath.Join(layout.WorkDir, filepath.FromSlash(config.AppRel))
	if err := copyAuthoredGoFiles(layout.AppDir, appWorkDir); err != nil {
		return result, err
	}
	if err := generateIntoDirectoryForCompiler(
		layout.AppDir,
		appWorkDir,
		false,
		layout.Compiler,
	); err != nil {
		return result, err
	}
	if config.CopyGoframeRuntime {
		if err := copyGoframeRuntimePackage(layout.WorkDir, config.ModuleRoot); err != nil {
			return result, err
		}
	}
	if err := writeWorkspaceGoMod(layout.WorkDir, layout.AppDir); err != nil {
		return result, err
	}
	entryRelative, err := filepath.Rel(layout.AppDir, entry)
	if err != nil {
		return result, fmt.Errorf("resolve entry workspace path: %w", err)
	}
	result.EntryPath = filepath.Join(appWorkDir, entryRelative)
	result.EmbedPlan, err = discoverAndMaterializeEmbedInputs(layout, appWorkDir, result.EntryPath)
	if err != nil {
		return result, err
	}
	return result, nil
}

func resolveEntryPackageDir(appDir, entry string) (string, error) {
	originalEntry := entry
	entry, err := cleanManifestEntry(entry)
	if err != nil {
		return "", fmt.Errorf("entry %q %s", originalEntry, err)
	}
	appDir, err = filepath.Abs(appDir)
	if err != nil {
		return "", fmt.Errorf("resolve application directory: %w", err)
	}
	entryDir := appDir
	if entry != "." {
		entryDir = filepath.Join(appDir, filepath.FromSlash(entry))
	}
	if err := validatePathBelowRoot(appDir, entryDir, "entry directory", false); err != nil {
		return "", err
	}
	relative, err := filepath.Rel(appDir, entryDir)
	if err != nil {
		return "", fmt.Errorf("resolve entry %q: %w", entry, err)
	}
	relative = filepath.ToSlash(filepath.Clean(relative))
	if relative == ".." || strings.HasPrefix(relative, "../") {
		return "", fmt.Errorf("entry %q must be a relative child package inside the app root", entry)
	}
	info, err := os.Stat(entryDir)
	if err != nil {
		return "", fmt.Errorf("entry %q does not exist or is not readable: %w", entry, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("entry %q points to a file; entry must be a Go package directory", entry)
	}
	return entryDir, nil
}

func refreshDirectory(directory string) error {
	if err := os.RemoveAll(directory); err != nil {
		return fmt.Errorf("remove stale workspace directory %s: %w", directory, err)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create workspace directory %s: %w", directory, err)
	}
	return nil
}

func validateWorkspaceRoot(layout BuildLayout) error {
	if layout.ExternalWorkspace {
		if err := ensureNoPhysicalOverlap(layout.WorkspaceRoot, layout.AppDir, "workspace root", "application directory"); err != nil {
			return err
		}
		return validatePathBelowRoot(layout.WorkspaceBase, layout.WorkspaceRoot, "workspace root", true)
	}
	return validatePathBelowRoot(layout.AppDir, layout.WorkspaceRoot, "workspace root", true)
}

func copyAuthoredGoFiles(sourceRoot, destinationRoot string) error {
	if pathsOverlap(sourceRoot, destinationRoot) && !isToolOwnedDestinationBelowSource(sourceRoot, destinationRoot) {
		return fmt.Errorf("copy destination %s must not overlap source root %s", destinationRoot, sourceRoot)
	}
	return filepath.WalkDir(sourceRoot, func(sourcePath string, entry os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("inspect source file %s: %w", sourcePath, err)
		}
		if sourcePath == sourceRoot {
			return nil
		}
		relative, err := filepath.Rel(sourceRoot, sourcePath)
		if err != nil {
			return fmt.Errorf("resolve source path %s: %w", sourcePath, err)
		}
		if shouldSkipWorkspaceSource(relative, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if filepath.Ext(sourcePath) == ".go" && !strings.HasSuffix(sourcePath, ".gox.go") {
				return fmt.Errorf("source path %s is a symlink; symlinked source files are not supported", sourcePath)
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Ext(sourcePath) != ".go" || strings.HasSuffix(sourcePath, ".gox.go") {
			return nil
		}
		if _, err := regularFileNoFollow(sourcePath, "source file"); err != nil {
			return err
		}
		if err := validatePathBelowRoot(destinationRoot, filepath.Join(destinationRoot, relative), "workspace authored file", true); err != nil {
			return err
		}
		return copyFile(sourcePath, filepath.Join(destinationRoot, relative))
	})
}

func isToolOwnedDestinationBelowSource(sourceRoot, destinationRoot string) bool {
	sourceRoot, err := filepath.Abs(sourceRoot)
	if err != nil {
		return false
	}
	destinationRoot, err = filepath.Abs(destinationRoot)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(sourceRoot, destinationRoot)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
		return false
	}
	parts := strings.Split(filepath.ToSlash(relative), "/")
	return len(parts) > 0 && parts[0] == defaultWorkspaceName
}

func generateIntoDirectory(sourceRoot, destinationRoot string, requireFiles bool) error {
	return generateIntoDirectoryWithSelection(
		sourceRoot,
		destinationRoot,
		requireFiles,
		defaultGenerationSourceSelection(),
	)
}

func generateIntoDirectoryForCompiler(
	sourceRoot,
	destinationRoot string,
	requireFiles bool,
	compiler string,
) error {
	selection, err := browserGenerationSourceSelection(compiler)
	if err != nil {
		return err
	}
	return generateIntoDirectoryWithSelection(
		sourceRoot,
		destinationRoot,
		requireFiles,
		selection,
	)
}

func generateIntoDirectoryWithSelection(
	sourceRoot,
	destinationRoot string,
	requireFiles bool,
	selection generationSourceSelection,
) error {
	files, err := findGOXFiles(sourceRoot)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		if requireFiles {
			return fmt.Errorf("no .gox files found below %s", sourceRoot)
		}
		return nil
	}
	active, err := generateFilesIntoDirectoryWithSelectionResult(
		sourceRoot,
		destinationRoot,
		files,
		selection,
	)
	if err != nil {
		return err
	}
	if len(active) == 0 && requireFiles {
		return noActiveGOXFilesError(sourceRoot, selection)
	}
	return nil
}

type goxGenerationTarget struct {
	source  string
	output  string
	content []byte
	publish bool
}

type goxGenerationRequest struct {
	allocationFiles  []string
	publicationFiles []string
}

type generatedGOXFile struct {
	path    string
	content []byte
}

func generateFilesIntoDirectory(sourceRoot, destinationRoot string, files []string) error {
	return generateFilesIntoDirectoryWithSelection(
		sourceRoot,
		destinationRoot,
		files,
		defaultGenerationSourceSelection(),
	)
}

func generateFilesIntoDirectoryWithSelection(
	sourceRoot,
	destinationRoot string,
	files []string,
	selection generationSourceSelection,
) error {
	_, err := generateFilesIntoDirectoryWithSelectionResult(
		sourceRoot,
		destinationRoot,
		files,
		selection,
	)
	return err
}

func generateFilesIntoDirectoryWithSelectionResult(
	sourceRoot,
	destinationRoot string,
	files []string,
	selection generationSourceSelection,
) ([]goxGenerationTarget, error) {
	return generateFilesIntoDirectoryWithSelectionRequestResult(
		sourceRoot,
		destinationRoot,
		goxGenerationRequest{
			allocationFiles:  files,
			publicationFiles: files,
		},
		selection,
	)
}

func generateFilesIntoDirectoryWithSelectionRequestResult(
	sourceRoot,
	destinationRoot string,
	request goxGenerationRequest,
	selection generationSourceSelection,
) ([]goxGenerationTarget, error) {
	publicationPaths := make(map[string]struct{}, len(request.publicationFiles))
	for _, file := range request.publicationFiles {
		if _, err := regularFileNoFollow(file, "published GOX source file"); err != nil {
			return nil, err
		}
		path, err := canonicalPathForComparison(file)
		if err != nil {
			return nil, fmt.Errorf("resolve published GOX source %s: %w", file, err)
		}
		publicationPaths[path] = struct{}{}
	}

	targets := make([]goxGenerationTarget, 0, len(request.allocationFiles))
	for _, file := range request.allocationFiles {
		if _, err := regularFileNoFollow(file, "GOX source file"); err != nil {
			return nil, err
		}
		relative, err := filepath.Rel(sourceRoot, file)
		if err != nil {
			return nil, fmt.Errorf("resolve GOX source %s: %w", file, err)
		}
		path, err := canonicalPathForComparison(file)
		if err != nil {
			return nil, fmt.Errorf("resolve GOX source identity %s: %w", file, err)
		}
		_, publish := publicationPaths[path]
		targets = append(targets, goxGenerationTarget{
			source:  file,
			output:  filepath.Join(destinationRoot, relative+".go"),
			publish: publish,
		})
	}
	return generatePackageTargetsSafely(
		sourceRoot,
		destinationRoot,
		targets,
		selection,
	)
}

func generatePackageTargetsSafely(
	sourceRoot,
	destinationRoot string,
	targets []goxGenerationTarget,
	selection generationSourceSelection,
) ([]goxGenerationTarget, error) {
	activeTargets := make([]goxGenerationTarget, 0, len(targets))
	publishedActiveTargets := make([]goxGenerationTarget, 0, len(targets))
	inactiveTargets := make([]goxGenerationTarget, 0, len(targets))
	for _, target := range targets {
		content, err := readGenerationSource(target.source, "GOX source file")
		if err != nil {
			return nil, err
		}
		matched, err := selection.matchGOX(
			filepath.Dir(target.source),
			filepath.Base(target.source),
			content,
		)
		if err != nil {
			return nil, err
		}
		target.content = content
		if matched {
			activeTargets = append(activeTargets, target)
			if target.publish {
				publishedActiveTargets = append(publishedActiveTargets, target)
			}
		} else {
			inactiveTargets = append(inactiveTargets, target)
		}
	}

	packages := make(map[string][]goxGenerationTarget)
	for _, target := range activeTargets {
		packageDir := filepath.Dir(target.source)
		packages[packageDir] = append(packages[packageDir], target)
	}
	packageDirs := make([]string, 0, len(packages))
	for packageDir := range packages {
		packageDirs = append(packageDirs, packageDir)
	}
	sort.Strings(packageDirs)

	generatedFiles := make([]generatedGOXFile, 0, len(activeTargets))
	for _, packageDir := range packageDirs {
		packageTargets := packages[packageDir]
		sort.Slice(packageTargets, func(left, right int) bool {
			return packageTargets[left].source < packageTargets[right].source
		})
		sources := make([]gox.PackageSource, 0, len(packageTargets))
		for _, target := range packageTargets {
			sources = append(sources, gox.PackageSource{
				Filename: target.source,
				Source:   target.content,
			})
		}
		authored, err := authoredPackageSources(packageDir, selection)
		if err != nil {
			return nil, err
		}
		generated, err := gox.GeneratePackageWithOptions(sources, gox.PackageGenerateOptions{
			PackageIdentity: packageIdentityForFile(sourceRoot, packageTargets[0].source),
			AuthoredSources: authored,
		})
		if err != nil {
			return nil, fmt.Errorf("generate failed for %s: %w", generationFailureSource(packageTargets, err), err)
		}
		for _, target := range packageTargets {
			if !target.publish {
				continue
			}
			generatedFiles = append(generatedFiles, generatedGOXFile{
				path:    target.output,
				content: generated[target.source],
			})
		}
	}

	removals := make([]string, 0, len(inactiveTargets))
	for _, target := range inactiveTargets {
		if !target.publish {
			continue
		}
		remove, err := shouldRemoveInactiveGeneratedOutput(
			destinationRoot,
			target.output,
		)
		if err != nil {
			return nil, err
		}
		if remove {
			removals = append(removals, target.output)
		}
	}

	for _, generated := range generatedFiles {
		if err := os.MkdirAll(filepath.Dir(generated.path), 0o755); err != nil {
			return nil, fmt.Errorf("create generated directory for %s: %w", generated.path, err)
		}
		if err := writeFileAtomic(generated.path, generated.content, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", generated.path, err)
		}
	}
	for _, path := range removals {
		if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("remove inactive generated output %s: %w", path, err)
		}
	}
	return publishedActiveTargets, nil
}

func activeGenerationGOXFile(
	path string,
	selection generationSourceSelection,
) (bool, error) {
	content, err := readGenerationSource(path, "GOX source file")
	if err != nil {
		return false, err
	}
	return selection.matchGOX(
		filepath.Dir(path),
		filepath.Base(path),
		content,
	)
}

func readGenerationSource(path, label string) ([]byte, error) {
	info, err := regularFileNoFollow(path, label)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if info.Size() < 0 {
		return nil, fmt.Errorf("inspect %s: invalid source size", path)
	}
	return content, nil
}

func authoredPackageSources(
	packageDir string,
	selection generationSourceSelection,
) ([]gox.PackageSource, error) {
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		return nil, fmt.Errorf("read Go package directory %s: %w", packageDir, err)
	}
	var sources []gox.PackageSource
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, ".gox.go") {
			continue
		}
		path := filepath.Join(packageDir, name)
		content, err := readGenerationSource(path, "authored Go source file")
		if err != nil {
			return nil, err
		}
		matched, err := selection.matchAuthoredGo(packageDir, name, content)
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}
		sources = append(sources, gox.PackageSource{Filename: path, Source: content})
	}
	return sources, nil
}

const generatedGOXFileHeader = "// Code generated by goxc; DO NOT EDIT.\n\n"

func shouldRemoveInactiveGeneratedOutput(root, path string) (bool, error) {
	if err := validatePathBelowRoot(root, path, "inactive generated output", true); err != nil {
		return false, err
	}
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect inactive generated output %s: %w", path, err)
	}
	content, err := readGenerationSource(path, "inactive generated output")
	if err != nil {
		return false, err
	}
	if !bytes.HasPrefix(content, []byte(generatedGOXFileHeader)) {
		return false, fmt.Errorf(
			"refuse to remove inactive generated output %s: file is not managed by goxc",
			path,
		)
	}
	return true, nil
}

func noActiveGOXFilesError(path string, selection generationSourceSelection) error {
	return fmt.Errorf(
		"no active .gox files found below %s for %s",
		path,
		selection.targetName(),
	)
}

func generationFailureSource(targets []goxGenerationTarget, err error) string {
	var diagnostic gox.DiagnosticError
	if errors.As(err, &diagnostic) && diagnostic.Diagnostic.Filename != "" {
		return diagnostic.Diagnostic.Filename
	}
	for _, target := range targets {
		if strings.Contains(err.Error(), target.source) {
			return target.source
		}
	}
	return targets[0].source
}

func generateFileSafely(file, output string, options gox.GenerateOptions) error {
	content, err := readGenerationSource(file, "GOX source file")
	if err != nil {
		return err
	}
	generated, err := gox.GenerateWithOptions(content, options)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("create generated directory for %s: %w", output, err)
	}
	if err := writeFileAtomic(output, generated, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", output, err)
	}
	return nil
}

func writeWorkspaceGoMod(workDir, appDir string) error {
	config := workspaceModuleConfigForApp(appDir)
	appModule, err := readWorkspaceModuleDirectives(config.ModuleRoot)
	if err != nil {
		return err
	}
	content := strings.Builder{}
	modulePath := config.ModulePath
	if modulePath == "" {
		modulePath = "goframe-app-build"
	}
	content.WriteString("module " + modulePath + "\n\n")
	content.WriteString("go 1.22\n\n")
	writeWorkspaceRequires(&content, appModule.Requires)
	if modulePath != canonicalModulePath {
		if repoRoot, ok := findRepositoryRootForWorkspace(appDir); ok {
			content.WriteString("require " + canonicalModulePath + " v0.0.0\n")
			content.WriteString("\nreplace " + canonicalModulePath + " => " + filepath.ToSlash(repoRoot) + "\n")
		} else if repoRoot, ok := findRepositoryRootForWorkspace("."); ok {
			content.WriteString("require " + canonicalModulePath + " v0.0.0\n")
			content.WriteString("\nreplace " + canonicalModulePath + " => " + filepath.ToSlash(repoRoot) + "\n")
		} else {
			version := goframeModuleVersionForBuild()
			if version == "" || version == "v0.0.0" {
				return fmt.Errorf("cannot create build workspace module: goframe repository root was not found and this goxc binary does not have a versioned %s module dependency; run goxc from the goframe checkout so a local module replace can be written, install a released goxc binary, or set GOFRAME_WORKSPACE/--workspace for read-only source while keeping goxc able to locate the repository", canonicalModulePath)
			}
			content.WriteString("require " + canonicalModulePath + " " + version + "\n")
		}
	}
	writeWorkspaceReplaces(&content, appModule.Replaces)
	if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte(content.String()), 0o644); err != nil {
		return fmt.Errorf("write workspace go.mod: %w", err)
	}
	if err := copyWorkspaceGoSum(workDir, config.ModuleRoot); err != nil {
		return err
	}
	return nil
}

func copyGoframeRuntimePackage(workDir, moduleRoot string) error {
	if moduleRoot == "" {
		return fmt.Errorf("cannot copy goframe runtime into workspace: module root was not found")
	}
	source := filepath.Join(moduleRoot, "pkg", "goframe")
	destination := filepath.Join(workDir, "pkg", "goframe")
	if err := copyAuthoredGoFiles(source, destination); err != nil {
		return fmt.Errorf("copy goframe runtime package: %w", err)
	}
	return nil
}

func shouldSkipWorkspaceSource(relative string, entry os.DirEntry) bool {
	parts := strings.Split(filepath.ToSlash(relative), "/")
	if len(parts) == 0 {
		return false
	}
	switch parts[0] {
	case defaultWorkspaceName, "build", "dist", "node_modules", ".git", ".goxc-tmp":
		return true
	}
	if entry.IsDir() && strings.HasPrefix(parts[0], ".") && parts[0] != "." {
		return true
	}
	return false
}

func findGOXFiles(path string) ([]string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("GOX source path %s is a symlink; symlinked source files are not supported", path)
	}
	if !info.IsDir() {
		if filepath.Ext(path) != ".gox" {
			return nil, fmt.Errorf("%s is not a .gox file", path)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s is not a regular .gox file", path)
		}
		return []string{path}, nil
	}

	var files []string
	err = filepath.WalkDir(path, func(current string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if current == path {
			return nil
		}
		relative, err := filepath.Rel(path, current)
		if err != nil {
			return err
		}
		if shouldSkipWorkspaceSource(relative, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if filepath.Ext(current) == ".gox" {
				return fmt.Errorf("GOX source path %s is a symlink; symlinked source files are not supported", current)
			}
			return nil
		}
		if !entry.IsDir() && filepath.Ext(current) == ".gox" {
			files = append(files, current)
		}
		return nil
	})
	return files, err
}

func findImmediatePackageGOXFiles(path string) ([]string, error) {
	packageDir := filepath.Dir(path)
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		return nil, fmt.Errorf("read GOX package directory %s: %w", packageDir, err)
	}

	files := make([]string, 0)
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".gox" {
			continue
		}
		candidate := filepath.Join(packageDir, entry.Name())
		if _, err := regularFileNoFollow(candidate, "GOX source file"); err != nil {
			return nil, err
		}
		files = append(files, candidate)
	}
	sort.Strings(files)
	return files, nil
}

func packageIdentityForFile(appDir, file string) string {
	appPath := workspaceModulePath(appDir)
	if appPath == "" {
		return ""
	}
	appDir, err := filepath.Abs(appDir)
	if err != nil {
		return ""
	}
	file, err = filepath.Abs(file)
	if err != nil {
		return ""
	}
	packageDir := filepath.Dir(file)
	relative, err := filepath.Rel(appDir, packageDir)
	if err != nil {
		return ""
	}
	relative = filepath.ToSlash(filepath.Clean(relative))
	if relative == "." {
		return appPath
	}
	return appPath + "/" + relative
}

func workspaceModulePath(appDir string) string {
	return workspaceModuleConfigForApp(appDir).AppImportPath
}

type workspaceModuleConfig struct {
	ModuleRoot         string
	ModulePath         string
	AppRel             string
	AppImportPath      string
	CopyGoframeRuntime bool
}

func workspaceModuleConfigForApp(appDir string) workspaceModuleConfig {
	appDir, err := filepath.Abs(appDir)
	if err != nil {
		return workspaceModuleConfig{
			ModulePath:    "goframe-app-build",
			AppRel:        ".",
			AppImportPath: "goframe-app-build",
		}
	}
	info, ok, err := findNearestGoModule(appDir)
	if err != nil || !ok {
		return workspaceModuleConfig{
			ModulePath:    "goframe-app-build",
			AppRel:        ".",
			AppImportPath: "goframe-app-build",
		}
	}
	relative, err := filepath.Rel(info.Root, appDir)
	if err != nil {
		relative = "."
	}
	relative = filepath.ToSlash(filepath.Clean(relative))
	if relative == "." {
		return workspaceModuleConfig{
			ModuleRoot:         info.Root,
			ModulePath:         info.Path,
			AppRel:             ".",
			AppImportPath:      info.Path,
			CopyGoframeRuntime: info.Path == canonicalModulePath,
		}
	}
	if strings.HasPrefix(relative, "../") || relative == ".." {
		relative = "."
	}
	return workspaceModuleConfig{
		ModuleRoot:         info.Root,
		ModulePath:         info.Path,
		AppRel:             relative,
		AppImportPath:      info.Path + "/" + relative,
		CopyGoframeRuntime: info.Path == canonicalModulePath,
	}
}

type goModuleInfo struct {
	Root string
	Path string
}

func findNearestGoModule(start string) (goModuleInfo, bool, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return goModuleInfo{}, false, err
	}
	if info, err := os.Stat(current); err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}
	for {
		goMod := filepath.Join(current, "go.mod")
		modulePath, err := readModulePath(goMod)
		if err == nil {
			return goModuleInfo{Root: current, Path: modulePath}, true, nil
		}
		if !os.IsNotExist(err) {
			return goModuleInfo{}, false, err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return goModuleInfo{}, false, nil
		}
		current = parent
	}
}

func readModulePath(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("module path not found in %s", path)
}
