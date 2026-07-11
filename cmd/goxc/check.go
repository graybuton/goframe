package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/graybuton/goframe/pkg/gox"
)

const checkSchemaVersion = 1

var errCheckDiagnostics = errors.New("GOX diagnostics found")

type checkFormat string

const (
	checkFormatText checkFormat = "text"
	checkFormatJSON checkFormat = "json"
)

type checkOptions struct {
	path   string
	format checkFormat
}

type checkDiagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Source   string `json:"source"`
}

type checkReport struct {
	SchemaVersion int               `json:"schemaVersion"`
	OK            bool              `json:"ok"`
	FilesChecked  int               `json:"filesChecked"`
	Diagnostics   []checkDiagnostic `json:"diagnostics"`
}

type checkSource struct {
	path    string
	content []byte
}

func checkCommand(args []string) error {
	return runCheckCommand(args, os.Stdout, os.Stderr)
}

func runCheckCommand(args []string, stdout, stderr io.Writer) error {
	options, err := parseCheckOptions(args)
	if err != nil {
		return err
	}
	report, err := checkPath(options.path)
	if err != nil {
		return err
	}

	switch options.format {
	case checkFormatJSON:
		err = writeCheckJSON(stdout, report)
	case checkFormatText:
		err = writeCheckText(stdout, stderr, report)
	default:
		return fmt.Errorf("unsupported check format %q", options.format)
	}
	if err != nil {
		return fmt.Errorf("write check report: %w", err)
	}
	if !report.OK {
		return errCheckDiagnostics
	}
	return nil
}

func parseCheckOptions(args []string) (checkOptions, error) {
	options := checkOptions{format: checkFormatText}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case strings.HasPrefix(arg, "--format="):
			format, err := parseCheckFormat(strings.TrimPrefix(arg, "--format="))
			if err != nil {
				return checkOptions{}, err
			}
			options.format = format
		case arg == "--format":
			index++
			if index >= len(args) {
				return checkOptions{}, errors.New("--format requires a value")
			}
			format, err := parseCheckFormat(args[index])
			if err != nil {
				return checkOptions{}, err
			}
			options.format = format
		case strings.HasPrefix(arg, "-"):
			return checkOptions{}, fmt.Errorf("unknown check flag %q", arg)
		case options.path == "":
			options.path = arg
		default:
			return checkOptions{}, fmt.Errorf("unexpected check argument %q", arg)
		}
	}
	if options.path == "" {
		return checkOptions{}, errors.New("usage: goxc check <file-or-directory> [--format=text|json]")
	}
	return options, nil
}

func parseCheckFormat(value string) (checkFormat, error) {
	switch checkFormat(value) {
	case checkFormatText:
		return checkFormatText, nil
	case checkFormatJSON:
		return checkFormatJSON, nil
	default:
		return "", fmt.Errorf("unsupported check format %q; expected text or json", value)
	}
}

func checkPath(path string) (checkReport, error) {
	files, err := findGOXFiles(path)
	if err != nil {
		return checkReport{}, err
	}
	if len(files) == 0 {
		return checkReport{}, fmt.Errorf("no .gox files found below %s", path)
	}
	appDir, err := generationAppDir(path)
	if err != nil {
		return checkReport{}, err
	}
	appDir, err = filepath.Abs(appDir)
	if err != nil {
		return checkReport{}, fmt.Errorf("resolve GOX application directory %s: %w", appDir, err)
	}

	sources := make([]checkSource, 0, len(files))
	for _, file := range files {
		absolute, err := filepath.Abs(file)
		if err != nil {
			return checkReport{}, fmt.Errorf("resolve GOX source %s: %w", file, err)
		}
		absolute = filepath.Clean(absolute)
		info, err := regularFileNoFollow(absolute, "GOX source file")
		if err != nil {
			return checkReport{}, err
		}
		if info.Size() < 0 {
			return checkReport{}, fmt.Errorf("inspect %s: invalid source size", absolute)
		}
		content, err := os.ReadFile(absolute)
		if err != nil {
			return checkReport{}, fmt.Errorf("read %s: %w", absolute, err)
		}
		sources = append(sources, checkSource{path: absolute, content: content})
	}
	sort.Slice(sources, func(left, right int) bool {
		return sources[left].path < sources[right].path
	})

	diagnostics := make([]checkDiagnostic, 0)
	for _, source := range sources {
		_, err := gox.GenerateWithOptions(source.content, gox.GenerateOptions{
			Filename:        source.path,
			PackageIdentity: packageIdentityForFile(appDir, source.path),
		})
		if err != nil {
			diagnostics = append(diagnostics, checkDiagnosticFromError(source.path, err))
		}
	}
	sort.Slice(diagnostics, func(left, right int) bool {
		first := diagnostics[left]
		second := diagnostics[right]
		if first.File != second.File {
			return first.File < second.File
		}
		if first.Line != second.Line {
			return first.Line < second.Line
		}
		if first.Column != second.Column {
			return first.Column < second.Column
		}
		return first.Message < second.Message
	})

	return checkReport{
		SchemaVersion: checkSchemaVersion,
		OK:            len(diagnostics) == 0,
		FilesChecked:  len(sources),
		Diagnostics:   diagnostics,
	}, nil
}

func checkDiagnosticFromError(filename string, err error) checkDiagnostic {
	var diagnosticError gox.DiagnosticError
	if errors.As(err, &diagnosticError) {
		diagnostic := diagnosticError.Diagnostic
		return checkDiagnostic{
			File:     filepath.Clean(diagnostic.Filename),
			Line:     diagnostic.Line,
			Column:   diagnostic.Column,
			Severity: "error",
			Message:  diagnostic.Message,
			Source:   diagnostic.Source,
		}
	}

	message := err.Error()
	prefix := filename + ":"
	if strings.HasPrefix(message, prefix) {
		message = strings.TrimSpace(strings.TrimPrefix(message, prefix))
	}
	return checkDiagnostic{
		File:     filename,
		Severity: "error",
		Message:  message,
		Source:   "",
	}
}

func writeCheckText(stdout, stderr io.Writer, report checkReport) error {
	if report.OK {
		_, err := fmt.Fprintf(stdout, "checked %d GOX files: no diagnostics\n", report.FilesChecked)
		return err
	}
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Line > 0 && diagnostic.Column > 0 {
			if _, err := fmt.Fprintf(stderr, "%s:%d:%d: %s\n", diagnostic.File, diagnostic.Line, diagnostic.Column, diagnostic.Message); err != nil {
				return err
			}
		} else if _, err := fmt.Fprintf(stderr, "%s: %s\n", diagnostic.File, diagnostic.Message); err != nil {
			return err
		}
		if diagnostic.Source != "" {
			if _, err := fmt.Fprintf(stderr, "  %s\n", diagnostic.Source); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintf(stderr, "checked %d GOX files: %s\n", report.FilesChecked, checkDiagnosticCount(len(report.Diagnostics)))
	return err
}

func checkDiagnosticCount(count int) string {
	if count == 1 {
		return "1 diagnostic"
	}
	return fmt.Sprintf("%d diagnostics", count)
}

func writeCheckJSON(output io.Writer, report checkReport) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(report)
}
