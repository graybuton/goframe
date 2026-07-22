package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type generateOptions struct {
	path      string
	outDir    string
	workspace string
	inPlace   bool
}

func generateCommand(args []string) error {
	options, err := parseGenerateOptions(args)
	if err != nil {
		return err
	}
	return generatePath(options, true)
}

func parseGenerateOptions(args []string) (generateOptions, error) {
	var options generateOptions
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--in-place":
			options.inPlace = true
		case strings.HasPrefix(arg, "--out="):
			options.outDir = strings.TrimPrefix(arg, "--out=")
		case arg == "--out":
			index++
			if index >= len(args) {
				return generateOptions{}, errors.New("--out requires a value")
			}
			options.outDir = args[index]
		case strings.HasPrefix(arg, "--workspace="):
			options.workspace = strings.TrimPrefix(arg, "--workspace=")
		case arg == "--workspace":
			index++
			if index >= len(args) {
				return generateOptions{}, errors.New("--workspace requires a value")
			}
			options.workspace = args[index]
		case strings.HasPrefix(arg, "-"):
			return generateOptions{}, fmt.Errorf("unknown generate flag %q", arg)
		case options.path == "":
			options.path = arg
		default:
			return generateOptions{}, fmt.Errorf("unexpected generate argument %q", arg)
		}
	}
	if options.path == "" {
		return generateOptions{}, errors.New("usage: goxc generate <file-or-directory> [--out=directory] [--workspace=directory] [--in-place]")
	}
	if options.inPlace && options.outDir != "" {
		return generateOptions{}, errors.New("--in-place cannot be combined with --out")
	}
	return options, nil
}

func generatePath(options generateOptions, requireFiles bool) error {
	files, err := findGOXFiles(options.path)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		if requireFiles {
			return fmt.Errorf("no .gox files found below %s", options.path)
		}
		fmt.Printf("no .gox files found below %s; building Go sources\n", options.path)
		return nil
	}

	if options.inPlace {
		appDir, err := generationAppDir(options.path)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "warning: --in-place writes generated compiler output into the source tree; use only for debugging or legacy workflows")
		for _, file := range files {
			output := file + ".go"
			if err := validatePathBelowRoot(appDir, output, "in-place generated file", true); err != nil {
				return err
			}
		}
		if err := generateFilesIntoDirectory(appDir, appDir, files); err != nil {
			return err
		}
		for _, file := range files {
			output := file + ".go"
			fmt.Printf("generated %s -> %s\n", file, output)
		}
		return nil
	}

	appDir, err := generationAppDir(options.path)
	if err != nil {
		return err
	}
	outputRoot := options.outDir
	if outputRoot == "" {
		layout, err := newBuildLayout(layoutOptions{appDir: appDir, workspace: options.workspace})
		if err != nil {
			return err
		}
		if err := validateWorkspaceRoot(layout); err != nil {
			return err
		}
		outputRoot = layout.GenDir
		if err := validatePathBelowRoot(layout.WorkspaceRoot, outputRoot, "generated output directory", true); err != nil {
			return err
		}
	} else {
		if err := ensureNoPhysicalOverlap(outputRoot, appDir, "generated output directory", "application directory"); err != nil {
			return err
		}
		if err := validateExplicitPathRoot(outputRoot, "generated output directory", true); err != nil {
			return err
		}
	}

	for _, file := range files {
		relative, err := filepath.Rel(appDir, file)
		if err != nil {
			return fmt.Errorf("resolve GOX source %s: %w", file, err)
		}
		output := filepath.Join(outputRoot, relative+".go")
		if err := validatePathBelowRoot(outputRoot, output, "generated file", true); err != nil {
			return err
		}
	}
	if err := generateFilesIntoDirectory(appDir, outputRoot, files); err != nil {
		return err
	}
	for _, file := range files {
		relative, err := filepath.Rel(appDir, file)
		if err != nil {
			return fmt.Errorf("resolve GOX source %s: %w", file, err)
		}
		output := filepath.Join(outputRoot, relative+".go")
		fmt.Printf("generated %s -> %s\n", file, output)
	}
	return nil
}

func generationAppDir(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%s is a symlink; symlink paths are not supported", path)
	}
	if info.IsDir() {
		return path, nil
	}
	if filepath.Ext(path) != ".gox" {
		return "", fmt.Errorf("%s is not a .gox file", path)
	}
	return filepath.Dir(path), nil
}
