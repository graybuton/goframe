package gox

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenOpenTag
	tokenCloseTag
	tokenTagEnd
	tokenSelfClose
	tokenIdentifier
	tokenEquals
	tokenString
	tokenText
	tokenExpression
)

type token struct {
	kind   tokenKind
	value  string
	offset int
}

type lexerMode int

const (
	lexerText lexerMode = iota
	lexerTag
)

// Lexer tokenizes the markup portion of a GOX file.
type Lexer struct {
	input        string
	pos          int
	mode         lexerMode
	filename     string
	lineOffset   int
	columnOffset int
}

// NewLexer creates a GOX markup lexer.
func NewLexer(input string) *Lexer {
	return newLexerAt(input, "gox", 1, 1)
}

func newLexerAt(input, filename string, line, column int) *Lexer {
	return &Lexer{
		input:        input,
		filename:     filename,
		lineOffset:   line,
		columnOffset: column,
	}
}

func (lexer *Lexer) next() (token, error) {
	if lexer.mode == lexerTag {
		return lexer.nextTagToken()
	}
	return lexer.nextTextToken()
}

func (lexer *Lexer) nextTextToken() (token, error) {
	if lexer.pos >= len(lexer.input) {
		return token{kind: tokenEOF, offset: lexer.pos}, nil
	}

	start := lexer.pos
	switch {
	case strings.HasPrefix(lexer.input[lexer.pos:], "</"):
		lexer.pos += 2
		lexer.mode = lexerTag
		return token{kind: tokenCloseTag, offset: start}, nil
	case lexer.input[lexer.pos] == '<':
		lexer.pos++
		lexer.mode = lexerTag
		return token{kind: tokenOpenTag, offset: start}, nil
	case lexer.input[lexer.pos] == '{':
		value, err := lexer.readExpression()
		return token{kind: tokenExpression, value: value, offset: start}, err
	default:
		for lexer.pos < len(lexer.input) && lexer.input[lexer.pos] != '<' && lexer.input[lexer.pos] != '{' {
			lexer.pos++
		}
		return token{kind: tokenText, value: lexer.input[start:lexer.pos], offset: start}, nil
	}
}

func (lexer *Lexer) nextTagToken() (token, error) {
	for lexer.pos < len(lexer.input) && unicode.IsSpace(rune(lexer.input[lexer.pos])) {
		lexer.pos++
	}
	if lexer.pos >= len(lexer.input) {
		return token{kind: tokenEOF, offset: lexer.pos}, nil
	}

	start := lexer.pos
	switch {
	case strings.HasPrefix(lexer.input[lexer.pos:], "/>"):
		lexer.pos += 2
		lexer.mode = lexerText
		return token{kind: tokenSelfClose, offset: start}, nil
	case lexer.input[lexer.pos] == '>':
		lexer.pos++
		lexer.mode = lexerText
		return token{kind: tokenTagEnd, offset: start}, nil
	case lexer.input[lexer.pos] == '=':
		lexer.pos++
		return token{kind: tokenEquals, offset: start}, nil
	case lexer.input[lexer.pos] == '"' || lexer.input[lexer.pos] == '\'':
		value, err := lexer.readString()
		return token{kind: tokenString, value: value, offset: start}, err
	case lexer.input[lexer.pos] == '{':
		value, err := lexer.readExpression()
		return token{kind: tokenExpression, value: value, offset: start}, err
	case lexer.input[lexer.pos] == '.':
		lexer.pos++
		for lexer.pos < len(lexer.input) && isIdentifierPart(lexer.input[lexer.pos]) {
			lexer.pos++
		}
		return token{kind: tokenIdentifier, value: lexer.input[start:lexer.pos], offset: start}, nil
	case isIdentifierStart(lexer.input[lexer.pos]):
		lexer.pos++
		for lexer.pos < len(lexer.input) && isIdentifierPart(lexer.input[lexer.pos]) {
			lexer.pos++
		}
		return token{kind: tokenIdentifier, value: lexer.input[start:lexer.pos], offset: start}, nil
	default:
		return token{}, lexer.errorAt(start, "unexpected character %q in tag", lexer.input[lexer.pos])
	}
}

