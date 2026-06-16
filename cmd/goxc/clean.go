package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jin-wu/goframe/pkg/gox"
)

type cleanOptions struct {
	appDir    string
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
	for _, arg := range args {
		switch {
		case arg == "--generated":
			options.generated = true
		case strings.HasPrefix(arg, "-"):
			return cleanOptions{}, fmt.Errorf("unknown clean flag %q", arg)
		case options.appDir == "":
			options.appDir = arg
		default:
			return cleanOptions{}, fmt.Errorf("unexpected clean argument %q", arg)
		}
	}
	if options.appDir == "" {
		return cleanOptions{}, errors.New("usage: goxc clean <app-directory> [--generated]")
	}
	return options, nil
}

func cleanApp(options cleanOptions) error {
	if err := ensureAppDirectory(options.appDir); err != nil {
		return err
	}
	manifest, err := loadManifest(options.appDir)
	if err != nil {
		return err
	}
	for _, directory := range []string{
		filepath.Join(options.appDir, "build"),
		filepath.Join(options.appDir, manifest.Output),
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
	return nil
}
