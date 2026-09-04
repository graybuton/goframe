package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	fakeGoProcessEnvironment         = "GOXC_TEST_FAKE_GO_PROCESS"
	fakeGoRootEnvironment            = "GOXC_TEST_FAKE_GO_ROOT"
	fakeGoRootErrorEnvironment       = "GOXC_TEST_FAKE_GO_ROOT_ERROR"
	fakeGoDirectoryEnvironment       = "GOXC_TEST_FAKE_GO_DIRECTORY"
	fakeGoCompilerContextEnvironment = "GOXC_TEST_FAKE_GO_COMPILER_CONTEXT"
	fakeGoBuildDirectoryEnvironment  = "GOXC_TEST_FAKE_GO_BUILD_DIRECTORY_FILE"
	fakeGoRootDirectoryEnvironment   = "GOXC_TEST_FAKE_GO_ROOT_DIRECTORY_FILE"
)

func TestMain(m *testing.M) {
	if os.Getenv(fakeGoProcessEnvironment) == "1" {
		os.Exit(runFakeGoCommand(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func TestWasmExecPathUsesPATHSelectedGoToolchain(t *testing.T) {
	for _, layout := range []string{
		filepath.Join("lib", "wasm", runtimeAssetName),
		filepath.Join("misc", "wasm", runtimeAssetName),
	} {
		t.Run(filepath.ToSlash(layout), func(t *testing.T) {
			want := installFakeSelectedGo(t, layout, "selected runtime")

			got, err := wasmExecPath("go")
			if err != nil {
				t.Fatalf("wasmExecPath(go) error: %v", err)
			}
			if got != want {
				t.Fatalf("wasmExecPath(go) = %q, want PATH-selected runtime %q", got, want)
			}
		})
	}
}

func TestWasmExecPathRejectsMissingSelectedGoToolchain(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GOROOT", "")

	_, err := wasmExecPath("go")
	if err == nil {
		t.Fatal("wasmExecPath(go) succeeded without a selected Go executable")
	}
	for _, want := range []string{"Go", "PATH"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("wasmExecPath(go) error %q does not contain %q", err, want)
		}
	}
}

func TestWasmExecPathRejectsSelectedGoWithoutRuntime(t *testing.T) {
	root := t.TempDir()
	installFakeGoCommand(t, root)

	_, err := wasmExecPath("go")
	if err == nil {
		t.Fatal("wasmExecPath(go) succeeded without wasm_exec.js in the selected GOROOT")
	}
	for _, want := range []string{"selected GOROOT", root} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("wasmExecPath(go) error %q does not contain %q", err, want)
		}
	}
}

func TestSelectedGoToolchainRootRejectsEmptyOutput(t *testing.T) {
	installFakeGoCommand(t, "")

	_, err := selectedGoToolchainRoot("")
	if err == nil {
		t.Fatal("selectedGoToolchainRoot() accepted empty go env GOROOT output")
	}
	if !strings.Contains(err.Error(), "empty output") {
		t.Fatalf("selectedGoToolchainRoot() error = %q, want empty-output context", err)
	}
}

func TestSelectedGoToolchainRootReportsGoEnvFailure(t *testing.T) {
	installFakeGoCommand(t, t.TempDir())
	t.Setenv(fakeGoRootErrorEnvironment, "selected go env failed")

	_, err := selectedGoToolchainRoot("")
	if err == nil {
		t.Fatal("selectedGoToolchainRoot() succeeded after go env GOROOT failed")
	}
	for _, want := range []string{"query selected Go toolchain GOROOT", "selected go env failed"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("selectedGoToolchainRoot() error %q does not contain %q", err, want)
		}
	}
}

