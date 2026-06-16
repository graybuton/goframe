package main

import (
	"errors"
	"fmt"

	"github.com/jin-wu/goframe/pkg/gox"
)

func generateCommand(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: goxc generate <file-or-directory>")
	}
	return generatePath(args[0], true)
}

func generateForBuild(path string) error {
	return generatePath(path, false)
}

func generatePath(path string, requireFiles bool) error {
	files, err := gox.FindFiles(path)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		if requireFiles {
			return fmt.Errorf("no .gox files found below %s", path)
		}
		fmt.Printf("no .gox files found below %s; building Go sources\n", path)
		return nil
	}

	for _, file := range files {
		output, err := gox.GenerateFile(file)
		if err != nil {
			return err
		}
		fmt.Printf("generated %s -> %s\n", file, output)
	}
	return nil
}
