package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestCompilerEnvironmentContract(t *testing.T) {
	root := t.TempDir()
	workingDirectory := filepath.Join(root, "lexical", "..", "lexical", "app")
	base := []string{
		"PATH=tool-path",
		"HOME=home-value",
		"GOROOT=go-root-value",
		"GOPATH=go-path-value",
		"GOCACHE=go-cache-value",
		"GOPROXY=proxy-value",
		"GOMODCACHE=module-cache-value",
		"GONOPROXY=no-proxy-value",
		"GOPRIVATE=private-value",
		"GONOSUMDB=no-sum-value",
		"GOSUMDB=sum-value",
		"GOINSECURE=insecure-value",
		"GOVCS=vcs-value",
		"GOTOOLCHAIN=toolchain-value",
		"GOENV=user-goenv-value",
		"GODEBUG=debug-value",
		"GOEXPERIMENT=experiment-value",
		"GOWASM=wasm-value",
		"XDG_CACHE_HOME=tinygo-cache-value",
		"TMPDIR=tmpdir-value",
		"TMP=tmp-value",
		"TEMP=temp-value",
		"SSL_CERT_FILE=cert-file-value",
		"SSL_CERT_DIR=cert-dir-value",
		"GIT_CONFIG_GLOBAL=git-config-value",
		"SSH_AUTH_SOCK=ssh-value",
		"SENTINEL=preserved-value",
		"PWD=stale-one",
		"PWD=stale-two",
		"GOWORK=parent.work",
		"GOWORK=other.work",
		"GO111MODULE=off",
		"GOFLAGS=-mod=vendor",
		"GOFLAGS=-tags=goxc_hostile",
		"GOOS=linux",
		"GOARCH=amd64",
		"CGO_ENABLED=1",
	}
	wantBase := append([]string(nil), base...)
	t.Setenv("GOWORK", "parent-process-work")
	t.Setenv("GOFLAGS", "parent-process-flags")
	t.Setenv("PWD", "parent-process-pwd")

	tests := []struct {
		name       string
		options    compilerEnvironmentOptions
		wantTarget bool
	}{
		{
			name: "Go build",
			options: compilerEnvironmentOptions{
				Compiler: "go", Invocation: compilerInvocationBuild,
				WorkingDirectory: workingDirectory, GoFlags: workspaceCompilerBaseGoFlags,
				StandardGoTarget: true,
			},
			wantTarget: true,
		},
		{
			name: "Go embed discovery",
			options: compilerEnvironmentOptions{
				Compiler: "go", Invocation: compilerInvocationEmbedDiscovery,
				WorkingDirectory: workingDirectory, GoFlags: workspaceCompilerBaseGoFlags,
				StandardGoTarget: true,
			},
			wantTarget: true,
		},
		{
			name: "TinyGo build",
			options: compilerEnvironmentOptions{
				Compiler: "tinygo", Invocation: compilerInvocationBuild,
				WorkingDirectory: workingDirectory, GoFlags: workspaceCompilerBaseGoFlags,
			},
		},
		{
			name: "TinyGo embed discovery",
			options: compilerEnvironmentOptions{
				Compiler: "tinygo", Invocation: compilerInvocationEmbedDiscovery,
				WorkingDirectory: workingDirectory,
				GoFlags:          workspaceCompilerBaseGoFlags + " " + strconv.Quote("-overlay="+filepath.Join(root, "overlay with spaces.json")),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			environment, directory, err := workspaceCompilerEnvironmentFrom(base, test.options)
			if err != nil {
				t.Fatalf("workspaceCompilerEnvironmentFrom() error: %v", err)
			}
			wantDirectory, err := filepath.Abs(workingDirectory)
			if err != nil {
				t.Fatal(err)
			}
			wantDirectory = filepath.Clean(wantDirectory)
			if directory != wantDirectory {
				t.Fatalf("directory = %q, want lexical %q", directory, wantDirectory)
			}
			for key, want := range map[string]string{
				"PWD": directory, "GOWORK": "off", "GO111MODULE": "on", "GOFLAGS": test.options.GoFlags,
			} {
				assertCompilerEnvironmentValue(t, environment, key, want)
			}
			if test.wantTarget {
				assertCompilerEnvironmentValue(t, environment, "GOOS", "js")
				assertCompilerEnvironmentValue(t, environment, "GOARCH", "wasm")
				assertCompilerEnvironmentValue(t, environment, "CGO_ENABLED", "0")
			} else {
				for _, key := range []string{"GOOS", "GOARCH", "CGO_ENABLED"} {
					if countCompilerEnvironmentKey(environment, key) != 0 {
						t.Fatalf("TinyGo environment retained %s: %#v", key, environment)
					}
				}
			}
			for _, want := range []string{
				"PATH=tool-path",
				"HOME=home-value",
				"GOROOT=go-root-value",
				"GOPATH=go-path-value",
				"GOCACHE=go-cache-value",
				"GOPROXY=proxy-value",
				"GOMODCACHE=module-cache-value",
				"GONOPROXY=no-proxy-value",
				"GOPRIVATE=private-value",
				"GONOSUMDB=no-sum-value",
				"GOSUMDB=sum-value",
				"GOINSECURE=insecure-value",
				"GOVCS=vcs-value",
				"GOTOOLCHAIN=toolchain-value",
				"GOENV=user-goenv-value",
				"GODEBUG=debug-value",
				"GOEXPERIMENT=experiment-value",
				"GOWASM=wasm-value",
				"XDG_CACHE_HOME=tinygo-cache-value",
				"TMPDIR=tmpdir-value",
				"TMP=tmp-value",
				"TEMP=temp-value",
				"SSL_CERT_FILE=cert-file-value",
				"SSL_CERT_DIR=cert-dir-value",
				"GIT_CONFIG_GLOBAL=git-config-value",
				"SSH_AUTH_SOCK=ssh-value",
				"SENTINEL=preserved-value",
			} {
				if !stringSliceContains(environment, want) {
					t.Fatalf("environment lost %q: %#v", want, environment)
				}
			}
			if !reflect.DeepEqual(base, wantBase) {
				t.Fatalf("input environment mutated:\ngot  %#v\nwant %#v", base, wantBase)
			}
		})
	}

	if got := os.Getenv("GOWORK"); got != "parent-process-work" {
		t.Fatalf("parent GOWORK = %q", got)
	}
	if got := os.Getenv("GOFLAGS"); got != "parent-process-flags" {
		t.Fatalf("parent GOFLAGS = %q", got)
	}
	if got := os.Getenv("PWD"); got != "parent-process-pwd" {
		t.Fatalf("parent PWD = %q", got)
	}
}

