package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type buildOptions struct {
	appDir        string
	compiler      string
	outDir        string
	workspace     string
	legacyRelease bool
}

func buildCommand(args []string) error {
	options, err := parseBuildOptions(args)
	if err != nil {
		return err
	}
	if options.legacyRelease {
		fmt.Fprintln(os.Stderr, "warning: build --release no longer packages or compresses; use `goxc package`")
	}
	_, err = buildApp(options)
	return err
}

func parseBuildOptions(args []string) (buildOptions, error) {
	var options buildOptions
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--release":
			options.legacyRelease = true
		case strings.HasPrefix(arg, "--compiler="):
			options.compiler = strings.TrimPrefix(arg, "--compiler=")
		case arg == "--compiler":
			index++
			if index >= len(args) {
				return buildOptions{}, errors.New("--compiler requires a value")
			}
			options.compiler = args[index]
		case strings.HasPrefix(arg, "--out="):
			options.outDir = strings.TrimPrefix(arg, "--out=")
		case arg == "--out":
			index++
			if index >= len(args) {
				return buildOptions{}, errors.New("--out requires a value")
			}
			options.outDir = args[index]
		case strings.HasPrefix(arg, "--workspace="):
			options.workspace = strings.TrimPrefix(arg, "--workspace=")
		case arg == "--workspace":
			index++
			if index >= len(args) {
				return buildOptions{}, errors.New("--workspace requires a value")
			}
			options.workspace = args[index]
		case strings.HasPrefix(arg, "-"):
			return buildOptions{}, fmt.Errorf("unknown build flag %q", arg)
		case options.appDir == "":
			options.appDir = arg
		default:
			return buildOptions{}, fmt.Errorf("unexpected build argument %q", arg)
		}
	}
	if options.appDir == "" {
		return buildOptions{}, errors.New("usage: goxc build <app-directory> [--compiler=go|tinygo] [--out=directory] [--workspace=directory]")
	}
	return options, nil
}

func buildApp(options buildOptions) (string, error) {
	manifest, err := loadManifest(options.appDir)
	if err != nil {
		return "", err
	}
	if options.compiler == "" {
		options.compiler = manifest.Compiler
	}
	if err := validateCompiler(options.compiler); err != nil {
		return "", err
	}
	if err := ensureAppDirectory(options.appDir); err != nil {
		return "", err
	}
	layout, err := newBuildLayout(layoutOptions{
		appDir:    options.appDir,
		compiler:  options.compiler,
		profile:   defaultProfileName,
		workspace: options.workspace,
	})
	if err != nil {
		return "", err
	}
	if err := validateWorkspaceRoot(layout); err != nil {
		return "", err
	}
	workspaceResult, err := prepareBuildWorkspaceResult(layout, manifest)
	if err != nil {
		return "", fmt.Errorf("prepare build workspace: %w", err)
	}
	entryPath := workspaceResult.EntryPath
	outputPath := buildOutputPath(options, manifest, layout)
	outputRoot := layout.BuildDir
	if options.outDir != "" {
		outputRoot = options.outDir
		if err := ensureNoPhysicalOverlap(outputRoot, layout.AppDir, "build output directory", "application directory"); err != nil {
			return "", err
		}
		if err := validateExplicitPathRoot(outputRoot, "build output directory", true); err != nil {
			return "", err
		}
	} else {
		if err := validatePathBelowRoot(layout.WorkspaceRoot, outputRoot, "build output directory", true); err != nil {
			return "", err
		}
	}
	if strings.ToLower(filepath.Ext(outputPath)) != ".wasm" {
		return "", fmt.Errorf("build output %s must end with .wasm", outputPath)
	}
	if err := validatePathBelowRoot(outputRoot, outputPath, "build output", true); err != nil {
		return "", err
	}
	if err := mkdirAllBelowRoot(outputRoot, filepath.Dir(outputPath), "build output directory"); err != nil {
		return "", err
	}

	fmt.Printf("building %s with %s compiler\n", options.appDir, options.compiler)
	if err := compileWASM(options.compiler, entryPath, outputPath); err != nil {
		return "", err
	}
	fmt.Printf("built %s\n", outputPath)
	return outputPath, nil
}

func buildOutputPath(options buildOptions, manifest projectManifest, layout BuildLayout) string {
	directory := options.outDir
	if directory == "" {
		directory = layout.BuildDir
	}
	return filepath.Join(directory, manifest.WASM)
}

func compileWASM(compiler, entryPath, outputPath string) error {
	compilerPath, err := exec.LookPath(compiler)
	if err != nil {
		if compiler == "tinygo" {
			return errors.New("TinyGo compiler not found in PATH; install TinyGo or use --compiler=go")
		}
		return errors.New("Go compiler not found in PATH")
	}
	entryPath, err = filepath.Abs(entryPath)
	if err != nil {
		return fmt.Errorf("resolve application entry: %w", err)
	}
	entryArg := entryPath
	commandDir := ""
	if info, err := os.Stat(entryPath); err == nil && info.IsDir() {
		commandDir = entryPath
		entryArg = "."
	} else {
		commandDir = filepath.Dir(entryPath)
		entryArg = "./" + filepath.Base(entryPath)
	}
	outputDir := filepath.Dir(outputPath)
	temp, err := os.CreateTemp(outputDir, ".goframe-build-*.wasm")
	if err != nil {
		return fmt.Errorf("create temporary compiler output: %w", err)
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("close temporary compiler output: %w", err)
	}
	defer os.Remove(tempPath)

	var command *exec.Cmd
	if compiler == "go" {
		command = exec.Command(compilerPath,
			"build",
			"-buildvcs=false",
			"-trimpath",
			"-ldflags=-s -w -buildid=",
			"-o", tempPath,
			entryArg,
		)
	} else {
		command = exec.Command(compilerPath,
			"build",
			"-target=wasm",
			"-no-debug",
			"-panic=trap",
			"-o", tempPath,
			entryArg,
		)
	}
	command.Dir = commandDir
	command.Env = compilerEnvironment(compiler)
	if compiler == "go" {
		command.Env = append(command.Env, "GOOS=js", "GOARCH=wasm")
	}
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s WASM build failed: %w", compiler, err)
	}
	if err := copyFile(tempPath, outputPath); err != nil {
		return err
	}
	return nil
}

func compilerEnvironment(compiler string) []string {
	environment := os.Environ()
	if compiler != "tinygo" || os.Getenv("XDG_CACHE_HOME") != "" {
		return environment
	}
	if goCache := os.Getenv("GOCACHE"); goCache != "" {
		cache := filepath.Join(filepath.Dir(goCache), "goxc-xdg-cache")
		if err := os.MkdirAll(cache, 0o755); err == nil {
			environment = append(environment, "XDG_CACHE_HOME="+cache)
		}
	}
	return environment
}

func validateCompiler(compiler string) error {
	if compiler != "go" && compiler != "tinygo" {
		return fmt.Errorf("unsupported compiler %q; use go or tinygo", compiler)
	}
	return nil
}

func ensureAppDirectory(path string) error {
	if err := directoryNoFollow(path, "application directory"); err != nil {
		return fmt.Errorf("open application directory: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("open application directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not an application directory", path)
	}
	return nil
}
