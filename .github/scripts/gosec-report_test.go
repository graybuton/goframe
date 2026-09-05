package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestGosecReportFindingsAreAdvisory(t *testing.T) {
	report := readTestGosecReport(t, `{
  "Golang errors": {},
  "Issues": [
    {"rule_id":"G304","details":"file path provided as input","file":"/repo/z.go","line":"9","column":"3"},
    {"rule_id":"G204","details":"subprocess launched with variable","file":"/repo/a.go","line":"4","column":"2"},
    {"rule_id":"G204","details":"subprocess launched with variable","file":"/repo/b.go","line":"5-7","column":"02"}
  ],
  "Stats": {"files":2,"lines":20,"nosec":0,"found":3},
  "GosecVersion":"dev"
}`)
	var output bytes.Buffer
	if err := writeGosecSummary(&output, report, "/repo", 2); err != nil {
		t.Fatalf("writeGosecSummary() error = %v", err)
	}
	if diagnostic := regexp.MustCompile(`(?m)^gosec: advisory .*\.go:[0-9-]+:[0-9-]+:`).FindString(output.String()); diagnostic != "" {
		t.Fatalf("advisory output contains compiler-diagnostic location syntax %q:\n%s", diagnostic, output.String())
	}
	want := []string{
		"packages=2 files=2 lines=20 findings=3",
		"gosec: G204=2",
		"gosec: G304=1",
		strings.Join([]string{
			"gosec: advisory G204 a.go (line 4, column 2): subprocess launched with variable",
			"gosec: advisory G204 b.go (line 5-7, column 02): subprocess launched with variable",
			"gosec: advisory G304 z.go (line 9, column 3): file path provided as input",
		}, "\n"),
		"gosec: findings are advisory; analyzer health and package coverage passed",
	}
	for _, fragment := range want {
		if !strings.Contains(output.String(), fragment) {
			t.Fatalf("summary missing %q:\n%s", fragment, output.String())
		}
	}
	t.Log(output.String())
}

func TestGosecReportCleanScanPasses(t *testing.T) {
	report := readTestGosecReport(t, `{
  "Golang errors": {},
  "Issues": [],
  "Stats": {"files":1,"lines":8,"nosec":0,"found":0},
  "GosecVersion":"dev"
}`)
	var output bytes.Buffer
	if err := writeGosecSummary(&output, report, "/repo", 1); err != nil {
		t.Fatalf("writeGosecSummary() error = %v", err)
	}
	if !strings.Contains(output.String(), "gosec: no findings") {
		t.Fatalf("summary = %q, want clean result", output.String())
	}
}

func TestGosecReportRejectsProcessingErrors(t *testing.T) {
	report := readTestGosecReport(t, `{
  "Golang errors": {"pkg/broken":[{"line":0,"column":0,"error":"mixed packages"}]},
  "Issues": [],
  "Stats": {"files":1,"lines":8,"nosec":0,"found":0},
  "GosecVersion":"dev"
}`)
	if err := writeGosecSummary(&bytes.Buffer{}, report, "/repo", 1); err == nil || !strings.Contains(err.Error(), "mixed packages") {
		t.Fatalf("writeGosecSummary() error = %v, want processing failure", err)
	}
}

func TestGosecReportRejectsMalformedOrIncompleteInput(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "empty", content: "", want: "report is empty"},
		{name: "malformed", content: "{", want: "decode JSON"},
		{name: "missing issues", content: `{"Golang errors":{},"Stats":{},"GosecVersion":"dev"}`, want: `missing required field "Issues"`},
		{name: "null issues", content: `{"Golang errors":{},"Issues":null,"Stats":{},"GosecVersion":"dev"}`, want: `required field "Issues" is null`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "report.json")
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := readGosecReport(path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("readGosecReport() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestGosecReportRejectsMissingInput(t *testing.T) {
	_, err := readGosecReport(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil || !strings.Contains(err.Error(), "no such file") {
		t.Fatalf("readGosecReport() error = %v, want missing file failure", err)
	}
}

func TestGosecReportRejectsUnprovenCoverage(t *testing.T) {
	report := readTestGosecReport(t, `{
  "Golang errors": {},
  "Issues": [],
  "Stats": {"files":1,"lines":8,"nosec":0,"found":0},
  "GosecVersion":"dev"
}`)
	if err := writeGosecSummary(&bytes.Buffer{}, report, "/repo", 0); err == nil || !strings.Contains(err.Error(), "empty or unproven") {
		t.Fatalf("writeGosecSummary() error = %v, want coverage failure", err)
	}
}

func TestGosecReportRejectsZeroAnalyzedFiles(t *testing.T) {
	report := readTestGosecReport(t, `{
  "Golang errors": {},
  "Issues": [],
  "Stats": {"files":0,"lines":0,"nosec":0,"found":0},
  "GosecVersion":"dev"
}`)
	if err := writeGosecSummary(&bytes.Buffer{}, report, "/repo", 1); err == nil || !strings.Contains(err.Error(), "no analyzed files") {
		t.Fatalf("writeGosecSummary() error = %v, want files failure", err)
	}
}

func readTestGosecReport(t *testing.T, content string) gosecReport {
	t.Helper()
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := readGosecReport(path)
	if err != nil {
		t.Fatalf("readGosecReport() error = %v", err)
	}
	return report
}
