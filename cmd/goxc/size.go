package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	fmt.Printf("size report: %s\n\n", directory)
	fmt.Printf("%-24s %12s %14s\n", "File", "Size", "Bytes")
	fmt.Printf("%-24s %12s %14s\n", "------------------------", "------------", "--------------")

	found := false
	for _, artifact := range reportableArtifacts(directory) {
		found = true
		fmt.Printf("%-24s %12s %14d\n", artifact.name, humanSize(artifact.size), artifact.size)
	}
	if !found {
		return fmt.Errorf("no WASM artifacts found in %s; run `goxc build` or `goxc package` first", directory)
	}
	return nil
}

type sizeArtifact struct {
	name string
	size int64
}

func reportableArtifacts(directory string) []sizeArtifact {
	artifacts := []sizeArtifact{}
	_ = filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return nil
		}
		relative = filepath.ToSlash(relative)
		if !reportableFile(relative) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		artifacts = append(artifacts, sizeArtifact{name: relative, size: info.Size()})
		return nil
	})
	sort.Slice(artifacts, func(first, second int) bool {
		return artifacts[first].name < artifacts[second].name
	})
	return artifacts
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
	found := false
	_ = filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(directory, path)
		if err == nil && reportableFile(filepath.ToSlash(relative)) {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func reportableFile(name string) bool {
	return strings.HasSuffix(name, ".wasm") ||
		strings.HasSuffix(name, ".wasm.gz") ||
		strings.HasSuffix(name, ".wasm.br") ||
		strings.HasSuffix(name, ".wasm.zst") ||
		strings.HasPrefix(filepath.Base(name), "wasm_exec") && strings.HasSuffix(name, ".js")
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
