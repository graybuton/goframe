package gox

import (
	"errors"
	"fmt"
)

// Diagnostic describes one source-oriented GOX compiler diagnostic.
type Diagnostic struct {
	Filename string
	Line     int
	Column   int
	Message  string
	Source   string
}

// DiagnosticError is returned when GOX can point an error back to source.
type DiagnosticError struct {
	Diagnostic Diagnostic
}

func (err DiagnosticError) Error() string {
	diagnostic := err.Diagnostic
	if diagnostic.Source == "" {
		return fmt.Sprintf("%s:%d:%d: %s", diagnostic.Filename, diagnostic.Line, diagnostic.Column, diagnostic.Message)
	}
	return fmt.Sprintf("%s:%d:%d: %s\n  %s", diagnostic.Filename, diagnostic.Line, diagnostic.Column, diagnostic.Message, diagnostic.Source)
}

func diagnosticError(filename string, line, column int, message, snippet string) error {
	return DiagnosticError{
		Diagnostic: Diagnostic{
			Filename: filename,
			Line:     line,
			Column:   column,
			Message:  message,
			Source:   snippet,
		},
	}
}

func asDiagnosticError(err error) (DiagnosticError, bool) {
	var diagnostic DiagnosticError
	if errors.As(err, &diagnostic) {
		return diagnostic, true
	}
	return DiagnosticError{}, false
}

func diagnosticMessage(err error) string {
	if diagnostic, ok := asDiagnosticError(err); ok {
		return diagnostic.Diagnostic.Message
	}
	return err.Error()
}
