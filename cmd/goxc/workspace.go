package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/graybuton/goframe/pkg/gox"
)

func prepareBuildWorkspace(layout BuildLayout, manifest projectManifest) (string, error) {
	if manifest.Entry != "." {
		return "", fmt.Errorf("entry %q is not supported by the hidden workspace builder yet; use entry %q for single-package apps", manifest.Entry, ".")
	}
	if err := refreshDirectory(layout.WorkDir); err != nil {
		return "", err
	}
	if err := copyAuthoredGoFiles(layout.AppDir, layout.WorkDir); err != nil {
		return "", err
	}
	if err := generateIntoDirectory(layout.AppDir, layout.WorkDir, false); err != nil {
		return "", err
	}
	if err := writeWorkspaceGoMod(layout.WorkDir, layout.AppDir); err != nil {
		return "", err
	}
	return layout.WorkDir, nil
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
	files, err := gox.FindFiles(sourceRoot)
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
		if _, err := gox.GenerateFileTo(file, output); err != nil {
			return err
		}
	}
	return nil
}

func writeWorkspaceGoMod(workDir, appDir string) error {
	content := strings.Builder{}
	content.WriteString("module goframe-app-build\n\n")
	content.WriteString("go 1.22\n\n")
	if repoRoot, ok := findRepositoryRoot(appDir); ok {
		content.WriteString("require " + canonicalModulePath + " v0.0.0\n")
		content.WriteString("\nreplace " + canonicalModulePath + " => " + filepath.ToSlash(repoRoot) + "\n")
	} else if repoRoot, ok := findRepositoryRoot("."); ok {
		content.WriteString("require " + canonicalModulePath + " v0.0.0\n")
		content.WriteString("\nreplace " + canonicalModulePath + " => " + filepath.ToSlash(repoRoot) + "\n")
	} else {
		content.WriteString("require " + canonicalModulePath + " " + goframeModuleVersion() + "\n")
	}
	if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte(content.String()), 0o644); err != nil {
		return fmt.Errorf("write workspace go.mod: %w", err)
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
