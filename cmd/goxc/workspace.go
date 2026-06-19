package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/graybuton/goframe/pkg/gox"
)

var (
	findRepositoryRootForWorkspace = findRepositoryRoot
	goframeModuleVersionForBuild   = goframeModuleVersion
)

func prepareBuildWorkspace(layout BuildLayout, manifest projectManifest) (string, error) {
	if manifest.Entry != "." {
		return "", fmt.Errorf("entry %q is not supported by the hidden workspace builder yet; MVP 20 supports multi-package apps under entry %q", manifest.Entry, ".")
	}
	if err := refreshDirectory(layout.WorkDir); err != nil {
		return "", err
	}
	config := workspaceModuleConfigForApp(layout.AppDir)
	appWorkDir := filepath.Join(layout.WorkDir, filepath.FromSlash(config.AppRel))
	if err := copyAuthoredGoFiles(layout.AppDir, appWorkDir); err != nil {
		return "", err
	}
	if err := generateIntoDirectory(layout.AppDir, appWorkDir, false); err != nil {
		return "", err
	}
	if config.CopyGoframeRuntime {
		if err := copyGoframeRuntimePackage(layout.WorkDir, config.ModuleRoot); err != nil {
			return "", err
		}
	}
	if err := writeWorkspaceGoMod(layout.WorkDir, layout.AppDir); err != nil {
		return "", err
	}
	return filepath.Join(appWorkDir, manifest.Entry), nil
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

func copyAuthoredGoFiles(sourceRoot, destinationRoot string) error {
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
		if entry.IsDir() {
			return nil
		}
		if filepath.Ext(sourcePath) != ".go" || strings.HasSuffix(sourcePath, ".gox.go") {
			return nil
		}
		return copyFile(sourcePath, filepath.Join(destinationRoot, relative))
	})
}

func generateIntoDirectory(sourceRoot, destinationRoot string, requireFiles bool) error {
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
	for _, file := range files {
		relative, err := filepath.Rel(sourceRoot, file)
		if err != nil {
			return fmt.Errorf("resolve GOX source %s: %w", file, err)
		}
		output := filepath.Join(destinationRoot, relative+".go")
		if _, err := gox.GenerateFileToWithOptions(file, output, gox.GenerateOptions{
			Filename:        file,
			PackageIdentity: packageIdentityForFile(sourceRoot, file),
		}); err != nil {
			return err
		}
	}
	return nil
}

func writeWorkspaceGoMod(workDir, appDir string) error {
	config := workspaceModuleConfigForApp(appDir)
	content := strings.Builder{}
	modulePath := config.ModulePath
	if modulePath == "" {
		modulePath = "goframe-app-build"
	}
	content.WriteString("module " + modulePath + "\n\n")
	content.WriteString("go 1.22\n\n")
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
	if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte(content.String()), 0o644); err != nil {
		return fmt.Errorf("write workspace go.mod: %w", err)
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
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if filepath.Ext(path) != ".gox" {
			return nil, fmt.Errorf("%s is not a .gox file", path)
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
		if !entry.IsDir() && filepath.Ext(current) == ".gox" {
			files = append(files, current)
		}
		return nil
	})
	return files, err
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
