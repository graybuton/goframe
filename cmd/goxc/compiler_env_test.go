package main

import (
	"os"
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

func TestCompilerEnvironmentErrorIdentifiesBoundary(t *testing.T) {
	directory := t.TempDir()
	_, _, err := workspaceCompilerEnvironmentFrom(nil, compilerEnvironmentOptions{
		Compiler: "tinygo", Invocation: compilerInvocationBuild,
		WorkingDirectory: directory,
	})
	if err == nil {
		t.Fatal("workspaceCompilerEnvironmentFrom() succeeded without owned GOFLAGS")
	}
	for _, want := range []string{"tinygo", "build", "GOFLAGS", directory} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
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
