package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func sizeCommand(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: goxc size <app-or-artifact-directory>")
	}

	directory, err := artifactDirectory(args[0])
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read artifact directory: %w", err)
	}

	fmt.Printf("size report: %s\n\n", directory)
	fmt.Printf("%-24s %12s %14s\n", "File", "Size", "Bytes")
	fmt.Printf("%-24s %12s %14s\n", "------------------------", "------------", "--------------")

	found := false
	for _, entry := range entries {
		if entry.IsDir() || !reportableFile(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect %s: %w", entry.Name(), err)
		}
		found = true
		fmt.Printf("%-24s %12s %14d\n", entry.Name(), humanSize(info.Size()), info.Size())
	}
	if !found {
		return fmt.Errorf("no WASM artifacts found in %s; run `goxc build` or `goxc package` first", directory)
	}
	return nil
}

func artifactDirectory(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", path)
	}
	for _, child := range []string{"dist", "build"} {
		candidate := filepath.Join(path, child)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() && containsArtifact(candidate) {
			return candidate, nil
		}
	}
	return path, nil
}

func containsArtifact(directory string) bool {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && reportableFile(entry.Name()) {
			return true
		}
	}
	return false
}

func reportableFile(name string) bool {
	return strings.HasSuffix(name, ".wasm") ||
		strings.HasSuffix(name, ".wasm.gz") ||
		strings.HasSuffix(name, ".wasm.br") ||
		name == "wasm_exec.js"
}

func humanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	for _, suffix := range []string{"KiB", "MiB", "GiB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f TiB", value/unit)
}
