package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type cleanOptions struct {
	appDir    string
	workspace string
	generated bool
	legacy    bool
}

type adjacentGeneratedFileCleanup struct {
	path   string
	info   os.FileInfo
	digest [sha256.Size]byte
}

type adjacentGeneratedFileCleanupPlan struct {
	root  string
	files []adjacentGeneratedFileCleanup
}

func cleanCommand(args []string) error {
	options, err := parseCleanOptions(args)
	if err != nil {
		return err
	}
	return cleanApp(options)
}

func parseCleanOptions(args []string) (cleanOptions, error) {
	var options cleanOptions
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--generated":
			options.generated = true
		case arg == "--legacy":
			options.legacy = true
		case strings.HasPrefix(arg, "--workspace="):
			options.workspace = strings.TrimPrefix(arg, "--workspace=")
		case arg == "--workspace":
			index++
			if index >= len(args) {
				return cleanOptions{}, errors.New("--workspace requires a value")
			}
			options.workspace = args[index]
		case strings.HasPrefix(arg, "-"):
			return cleanOptions{}, fmt.Errorf("unknown clean flag %q", arg)
		case options.appDir == "":
			options.appDir = arg
		default:
			return cleanOptions{}, fmt.Errorf("unexpected clean argument %q", arg)
		}
	}
	if options.appDir == "" {
		return cleanOptions{}, errors.New("usage: goxc clean <app-directory> [--generated] [--legacy] [--workspace=directory]")
	}
	return options, nil
}

func cleanApp(options cleanOptions) error {
	if err := ensureAppDirectory(options.appDir); err != nil {
		return err
	}
	layout, err := newBuildLayout(layoutOptions{appDir: options.appDir, workspace: options.workspace})
	if err != nil {
		return err
	}
	if err := validateWorkspaceRoot(layout); err != nil {
		return err
	}
	var adjacentGeneratedFiles adjacentGeneratedFileCleanupPlan
	if options.generated || options.legacy {
		adjacentGeneratedFiles, err = planAdjacentGeneratedFileCleanup(options.appDir)
		if err != nil {
			return err
		}
	}
	for _, directory := range []string{
		filepath.Join(layout.WorkspaceRoot, "work"),
		filepath.Join(layout.WorkspaceRoot, "build"),
		filepath.Join(layout.WorkspaceRoot, "package"),
	} {
		if err := removeDirectoryIfExistsBelowRoot(layout.WorkspaceRoot, directory); err != nil {
			return err
		}
	}
	if options.legacy {
		if err := cleanLegacyArtifacts(options.appDir); err != nil {
			return err
		}
	}
	if options.generated || options.legacy {
		if err := cleanAdjacentGeneratedFiles(adjacentGeneratedFiles); err != nil {
			return err
		}
	}
	if !options.generated {
		return nil
	}
	if err := removeDirectoryIfExistsBelowRoot(layout.WorkspaceRoot, layout.GenDir); err != nil {
		return err
	}
	return nil
}

func planAdjacentGeneratedFileCleanup(appDir string) (adjacentGeneratedFileCleanupPlan, error) {
	files, err := findGOXFiles(appDir)
	if err != nil {
		return adjacentGeneratedFileCleanupPlan{}, err
	}
	plan := adjacentGeneratedFileCleanupPlan{
		root:  appDir,
		files: make([]adjacentGeneratedFileCleanup, 0, len(files)),
	}
	for _, file := range files {
		generated := file + ".go"
		entry, exists, err := inspectAdjacentGeneratedFile(appDir, generated)
		if err != nil {
			return adjacentGeneratedFileCleanupPlan{}, err
		}
		if !exists {
			continue
		}
		plan.files = append(plan.files, entry)
	}
	return plan, nil
}

