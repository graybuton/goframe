package gox

import (
	"strings"
	"unicode"
)

type componentTag struct {
	raw       string
	propsType string
	render    string
	id        string
	debugName string
	varPart   string
}

func splitQualifiedTag(tag string) (string, string, bool) {
	if strings.Count(tag, ".") != 1 {
		return "", "", false
	}
	parts := strings.SplitN(tag, ".", 2)
	if parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func isExportedIdentifier(value string) bool {
	if value == "" {
		return false
	}
	return unicode.IsUpper([]rune(value)[0])
}