func TestTinyGoCacheFallbackFailureIsBestEffort(t *testing.T) {
	root := t.TempDir()
	blockingPath := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blockingPath, []byte("blocking file"), 0o600); err != nil {
		t.Fatal(err)
	}

	goCache := filepath.Join(blockingPath, "go-build")
	workingDirectory := filepath.Join(root, "working", "..", "app")
	base := []string{
		"GOCACHE=" + goCache,
		"SENTINEL=preserved",
		"PWD=stale",
		"GOWORK=parent.work",
		"GO111MODULE=off",
		"GOFLAGS=-mod=vendor",
		"GOOS=linux",
		"GOARCH=amd64",
		"CGO_ENABLED=1",
	}
	wantBase := append([]string(nil), base...)
	t.Setenv("GOCACHE", "parent-cache")
	t.Setenv("XDG_CACHE_HOME", "parent-xdg-cache")

	tests := []struct {
		name       string
		invocation compilerInvocationKind
		goFlags    string
	}{
		{
			name:       "build",
			invocation: compilerInvocationBuild,
			goFlags:    workspaceCompilerBaseGoFlags,
		},
		{
			name:       "embed discovery",
			invocation: compilerInvocationEmbedDiscovery,
			goFlags: workspaceCompilerBaseGoFlags + " " + strconv.Quote(
				"-overlay="+filepath.Join(root, "overlay with spaces.json"),
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			environment, directory, err := workspaceCompilerEnvironmentFrom(base, compilerEnvironmentOptions{
				Compiler:         "tinygo",
				Invocation:       test.invocation,
				WorkingDirectory: workingDirectory,
				GoFlags:          test.goFlags,
			})
			if err != nil {
				t.Fatalf("workspaceCompilerEnvironmentFrom() error: %v", err)
			}

			wantDirectory, err := filepath.Abs(workingDirectory)
			if err != nil {
				t.Fatal(err)
			}
			wantDirectory = filepath.Clean(wantDirectory)
			if directory != wantDirectory {
				t.Fatalf("directory = %q, want %q", directory, wantDirectory)
			}
			for key, want := range map[string]string{
				"GOCACHE":     goCache,
				"SENTINEL":    "preserved",
				"PWD":         wantDirectory,
				"GOWORK":      "off",
				"GO111MODULE": "on",
				"GOFLAGS":     test.goFlags,
			} {
				assertCompilerEnvironmentValue(t, environment, key, want)
			}
			if count := countCompilerEnvironmentKey(environment, "XDG_CACHE_HOME"); count != 0 {
				t.Fatalf("environment contains %d XDG_CACHE_HOME values, want 0: %#v", count, environment)
			}
			for _, key := range []string{"GOOS", "GOARCH", "CGO_ENABLED"} {
				if count := countCompilerEnvironmentKey(environment, key); count != 0 {
					t.Fatalf("TinyGo environment retained %s: %#v", key, environment)
				}
			}
			if !reflect.DeepEqual(base, wantBase) {
				t.Fatalf("input environment mutated:\ngot  %#v\nwant %#v", base, wantBase)
			}
			if got := os.Getenv("GOCACHE"); got != "parent-cache" {
				t.Fatalf("parent GOCACHE = %q", got)
			}
			if got := os.Getenv("XDG_CACHE_HOME"); got != "parent-xdg-cache" {
				t.Fatalf("parent XDG_CACHE_HOME = %q", got)
			}
		})
	}
}