func inspectAdjacentGeneratedFile(root, path string) (adjacentGeneratedFileCleanup, bool, error) {
	if err := validatePathBelowRoot(root, path, "adjacent generated output", true); err != nil {
		return adjacentGeneratedFileCleanup{}, false, err
	}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return adjacentGeneratedFileCleanup{}, false, nil
	} else if err != nil {
		return adjacentGeneratedFileCleanup{}, false, fmt.Errorf("inspect adjacent generated output %s: %w", path, err)
	}

	info, err := regularFileNoFollow(path, "adjacent generated output")
	if err != nil {
		return adjacentGeneratedFileCleanup{}, false, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return adjacentGeneratedFileCleanup{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	verifiedInfo, err := regularFileNoFollow(path, "adjacent generated output")
	if errors.Is(err, os.ErrNotExist) {
		return adjacentGeneratedFileCleanup{}, false, nil
	}
	if err != nil {
		return adjacentGeneratedFileCleanup{}, false, err
	}
	if !os.SameFile(info, verifiedInfo) {
		return adjacentGeneratedFileCleanup{}, false, fmt.Errorf(
			"refuse to remove adjacent generated output %s: file identity changed while validating cleanup ownership",
			path,
		)
	}
	if !bytes.HasPrefix(content, []byte(generatedGOXFileHeader)) {
		return adjacentGeneratedFileCleanup{}, false, fmt.Errorf(
			"refuse to remove adjacent generated output %s: file is not managed by goxc",
			path,
		)
	}
	return adjacentGeneratedFileCleanup{
		path:   path,
		info:   verifiedInfo,
		digest: sha256.Sum256(content),
	}, true, nil
}

func revalidateAdjacentGeneratedFile(root string, planned adjacentGeneratedFileCleanup) (bool, error) {
	current, exists, err := inspectAdjacentGeneratedFile(root, planned.path)
	if err != nil || !exists {
		return exists, err
	}
	if !os.SameFile(planned.info, current.info) {
		return false, fmt.Errorf(
			"refuse to remove adjacent generated output %s: file identity changed after cleanup planning",
			planned.path,
		)
	}
	if planned.digest != current.digest {
		return false, fmt.Errorf(
			"refuse to remove adjacent generated output %s: file content changed after cleanup planning",
			planned.path,
		)
	}
	return true, nil
}

func cleanAdjacentGeneratedFiles(plan adjacentGeneratedFileCleanupPlan) error {
	for _, planned := range plan.files {
		if _, err := revalidateAdjacentGeneratedFile(plan.root, planned); err != nil {
			return err
		}
	}
	for _, planned := range plan.files {
		exists, err := revalidateAdjacentGeneratedFile(plan.root, planned)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		if err := os.Remove(planned.path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return fmt.Errorf("remove %s: %w", planned.path, err)
		}
		fmt.Printf("removed %s\n", planned.path)
	}
	return nil
}

func cleanLegacyArtifacts(appDir string) error {
	if err := removeDirectoryIfExistsBelowRoot(appDir, filepath.Join(appDir, "build")); err != nil {
		return err
	}
	dist := filepath.Join(appDir, "dist")
	if isGoframeOwnedExport(dist) {
		return removeDirectoryIfExistsBelowRoot(appDir, dist)
	}
	if entries, err := os.ReadDir(dist); err == nil && len(entries) > 0 {
		fmt.Printf("skipped %s; it does not look like a GoFrame package export\n", dist)
	}
	return nil
}

func removeDirectoryIfExistsBelowRoot(root, directory string) error {
	if err := validatePathBelowRoot(root, filepath.Dir(directory), "cleanup parent", false); err != nil {
		return err
	}
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s: %w", directory, err)
	}
	if info.Mode()&os.ModeSymlink == 0 && !info.IsDir() {
		return fmt.Errorf("cleanup path %s is not a directory", directory)
	}
	if err := os.RemoveAll(directory); err != nil {
		return fmt.Errorf("remove %s: %w", directory, err)
	}
	fmt.Printf("removed %s\n", directory)
	return nil
}
