package main

import (
	"bytes"
	"fmt"
	"go/build"
	"io"
	"path/filepath"
	"strings"
)

type authoredSourceSelection struct {
	buildContext *build.Context
	excludeTests bool
}

func browserAuthoredSourceSelection(compiler string) (authoredSourceSelection, error) {
	context := build.Default
	context.GOOS = "js"
	context.GOARCH = "wasm"
	context.Compiler = "gc"
	context.BuildTags = append([]string(nil), context.BuildTags...)
	context.ToolTags = append([]string(nil), context.ToolTags...)
	context.ReleaseTags = append([]string(nil), context.ReleaseTags...)

	switch compiler {
	case "go":
		context.CgoEnabled = false
	case "tinygo":
		context.CgoEnabled = true
		context.BuildTags = append(context.BuildTags,
			"tinygo.wasm",
			"tinygo",
			"purego",
			"osusergo",
			"math_big_pure_go",
			"gc.precise",
			"scheduler.asyncify",
			"serial.none",
			"tinygo.unicore",
		)
	default:
		return authoredSourceSelection{}, fmt.Errorf("unsupported compiler %q; use go or tinygo", compiler)
	}

	return authoredSourceSelection{
		buildContext: &context,
		excludeTests: true,
	}, nil
}

func (selection authoredSourceSelection) match(
	packageDir,
	filename string,
	content []byte,
) (bool, error) {
	if selection.excludeTests && strings.HasSuffix(filename, "_test.go") {
		return false, nil
	}
	if selection.buildContext == nil {
		return true, nil
	}

	context := *selection.buildContext
	expectedPath := filepath.Clean(filepath.Join(packageDir, filename))
	context.OpenFile = func(path string) (io.ReadCloser, error) {
		if filepath.Clean(path) != expectedPath {
			return nil, fmt.Errorf(
				"build constraint matcher requested unexpected source %s",
				path,
			)
		}
		return io.NopCloser(bytes.NewReader(content)), nil
	}
	matched, err := context.MatchFile(packageDir, filename)
	if err != nil {
		return false, fmt.Errorf(
			"match authored Go source %s against browser target: %w",
			expectedPath,
			err,
		)
	}
	return matched, nil
}