func TestTinyGoCacheFallbackSuccess(t *testing.T) {
	root := t.TempDir()
	goCache := filepath.Join(root, "go-build")
	environment, _, err := workspaceCompilerEnvironmentFrom([]string{
		"GOCACHE=" + goCache,
		"SENTINEL=preserved",
		"PWD=stale",
		"GOWORK=parent.work",
		"GOFLAGS=-mod=vendor",
		"GOOS=linux",
		"GOARCH=amd64",
		"CGO_ENABLED=1",
	}, compilerEnvironmentOptions{
		Compiler:         "tinygo",
		Invocation:       compilerInvocationBuild,
		WorkingDirectory: filepath.Join(root, "app"),
		GoFlags:          workspaceCompilerBaseGoFlags,
	})
	if err != nil {
		t.Fatalf("workspaceCompilerEnvironmentFrom() error: %v", err)
	}

	fallback := filepath.Join(root, "goxc-xdg-cache")
	info, err := os.Stat(fallback)
	if err != nil {
		t.Fatalf("stat TinyGo cache fallback: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("TinyGo cache fallback mode = %s, want directory", info.Mode())
	}
	assertCompilerEnvironmentValue(t, environment, "XDG_CACHE_HOME", fallback)
	assertCompilerEnvironmentValue(t, environment, "GOCACHE", goCache)
	assertCompilerEnvironmentValue(t, environment, "GOWORK", "off")
	assertCompilerEnvironmentValue(t, environment, "GOFLAGS", workspaceCompilerBaseGoFlags)
}

func TestTinyGoCacheFallbackPreservesExistingValue(t *testing.T) {
	root := t.TempDir()
	blockingPath := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blockingPath, []byte("blocking file"), 0o600); err != nil {
		t.Fatal(err)
	}
	customCache := filepath.Join(root, "custom-cache")
	environment, _, err := workspaceCompilerEnvironmentFrom([]string{
		"XDG_CACHE_HOME=" + customCache,
		"GOCACHE=" + filepath.Join(blockingPath, "go-build"),
	}, compilerEnvironmentOptions{
		Compiler:         "tinygo",
		Invocation:       compilerInvocationBuild,
		WorkingDirectory: filepath.Join(root, "app"),
		GoFlags:          workspaceCompilerBaseGoFlags,
	})
	if err != nil {
		t.Fatalf("workspaceCompilerEnvironmentFrom() error: %v", err)
	}
	assertCompilerEnvironmentValue(t, environment, "XDG_CACHE_HOME", customCache)
}

