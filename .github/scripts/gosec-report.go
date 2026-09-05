package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type gosecIssue struct {
	RuleID  string `json:"rule_id"`
	Details string `json:"details"`
	File    string `json:"file"`
	Line    string `json:"line"`
	Column  string `json:"column"`
}

type gosecProcessingError struct {
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Error  string `json:"error"`
}

type gosecStats struct {
	Files *int `json:"files"`
	Lines *int `json:"lines"`
	NoSec *int `json:"nosec"`
	Found *int `json:"found"`
}

type gosecReport struct {
	Version          string
	ProcessingErrors map[string][]gosecProcessingError
	Issues           []gosecIssue
	Stats            gosecStats
}

func main() {
	reportPath := flag.String("report", "", "path to the gosec JSON report")
	repositoryRoot := flag.String("root", "", "repository root used to shorten finding paths")
	packageCount := flag.Int("packages", 0, "number of Go packages supplied to gosec")
	flag.Parse()

	if *reportPath == "" || *repositoryRoot == "" || *packageCount <= 0 || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: gosec-report -report <path> -root <path> -packages <positive count>")
		os.Exit(2)
	}
	report, err := readGosecReport(*reportPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gosec report: %v\n", err)
		os.Exit(1)
	}
	if err := writeGosecSummary(os.Stdout, report, *repositoryRoot, *packageCount); err != nil {
		fmt.Fprintf(os.Stderr, "gosec report: %v\n", err)
		os.Exit(1)
	}
}

func readGosecReport(path string) (gosecReport, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return gosecReport{}, fmt.Errorf("read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return gosecReport{}, errors.New("report is empty")
	}

	decoder := json.NewDecoder(bytes.NewReader(content))
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil {
		return gosecReport{}, fmt.Errorf("decode JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return gosecReport{}, errors.New("report contains more than one JSON value")
		}
		return gosecReport{}, fmt.Errorf("decode trailing JSON: %w", err)
	}

	var report gosecReport
	if err := decodeRequiredGosecField(fields, "GosecVersion", &report.Version); err != nil {
		return gosecReport{}, err
	}
	if strings.TrimSpace(report.Version) == "" {
		return gosecReport{}, errors.New("GosecVersion must be a non-empty string")
	}
	if err := decodeRequiredGosecField(fields, "Golang errors", &report.ProcessingErrors); err != nil {
		return gosecReport{}, err
	}
	if report.ProcessingErrors == nil {
		return gosecReport{}, errors.New("Golang errors must be an object")
	}
	if err := decodeRequiredGosecField(fields, "Issues", &report.Issues); err != nil {
		return gosecReport{}, err
	}
	if report.Issues == nil {
		return gosecReport{}, errors.New("Issues must be an array")
	}
	if err := decodeRequiredGosecField(fields, "Stats", &report.Stats); err != nil {
		return gosecReport{}, err
	}
	return report, nil
}

func decodeRequiredGosecField(fields map[string]json.RawMessage, name string, destination any) error {
	raw, ok := fields[name]
	if !ok {
		return fmt.Errorf("missing required field %q", name)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("required field %q is null", name)
	}
	if err := json.Unmarshal(raw, destination); err != nil {
		return fmt.Errorf("decode field %q: %w", name, err)
	}
	return nil
}

func writeGosecSummary(writer io.Writer, report gosecReport, repositoryRoot string, packageCount int) error {
	if packageCount <= 0 {
		return errors.New("package coverage is empty or unproven")
	}
	if err := validateGosecStats(report); err != nil {
		return err
	}
	if len(report.ProcessingErrors) != 0 {
		return formatGosecProcessingErrors(report.ProcessingErrors)
	}

	issues := append([]gosecIssue(nil), report.Issues...)
	for index := range issues {
		if strings.TrimSpace(issues[index].RuleID) == "" ||
			strings.TrimSpace(issues[index].Details) == "" ||
			strings.TrimSpace(issues[index].File) == "" ||
			strings.TrimSpace(issues[index].Line) == "" ||
			strings.TrimSpace(issues[index].Column) == "" {
			return fmt.Errorf("issue %d is missing a required rule, location, or message field", index)
		}
		issues[index].File = displayGosecPath(repositoryRoot, issues[index].File)
	}
	sort.Slice(issues, func(first, second int) bool {
		left := issues[first]
		right := issues[second]
		if left.RuleID != right.RuleID {
			return left.RuleID < right.RuleID
		}
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		return left.Details < right.Details
	})

	counts := make(map[string]int)
	for _, issue := range issues {
		counts[issue.RuleID]++
	}
	rules := make([]string, 0, len(counts))
	for rule := range counts {
		rules = append(rules, rule)
	}
	sort.Strings(rules)

	fmt.Fprintf(writer, "gosec: version=%s packages=%d files=%d lines=%d findings=%d\n",
		report.Version, packageCount, *report.Stats.Files, *report.Stats.Lines, len(issues))
	for _, rule := range rules {
		fmt.Fprintf(writer, "gosec: %s=%d\n", rule, counts[rule])
	}
	for _, issue := range issues {
		fmt.Fprintf(writer, "gosec: advisory %s %s (line %s, column %s): %s\n",
			issue.RuleID, issue.File, issue.Line, issue.Column, issue.Details)
	}
	if len(issues) == 0 {
		fmt.Fprintln(writer, "gosec: no findings")
	} else {
		fmt.Fprintln(writer, "gosec: findings are advisory; analyzer health and package coverage passed")
	}
	return nil
}

func validateGosecStats(report gosecReport) error {
	stats := report.Stats
	if stats.Files == nil || stats.Lines == nil || stats.NoSec == nil || stats.Found == nil {
		return errors.New("Stats is missing files, lines, nosec, or found")
	}
	if *stats.Files <= 0 {
		return errors.New("report proves no analyzed files")
	}
	if *stats.Lines < 0 || *stats.NoSec < 0 || *stats.Found < 0 {
		return errors.New("Stats contains a negative value")
	}
	if *stats.NoSec != 0 {
		return fmt.Errorf("report contains %d source suppressions", *stats.NoSec)
	}
	if *stats.Found != len(report.Issues) {
		return fmt.Errorf("Stats found=%d does not match Issues count=%d", *stats.Found, len(report.Issues))
	}
	return nil
}

func formatGosecProcessingErrors(processingErrors map[string][]gosecProcessingError) error {
	paths := make([]string, 0, len(processingErrors))
	for path := range processingErrors {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	messages := make([]string, 0)
	for _, path := range paths {
		errorsForPath := processingErrors[path]
		if len(errorsForPath) == 0 {
			messages = append(messages, fmt.Sprintf("%s: unspecified processing error", path))
			continue
		}
		for _, processingError := range errorsForPath {
			message := strings.TrimSpace(processingError.Error)
			if message == "" {
				message = "unspecified processing error"
			}
			messages = append(messages, fmt.Sprintf("%s:%d:%d: %s", path, processingError.Line, processingError.Column, message))
		}
	}
	return fmt.Errorf("Go/package processing errors: %s", strings.Join(messages, "; "))
}

func displayGosecPath(repositoryRoot, path string) string {
	relative, err := filepath.Rel(repositoryRoot, path)
	if err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(relative)
	}
	return filepath.ToSlash(path)
}
