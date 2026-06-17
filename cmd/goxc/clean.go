package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/graybuton/goframe/pkg/gox"
)

type cleanOptions struct {
	appDir    string
	workspace string
	generated bool
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
		return cleanOptions{}, errors.New("usage: goxc clean <app-directory> [--generated] [--workspace=directory]")
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
	for _, directory := range []string{
		filepath.Join(layout.WorkspaceRoot, "work"),
		filepath.Join(layout.WorkspaceRoot, "build"),
		filepath.Join(layout.WorkspaceRoot, "package"),
	} {
		if err := os.RemoveAll(directory); err != nil {
			return fmt.Errorf("remove %s: %w", directory, err)
		}
		fmt.Printf("removed %s\n", directory)
	}
	if !options.generated {
		return nil
	}

	files, err := gox.FindFiles(options.appDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		generated := file + ".go"
		if err := os.Remove(generated); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return fmt.Errorf("remove %s: %w", generated, err)
		}
		fmt.Printf("removed %s\n", generated)
	}
	if err := os.RemoveAll(layout.GenDir); err != nil {
		return fmt.Errorf("remove %s: %w", layout.GenDir, err)
	}
	fmt.Printf("removed %s\n", layout.GenDir)
	return nil
}