func TestTinyGoCacheFallbackEmptyValue(t *testing.T) {
	t.Run("successful fallback replaces empty value", func(t *testing.T) {
		root := t.TempDir()
		goCache := filepath.Join(root, "go-build")
		environment, _, err := workspaceCompilerEnvironmentFrom([]string{
			"XDG_CACHE_HOME=",
			"GOCACHE=" + goCache,
		}, compilerEnvironmentOptions{
			Compiler:         "tinygo",
			Invocation:       compilerInvocationBuild,
			WorkingDirectory: filepath.Join(root, "app"),
			GoFlags:          workspaceCompilerBaseGoFlags,
		})
		if err != nil {
			t.Fatalf("workspaceCompilerEnvironmentFrom() error: %v", err)
		}
		assertCompilerEnvironmentValue(t, environment, "XDG_CACHE_HOME", filepath.Join(root, "goxc-xdg-cache"))
	})

	t.Run("failed fallback preserves empty value", func(t *testing.T) {
		root := t.TempDir()
		blockingPath := filepath.Join(root, "not-a-directory")
		if err := os.WriteFile(blockingPath, []byte("blocking file"), 0o600); err != nil {
			t.Fatal(err)
		}
		environment, _, err := workspaceCompilerEnvironmentFrom([]string{
			"XDG_CACHE_HOME=",
			"GOCACHE=" + filepath.Join(blockingPath, "go-build"),
		}, compilerEnvironmentOptions{
			Compiler:         "tinygo",
			Invocation:       compilerInvocationBuild,
			WorkingDirectory: filepath.Join(root, "app"),
			GoFlags:          workspaceCompilerBaseGoFlags,
		})
		if err != nil {
			t.Fatalf("workspaceCompilerEnvironmentFrom() error: %v", err)
		}
		assertCompilerEnvironmentValue(t, environment, "XDG_CACHE_HOME", "")
	})
}

func TestTinyGoCacheFallbackWithoutGoCacheLeavesXDGUnset(t *testing.T) {
	for _, test := range []struct {
		name string
		base []string
	}{
		{name: "missing", base: []string{"SENTINEL=preserved"}},
		{name: "empty", base: []string{"GOCACHE=", "SENTINEL=preserved"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			environment, _, err := workspaceCompilerEnvironmentFrom(test.base, compilerEnvironmentOptions{
				Compiler:         "tinygo",
				Invocation:       compilerInvocationBuild,
				WorkingDirectory: t.TempDir(),
				GoFlags:          workspaceCompilerBaseGoFlags,
			})
			if err != nil {
				t.Fatalf("workspaceCompilerEnvironmentFrom() error: %v", err)
			}
			if count := countCompilerEnvironmentKey(environment, "XDG_CACHE_HOME"); count != 0 {
				t.Fatalf("environment contains %d XDG_CACHE_HOME values, want 0: %#v", count, environment)
			}
			assertCompilerEnvironmentValue(t, environment, "SENTINEL", "preserved")
		})
	}
}

