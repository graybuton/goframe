package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func wasmExecPath(compiler string) (string, error) {
	if compiler == "tinygo" {
		command := exec.Command("tinygo", "env", "TINYGOROOT")
		output, err := command.Output()
		if err != nil {
			return "", fmt.Errorf("locate TinyGo wasm_exec.js: %w", err)
		}
		path := filepath.Join(strings.TrimSpace(string(output)), "targets", "wasm_exec.js")
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("locate TinyGo wasm_exec.js at %s: %w", path, err)
		}
		return path, nil
	}

	for _, path := range []string{
		filepath.Join(runtime.GOROOT(), "lib", "wasm", "wasm_exec.js"),
		filepath.Join(runtime.GOROOT(), "misc", "wasm", "wasm_exec.js"),
	} {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("locate Go wasm_exec.js below %s", runtime.GOROOT())
}

func copyFile(sourcePath, destinationPath string) error {
	if samePath(sourcePath, destinationPath) {
		return nil
	}
	info, err := regularFileNoFollow(sourcePath, "source file")
	if err != nil {
		return err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", sourcePath, err)
	}
	defer source.Close()

	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", destinationPath, err)
	}
	return writeStreamAtomic(destinationPath, source, info.Mode().Perm())
}

func writeFileAtomic(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	return writeStreamAtomic(path, bytes.NewReader(content), mode)
}

func writeStreamAtomic(destinationPath string, source io.Reader, mode os.FileMode) error {
	directory := filepath.Dir(destinationPath)
	if info, err := os.Lstat(destinationPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("destination %s is a symlink; symlink paths are not supported", destinationPath)
		}
		if info.IsDir() {
			return fmt.Errorf("destination %s is a directory", destinationPath)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect destination %s: %w", destinationPath, err)
	}
	temp, err := os.CreateTemp(directory, ".goframe-write-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", destinationPath, err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	_, copyErr := io.Copy(temp, source)
	chmodErr := temp.Chmod(mode)
	closeErr := temp.Close()
	if copyErr != nil {
		return fmt.Errorf("write %s: %w", destinationPath, copyErr)
	}
	if chmodErr != nil {
		return fmt.Errorf("chmod %s: %w", tempPath, chmodErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", tempPath, closeErr)
	}
	if err := os.Rename(tempPath, destinationPath); err != nil {
		return fmt.Errorf("replace %s: %w", destinationPath, err)
	}
	return nil
}

func samePath(first, second string) bool {
	firstPath, firstErr := filepath.Abs(first)
	secondPath, secondErr := filepath.Abs(second)
	return firstErr == nil && secondErr == nil && firstPath == secondPath
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func rejectSymlinkPath(path string, description string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s %s: %w", description, path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s %s is a symlink; symlink paths are not supported", description, path)
	}
	return nil
}

func regularFileNoFollow(path string, description string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect %s %s: %w", description, path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s %s is a symlink; symlink paths are not supported", description, path)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s %s is not a regular file", description, path)
	}
	return info, nil
}

func directoryNoFollow(path string, description string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect %s %s: %w", description, path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s %s is a symlink; symlink paths are not supported", description, path)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s %s is not a directory", description, path)
	}
	return nil
}

func validatePathBelowRoot(root, target, description string, allowMissingTail bool) error {
	root, target, err := cleanRootAndTarget(root, target)
	if err != nil {
		return err
	}
	if err := rejectSymlinkPath(root, description+" root"); err != nil {
		return err
	}
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return fmt.Errorf("resolve %s %s below %s: %w", description, target, root, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s %s must stay inside %s", description, target, root)
	}
	if relative == "." {
		return rejectSymlinkPath(root, description)
	}
	return rejectSymlinkComponents(root, target, description, allowMissingTail)
}

func cleanRootAndTarget(root, target string) (string, string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve root %s: %w", root, err)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", "", fmt.Errorf("resolve path %s: %w", target, err)
	}
	return filepath.Clean(root), filepath.Clean(target), nil
}

func rejectSymlinkComponents(root, target, description string, allowMissingTail bool) error {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return fmt.Errorf("resolve %s %s below %s: %w", description, target, root, err)
	}
	if relative == "." {
		return rejectSymlinkPath(root, description)
	}
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if allowMissingTail {
				return nil
			}
			return fmt.Errorf("inspect %s %s: %w", description, current, err)
		}
		if err != nil {
			return fmt.Errorf("inspect %s %s: %w", description, current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s %s is a symlink; symlink paths are not supported", description, current)
		}
	}
	return nil
}

func mkdirAllBelowRoot(root, directory, description string) error {
	if err := validatePathBelowRoot(root, directory, description, true); err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create %s %s: %w", description, directory, err)
	}
	return validatePathBelowRoot(root, directory, description, false)
}

func validateExplicitPathRoot(path string, description string, allowMissing bool) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve %s %s: %w", description, path, err)
	}
	parent := filepath.Dir(path)
	return validatePathBelowRoot(parent, path, description, allowMissing)
}

func pathRelation(first, second string) (string, error) {
	first, err := filepath.Abs(first)
	if err != nil {
		return "", err
	}
	second, err = filepath.Abs(second)
	if err != nil {
		return "", err
	}
	first = filepath.Clean(first)
	second = filepath.Clean(second)
	if first == second {
		return "same", nil
	}
	if pathContains(first, second) {
		return "contains", nil
	}
	if pathContains(second, first) {
		return "inside", nil
	}
	return "separate", nil
}

func pathContains(root, child string) bool {
	relative, err := filepath.Rel(root, child)
	if err != nil {
		return false
	}
	return relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func pathsOverlap(first, second string) bool {
	relation, err := pathRelation(first, second)
	return err == nil && relation != "separate"
}
