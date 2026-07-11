package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const validCheckSource = `package main

func App() any {
	return <main>Hello</main>
}
`

const invalidCheckSource = `package main

func App() any {
	return <main>{}</main>
}
`

func TestParseCheckOptions(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantPath   string
		wantFormat checkFormat
		wantError  string
	}{
		{name: "default text", args: []string{"app.gox"}, wantPath: "app.gox", wantFormat: checkFormatText},
		{name: "explicit text equals", args: []string{"app.gox", "--format=text"}, wantPath: "app.gox", wantFormat: checkFormatText},
		{name: "explicit text separate", args: []string{"--format", "text", "app.gox"}, wantPath: "app.gox", wantFormat: checkFormatText},
		{name: "json equals", args: []string{"app.gox", "--format=json"}, wantPath: "app.gox", wantFormat: checkFormatJSON},
		{name: "json separate", args: []string{"--format", "json", "app.gox"}, wantPath: "app.gox", wantFormat: checkFormatJSON},
		{name: "missing path", wantError: "usage: goxc check"},
		{name: "format without path", args: []string{"--format=json"}, wantError: "usage: goxc check"},
		{name: "extra positional", args: []string{"one.gox", "two.gox"}, wantError: "unexpected check argument"},
		{name: "unknown flag", args: []string{"app.gox", "--watch"}, wantError: "unknown check flag"},
		{name: "no json alias", args: []string{"app.gox", "--json"}, wantError: "unknown check flag"},
		{name: "no short format", args: []string{"app.gox", "-f", "json"}, wantError: "unknown check flag"},
		{name: "missing format value", args: []string{"app.gox", "--format"}, wantError: "--format requires a value"},
		{name: "unsupported format", args: []string{"app.gox", "--format=yaml"}, wantError: "unsupported check format"},
		{name: "no out", args: []string{"app.gox", "--out=gen"}, wantError: "unknown check flag"},
		{name: "no workspace", args: []string{"app.gox", "--workspace=work"}, wantError: "unknown check flag"},
		{name: "no in place", args: []string{"app.gox", "--in-place"}, wantError: "unknown check flag"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options, err := parseCheckOptions(test.args)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("parseCheckOptions() error = %v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCheckOptions() error: %v", err)
			}
			if options.path != test.wantPath || options.format != test.wantFormat {
				t.Fatalf("parseCheckOptions() = %+v, want path %q format %q", options, test.wantPath, test.wantFormat)
			}
		})
	}
}

func TestCheckValidSingleFileText(t *testing.T) {
	root := t.TempDir()
	path := writeCheckSource(t, root, "app.gox", validCheckSource)
	before := snapshotCheckTree(t, root)

	stdout, stderr, err := runCheckForTest([]string{path})
	if err != nil {
		t.Fatalf("runCheckCommand() error: %v", err)
	}
	if stdout != "checked 1 GOX files: no diagnostics\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	assertCheckTreeUnchanged(t, root, before)
}

