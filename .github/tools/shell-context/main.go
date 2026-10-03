package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

type commandEvent struct {
	line          uint
	pipeInput     bool
	redirectInput bool
	assigns       int
	words         []string
	command       string
}

func sourceText(src []byte, node syntax.Node) (string, error) {
	start, end := int(node.Pos().Offset()), int(node.End().Offset())
	if start < 0 || end < start || end > len(src) {
		return "", fmt.Errorf("invalid shell source span %d:%d", start, end)
	}
	return string(src[start:end]), nil
}

func policyPartsText(src []byte, start, end int, parts []syntax.WordPart) (string, error) {
	if start < 0 || end < start || end > len(src) {
		return "", fmt.Errorf("invalid shell word span %d:%d", start, end)
	}
	cursor := start
	var text strings.Builder
	var literal strings.Builder
	// Bash removes continued newlines from literal syntax, but not single-quoted data.
	flushLiteral := func() {
		text.WriteString(strings.ReplaceAll(literal.String(), "\\\n", ""))
		literal.Reset()
	}
	for _, part := range parts {
		partStart, partEnd := int(part.Pos().Offset()), int(part.End().Offset())
		if partStart < cursor || partEnd < partStart || partEnd > end {
			return "", fmt.Errorf("invalid shell word part span %d:%d", partStart, partEnd)
		}
		literal.Write(src[cursor:partStart])
		raw := src[partStart:partEnd]
		switch node := part.(type) {
		case *syntax.Lit:
			literal.Write(raw)
		case *syntax.SglQuoted:
			flushLiteral()
			if node.Dollar && !strings.Contains(node.Value, "\\") {
				if !bytes.HasPrefix(raw, []byte("$'")) {
					return "", fmt.Errorf("invalid ANSI-C quoted word span %d:%d", partStart, partEnd)
				}
				text.Write(raw[1:])
			} else {
				text.Write(raw)
			}
		case *syntax.DblQuoted:
			flushLiteral()
			if node.Dollar {
				text.Write(raw)
				break
			}
			if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
				return "", fmt.Errorf("invalid double-quoted word span %d:%d", partStart, partEnd)
			}
			inner, err := policyPartsText(src, partStart+1, partEnd-1, node.Parts)
			if err != nil {
				return "", err
			}
			text.WriteByte('"')
			text.WriteString(inner)
			text.WriteByte('"')
		default:
			flushLiteral()
			text.Write(raw)
		}
		cursor = partEnd
	}
	literal.Write(src[cursor:end])
	flushLiteral()
	return text.String(), nil
}

func policyWordText(src []byte, word *syntax.Word) (string, error) {
	return policyPartsText(src, int(word.Pos().Offset()), int(word.End().Offset()), word.Parts)
}

func directCommandText(src []byte, call *syntax.CallExpr) (string, error) {
	start, end := int(call.Pos().Offset()), int(call.End().Offset())
	if start < 0 || end < start || end > len(src) {
		return "", fmt.Errorf("invalid shell command source span %d:%d", start, end)
	}
	type span struct{ start, end int }
	var substitutions []span
	syntax.Walk(call, func(node syntax.Node) bool {
		switch node.(type) {
		case *syntax.CmdSubst, *syntax.ProcSubst:
			substitutions = append(substitutions, span{int(node.Pos().Offset()), int(node.End().Offset())})
			return false
		}
		return true
	})
	sort.Slice(substitutions, func(i, j int) bool { return substitutions[i].start < substitutions[j].start })
	var text bytes.Buffer
	for _, substitution := range substitutions {
		if substitution.start < start || substitution.end < substitution.start || substitution.end > end {
			return "", fmt.Errorf("invalid nested shell source span %d:%d", substitution.start, substitution.end)
		}
		text.Write(src[start:substitution.start])
		text.WriteString("$SUBSTITUTION")
		start = substitution.end
	}
	text.Write(src[start:end])
	return text.String(), nil
}

func analyze(src []byte, name string) ([]commandEvent, error) {
	if bytes.IndexByte(src, 0) >= 0 {
		return nil, fmt.Errorf("%s: NUL byte in shell source", name)
	}
	file, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(bytes.NewReader(src), name)
	if err != nil {
		return nil, err
	}

	piped := make(map[*syntax.CallExpr]bool)
	redirected := make(map[*syntax.CallExpr]bool)
	syntax.Walk(file, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.BinaryCmd:
			if n.Op == syntax.Pipe || n.Op == syntax.PipeAll {
				syntax.Walk(n.Y, func(child syntax.Node) bool {
					if call, ok := child.(*syntax.CallExpr); ok {
						piped[call] = true
					}
					return true
				})
			}
		case *syntax.Stmt:
			if call, ok := n.Cmd.(*syntax.CallExpr); ok {
				for _, redir := range n.Redirs {
					switch redir.Op {
					case syntax.RdrIn, syntax.RdrInOut, syntax.DplIn,
						syntax.Hdoc, syntax.DashHdoc, syntax.WordHdoc:
						redirected[call] = true
					}
				}
			}
		}
		return true
	})

	var events []commandEvent
	var spanErr error
	syntax.Walk(file, func(node syntax.Node) bool {
		if spanErr != nil {
			return false
		}
		call, ok := node.(*syntax.CallExpr)
		if !ok || (len(call.Args) == 0 && len(call.Assigns) == 0) {
			return true
		}
		event := commandEvent{
			line:          call.Pos().Line(),
			pipeInput:     piped[call],
			redirectInput: redirected[call],
			assigns:       len(call.Assigns),
		}
		for _, assign := range call.Assigns {
			word, err := sourceText(src, assign)
			if err != nil {
				spanErr = err
				return false
			}
			event.words = append(event.words, word)
		}
		for _, arg := range call.Args {
			word, err := policyWordText(src, arg)
			if err != nil {
				spanErr = err
				return false
			}
			event.words = append(event.words, word)
		}
		event.command, spanErr = directCommandText(src, call)
		events = append(events, event)
		return true
	})
	return events, spanErr
}

func writeEvents(w io.Writer, events []commandEvent) error {
	field := func(value string) error {
		_, err := io.WriteString(w, value+"\x00")
		return err
	}
	for _, event := range events {
		values := []string{"C", strconv.FormatUint(uint64(event.line), 10),
			strconv.FormatBool(event.pipeInput), strconv.FormatBool(event.redirectInput),
			strconv.Itoa(event.assigns), strconv.Itoa(len(event.words))}
		values = append(values, event.words...)
		values = append(values, event.command)
		for _, value := range values {
			if err := field(value); err != nil {
				return err
			}
		}
	}
	if err := field("E"); err != nil {
		return err
	}
	return field(strconv.Itoa(len(events)))
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: shell-context <repository-source-name>")
		os.Exit(2)
	}
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: read shell source: %v\n", os.Args[1], err)
		os.Exit(1)
	}
	events, err := analyze(src, os.Args[1])
	if err == nil {
		err = writeEvents(os.Stdout, events)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: inspect shell source: %v\n", os.Args[1], err)
		os.Exit(1)
	}
}