func (lexer *Lexer) readString() (string, error) {
	start := lexer.pos
	quote := lexer.input[lexer.pos]
	lexer.pos++

	for lexer.pos < len(lexer.input) {
		if lexer.input[lexer.pos] == '\\' {
			lexer.pos += 2
			continue
		}
		if lexer.input[lexer.pos] == quote {
			lexer.pos++
			raw := lexer.input[start:lexer.pos]
			if quote == '\'' {
				return raw[1 : len(raw)-1], nil
			}
			value, err := strconv.Unquote(raw)
			if err != nil {
				return "", lexer.errorAt(start, "invalid quoted attribute: %v", err)
			}
			return value, nil
		}
		lexer.pos++
	}

	return "", lexer.errorAt(start, "unterminated quoted attribute")
}

func (lexer *Lexer) readExpression() (string, error) {
	start := lexer.pos
	lexer.pos++
	depth := 1

	for lexer.pos < len(lexer.input) {
		switch {
		case lexer.input[lexer.pos] == '"' || lexer.input[lexer.pos] == '\'' || lexer.input[lexer.pos] == '`':
			if err := lexer.skipQuoted(lexer.input[lexer.pos]); err != nil {
				return "", err
			}
		case strings.HasPrefix(lexer.input[lexer.pos:], "//"):
			lexer.pos += 2
			for lexer.pos < len(lexer.input) && lexer.input[lexer.pos] != '\n' {
				lexer.pos++
			}
		case strings.HasPrefix(lexer.input[lexer.pos:], "/*"):
			end := strings.Index(lexer.input[lexer.pos+2:], "*/")
			if end < 0 {
				return "", lexer.errorAt(lexer.pos, "unterminated block comment in expression")
			}
			lexer.pos += end + 4
		case lexer.input[lexer.pos] == '{':
			depth++
			lexer.pos++
		case lexer.input[lexer.pos] == '}':
			depth--
			lexer.pos++
			if depth == 0 {
				return strings.TrimSpace(lexer.input[start+1 : lexer.pos-1]), nil
			}
		default:
			lexer.pos++
		}
	}

	return "", lexer.errorAt(start, "unterminated Go expression")
}

func (lexer *Lexer) skipQuoted(quote byte) error {
	start := lexer.pos
	lexer.pos++
	for lexer.pos < len(lexer.input) {
		if quote != '`' && lexer.input[lexer.pos] == '\\' {
			lexer.pos += 2
			continue
		}
		if lexer.input[lexer.pos] == quote {
			lexer.pos++
			return nil
		}
		lexer.pos++
	}
	return lexer.errorAt(start, "unterminated string in expression")
}

func (lexer *Lexer) errorAt(offset int, format string, args ...any) error {
	line, column := lineColumn(lexer.input, offset)
	if line == 1 {
		column += lexer.columnOffset - 1
	}
	line += lexer.lineOffset - 1
	return diagnosticError(lexer.filename, line, column, fmt.Sprintf(format, args...), sourceLine(lexer.input, offset))
}

func lineColumn(input string, offset int) (int, int) {
	line := 1
	column := 1
	for index := 0; index < offset && index < len(input); index++ {
		if input[index] == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	return line, column
}

func isIdentifierStart(value byte) bool {
	return value == '_' || unicode.IsLetter(rune(value))
}

func isIdentifierPart(value byte) bool {
	return isIdentifierStart(value) || unicode.IsDigit(rune(value)) || value == '-' || value == ':' || value == '.'
}

func sourceLine(input string, offset int) string {
	start := strings.LastIndex(input[:min(offset, len(input))], "\n") + 1
	end := strings.Index(input[min(offset, len(input)):], "\n")
	if end < 0 {
		end = len(input)
	} else {
		end += min(offset, len(input))
	}
	return strings.TrimSpace(input[start:end])
}
