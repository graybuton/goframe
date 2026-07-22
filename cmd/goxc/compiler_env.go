package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type compilerInvocationKind string

const (
	compilerInvocationBuild          compilerInvocationKind = "build"
	compilerInvocationEmbedDiscovery compilerInvocationKind = "embed discovery"
	workspaceCompilerBaseGoFlags                            = "-buildvcs=false"
)

type compilerEnvironmentOptions struct {
	Compiler         string
	Invocation       compilerInvocationKind
	WorkingDirectory string
	GoFlags          string
	StandardGoTarget bool
}

type compilerEnvironmentValue struct {
	key   string
	value string
}

func configureWorkspaceCompilerCommand(command *exec.Cmd, options compilerEnvironmentOptions) error {
	environment, directory, err := workspaceCompilerEnvironment(options)
	if err != nil {
		return err
	}
	command.Dir = directory
	command.Env = environment
	return nil
}

func workspaceCompilerEnvironment(options compilerEnvironmentOptions) ([]string, string, error) {
	return workspaceCompilerEnvironmentFrom(os.Environ(), options)
}

func workspaceCompilerEnvironmentFrom(base []string, options compilerEnvironmentOptions) ([]string, string, error) {
	if options.Compiler != "go" && options.Compiler != "tinygo" {
		return nil, "", compilerEnvironmentError(options, "compiler", fmt.Errorf("unsupported compiler %q", options.Compiler))
	}
	if options.Invocation == "" {
		return nil, "", compilerEnvironmentError(options, "invocation", fmt.Errorf("invocation kind is required"))
	}
	directory, err := filepath.Abs(options.WorkingDirectory)
	if err != nil {
		return nil, "", compilerEnvironmentError(options, "PWD", err)
	}
	directory = filepath.Clean(directory)
	if strings.TrimSpace(options.GoFlags) == "" {
		return nil, "", compilerEnvironmentError(options, "GOFLAGS", fmt.Errorf("an explicit non-empty owned value is required to override GOENV defaults"))
	}

	environment := append([]string(nil), base...)
	if options.Compiler == "tinygo" {
		environment, err = withTinyGoCacheFallback(environment)
		if err != nil {
			return nil, "", compilerEnvironmentError(options, "XDG_CACHE_HOME", err)
		}
	}

	controlled := []string{"PWD", "GOWORK", "GO111MODULE", "GOFLAGS", "GOOS", "GOARCH", "CGO_ENABLED"}
	environment = removeEnvironmentKeys(environment, controlled)
	values := []compilerEnvironmentValue{
		{key: "PWD", value: directory},
		{key: "GOWORK", value: "off"},
		{key: "GO111MODULE", value: "on"},
		{key: "GOFLAGS", value: options.GoFlags},
	}
	if options.StandardGoTarget {
		values = append(values,
			compilerEnvironmentValue{key: "GOOS", value: "js"},
			compilerEnvironmentValue{key: "GOARCH", value: "wasm"},
			compilerEnvironmentValue{key: "CGO_ENABLED", value: "0"},
		)
	}
	for _, value := range values {
		environment = append(environment, value.key+"="+value.value)
	}
	return environment, directory, nil
}

func compilerEnvironmentError(options compilerEnvironmentOptions, key string, err error) error {
	directory := options.WorkingDirectory
	if directory == "" {
		directory = "."
	}
	invocation := options.Invocation
	if invocation == "" {
		invocation = "unknown"
	}
	compiler := options.Compiler
	if compiler == "" {
		compiler = "unknown"
	}
	return fmt.Errorf("prepare %s %s compiler environment for %s: %s: %w", compiler, invocation, directory, key, err)
}

func withTinyGoCacheFallback(environment []string) ([]string, error) {
	if value, ok := environmentValue(environment, "XDG_CACHE_HOME"); ok && value != "" {
		return environment, nil
	}
	goCache, ok := environmentValue(environment, "GOCACHE")
	if !ok || goCache == "" {
		return environment, nil
	}
	cache := filepath.Join(filepath.Dir(goCache), "goxc-xdg-cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return nil, fmt.Errorf("create TinyGo cache fallback %s: %w", cache, err)
	}
	return setEnvironmentValue(environment, "XDG_CACHE_HOME", cache), nil
}

func removeEnvironmentKeys(environment []string, keys []string) []string {
	result := make([]string, 0, len(environment))
	for _, item := range environment {
		key, _, ok := strings.Cut(item, "=")
		if !ok || !environmentKeyIn(key, keys) {
			result = append(result, item)
		}
	}
	return result
}

func setEnvironmentValue(environment []string, key, value string) []string {
	result := removeEnvironmentKeys(environment, []string{key})
	return append(result, key+"="+value)
}

func environmentValue(environment []string, key string) (string, bool) {
	for index := len(environment) - 1; index >= 0; index-- {
		itemKey, value, ok := strings.Cut(environment[index], "=")
		if ok && environmentKeysEqual(itemKey, key) {
			return value, true
		}
	}
	return "", false
}

func environmentKeyIn(key string, keys []string) bool {
	for _, candidate := range keys {
		if environmentKeysEqual(key, candidate) {
			return true
		}
	}
	return false
}

func environmentKeysEqual(first, second string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(first, second)
	}
	return first == second
}
