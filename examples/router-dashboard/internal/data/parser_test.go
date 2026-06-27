package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseIssuesValidInput(t *testing.T) {
	text := strings.Join([]string{
		"RD-1|Auth token refresh fails on slow networks|open|high|Identity|Refresh state is not visible.",
		"",
		"RD-2|Billing dashboard needs clearer empty state|review|medium|Billing|Empty invoices should explain filters.",
	}, "\n")

	issues, err := ParseIssues(text)
	if err != nil {
		t.Fatalf("ParseIssues returned error: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("len(ParseIssues) = %d, want 2", len(issues))
	}
	if issues[0].ID != "RD-1" || issues[1].ID != "RD-2" {
		t.Fatalf("issues parsed out of order: %#v", issues)
	}
	if issues[0].Status != "open" || issues[0].Priority != "high" {
		t.Fatalf("first issue normalized fields = %q/%q", issues[0].Status, issues[0].Priority)
	}
}

func TestParseIssuesPackagedDataset(t *testing.T) {
	path := filepath.Join("..", "..", "assets", "data", "issues.txt")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	issues, err := ParseIssues(string(raw))
	if err != nil {
		t.Fatalf("ParseIssues(packaged dataset): %v", err)
	}
	if len(issues) < 10 || len(issues) > 30 {
		t.Fatalf("packaged dataset len = %d, want compact reference range", len(issues))
	}
	if issues[1].ID != "RD-2" {
		t.Fatalf("second packaged issue = %q, want RD-2", issues[1].ID)
	}
}

func TestParseIssuesRejectsMalformedRow(t *testing.T) {
	_, err := ParseIssues("RD-1|Missing fields")
	if err == nil || err.Error() != "invalid issue data line" {
		t.Fatalf("ParseIssues malformed row error = %v", err)
	}
}

func TestParseIssuesRejectsInvalidStatus(t *testing.T) {
	_, err := ParseIssues("RD-1|Title|blocked|high|Owner|Description")
	if err == nil || err.Error() != "issue data contains an invalid status" {
		t.Fatalf("ParseIssues invalid status error = %v", err)
	}
}

func TestParseIssuesRejectsInvalidPriority(t *testing.T) {
	_, err := ParseIssues("RD-1|Title|open|urgent|Owner|Description")
	if err == nil || err.Error() != "issue data contains an invalid priority" {
		t.Fatalf("ParseIssues invalid priority error = %v", err)
	}
}

func TestParseIssuesRejectsEmptyDataset(t *testing.T) {
	_, err := ParseIssues("\n\n")
	if err == nil || err.Error() != "issue data is empty" {
		t.Fatalf("ParseIssues empty dataset error = %v", err)
	}
}