func TestCheckValidSingleFileJSON(t *testing.T) {
	root := t.TempDir()
	path := writeCheckSource(t, root, "app.gox", validCheckSource)
	before := snapshotCheckTree(t, root)

	stdout, stderr, err := runCheckForTest([]string{path, "--format=json"})
	if err != nil {
		t.Fatalf("runCheckCommand() error: %v", err)
	}
	report := decodeCheckReport(t, stdout)
	if report.SchemaVersion != 1 || !report.OK || report.FilesChecked != 1 {
		t.Fatalf("report = %+v", report)
	}
	if report.Diagnostics == nil || len(report.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want non-nil empty slice", report.Diagnostics)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	assertCheckTreeUnchanged(t, root, before)
}

func TestCheckInvalidSourceText(t *testing.T) {
	root := t.TempDir()
	path := writeCheckSource(t, root, "app.gox", invalidCheckSource)
	before := snapshotCheckTree(t, root)

	stdout, stderr, err := runCheckForTest([]string{path})
	if !errors.Is(err, errCheckDiagnostics) {
		t.Fatalf("runCheckCommand() error = %v, want errCheckDiagnostics", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	want := path + ":4:15: gox: empty child expression\n  <main>{}</main>\nchecked 1 GOX files: 1 diagnostic\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if strings.Contains(stderr, ".goframe") {
		t.Fatalf("stderr contains generated workspace path: %q", stderr)
	}
	assertCheckTreeUnchanged(t, root, before)
}

func TestCheckInvalidSourceJSON(t *testing.T) {
	root := t.TempDir()
	path := writeCheckSource(t, root, "app.gox", invalidCheckSource)
	before := snapshotCheckTree(t, root)

	stdout, stderr, err := runCheckForTest([]string{path, "--format", "json"})
	if !errors.Is(err, errCheckDiagnostics) {
		t.Fatalf("runCheckCommand() error = %v, want errCheckDiagnostics", err)
	}
	report := decodeCheckReport(t, stdout)
	if report.SchemaVersion != 1 || report.OK || report.FilesChecked != 1 || len(report.Diagnostics) != 1 {
		t.Fatalf("report = %+v", report)
	}
	diagnostic := report.Diagnostics[0]
	want := checkDiagnostic{
		File:     path,
		Line:     4,
		Column:   15,
		Severity: "error",
		Message:  "gox: empty child expression",
		Source:   "<main>{}</main>",
	}
	if diagnostic != want {
		t.Fatalf("diagnostic = %+v, want %+v", diagnostic, want)
	}
	if !filepath.IsAbs(diagnostic.File) {
		t.Fatalf("diagnostic file = %q, want absolute path", diagnostic.File)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if strings.Contains(stdout, `\u003c`) || strings.Contains(stdout, `\u003e`) {
		t.Fatalf("JSON escaped GOX markup as HTML: %q", stdout)
	}
	assertCheckTreeUnchanged(t, root, before)
}

func TestCheckCollectsDiagnosticsFromMultipleFiles(t *testing.T) {
	root := t.TempDir()
	writeCheckSource(t, root, "valid.gox", validCheckSource)
	firstPath := writeCheckSource(t, root, "a/first.gox", invalidCheckSource)
	secondPath := writeCheckSource(t, root, "z/second.gox", invalidCheckSource)
	before := snapshotCheckTree(t, root)

	stdout, stderr, err := runCheckForTest([]string{root, "--format=json"})
	if !errors.Is(err, errCheckDiagnostics) {
		t.Fatalf("runCheckCommand() error = %v, want errCheckDiagnostics", err)
	}
	report := decodeCheckReport(t, stdout)
	if report.FilesChecked != 3 || len(report.Diagnostics) != 2 {
		t.Fatalf("report = %+v", report)
	}
	if report.Diagnostics[0].File != firstPath || report.Diagnostics[1].File != secondPath {
		t.Fatalf("diagnostic order = %#v", report.Diagnostics)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	assertCheckTreeUnchanged(t, root, before)
}

func TestCheckNestedPackageTreeWithoutWorkspace(t *testing.T) {
	root := t.TempDir()
	writeCheckSource(t, root, "go.mod", "module example.com/app\n\ngo 1.22\n")
	writeCheckSource(t, root, "app.gox", validCheckSource)
	writeCheckSource(t, root, "internal/ui/view.gox", strings.Replace(validCheckSource, "package main", "package ui", 1))
	before := snapshotCheckTree(t, root)

	stdout, stderr, err := runCheckForTest([]string{root, "--format=json"})
	if err != nil {
		t.Fatalf("runCheckCommand() error: %v", err)
	}
	report := decodeCheckReport(t, stdout)
	if !report.OK || report.FilesChecked != 2 {
		t.Fatalf("report = %+v", report)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	assertCheckTreeUnchanged(t, root, before)
}

func TestCheckGenericCompilerErrorUsesSourceFile(t *testing.T) {
	root := t.TempDir()
	path := writeCheckSource(t, root, "plain.gox", "package main\n\nfunc App() any { return nil }\n")

	stdout, stderr, err := runCheckForTest([]string{path, "--format=json"})
	if !errors.Is(err, errCheckDiagnostics) {
		t.Fatalf("runCheckCommand() error = %v, want errCheckDiagnostics", err)
	}
	report := decodeCheckReport(t, stdout)
	if len(report.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", report.Diagnostics)
	}
	diagnostic := report.Diagnostics[0]
	if diagnostic.File != path || diagnostic.Line != 0 || diagnostic.Column != 0 || diagnostic.Source != "" || diagnostic.Severity != "error" {
		t.Fatalf("diagnostic = %+v", diagnostic)
	}
	if diagnostic.Message != "no GOX markup elements found" || strings.Contains(diagnostic.Message, path) {
		t.Fatalf("diagnostic message = %q", diagnostic.Message)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	stdout, stderr, err = runCheckForTest([]string{path})
	if !errors.Is(err, errCheckDiagnostics) {
		t.Fatalf("text runCheckCommand() error = %v, want errCheckDiagnostics", err)
	}
	if stdout != "" {
		t.Fatalf("text stdout = %q, want empty", stdout)
	}
	wantText := path + ": no GOX markup elements found\nchecked 1 GOX files: 1 diagnostic\n"
	if stderr != wantText || strings.Contains(stderr, ":0:0") {
		t.Fatalf("text stderr = %q, want %q without zero location", stderr, wantText)
	}
}

func TestCheckDirectoryWithNoGOXFilesIsOperationalError(t *testing.T) {
	root := t.TempDir()
	writeCheckSource(t, root, "main.go", "package main\n")
	before := snapshotCheckTree(t, root)

	stdout, stderr, err := runCheckForTest([]string{root, "--format=json"})
	if err == nil || errors.Is(err, errCheckDiagnostics) || !strings.Contains(err.Error(), "no .gox files found") {
		t.Fatalf("runCheckCommand() error = %v", err)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("operational error output: stdout=%q stderr=%q", stdout, stderr)
	}
	assertCheckTreeUnchanged(t, root, before)
}

func TestCheckMissingPathIsOperationalError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.gox")
	stdout, stderr, err := runCheckForTest([]string{path, "--format=json"})
	if err == nil || errors.Is(err, errCheckDiagnostics) {
		t.Fatalf("runCheckCommand() error = %v", err)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("missing path output: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestCheckRejectsSymlinkedGOXSource(t *testing.T) {
	requireSymlinkSupport(t)
	root := t.TempDir()
	target := writeCheckSource(t, t.TempDir(), "target.gox", validCheckSource)
	link := filepath.Join(root, "app.gox")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runCheckForTest([]string{link, "--format=json"})
	if err == nil || errors.Is(err, errCheckDiagnostics) || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("runCheckCommand() error = %v", err)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("symlink error output: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestCheckUsageIsListed(t *testing.T) {
	var output bytes.Buffer
	usage(&output)
	if !strings.Contains(output.String(), "check <path>") {
		t.Fatalf("usage() does not list check: %q", output.String())
	}
}

func runCheckForTest(args []string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runCheckCommand(args, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func decodeCheckReport(t *testing.T, output string) checkReport {
	t.Helper()
	if !strings.HasSuffix(output, "\n") || strings.Count(output, "\n") != 1 {
		t.Fatalf("JSON output must have exactly one trailing newline: %q", output)
	}
	decoder := json.NewDecoder(strings.NewReader(output))
	var report checkReport
	if err := decoder.Decode(&report); err != nil {
		t.Fatalf("decode check report: %v\n%s", err, output)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatalf("JSON output contains extra content: %v\n%s", err, output)
	}
	return report
}

func writeCheckSource(t *testing.T, root, relative, content string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(absolute)
}

type checkTreeEntry struct {
	Mode    os.FileMode
	Content string
}

func snapshotCheckTree(t *testing.T, root string) map[string]checkTreeEntry {
	t.Helper()
	snapshot := map[string]checkTreeEntry{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entry := checkTreeEntry{Mode: info.Mode()}
		if info.Mode().IsRegular() {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			entry.Content = string(content)
		}
		snapshot[filepath.ToSlash(relative)] = entry
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot check tree: %v", err)
	}
	return snapshot
}

func assertCheckTreeUnchanged(t *testing.T, root string, before map[string]checkTreeEntry) {
	t.Helper()
	after := snapshotCheckTree(t, root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("checked tree changed\nbefore: %#v\nafter:  %#v", before, after)
	}
	if _, err := os.Lstat(filepath.Join(root, defaultWorkspaceName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("check created %s: %v", defaultWorkspaceName, err)
	}
	for path := range after {
		if strings.HasSuffix(path, ".gox.go") {
			t.Fatalf("check created generated file %s", path)
		}
	}
}
