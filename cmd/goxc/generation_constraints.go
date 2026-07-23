package main

import (
	"bytes"
	"fmt"
	"go/build"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type generationSourceSelection struct {
	buildContext   *build.Context
	excludeGoTests bool
}

func defaultGenerationSourceSelection() generationSourceSelection {
	context := cloneBuildContext(build.Default)
	return generationSourceSelection{
		buildContext:   &context,
		excludeGoTests: true,
	}
}

func browserGenerationSourceSelection(compiler string) (generationSourceSelection, error) {
	context, err := browserBuildContext(compiler, build.Default, os.Environ())
	if err != nil {
		return generationSourceSelection{}, err
	}

	return generationSourceSelection{
		buildContext:   &context,
		excludeGoTests: true,
	}, nil
}

func cloneBuildContext(context build.Context) build.Context {
	context.BuildTags = append([]string(nil), context.BuildTags...)
	context.ToolTags = append([]string(nil), context.ToolTags...)
	context.ReleaseTags = append([]string(nil), context.ReleaseTags...)
	return context
}

func browserBuildContext(
	compiler string,
	base build.Context,
	environment []string,
) (build.Context, error) {
	context := base
	context.GOOS = "js"
	context.GOARCH = "wasm"
	context.Compiler = "gc"
	context.BuildTags = nil
	context.ReleaseTags = append([]string(nil), context.ReleaseTags...)
	toolTags, err := browserTargetToolTags(base.ToolTags, environment)
	if err != nil {
		return build.Context{}, err
	}
	context.ToolTags = toolTags

	switch compiler {
	case "go":
		context.CgoEnabled = false
	case "tinygo":
		context.CgoEnabled = true
		context.BuildTags = sortedUniqueStrings([]string{
			"tinygo.wasm",
			"tinygo",
			"purego",
			"osusergo",
			"math_big_pure_go",
			"gc.precise",
			"scheduler.asyncify",
			"serial.none",
			"tinygo.unicore",
		})
	default:
		return build.Context{}, fmt.Errorf("unsupported compiler %q; use go or tinygo", compiler)
	}

	return context, nil
}

func browserTargetToolTags(base []string, environment []string) ([]string, error) {
	tags := make([]string, 0, len(base)+2)
	for _, tag := range base {
		if !architectureFeatureToolTag(tag) {
			tags = append(tags, tag)
		}
	}

	goWASM, _ := environmentValue(environment, "GOWASM")
	for _, feature := range strings.Split(goWASM, ",") {
		switch feature {
		case "":
		case "satconv", "signext":
			tags = append(tags, "wasm."+feature)
		default:
			return nil, fmt.Errorf("invalid GOWASM feature %q for browser source selection", feature)
		}
	}
	return sortedUniqueStrings(tags), nil
}

func architectureFeatureToolTag(tag string) bool {
	architecture, _, ok := strings.Cut(tag, ".")
	if !ok {
		return false
	}
	switch architecture {
	case "386",
		"amd64",
		"arm",
		"arm64",
		"mips",
		"mipsle",
		"mips64",
		"mips64le",
		"ppc64",
		"ppc64le",
		"riscv64",
		"wasm":
		return true
	default:
		return false
	}
}

func sortedUniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (selection generationSourceSelection) matchAuthoredGo(
	packageDir,
	filename string,
	content []byte,
) (bool, error) {
	if selection.excludeGoTests && strings.HasSuffix(filename, "_test.go") {
		return false, nil
	}
	matched, err := selection.matchBuildConstraints(packageDir, filename, content)
	if err != nil {
		return false, fmt.Errorf(
			"match authored Go source %s against %s target: %w",
			filepath.Join(packageDir, filename),
			selection.targetName(),
			err,
		)
	}
	return matched, nil
}

func (selection generationSourceSelection) matchGOX(
	packageDir,
	filename string,
	content []byte,
) (bool, error) {
	syntheticName := generatedGOXSourceFilename(filename)
	matched, err := selection.matchBuildConstraints(
		packageDir,
		syntheticName,
		content,
	)
	if err != nil {
		return false, fmt.Errorf(
			"match GOX source %s as %s against %s target: %w",
			filepath.Join(packageDir, filename),
			syntheticName,
			selection.targetName(),
			err,
		)
	}
	return matched, nil
}

func generatedGOXSourceFilename(filename string) string {
	return filepath.Base(filename) + ".go"
}

func (selection generationSourceSelection) matchBuildConstraints(
	packageDir,
	filename string,
	content []byte,
) (bool, error) {
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
		return false, err
	}
	return matched, nil
}

func (selection generationSourceSelection) targetName() string {
	if selection.buildContext == nil {
		return "unfiltered"
	}
	return selection.buildContext.GOOS + "/" + selection.buildContext.GOARCH
}
