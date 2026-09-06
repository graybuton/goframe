package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var workflowLogPayloads = []struct {
	name string
	raw  string
	want string
}{
	{"V2 warning LF", "\n::warning file=fake.go,line=1::forged", `\n::warning file=fake.go,line=1::forged`},
	{"V2 error CR", "\r::error::forged", `\r::error::forged`},
	{"V2 mask CRLF", "\r\n::add-mask::secret", `\r\n::add-mask::secret`},
	{"V2 stop LF", "\n::stop-commands::TOKEN", `\n::stop-commands::TOKEN`},
	{"legacy warning", "##[warning]forged", `##\[warning]forged`},
	{"legacy error", "prefix ##[error]forged", `prefix ##\[error]forged`},
	{"legacy mask", "##[add-mask]secret", `##\[add-mask]secret`},
	{"legacy stop", "##[stop-commands]TOKEN", `##\[stop-commands]TOKEN`},
	{"bare prefix", "##[", `##\[`},
	{"three hashes", "###[", `###\[`},
	{"four hashes", "####[", `####\[`},
	{"repeated prefix", "##[\n##[", `##\[\n##\[`},
	{"CR prefix", "\r##[", `\r##\[`},
	{"LF prefix", "\n##[warning]", `\n##\[warning]`},
}

func TestSanitizeWorkflowLogField(t *testing.T) {
	for _, payload := range workflowLogPayloads {
		t.Run(payload.name, func(t *testing.T) {
			got := sanitizeWorkflowLogField(payload.raw)
			if got != payload.want {
				t.Errorf("sanitized field = %q, want %q", got, payload.want)
			}
			if strings.ContainsAny(got, "\r\n") || strings.Contains(got, "##[") {
				t.Errorf("unsafe sanitized field %q", got)
			}
			if sanitizeWorkflowLogField(got) != got {
				t.Errorf("sanitizing already-encoded field changed %q", got)
			}
		})
	}
	for _, safe := range []string{"", "G204 a.go (line 4, column 2): message", "data ::warning::text", `C:\repo\file.go`, `literal \r\n ##\[ text`} {
		if got := sanitizeWorkflowLogField(safe); got != safe {
			t.Errorf("safe text changed: %q became %q", safe, got)
		}
	}
}

func TestGosecReportWorkflowLogFields(t *testing.T) {
	for _, field := range []string{"GosecVersion", "rule_id", "file", "line", "column", "details"} {
		for _, payload := range workflowLogPayloads {
			t.Run(field+"/"+payload.name, func(t *testing.T) {
				issue := map[string]string{
					"rule_id": "G204", "file": "/repo/a.go", "line": "4", "column": "2", "details": "message",
				}
				version := "dev"
				if field == "GosecVersion" {
					version += payload.raw
				} else {
					issue[field] += payload.raw
				}
				report := readWorkflowLogTestReport(t, version, issue, map[string][]gosecProcessingError{})
				before, err := json.Marshal(report)
				if err != nil {
					t.Fatal(err)
				}
				var output bytes.Buffer
				if err := writeGosecSummary(&output, report, "/repo", 1); err != nil {
					t.Fatalf("advisory report rejected: %v", err)
				}
				assertWorkflowLogSafe(t, output.String(), 4)
				if !strings.Contains(output.String(), payload.want) {
					t.Errorf("escaped payload %q missing from %q", payload.want, output.String())
				}
				if !strings.Contains(output.String(), "gosec: findings are advisory; analyzer health and package coverage passed\n") {
					t.Errorf("advisory conclusion missing: %q", output.String())
				}
				after, err := json.Marshal(report)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(before, after) {
					t.Fatal("logging mutated raw report data")
				}
			})
		}
	}
}

func TestGosecReportWorkflowLogProcessingErrors(t *testing.T) {
	for _, field := range []string{"path", "message", "path without details"} {
		for _, payload := range workflowLogPayloads {
			t.Run(field+"/"+payload.name, func(t *testing.T) {
				path, message := "pkg/broken", "mixed packages"
				if field == "message" {
					message += payload.raw
				} else {
					path += payload.raw
				}
				processingErrors := map[string][]gosecProcessingError{
					path: {{Line: 4, Column: 2, Error: message}},
				}
				if field == "path without details" {
					processingErrors[path] = nil
					message = "unspecified processing error"
				}
				report := readWorkflowLogTestReport(t, "dev", nil, processingErrors)
				var output bytes.Buffer
				err := writeGosecSummary(&output, report, "/repo", 1)
				if err == nil {
					t.Fatal("processing error did not block report")
				}
				if output.Len() != 0 {
					t.Errorf("processing failure emitted a success summary: %q", output.String())
				}
				logged := "gosec report: " + err.Error() + "\n"
				assertWorkflowLogSafe(t, logged, 1)
				for _, want := range []string{"Go/package processing errors", "pkg/broken", payload.want} {
					if !strings.Contains(logged, want) {
						t.Errorf("processing diagnostic missing %q: %q", want, logged)
					}
				}
				if field == "path without details" && !strings.Contains(logged, message) {
					t.Errorf("unspecified processing diagnostic missing: %q", logged)
				}
				if field != "path without details" && !strings.Contains(logged, ":4:2: mixed packages") {
					t.Errorf("blocking location/message changed: %q", logged)
				}
			})
		}
	}
}

func assertWorkflowLogSafe(t *testing.T, output string, records int) {
	t.Helper()
	if strings.Contains(output, "\r") || strings.Contains(output, "##[") {
		t.Errorf("unsafe workflow log token in %q", output)
	}
	if strings.Count(output, "\n") != records || !strings.HasSuffix(output, "\n") {
		t.Errorf("expected %d formatter-owned lines, got %q", records, output)
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "::") {
			t.Errorf("workflow command at start of physical line: %q", line)
		}
	}
}

func readWorkflowLogTestReport(t *testing.T, version string, issue map[string]string, processingErrors map[string][]gosecProcessingError) gosecReport {
	t.Helper()
	issues := []map[string]string{}
	if issue != nil {
		issues = append(issues, issue)
	}
	content, err := json.Marshal(map[string]any{
		"GosecVersion": version, "Golang errors": processingErrors, "Issues": issues,
		"Stats": map[string]int{"files": 1, "lines": 8, "nosec": 0, "found": len(issues)},
	})
	if err != nil {
		t.Fatal(err)
	}
	return readTestGosecReport(t, string(content))
}

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
		"gosec: version=dev packages=2 files=2 lines=20 findings=3",
		"gosec: G204=2",
		"gosec: G304=1",
		strings.Join([]string{
			"gosec: advisory G204 a.go (line 4, column 2): subprocess launched with variable",
			"gosec: advisory G204 b.go (line 5-7, column 02): subprocess launched with variable",
			"gosec: advisory G304 z.go (line 9, column 3): file path provided as input",
		}, "\n"),
		"gosec: findings are advisory; analyzer health and package coverage passed",
	}
	if expected := strings.Join(want, "\n") + "\n"; output.String() != expected {
		t.Fatalf("summary = %q, want unchanged ordinary output %q", output.String(), expected)
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
