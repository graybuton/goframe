package main

import (
	"fmt"
	"strings"
)

const upperHexDigits = "0123456789ABCDEF"

// encodePackagePathAsBrowserURL serializes one canonical package path as a
// relative URL. Package separators remain slashes; every other segment byte
// outside the unreserved ASCII set is percent-encoded.
func encodePackagePathAsBrowserURL(value string) (string, error) {
	if strings.IndexByte(value, 0) >= 0 {
		return "", fmt.Errorf("generated package path contains a NUL byte")
	}
	if err := validateGeneratedPackagePath(value, "generated package path"); err != nil {
		return "", err
	}

	var encoded strings.Builder
	encoded.Grow(len(value))
	for offset := 0; offset < len(value); offset++ {
		current := value[offset]
		if current == '/' || isBrowserURLUnreserved(current) {
			encoded.WriteByte(current)
			continue
		}
		encoded.WriteByte('%')
		encoded.WriteByte(upperHexDigits[current>>4])
		encoded.WriteByte(upperHexDigits[current&0x0f])
	}
	return encoded.String(), nil
}

func isBrowserURLUnreserved(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' ||
		value >= '0' && value <= '9' || value == '-' || value == '.' ||
		value == '_' || value == '~'
}

func decodeLegacyURLPathOnce(value string) (string, bool) {
	if strings.IndexByte(value, '%') < 0 {
		return value, strings.IndexByte(value, 0) < 0
	}

	var decoded strings.Builder
	decoded.Grow(len(value))
	for offset := 0; offset < len(value); {
		if value[offset] != '%' {
			if value[offset] == 0 {
				return "", false
			}
			decoded.WriteByte(value[offset])
			offset++
			continue
		}
		if offset+2 >= len(value) {
			return "", false
		}
		high, ok := asciiHexValue(value[offset+1])
		if !ok {
			return "", false
		}
		low, ok := asciiHexValue(value[offset+2])
		if !ok {
			return "", false
		}
		current := high<<4 | low
		if current == 0 || current == '/' || current == '\\' {
			return "", false
		}
		decoded.WriteByte(current)
		offset += 3
	}
	return decoded.String(), true
}

func asciiHexValue(value byte) (byte, bool) {
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