func TestCompilerEnvironmentErrorIdentifiesBoundary(t *testing.T) {
	directory := t.TempDir()
	tests := []struct {
		name    string
		options compilerEnvironmentOptions
		want    []string
	}{
		{
			name: "unsupported compiler",
			options: compilerEnvironmentOptions{
				Compiler: "other", Invocation: compilerInvocationBuild,
				WorkingDirectory: directory, GoFlags: workspaceCompilerBaseGoFlags,
			},
			want: []string{"other", "build", "compiler", directory},
		},
		{
			name: "missing invocation",
			options: compilerEnvironmentOptions{
				Compiler: "tinygo", WorkingDirectory: directory,
				GoFlags: workspaceCompilerBaseGoFlags,
			},
			want: []string{"tinygo", "unknown", "invocation", directory},
		},
		{
			name: "empty owned GOFLAGS",
			options: compilerEnvironmentOptions{
				Compiler: "tinygo", Invocation: compilerInvocationBuild,
				WorkingDirectory: directory,
			},
			want: []string{"tinygo", "build", "GOFLAGS", directory},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := workspaceCompilerEnvironmentFrom(nil, test.options)
			if err == nil {
				t.Fatal("workspaceCompilerEnvironmentFrom() succeeded")
			}
			for _, want := range test.want {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}

func TestAmbientGoWorkIsolation(t *testing.T) {
	for _, test := range []struct {
		name     string
		explicit bool
	}{
		{name: "auto"},
		{name: "explicit", explicit: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			appDir := filepath.Join(root, "app")
			writeCompilerEnvironmentTestApp(t, appDir, "")
			writeTestFile(t, root, "host/go.mod", "module example.com/host\n\ngo 1.22\n")
			workContent := "go 1.22\n\nuse ./host\n"
			writeTestFile(t, root, "go.work", workContent)
			workPath := filepath.Join(root, "go.work")
			if test.explicit {
				t.Setenv("GOWORK", workPath)
			} else {
				t.Setenv("GOWORK", "")
			}
			setOfflineCompilerTestEnvironment(t)

			output, err := buildApp(buildOptions{appDir: appDir, compiler: "go"})
			if err != nil {
				t.Fatalf("buildApp() error below parent go.work: %v", err)
			}
			assertNonEmptyCompilerOutput(t, output)
			content, err := os.ReadFile(workPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != workContent {
				t.Fatalf("parent go.work changed:\n%s", content)
			}
		})
	}
}

func TestAmbientGoFlagsIsolation(t *testing.T) {
	tests := []struct {
		name    string
		goFlags func(root string) string
		hostile bool
	}{
		{name: "vendor", goFlags: func(string) string { return "-mod=vendor" }},
		{name: "modfile", goFlags: func(root string) string { return "-modfile=" + filepath.Join(root, "poison.mod") }},
		{name: "overlay", goFlags: func(root string) string { return "-overlay=" + filepath.Join(root, "missing-overlay.json") }},
		{name: "tags", goFlags: func(string) string { return "-tags=goxc_hostile" }, hostile: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			appDir := filepath.Join(root, "app")
			writeCompilerEnvironmentTestApp(t, appDir, "")
			if test.name == "modfile" {
				writeTestFile(t, root, "poison.mod", "module poison.invalid\n\ngo 1.22\n\nrequire broken\n")
			}
			if test.hostile {
				writeTestFile(t, appDir, "hostile.go", "//go:build goxc_hostile\n\npackage main\n\nvar _ = missingHostileSymbol\n")
			}
			t.Setenv("GOWORK", "off")
			t.Setenv("GOFLAGS", test.goFlags(root))
			setOfflineCompilerTestEnvironment(t)

			output, err := buildApp(buildOptions{appDir: appDir, compiler: "go"})
			if err != nil {
				t.Fatalf("buildApp() error with ambient GOFLAGS=%q: %v", test.goFlags(root), err)
			}
			assertNonEmptyCompilerOutput(t, output)
		})
	}
}

func writeCompilerEnvironmentTestApp(t *testing.T, appDir, extraMain string) {
	t.Helper()
	writeTestFile(t, appDir, "go.mod", "module example.com/compiler-environment\n\ngo 1.22\n")
	writeTestFile(t, appDir, manifestName, `{"name":"compiler-environment","compiler":"go"}`)
	writeTestFile(t, appDir, "main.go", "package main\n\nfunc main() {}\n"+extraMain)
}

func setOfflineCompilerTestEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOENV", "off")
}

func assertNonEmptyCompilerOutput(t *testing.T, output string) {
	t.Helper()
	info, err := os.Stat(output)
	if err != nil {
		t.Fatalf("build output missing: %v", err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		t.Fatalf("build output = mode %s size %d", info.Mode(), info.Size())
	}
}

func assertCompilerEnvironmentValue(t *testing.T, environment []string, key, want string) {
	t.Helper()
	if count := countCompilerEnvironmentKey(environment, key); count != 1 {
		t.Fatalf("environment contains %d %s values, want 1: %#v", count, key, environment)
	}
	if got, ok := environmentValue(environment, key); !ok || got != want {
		t.Fatalf("environment %s = %q, present=%v, want %q", key, got, ok, want)
	}
}

func countCompilerEnvironmentKey(environment []string, key string) int {
	count := 0
	for _, item := range environment {
		itemKey, _, ok := strings.Cut(item, "=")
		if ok && environmentKeysEqual(itemKey, key) {
			count++
		}
	}
	return count
}

func TestCompilerEnvironmentWindowsKeyIdentity(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows environment keys are case-insensitive")
	}
	environment, _, err := workspaceCompilerEnvironmentFrom([]string{
		"Path=one", "pwd=stale", "GoWork=parent", "goflags=-mod=vendor", "XDG_CACHE_HOME=cache",
	}, compilerEnvironmentOptions{
		Compiler: "tinygo", Invocation: compilerInvocationBuild,
		WorkingDirectory: t.TempDir(), GoFlags: workspaceCompilerBaseGoFlags,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"PWD", "GOWORK", "GOFLAGS"} {
		if count := countCompilerEnvironmentKey(environment, key); count != 1 {
			t.Fatalf("case-insensitive %s count = %d: %#v", key, count, environment)
		}
	}
}

func TestCompilerEnvironmentIgnoresPersistentGoFlags(t *testing.T) {
	compilers := []string{"go"}
	if _, err := exec.LookPath("tinygo"); err == nil {
		compilers = append(compilers, "tinygo")
	}
	for _, compiler := range compilers {
		t.Run(compiler, func(t *testing.T) {
			root := t.TempDir()
			appDir := filepath.Join(root, "app")
			writeCompilerEnvironmentTestApp(t, appDir, "")
			goEnvPath := filepath.Join(root, "go-env")
			goEnvContent := "GOFLAGS=-mod=vendor\n"
			if err := os.WriteFile(goEnvPath, []byte(goEnvContent), 0o600); err != nil {
				t.Fatal(err)
			}

			t.Setenv("GOENV", goEnvPath)
			unsetCompilerEnvironmentForTest(t, "GOFLAGS")
			t.Setenv("GOWORK", "off")
			t.Setenv("GOPROXY", "off")
			t.Setenv("GOSUMDB", "off")

			output, err := buildApp(buildOptions{appDir: appDir, compiler: compiler})
			if err != nil {
				t.Fatalf("buildApp() with GOENV-persisted GOFLAGS using %s: %v", compiler, err)
			}
			assertNonEmptyCompilerOutput(t, output)
			assertTestFileUnchanged(t, goEnvPath, goEnvContent)
		})
	}
}

func TestPackageEnvironmentIsolation(t *testing.T) {
	compilers := []string{"go"}
	if _, err := exec.LookPath("tinygo"); err == nil {
		compilers = append(compilers, "tinygo")
	}
	for _, compiler := range compilers {
		t.Run(compiler, func(t *testing.T) {
			root := t.TempDir()
			appDir := filepath.Join(root, "app")
			writeCompilerEnvironmentTestApp(t, appDir, "")
			workPath, workContent := writeHostileParentWorkspace(t, root)
			setHostileCompilerWorkflowEnvironment(t, workPath, "-mod=vendor")

			outDir := filepath.Join(t.TempDir(), "package")
			err := packageApp(packageOptions{
				appDir: appDir, compiler: compiler, outDir: outDir,
				workspace: filepath.Join(t.TempDir(), "workspace"),
				compress:  map[string]bool{},
			})
			if err != nil {
				t.Fatalf("packageApp() with hostile environment using %s: %v", compiler, err)
			}

			metadataContent, err := os.ReadFile(filepath.Join(outDir, packageMetadataName))
			if err != nil {
				t.Fatal(err)
			}
			var metadata packageMetadata
			if err := json.Unmarshal(metadataContent, &metadata); err != nil {
				t.Fatalf("decode package metadata: %v", err)
			}
			if metadata.Compiler != compiler || metadata.Entrypoints.WASM == "" {
				t.Fatalf("package metadata = %+v, want compiler %q and WASM entrypoint", metadata, compiler)
			}
			wasmPath := filepath.Join(outDir, filepath.FromSlash(metadata.Entrypoints.WASM))
			assertNonEmptyCompilerOutput(t, wasmPath)
			assertTestFileUnchanged(t, workPath, workContent)
		})
	}
}

func writeHostileParentWorkspace(t *testing.T, root string) (string, string) {
	t.Helper()
	writeTestFile(t, root, "host/go.mod", "module example.com/host\n\ngo 1.22\n")
	content := "go 1.22\n\nuse ./host\n"
	writeTestFile(t, root, "go.work", content)
	return filepath.Join(root, "go.work"), content
}

func setHostileCompilerWorkflowEnvironment(t *testing.T, workPath, goFlags string) {
	t.Helper()
	t.Setenv("GOWORK", workPath)
	t.Setenv("GOFLAGS", goFlags)
	t.Setenv("GOENV", "off")
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
}

func unsetCompilerEnvironmentForTest(t *testing.T, key string) {
	t.Helper()
	value, present := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		var err error
		if present {
			err = os.Setenv(key, value)
		} else {
			err = os.Unsetenv(key)
		}
		if err != nil {
			t.Errorf("restore %s: %v", key, err)
		}
	})
}

func assertTestFileUnchanged(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("%s changed:\ngot  %q\nwant %q", path, content, want)
	}
}
