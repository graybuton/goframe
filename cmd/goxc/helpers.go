package main

import (
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
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", sourcePath, err)
	}
	defer source.Close()

	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("inspect %s: %w", sourcePath, err)
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", destinationPath, err)
	}
	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create %s: %w", destinationPath, err)
	}
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if copyErr != nil {
		return fmt.Errorf("copy %s: %w", sourcePath, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", destinationPath, closeErr)
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
