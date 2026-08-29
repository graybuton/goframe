package main

import (
	"unicode/utf16"
	"unicode/utf8"
)

type decodedJavaScriptString struct {
	units      []sourceByte
	valueStart int
	valueEnd   int
	end        int
	quote      byte
}

func decodeJavaScriptString(content string, start, end int) (decodedJavaScriptString, bool) {
	if start >= end || content[start] != '\'' && content[start] != '"' {
		return decodedJavaScriptString{}, false
	}
	quote := content[start]
	valueStart := start + 1
	units := make([]sourceByte, 0, 32)

	for offset := valueStart; offset < end; {
		if content[offset] == quote {
			return decodedJavaScriptString{
				units:      units,
				valueStart: valueStart,
				valueEnd:   offset,
				end:        offset + 1,
				quote:      quote,
			}, true
		}
		if ecmascriptLineTerminatorLength(content, offset, end) != 0 {
			return decodedJavaScriptString{}, false
		}
		if content[offset] != '\\' {
			value, size := utf8.DecodeRuneInString(content[offset:end])
			if value == utf8.RuneError && size == 1 {
				return decodedJavaScriptString{}, false
			}
			units = appendSourceMappedRune(units, value, offset-valueStart, offset+size-valueStart)
			offset += size
			continue
		}

		escapeStart := offset
		offset++
		if offset >= end {
			return decodedJavaScriptString{}, false
		}
		if length := ecmascriptLineTerminatorLength(content, offset, end); length != 0 {
			offset += length
			continue
		}

		var value rune
		switch content[offset] {
		case '\'', '"', '\\':
			value = rune(content[offset])
			offset++
		case 'b':
			value = '\b'
			offset++
		case 'f':
			value = '\f'
			offset++
		case 'n':
			value = '\n'
			offset++
		case 'r':
			value = '\r'
			offset++
		case 't':
			value = '\t'
			offset++
		case 'v':
			value = '\v'
			offset++
		case '0':
			if offset+1 < end && isASCIIDecimalDigit(content[offset+1]) {
				return decodedJavaScriptString{}, false
			}
			value = 0
			offset++
		case '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return decodedJavaScriptString{}, false
		case 'x':
			decoded, next, ok := decodeFixedECMAScriptHex(content, offset+1, end, 2)
			if !ok {
				return decodedJavaScriptString{}, false
			}
			value = rune(decoded)
			offset = next
		case 'u':
			decoded, next, ok := decodeECMAScriptUnicodeEscape(content, offset+1, end)
			if !ok {
				return decodedJavaScriptString{}, false
			}
			value = decoded
			offset = next
		default:
			decoded, size := utf8.DecodeRuneInString(content[offset:end])
			if decoded == utf8.RuneError && size == 1 {
				return decodedJavaScriptString{}, false
			}
			value = decoded
			offset += size
		}
		units = appendSourceMappedRune(units, value, escapeStart-valueStart, offset-valueStart)
	}
	return decodedJavaScriptString{}, false
}

func decodeECMAScriptUnicodeEscape(content string, digitsStart, end int) (rune, int, bool) {
	if digitsStart < end && content[digitsStart] == '{' {
		offset := digitsStart + 1
		digits := 0
		value := uint32(0)
		for offset < end && digits < 6 {
			digit, ok := ecmascriptHexDigit(content[offset])
			if !ok {
				break
			}
			value = value*16 + uint32(digit)
			digits++
			offset++
		}
		if digits == 0 || offset >= end || content[offset] != '}' || value > uint32(utf8.MaxRune) || value >= 0xD800 && value <= 0xDFFF {
			return 0, 0, false
		}
		return rune(value), offset + 1, true
	}

	first, next, ok := decodeFixedECMAScriptHex(content, digitsStart, end, 4)
	if !ok {
		return 0, 0, false
	}
	if first >= 0xDC00 && first <= 0xDFFF {
		return 0, 0, false
	}
	if first < 0xD800 || first > 0xDBFF {
		return rune(first), next, true
	}
	if next+6 > end || content[next] != '\\' || content[next+1] != 'u' {
		return 0, 0, false
	}
	second, pairEnd, ok := decodeFixedECMAScriptHex(content, next+2, end, 4)
	if !ok || second < 0xDC00 || second > 0xDFFF {
		return 0, 0, false
	}
	return utf16.DecodeRune(rune(first), rune(second)), pairEnd, true
}

func decodeFixedECMAScriptHex(content string, start, end, length int) (uint32, int, bool) {
	if start < 0 || length < 0 || start+length > end {
		return 0, 0, false
	}
	value := uint32(0)
	for offset := start; offset < start+length; offset++ {
		digit, ok := ecmascriptHexDigit(content[offset])
		if !ok {
			return 0, 0, false
		}
		value = value*16 + uint32(digit)
	}
	return value, start + length, true
}

func ecmascriptHexDigit(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
	}
}

func appendSourceMappedRune(units []sourceByte, value rune, start, end int) []sourceByte {
	var encoded [utf8.UTFMax]byte
	length := utf8.EncodeRune(encoded[:], value)
	for _, current := range encoded[:length] {
		units = append(units, sourceByte{value: current, start: start, end: end})
	}
	return units
}

func isASCIIDecimalDigit(value byte) bool {
	return value >= '0' && value <= '9'
}
