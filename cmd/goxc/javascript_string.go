package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func encodeGeneratedJavaScriptString(value string) (string, error) {
	contents, err := encodeGeneratedJavaScriptStringContents(value, '"')
	if err != nil {
		return "", err
	}
	return `"` + contents + `"`, nil
}

func encodeGeneratedJavaScriptStringContents(value string, quote byte) (string, error) {
	if quote != '\'' && quote != '"' {
		return "", fmt.Errorf("generated package URL has an invalid JavaScript string context")
	}
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("generated package URL is not valid UTF-8")
	}

	var encoded strings.Builder
	encoded.Grow(len(value))
	for _, current := range value {
		switch current {
		case '\\':
			encoded.WriteString(`\\`)
		case '\b':
			encoded.WriteString(`\b`)
		case '\t':
			encoded.WriteString(`\t`)
		case '\n':
			encoded.WriteString(`\n`)
		case '\v':
			encoded.WriteString(`\v`)
		case '\f':
			encoded.WriteString(`\f`)
		case '\r':
			encoded.WriteString(`\r`)
		case '<':
			encoded.WriteString(`\u003C`)
		case '\'', '"':
			if byte(current) == quote {
				encoded.WriteByte('\\')
			}
			encoded.WriteRune(current)
		default:
			switch {
			case current <= 0x1f || current >= 0x7f && current <= 0xff:
				writeJavaScriptHexByte(&encoded, byte(current))
			case current <= 0x7e:
				encoded.WriteRune(current)
			case current <= 0xffff:
				writeJavaScriptUnicodeUnit(&encoded, uint16(current))
			default:
				value := uint32(current) - 0x10000
				writeJavaScriptUnicodeUnit(&encoded, uint16(0xd800+(value>>10)))
				writeJavaScriptUnicodeUnit(&encoded, uint16(0xdc00+(value&0x3ff)))
			}
		}
	}
	return encoded.String(), nil
}

func writeJavaScriptHexByte(builder *strings.Builder, value byte) {
	builder.WriteString(`\x`)
	builder.WriteByte(upperHexDigits[value>>4])
	builder.WriteByte(upperHexDigits[value&0x0f])
}

func writeJavaScriptUnicodeUnit(builder *strings.Builder, value uint16) {
	builder.WriteString(`\u`)
	builder.WriteByte(upperHexDigits[value>>12])
	builder.WriteByte(upperHexDigits[value>>8&0x0f])
	builder.WriteByte(upperHexDigits[value>>4&0x0f])
	builder.WriteByte(upperHexDigits[value&0x0f])
}
