package main

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type legacyBootstrapTrivia struct {
	sawLineTerminator bool
}

func (parser *legacyBootstrapParser) skipTrivia() (legacyBootstrapTrivia, bool) {
	var trivia legacyBootstrapTrivia
	for parser.offset < parser.end {
		if strings.HasPrefix(parser.content[parser.offset:parser.end], "//") {
			parser.offset += 2
			for parser.offset < parser.end {
				if ecmascriptLineTerminatorLength(parser.content, parser.offset, parser.end) != 0 {
					break
				}
				_, size := utf8.DecodeRuneInString(parser.content[parser.offset:parser.end])
				if size == 1 && parser.content[parser.offset] >= utf8.RuneSelf {
					return legacyBootstrapTrivia{}, false
				}
				parser.offset += size
			}
			continue
		}
		if strings.HasPrefix(parser.content[parser.offset:parser.end], "/*") {
			parser.offset += 2
			closed := false
			for parser.offset < parser.end {
				if strings.HasPrefix(parser.content[parser.offset:parser.end], "*/") {
					parser.offset += 2
					closed = true
					break
				}
				if length := ecmascriptLineTerminatorLength(parser.content, parser.offset, parser.end); length != 0 {
					trivia.sawLineTerminator = true
					parser.offset += length
					continue
				}
				_, size := utf8.DecodeRuneInString(parser.content[parser.offset:parser.end])
				if size == 1 && parser.content[parser.offset] >= utf8.RuneSelf {
					return legacyBootstrapTrivia{}, false
				}
				parser.offset += size
			}
			if !closed {
				return legacyBootstrapTrivia{}, false
			}
			continue
		}

		if length := ecmascriptLineTerminatorLength(parser.content, parser.offset, parser.end); length != 0 {
			trivia.sawLineTerminator = true
			parser.offset += length
			continue
		}
		current, size := utf8.DecodeRuneInString(parser.content[parser.offset:parser.end])
		if current == utf8.RuneError && size == 1 {
			return legacyBootstrapTrivia{}, false
		}
		if !isECMAScriptWhiteSpace(current) {
			return trivia, true
		}
		parser.offset += size
	}
	return trivia, true
}

func isECMAScriptWhiteSpace(value rune) bool {
	switch value {
	case '\t', '\v', '\f', '\uFEFF':
		return true
	default:
		return unicode.Is(unicode.Zs, value)
	}
}

func ecmascriptLineTerminatorLength(content string, offset, end int) int {
	if offset >= end {
		return 0
	}
	switch content[offset] {
	case '\n':
		return 1
	case '\r':
		if offset+1 < end && content[offset+1] == '\n' {
			return 2
		}
		return 1
	}
	if content[offset] < utf8.RuneSelf {
		return 0
	}
	value, size := utf8.DecodeRuneInString(content[offset:end])
	if value == '\u2028' || value == '\u2029' {
		return size
	}
	return 0
}