func TestWasmExecPathUsesCompilerWorkingDirectory(t *testing.T) {
	want := installFakeSelectedGo(
		t,
		filepath.Join("lib", "wasm", runtimeAssetName),
		"selected runtime",
	)
	workingDirectory := filepath.Join(t.TempDir(), "workspace", "app")
	if err := os.MkdirAll(workingDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(fakeGoDirectoryEnvironment, workingDirectory)

	got, err := wasmExecPathForWorkingDirectory("go", workingDirectory)
	if err != nil {
		t.Fatalf("wasmExecPathForWorkingDirectory() error: %v", err)
	}
	if got != want {
		t.Fatalf("wasmExecPathForWorkingDirectory() = %q, want %q", got, want)
	}
}

func TestPackageUsesPATHSelectedGoRuntime(t *testing.T) {
	wantRuntime := "runtime from selected Go toolchain"
	installFakeSelectedGo(t, filepath.Join("lib", "wasm", runtimeAssetName), wantRuntime)

	appDir := t.TempDir()
	writeTestFile(t, appDir, "go.mod", "module example.com/selected-runtime\n\ngo 1.26.6\n")
	writeTestFile(t, appDir, manifestName, `{"name":"selected-runtime","compiler":"go","assets":[]}`)
	writeTestFile(t, appDir, "main.go", "package main\n\nfunc main() {}\n")
	outDir := filepath.Join(t.TempDir(), "package")

	if err := packageApp(packageOptions{
		appDir: appDir, compiler: "go", outDir: outDir, compress: map[string]bool{},
	}); err != nil {
		t.Fatalf("packageApp() error: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(outDir, assetDirectoryName, runtimeAssetName))
	if err != nil {
		t.Fatalf("read packaged runtime: %v", err)
	}
	if string(content) != wantRuntime {
		t.Fatalf("packaged runtime = %q, want selected-toolchain content %q", content, wantRuntime)
	}
}

func TestPackageRuntimeDiscoveryUsesCompilerSelectionContext(t *testing.T) {
	wantRuntime := "runtime from context-selected Go toolchain"
	installFakeSelectedGo(t, filepath.Join("lib", "wasm", runtimeAssetName), wantRuntime)

	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	writeTestFile(t, root, "go.work", "go 1.26.6\n\nuse ./app\n")
	writeTestFile(t, appDir, "go.mod", "module example.com/context-selected-runtime\n\ngo 1.26.6\n")
	writeTestFile(t, appDir, manifestName, `{"name":"context-selected-runtime","compiler":"go","assets":[]}`)
	writeTestFile(t, appDir, "main.go", "package main\n\nfunc main() {}\n")

	buildDirectoryFile := filepath.Join(t.TempDir(), "build-directory")
	rootDirectoryFile := filepath.Join(t.TempDir(), "root-directory")
	t.Setenv("GOWORK", filepath.Join(root, "go.work"))
	t.Setenv("GO111MODULE", "off")
	t.Setenv("GOFLAGS", "-mod=vendor")
	t.Setenv(fakeGoCompilerContextEnvironment, "1")
	t.Setenv(fakeGoBuildDirectoryEnvironment, buildDirectoryFile)
	t.Setenv(fakeGoRootDirectoryEnvironment, rootDirectoryFile)

	if err := packageApp(packageOptions{
		appDir: appDir, compiler: "go", outDir: filepath.Join(t.TempDir(), "package"), compress: map[string]bool{},
	}); err != nil {
		t.Fatalf("packageApp() error: %v", err)
	}

	buildDirectory, err := os.ReadFile(buildDirectoryFile)
	if err != nil {
		t.Fatalf("read compiler working-directory observation: %v", err)
	}
	rootDirectory, err := os.ReadFile(rootDirectoryFile)
	if err != nil {
		t.Fatalf("read runtime-root working-directory observation: %v", err)
	}
	if string(rootDirectory) != string(buildDirectory) {
		t.Fatalf(
			"runtime-root working directory = %q, compiler working directory = %q",
			rootDirectory,
			buildDirectory,
		)
	}
}

func TestDoctorReportsPATHSelectedGoRuntime(t *testing.T) {
	want := installFakeSelectedGo(
		t,
		filepath.Join("lib", "wasm", runtimeAssetName),
		"selected runtime",
	)

	var err error
	output := captureStdout(t, func() {
		err = doctorCommand(nil)
	})
	if err != nil {
		t.Fatalf("doctorCommand() error: %v\n%s", err, output)
	}
	if !strings.Contains(output, "wasm_exec.js: found, "+want) {
		t.Fatalf("doctor output does not report the PATH-selected runtime %q:\n%s", want, output)
	}
}

func TestRelocatedTrimpathGoxcUsesPATHSelectedGoRuntime(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	buildDir := t.TempDir()
	binaryName := "goxc"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	builtPath := filepath.Join(buildDir, binaryName)
	command := exec.Command("go", "build", "-trimpath", "-o", builtPath, "./cmd/goxc")
	command.Dir = repositoryRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build trimpath goxc: %v\n%s", err, output)
	}

	relocatedDir := t.TempDir()
	relocatedPath := filepath.Join(relocatedDir, binaryName)
	if err := os.Rename(builtPath, relocatedPath); err != nil {
		t.Fatalf("relocate goxc: %v", err)
	}
	want := installFakeSelectedGo(
		t,
		filepath.Join("lib", "wasm", runtimeAssetName),
		"selected runtime",
	)

	command = exec.Command(relocatedPath, "doctor")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run relocated goxc doctor: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "wasm_exec.js: found, "+want) {
		t.Fatalf("relocated goxc did not report PATH-selected runtime %q:\n%s", want, output)
	}
}

func installFakeSelectedGo(t *testing.T, runtimeRelativePath, runtimeContent string) string {
	t.Helper()
	root := t.TempDir()
	runtimePath := filepath.Join(root, runtimeRelativePath)
	if err := os.MkdirAll(filepath.Dir(runtimePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimePath, []byte(runtimeContent), 0o644); err != nil {
		t.Fatal(err)
	}
	installFakeGoCommand(t, root)
	return runtimePath
}

func installFakeGoCommand(t *testing.T, root string) {
	t.Helper()
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	goName := "go"
	if runtime.GOOS == "windows" {
		goName += ".exe"
	}
	if err := copyExecutableForTest(executable, filepath.Join(binDir, goName)); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", binDir)
	t.Setenv("GOROOT", "")
	t.Setenv(fakeGoProcessEnvironment, "1")
	t.Setenv(fakeGoRootEnvironment, root)
}

func copyExecutableForTest(sourcePath, destinationPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func runFakeGoCommand(args []string) int {
	root := os.Getenv(fakeGoRootEnvironment)
	switch {
	case len(args) == 1 && args[0] == "version":
		fmt.Println("go version go1.26.6 test/amd64")
		return 0
	case len(args) == 2 && args[0] == "env" && args[1] == "GOROOT":
		if err := validateFakeGoCompilerContext(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := recordFakeGoWorkingDirectory(fakeGoRootDirectoryEnvironment); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if message := os.Getenv(fakeGoRootErrorEnvironment); message != "" {
			fmt.Fprintln(os.Stderr, message)
			return 1
		}
		if want := os.Getenv(fakeGoDirectoryEnvironment); want != "" {
			got, err := os.Getwd()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			if got != want {
				fmt.Fprintf(os.Stderr, "go env GOROOT directory = %q, want %q\n", got, want)
				return 1
			}
		}
		fmt.Println(root)
		return 0
	case len(args) == 2 && args[0] == "env" && args[1] == "GOVERSION":
		fmt.Println("go1.26.6")
		return 0
	case len(args) > 0 && args[0] == "list":
		return 0
	case len(args) > 0 && args[0] == "build":
		if err := validateFakeGoCompilerContext(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := recordFakeGoWorkingDirectory(fakeGoBuildDirectoryEnvironment); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		for index := 1; index+1 < len(args); index++ {
			if args[index] != "-o" {
				continue
			}
			if err := os.WriteFile(args[index+1], []byte("\x00asmfake"), 0o600); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			return 0
		}
		fmt.Fprintln(os.Stderr, "fake go build did not receive -o")
		return 2
	default:
		fmt.Fprintf(os.Stderr, "unsupported fake go command: %q\n", args)
		return 2
	}
}

func validateFakeGoCompilerContext() error {
	if os.Getenv(fakeGoCompilerContextEnvironment) != "1" {
		return nil
	}
	for _, expected := range []struct {
		key   string
		value string
	}{
		{key: "GOWORK", value: "off"},
		{key: "GO111MODULE", value: "on"},
		{key: "GOFLAGS", value: workspaceCompilerBaseGoFlags},
		{key: "GOOS", value: "js"},
		{key: "GOARCH", value: "wasm"},
		{key: "CGO_ENABLED", value: "0"},
	} {
		if got := os.Getenv(expected.key); got != expected.value {
			return fmt.Errorf("%s = %q, want %q", expected.key, got, expected.value)
		}
	}
	return nil
}

func recordFakeGoWorkingDirectory(environmentKey string) error {
	path := os.Getenv(environmentKey)
	if path == "" {
		return nil
	}
	directory, err := os.Getwd()
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(directory), 0o644)
}
