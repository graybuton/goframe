package main

import (
	"fmt"
	stdhtml "html"
	"sort"
	"strings"
)

//go:generate go run html_entities_generate.go

type htmlReplacement struct {
	start       int
	end         int
	value       string
	description string
}

type htmlRewritePlan struct {
	content      string
	replacements []htmlReplacement
}

func (plan *htmlRewritePlan) add(replacement htmlReplacement) {
	plan.replacements = append(plan.replacements, replacement)
}

func (plan htmlRewritePlan) apply() (string, error) {
	replacements := append([]htmlReplacement(nil), plan.replacements...)
	sort.Slice(replacements, func(first, second int) bool {
		if replacements[first].start != replacements[second].start {
			return replacements[first].start < replacements[second].start
		}
		return replacements[first].end < replacements[second].end
	})
	for index, replacement := range replacements {
		if replacement.start < 0 || replacement.end < replacement.start || replacement.end > len(plan.content) {
			return "", fmt.Errorf("custom index rewrite produced an invalid %s span", replacement.description)
		}
		if index == 0 {
			continue
		}
		previous := replacements[index-1]
		if replacement.start < previous.end || (replacement.start == previous.start && replacement.end == previous.end) {
			return "", fmt.Errorf(
				"custom index rewrite spans overlap between %s and %s; use explicit non-overlapping GoFrame blocks",
				previous.description,
				replacement.description,
			)
		}
	}

	result := plan.content
	for index := len(replacements) - 1; index >= 0; index-- {
		replacement := replacements[index]
		result = result[:replacement.start] + replacement.value + result[replacement.end:]
	}
	return result, nil
}

type scannedHTML struct {
	comments []htmlComment
	tags     []htmlTag
	profile  htmlRewriteProfile
}

type htmlComment struct {
	start                int
	end                  int
	eof                  bool
	unsupportedEnclosing string
}

type htmlTag struct {
	start                 int
	end                   int
	name                  string
	namespace             htmlNamespace
	closing               bool
	selfClosing           bool
	attributes            []htmlAttribute
	templateDepth         int
	ordinaryTemplateDepth int
	declarativeShadowMode string
	rawStart              int
	rawEnd                int
}

type htmlAttribute struct {
	name        string
	valueStart  int
	valueEnd    int
	valueSyntax htmlAttributeValueSyntax
	hasValue    bool
}

type htmlAttributeValueSyntax uint8

const (
	htmlAttributeValueNone htmlAttributeValueSyntax = iota
	htmlAttributeValueUnquoted
	htmlAttributeValueSingleQuoted
	htmlAttributeValueDoubleQuoted
)

type htmlNamespace uint8

const (
	htmlNamespaceHTML htmlNamespace = iota
	htmlNamespaceSVG
	htmlNamespaceMathML
)

type htmlElementContext struct {
	name                    string
	namespace               htmlNamespace
	htmlIntegrationPoint    bool
	mathMLTextIntegrationPt bool
}

type htmlScannerContext struct {
	elements []htmlElementContext
}

func (tag htmlTag) attribute(name string) (*htmlAttribute, error) {
	var found *htmlAttribute
	for index := range tag.attributes {
		attribute := &tag.attributes[index]
		if attribute.name != name {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("custom index <%s> has duplicate %s attributes; keep one unambiguous attribute", tag.name, name)
		}
		found = attribute
	}
	return found, nil
}

func scanCustomIndexHTML(content string) (scannedHTML, error) {
	var document scannedHTML
	var context htmlScannerContext
	templateDepth := 0
	for offset := 0; offset < len(content); {
		if content[offset] != '<' {
			offset++
			continue
		}
		if strings.HasPrefix(content[offset:], "<!--") {
			end, eof := scanHTMLComment(content, offset)
			document.comments = append(document.comments, htmlComment{
				start: offset,
				end:   end,
				eof:   eof,
			})
			offset = end
			continue
		}
		if strings.HasPrefix(content[offset:], "<![CDATA[") && context.currentNamespace() != htmlNamespaceHTML {
			end, err := scanForeignCDATAEnd(content, offset, context.currentNamespace())
			if err != nil {
				return scannedHTML{}, err
			}
			offset = end
			continue
		}
		if strings.HasPrefix(content[offset:], "<!") || strings.HasPrefix(content[offset:], "<?") {
			end, err := scanHTMLConstructEnd(content, offset+2)
			if err != nil {
				return scannedHTML{}, err
			}
			offset = end
			continue
		}

		tag, ok, err := scanHTMLTag(content, offset)
		if err != nil {
			return scannedHTML{}, err
		}
		if !ok {
			offset++
			continue
		}
		if context.applyForeignBreakout(tag) {
			document.profile.markComplex("foreign-content parser recovery")
			if tag.closing {
				tag.namespace = htmlNamespaceHTML
			} else {
				tag.namespace = namespaceForHTMLStartTag(tag.name)
			}
		} else if tag.closing {
			tag.namespace = context.closingNamespace(tag.name)
		} else {
			tag.namespace = context.startTagNamespace(tag.name)
		}
		if tag.closing && tag.namespace == htmlNamespaceHTML && tag.name == "template" {
			if templateDepth == 0 {
				return scannedHTML{}, fmt.Errorf("custom index has a closing </template> without a matching opening tag")
			}
			templateDepth--
		}
		tag.templateDepth = templateDepth
		if !tag.closing && tag.namespace == htmlNamespaceHTML && tag.name == "plaintext" {
			document.tags = append(document.tags, tag)
			classifyHTMLRewriteProfile(content, &document)
			return document, nil
		}
		if !tag.closing && tag.namespace == htmlNamespaceHTML && opaqueHTMLTextElement(tag.name) {
			var closing htmlTag
			var err error
			if tag.name == "script" {
				closing, err = scanScriptElementClose(content, tag.end)
			} else {
				closing, err = scanRawElementClose(content, tag.end, tag.name)
			}
			if err != nil {
				return scannedHTML{}, err
			}
			closing.namespace = tag.namespace
			tag.rawStart = tag.end
			tag.rawEnd = closing.start
			document.tags = append(document.tags, tag)
			closing.templateDepth = templateDepth
			document.tags = append(document.tags, closing)
			offset = closing.end
			continue
		}
		document.tags = append(document.tags, tag)
		if tag.closing {
			context.close(tag.name)
		} else if tag.namespace == htmlNamespaceHTML {
			if !voidHTMLElement(tag.name) {
				context.push(content, tag)
			}
		} else if !tag.selfClosing {
			context.push(content, tag)
		}
		if !tag.closing && tag.namespace == htmlNamespaceHTML && tag.name == "template" {
			templateDepth++
		}
		offset = tag.end
	}
	if templateDepth != 0 {
		return scannedHTML{}, fmt.Errorf("custom index has an opening <template> without a matching closing tag")
	}
	classifyHTMLRewriteProfile(content, &document)
	return document, nil
}

type htmlCommentState uint8

const (
	htmlCommentStart htmlCommentState = iota
	htmlCommentStartDash
	htmlCommentData
	htmlCommentLessThan
	htmlCommentLessThanBang
	htmlCommentLessThanBangDash
	htmlCommentLessThanBangDashDash
	htmlCommentEndDash
	htmlCommentEnd
	htmlCommentEndBang
)

// scanHTMLComment follows the bounded comment tokenizer states. The scanner
// only needs the raw source span; comment data remains authored and opaque.
func scanHTMLComment(content string, start int) (end int, eof bool) {
	state := htmlCommentStart
	for offset := start + len("<!--"); ; {
		if offset >= len(content) {
			return len(content), true
		}
		current := content[offset]
		switch state {
		case htmlCommentStart:
			switch current {
			case '-':
				state = htmlCommentStartDash
				offset++
			case '>':
				return offset + 1, false
			default:
				state = htmlCommentData
			}
		case htmlCommentStartDash:
			switch current {
			case '-':
				state = htmlCommentEnd
				offset++
			case '>':
				return offset + 1, false
			default:
				state = htmlCommentData
			}
		case htmlCommentData:
			switch current {
			case '<':
				state = htmlCommentLessThan
			case '-':
				state = htmlCommentEndDash
			}
			offset++
		case htmlCommentLessThan:
			switch current {
			case '!':
				state = htmlCommentLessThanBang
				offset++
			case '<':
				offset++
			default:
				state = htmlCommentData
			}
		case htmlCommentLessThanBang:
			if current == '-' {
				state = htmlCommentLessThanBangDash
				offset++
			} else {
				state = htmlCommentData
			}
		case htmlCommentLessThanBangDash:
			if current == '-' {
				state = htmlCommentLessThanBangDashDash
				offset++
			} else {
				state = htmlCommentEndDash
			}
		case htmlCommentLessThanBangDashDash:
			state = htmlCommentEnd
		case htmlCommentEndDash:
			if current == '-' {
				state = htmlCommentEnd
				offset++
			} else {
				state = htmlCommentData
			}
		case htmlCommentEnd:
			switch current {
			case '>':
				return offset + 1, false
			case '!':
				state = htmlCommentEndBang
				offset++
			case '-':
				offset++
			default:
				state = htmlCommentData
			}
		case htmlCommentEndBang:
			switch current {
			case '-':
				state = htmlCommentEndDash
				offset++
			case '>':
				return offset + 1, false
			default:
				state = htmlCommentData
			}
		}
	}
}

func scanForeignCDATAEnd(content string, start int, namespace htmlNamespace) (int, error) {
	const opening = "<![CDATA["
	end := strings.Index(content[start+len(opening):], "]]>")
	if end < 0 {
		name := "foreign-content"
		switch namespace {
		case htmlNamespaceSVG:
			name = "SVG"
		case htmlNamespaceMathML:
			name = "MathML"
		}
		return 0, fmt.Errorf("custom index has an unterminated %s CDATA section; close it with ]]>", name)
	}
	return start + len(opening) + end + len("]]>"), nil
}

func (context htmlScannerContext) currentNamespace() htmlNamespace {
	if len(context.elements) == 0 {
		return htmlNamespaceHTML
	}
	return context.elements[len(context.elements)-1].namespace
}

func (context htmlScannerContext) startTagNamespace(name string) htmlNamespace {
	if len(context.elements) == 0 {
		return namespaceForHTMLStartTag(name)
	}
	current := context.elements[len(context.elements)-1]
	if current.namespace == htmlNamespaceMathML && current.name == "annotation-xml" && name == "svg" {
		return htmlNamespaceSVG
	}
	if current.namespace == htmlNamespaceHTML || current.htmlIntegrationPoint ||
		current.mathMLTextIntegrationPt && name != "mglyph" && name != "malignmark" {
		return namespaceForHTMLStartTag(name)
	}
	return current.namespace
}

func (context *htmlScannerContext) applyForeignBreakout(tag htmlTag) bool {
	if !context.usesForeignContentRules(tag) || !foreignContentBreakoutTag(tag) {
		return false
	}
	for len(context.elements) > 0 {
		current := context.elements[len(context.elements)-1]
		if current.namespace == htmlNamespaceHTML || current.htmlIntegrationPoint || current.mathMLTextIntegrationPt {
			break
		}
		context.elements = context.elements[:len(context.elements)-1]
	}
	return true
}

func (context htmlScannerContext) usesForeignContentRules(tag htmlTag) bool {
	if len(context.elements) == 0 {
		return false
	}
	current := context.elements[len(context.elements)-1]
	if current.namespace == htmlNamespaceHTML {
		return false
	}
	if tag.closing {
		return true
	}
	if current.mathMLTextIntegrationPt && tag.name != "mglyph" && tag.name != "malignmark" {
		return false
	}
	if current.namespace == htmlNamespaceMathML && current.name == "annotation-xml" && tag.name == "svg" {
		return false
	}
	return !current.htmlIntegrationPoint
}

func foreignContentBreakoutTag(tag htmlTag) bool {
	if tag.closing {
		return tag.name == "br" || tag.name == "p"
	}
	switch tag.name {
	case "b", "big", "blockquote", "body", "br", "center", "code", "dd", "div", "dl", "dt",
		"em", "embed", "h1", "h2", "h3", "h4", "h5", "h6", "head", "hr", "i", "img",
		"li", "listing", "menu", "meta", "nobr", "ol", "p", "pre", "ruby", "s", "small",
		"span", "strong", "strike", "sub", "sup", "table", "tt", "u", "ul", "var":
		return true
	case "font":
		for _, attribute := range tag.attributes {
			switch attribute.name {
			case "color", "face", "size":
				return true
			}
		}
	}
	return false
}

func namespaceForHTMLStartTag(name string) htmlNamespace {
	switch name {
	case "svg":
		return htmlNamespaceSVG
	case "math":
		return htmlNamespaceMathML
	default:
		return htmlNamespaceHTML
	}
}

func (context *htmlScannerContext) push(content string, tag htmlTag) {
	element := htmlElementContext{name: tag.name, namespace: tag.namespace}
	switch tag.namespace {
	case htmlNamespaceSVG:
		switch tag.name {
		case "foreignobject", "desc", "title":
			element.htmlIntegrationPoint = true
		}
	case htmlNamespaceMathML:
		switch tag.name {
		case "mi", "mo", "mn", "ms", "mtext":
			element.mathMLTextIntegrationPt = true
		case "annotation-xml":
			if encoding, ok := firstHTMLAttributeValue(content, tag, "encoding"); ok {
				element.htmlIntegrationPoint = asciiFoldEqual(encoding, "text/html") ||
					asciiFoldEqual(encoding, "application/xhtml+xml")
			}
		}
	}
	context.elements = append(context.elements, element)
}

func (context htmlScannerContext) closingNamespace(name string) htmlNamespace {
	for index := len(context.elements) - 1; index >= 0; index-- {
		if context.elements[index].name == name {
			return context.elements[index].namespace
		}
	}
	return context.currentNamespace()
}

func (context *htmlScannerContext) close(name string) {
	for index := len(context.elements) - 1; index >= 0; index-- {
		if context.elements[index].name == name {
			context.elements = context.elements[:index]
			return
		}
	}
}

func firstHTMLAttributeValue(content string, tag htmlTag, name string) (string, bool) {
	for _, attribute := range tag.attributes {
		if attribute.name == name && attribute.hasValue {
			return semanticHTMLAttributeValue(content, &attribute), true
		}
	}
	return "", false
}

type sourceByte struct {
	value byte
	start int
	end   int
}

func semanticHTMLAttributeValue(content string, attribute *htmlAttribute) string {
	if attribute == nil || !attribute.hasValue {
		return ""
	}
	units := htmlAttributeSourceBytes(content, attribute)
	var result strings.Builder
	result.Grow(len(units))
	for _, unit := range units {
		result.WriteByte(unit.value)
	}
	return result.String()
}

func htmlAttributeSourceBytes(content string, attribute *htmlAttribute) []sourceByte {
	value := content[attribute.valueStart:attribute.valueEnd]
	units := make([]sourceByte, 0, len(value))
	for offset := 0; offset < len(value); {
		if value[offset] == 0 {
			for _, current := range []byte("\uFFFD") {
				units = append(units, sourceByte{value: current, start: offset, end: offset + 1})
			}
			offset++
			continue
		}
		decoded, end, ok := decodeHTMLCharacterReference(value, offset)
		if !ok {
			units = append(units, sourceByte{value: value[offset], start: offset, end: offset + 1})
			offset++
			continue
		}
		for index := range len(decoded) {
			units = append(units, sourceByte{value: decoded[index], start: offset, end: end})
		}
		offset = end
	}
	return units
}

func decodeHTMLCharacterReference(value string, start int) (string, int, bool) {
	if decoded, end, ok := decodeNumericHTMLCharacterReference(value, start); ok {
		return decoded, end, true
	}
	return decodeNamedHTMLCharacterReference(value, start)
}

func decodeNamedHTMLCharacterReference(value string, start int) (string, int, bool) {
	if start >= len(value) || value[start] != '&' {
		return "", 0, false
	}
	remaining := len(value) - start - 1
	if remaining > longestHTMLNamedCharacterReference {
		remaining = longestHTMLNamedCharacterReference
	}
	for length := remaining; length > 0; length-- {
		name := value[start+1 : start+1+length]
		decoded, ok := htmlNamedCharacterReferences[name]
		if !ok {
			continue
		}
		end := start + 1 + length
		if name[len(name)-1] != ';' && end < len(value) && (isASCIIAlphaNumeric(value[end]) || value[end] == '=') {
			return "", 0, false
		}
		return decoded, end, true
	}
	return "", 0, false
}

func decodeNumericHTMLCharacterReference(value string, start int) (string, int, bool) {
	if start+2 >= len(value) || value[start] != '&' || value[start+1] != '#' {
		return "", 0, false
	}
	offset := start + 2
	base := byte(10)
	if offset < len(value) && (value[offset] == 'x' || value[offset] == 'X') {
		base = 16
		offset++
	}
	digits := offset
	for offset < len(value) && isHTMLCharacterReferenceDigit(value[offset], base) {
		offset++
	}
	if offset == digits {
		return "", 0, false
	}
	if offset < len(value) && value[offset] == ';' {
		offset++
	}
	raw := value[start:offset]
	decoded := stdhtml.UnescapeString(raw)
	if decoded == raw {
		return "", 0, false
	}
	return decoded, offset, true
}

func isHTMLCharacterReferenceDigit(value byte, base byte) bool {
	if value >= '0' && value <= '9' {
		return true
	}
	return base == 16 && (value >= 'a' && value <= 'f' || value >= 'A' && value <= 'F')
}

func isASCIIAlphaNumeric(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func voidHTMLElement(name string) bool {
	switch name {
	case "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr":
		return true
	default:
		return false
	}
}

func opaqueHTMLTextElement(name string) bool {
	switch name {
	case "script", "style", "title", "textarea", "xmp", "iframe", "noembed", "noframes", "noscript":
		return true
	default:
		return false
	}
}

func scanHTMLConstructEnd(content string, start int) (int, error) {
	quote := byte(0)
	for offset := start; offset < len(content); offset++ {
		current := content[offset]
		if quote != 0 {
			if current == quote {
				quote = 0
			}
			continue
		}
		if current == '\'' || current == '"' {
			quote = current
			continue
		}
		if current == '>' {
			return offset + 1, nil
		}
	}
	return 0, fmt.Errorf("custom index has an unterminated declaration or processing instruction")
}

func scanHTMLTag(content string, start int) (htmlTag, bool, error) {
	offset := start + 1
	closing := false
	if offset < len(content) && content[offset] == '/' {
		closing = true
		offset++
	}
	if offset >= len(content) || !isHTMLNameStart(content[offset]) {
		return htmlTag{}, false, nil
	}
	nameStart := offset
	for offset < len(content) && !isHTMLSpace(content[offset]) && content[offset] != '/' && content[offset] != '>' {
		offset++
	}
	name := semanticHTMLTagName(content[nameStart:offset])
	tagEnd, err := scanHTMLConstructEnd(content, offset)
	if err != nil {
		return htmlTag{}, false, fmt.Errorf("custom index <%s> tag is unterminated: %w", name, err)
	}
	attributes, selfClosing, err := scanHTMLAttributes(content, offset, tagEnd-1, name)
	if err != nil {
		return htmlTag{}, false, err
	}
	return htmlTag{
		start:       start,
		end:         tagEnd,
		name:        name,
		closing:     closing,
		selfClosing: selfClosing,
		attributes:  attributes,
	}, true, nil
}

func semanticHTMLTagName(value string) string {
	var name strings.Builder
	name.Grow(len(value))
	for offset := 0; offset < len(value); offset++ {
		switch current := value[offset]; {
		case current == 0:
			name.WriteRune('\uFFFD')
		case current >= 'A' && current <= 'Z':
			name.WriteByte(current + 'a' - 'A')
		default:
			name.WriteByte(current)
		}
	}
	return name.String()
}

func scanHTMLAttributes(content string, start, end int, tagName string) ([]htmlAttribute, bool, error) {
	var attributes []htmlAttribute
	for offset := start; offset < end; {
		for offset < end && isHTMLSpace(content[offset]) {
			offset++
		}
		if offset >= end {
			break
		}
		if content[offset] == '/' {
			if offset+1 == end {
				return attributes, true, nil
			}
			return nil, false, fmt.Errorf("custom index <%s> has a malformed solidus near byte %d", tagName, offset)
		}
		nameStart := offset
		for offset < end && !isHTMLSpace(content[offset]) && content[offset] != '=' && content[offset] != '/' && content[offset] != '>' {
			offset++
		}
		if nameStart == offset {
			return nil, false, fmt.Errorf("custom index <%s> has a malformed attribute near byte %d", tagName, offset)
		}
		attribute := htmlAttribute{name: asciiLower(content[nameStart:offset])}
		for offset < end && isHTMLSpace(content[offset]) {
			offset++
		}
		if offset < end && content[offset] == '=' {
			attribute.hasValue = true
			offset++
			for offset < end && isHTMLSpace(content[offset]) {
				offset++
			}
			if offset >= end {
				return nil, false, fmt.Errorf("custom index <%s> attribute %s has no value", tagName, attribute.name)
			}
			if content[offset] == '\'' || content[offset] == '"' {
				quote := content[offset]
				if quote == '\'' {
					attribute.valueSyntax = htmlAttributeValueSingleQuoted
				} else {
					attribute.valueSyntax = htmlAttributeValueDoubleQuoted
				}
				offset++
				attribute.valueStart = offset
				for offset < end && content[offset] != quote {
					offset++
				}
				if offset >= end {
					return nil, false, fmt.Errorf("custom index <%s> attribute %s has an unterminated quoted value", tagName, attribute.name)
				}
				attribute.valueEnd = offset
				offset++
			} else {
				attribute.valueSyntax = htmlAttributeValueUnquoted
				attribute.valueStart = offset
				for offset < end && !isHTMLSpace(content[offset]) && content[offset] != '>' {
					offset++
				}
				attribute.valueEnd = offset
				if attribute.valueStart == attribute.valueEnd {
					return nil, false, fmt.Errorf("custom index <%s> attribute %s has an empty unquoted value", tagName, attribute.name)
				}
			}
		}
		attributes = append(attributes, attribute)
	}
	return attributes, false, nil
}

type scriptDataState uint8

const (
	scriptData scriptDataState = iota
	scriptDataLessThan
	scriptDataEndTagOpen
	scriptDataEndTagName
	scriptDataEscapeStart
	scriptDataEscapeStartDash
	scriptDataEscaped
	scriptDataEscapedDash
	scriptDataEscapedDashDash
	scriptDataEscapedLessThan
	scriptDataEscapedEndTagOpen
	scriptDataEscapedEndTagName
	scriptDataDoubleEscapeStart
	scriptDataDoubleEscaped
	scriptDataDoubleEscapedDash
	scriptDataDoubleEscapedDashDash
	scriptDataDoubleEscapedLessThan
	scriptDataDoubleEscapeEnd
)

// scanScriptElementClose follows the HTML tokenizer's script-data states. It
// deliberately does not interpret JavaScript syntax; only HTML script closing
// sequences affect where structural scanning resumes.
func scanScriptElementClose(content string, start int) (htmlTag, error) {
	state := scriptData
	tagStart := -1
	temporaryLength := 0
	temporaryMatchesScript := false
	for offset := start; offset < len(content); {
		current := content[offset]
		switch state {
		case scriptData:
			if current == '<' {
				state = scriptDataLessThan
			}
			offset++
		case scriptDataLessThan:
			switch current {
			case '/':
				tagStart = offset - 1
				state = scriptDataEndTagOpen
				offset++
			case '!':
				state = scriptDataEscapeStart
				offset++
			default:
				state = scriptData
			}
		case scriptDataEndTagOpen:
			if isHTMLNameStart(current) {
				state = scriptDataEndTagName
			} else {
				state = scriptData
			}
		case scriptDataEndTagName:
			tag, found, resume, err := scanScriptEndTagCandidate(content, tagStart, offset)
			if err != nil {
				return htmlTag{}, err
			}
			if found {
				return tag, nil
			}
			offset = resume
			state = scriptData
		case scriptDataEscapeStart:
			if current == '-' {
				state = scriptDataEscapeStartDash
				offset++
			} else {
				state = scriptData
			}
		case scriptDataEscapeStartDash:
			if current == '-' {
				state = scriptDataEscapedDashDash
				offset++
			} else {
				state = scriptData
			}
		case scriptDataEscaped:
			switch current {
			case '-':
				state = scriptDataEscapedDash
			case '<':
				state = scriptDataEscapedLessThan
			}
			offset++
		case scriptDataEscapedDash:
			switch current {
			case '-':
				state = scriptDataEscapedDashDash
			case '<':
				state = scriptDataEscapedLessThan
			default:
				state = scriptDataEscaped
			}
			offset++
		case scriptDataEscapedDashDash:
			switch current {
			case '<':
				state = scriptDataEscapedLessThan
			case '>':
				state = scriptData
			case '-':
			default:
				state = scriptDataEscaped
			}
			offset++
		case scriptDataEscapedLessThan:
			switch {
			case current == '/':
				tagStart = offset - 1
				state = scriptDataEscapedEndTagOpen
				offset++
			case isHTMLNameStart(current):
				temporaryLength = 0
				temporaryMatchesScript = true
				state = scriptDataDoubleEscapeStart
			default:
				state = scriptDataEscaped
			}
		case scriptDataEscapedEndTagOpen:
			if isHTMLNameStart(current) {
				state = scriptDataEscapedEndTagName
			} else {
				state = scriptDataEscaped
			}
		case scriptDataEscapedEndTagName:
			tag, found, resume, err := scanScriptEndTagCandidate(content, tagStart, offset)
			if err != nil {
				return htmlTag{}, err
			}
			if found {
				return tag, nil
			}
			offset = resume
			state = scriptDataEscaped
		case scriptDataDoubleEscapeStart:
			switch {
			case isHTMLNameStart(current):
				temporaryMatchesScript = updateScriptTemporaryBuffer(current, temporaryLength, temporaryMatchesScript)
				temporaryLength++
				offset++
			case isHTMLSpace(current) || current == '/' || current == '>':
				if temporaryMatchesScript && temporaryLength == len("script") {
					state = scriptDataDoubleEscaped
				} else {
					state = scriptDataEscaped
				}
				offset++
			default:
				state = scriptDataEscaped
			}
		case scriptDataDoubleEscaped:
			switch current {
			case '-':
				state = scriptDataDoubleEscapedDash
			case '<':
				state = scriptDataDoubleEscapedLessThan
			}
			offset++
		case scriptDataDoubleEscapedDash:
			switch current {
			case '-':
				state = scriptDataDoubleEscapedDashDash
			case '<':
				state = scriptDataDoubleEscapedLessThan
			default:
				state = scriptDataDoubleEscaped
			}
			offset++
		case scriptDataDoubleEscapedDashDash:
			switch current {
			case '<':
				state = scriptDataDoubleEscapedLessThan
			case '>':
				state = scriptData
			case '-':
			default:
				state = scriptDataDoubleEscaped
			}
			offset++
		case scriptDataDoubleEscapedLessThan:
			if current == '/' {
				temporaryLength = 0
				temporaryMatchesScript = true
				state = scriptDataDoubleEscapeEnd
				offset++
			} else {
				state = scriptDataDoubleEscaped
			}
		case scriptDataDoubleEscapeEnd:
			switch {
			case isHTMLNameStart(current):
				temporaryMatchesScript = updateScriptTemporaryBuffer(current, temporaryLength, temporaryMatchesScript)
				temporaryLength++
				offset++
			case isHTMLSpace(current) || current == '/' || current == '>':
				if temporaryMatchesScript && temporaryLength == len("script") {
					state = scriptDataEscaped
				} else {
					state = scriptDataDoubleEscaped
				}
				offset++
			default:
				state = scriptDataDoubleEscaped
			}
		}
	}
	return htmlTag{}, fmt.Errorf("custom index <script> element has no closing </script> tag")
}

func scanScriptEndTagCandidate(content string, tagStart, nameStart int) (htmlTag, bool, int, error) {
	offset := nameStart
	for offset < len(content) && isHTMLNameStart(content[offset]) {
		offset++
	}
	if offset-nameStart != len("script") || !asciiFoldPrefixAt(content, nameStart, "script") {
		return htmlTag{}, false, offset, nil
	}
	if offset >= len(content) || !isHTMLSpace(content[offset]) && content[offset] != '/' && content[offset] != '>' {
		return htmlTag{}, false, offset, nil
	}
	tag, ok, err := scanHTMLTag(content, tagStart)
	if err != nil {
		return htmlTag{}, false, offset, err
	}
	if !ok || !tag.closing || tag.name != "script" {
		return htmlTag{}, false, offset, nil
	}
	return tag, true, tag.end, nil
}

func updateScriptTemporaryBuffer(value byte, length int, matches bool) bool {
	return matches && length < len("script") && asciiLowerByte(value) == "script"[length]
}

func scanRawElementClose(content string, start int, name string) (htmlTag, error) {
	needleLength := len(name) + 2
	for offset := start; offset+needleLength <= len(content); offset++ {
		if content[offset] != '<' || content[offset+1] != '/' || !asciiFoldPrefixAt(content, offset+2, name) {
			continue
		}
		afterName := offset + needleLength
		if afterName < len(content) && !isHTMLSpace(content[afterName]) && content[afterName] != '>' && content[afterName] != '/' {
			continue
		}
		tag, ok, err := scanHTMLTag(content, offset)
		if err != nil {
			return htmlTag{}, err
		}
		if ok && tag.closing && tag.name == name {
			return tag, nil
		}
	}
	return htmlTag{}, fmt.Errorf("custom index <%s> element has no closing </%s> tag", name, name)
}

func isHTMLNameStart(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func isHTMLSpace(value byte) bool {
	switch value {
	case ' ', '\t', '\n', '\r', '\f':
		return true
	default:
		return false
	}
}

func asciiLower(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for index := range len(value) {
		current := value[index]
		if current >= 'A' && current <= 'Z' {
			current += 'a' - 'A'
		}
		builder.WriteByte(current)
	}
	return builder.String()
}

func asciiLowerByte(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + 'a' - 'A'
	}
	return value
}

func asciiFoldPrefixAt(content string, offset int, expected string) bool {
	if offset < 0 || len(content)-offset < len(expected) {
		return false
	}
	for index := range len(expected) {
		if asciiLowerByte(content[offset+index]) != asciiLowerByte(expected[index]) {
			return false
		}
	}
	return true
}

func asciiFoldEqual(first, second string) bool {
	return len(first) == len(second) && asciiFoldPrefixAt(first, 0, second)
}

type managedHTMLBlock struct {
	name  string
	start int
	end   int
}

type managedMarker struct {
	name  string
	start bool
	span  htmlComment
}

func managedMarkerText(name string, start bool) string {
	if start {
		return "<!-- goframe:" + name + " -->"
	}
	return "<!-- /goframe:" + name + " -->"
}

func validateManagedHTMLBlocks(content string, document scannedHTML) (map[string]managedHTMLBlock, error) {
	names := []string{preloadBlockName, runtimeBlockName, bootstrapBlockName}
	markers := map[string][]managedMarker{}
	for _, comment := range document.comments {
		raw := content[comment.start:comment.end]
		for _, name := range names {
			for _, start := range []bool{true, false} {
				if raw == managedMarkerText(name, start) {
					if comment.unsupportedEnclosing != "" {
						return nil, fmt.Errorf(
							"goframe:%s managed block marker is nested inside %s; move the complete block to a safe top-level source location",
							name,
							comment.unsupportedEnclosing,
						)
					}
					markers[name] = append(markers[name], managedMarker{name: name, start: start, span: comment})
				}
			}
		}
	}

	blocks := map[string]managedHTMLBlock{}
	for _, name := range names {
		var starts, ends []managedMarker
		for _, marker := range markers[name] {
			if marker.start {
				starts = append(starts, marker)
			} else {
				ends = append(ends, marker)
			}
		}
		if len(starts) > 1 {
			return nil, fmt.Errorf("goframe:%s managed block has duplicate start markers; keep exactly one complete block", name)
		}
		if len(ends) > 1 {
			return nil, fmt.Errorf("goframe:%s managed block has duplicate end markers; keep exactly one complete block", name)
		}
		if len(starts) == 0 && len(ends) == 0 {
			continue
		}
		if len(starts) == 0 {
			return nil, fmt.Errorf("goframe:%s managed block has an orphan end marker; add the matching start marker or remove the end marker", name)
		}
		if len(ends) == 0 {
			return nil, fmt.Errorf("goframe:%s managed block has an orphan start marker; add the matching end marker or remove the start marker", name)
		}
		if ends[0].span.start < starts[0].span.start {
			return nil, fmt.Errorf("goframe:%s managed block end marker appears before its start marker; place the markers in start/end order", name)
		}
		blocks[name] = managedHTMLBlock{
			name:  name,
			start: starts[0].span.start,
			end:   ends[0].span.end,
		}
	}

	ordered := make([]managedHTMLBlock, 0, len(blocks))
	for _, block := range blocks {
		ordered = append(ordered, block)
	}
	sort.Slice(ordered, func(first, second int) bool { return ordered[first].start < ordered[second].start })
	for index := 1; index < len(ordered); index++ {
		previous := ordered[index-1]
		current := ordered[index]
		if current.start < previous.end {
			return nil, fmt.Errorf(
				"goframe:%s and goframe:%s managed blocks are nested or interleaved; keep each complete block separate",
				previous.name,
				current.name,
			)
		}
	}
	return blocks, nil
}

func managedBlockContains(blocks map[string]managedHTMLBlock, offset int) bool {
	for _, block := range blocks {
		if offset >= block.start && offset < block.end {
			return true
		}
	}
	return false
}

func rewriteIndexHTML(content string, options htmlRewriteOptions) (string, error) {
	document, err := scanCustomIndexHTML(content)
	if err != nil {
		return "", err
	}
	blocks, err := validateManagedHTMLBlocks(content, document)
	if err != nil {
		return "", err
	}
	plan := htmlRewritePlan{content: content}
	preload, err := preloadHTML(options)
	if err != nil {
		return "", err
	}
	runtime, err := runtimeHTML(options)
	if err != nil {
		return "", err
	}
	bootstrap, err := bootstrapHTML(options)
	if err != nil {
		return "", err
	}

	managedValues := map[string]string{
		preloadBlockName:   preload,
		runtimeBlockName:   runtime,
		bootstrapBlockName: bootstrap,
	}
	for _, name := range []string{preloadBlockName, runtimeBlockName, bootstrapBlockName} {
		block, ok := blocks[name]
		if !ok {
			continue
		}
		plan.add(htmlReplacement{
			start:       block.start,
			end:         block.end,
			value:       managedMarkerText(name, true) + "\n" + managedValues[name] + "\n" + managedMarkerText(name, false),
			description: "goframe:" + name + " managed block",
		})
	}

	if _, managed := blocks[runtimeBlockName]; !managed {
		if err := planLegacyRuntimeRewrites(&plan, document, blocks, options.runtimePath); err != nil {
			return "", err
		}
	}
	if _, managed := blocks[bootstrapBlockName]; !managed {
		if err := planLegacyWASMRewrites(&plan, document, blocks, options.wasmPath); err != nil {
			return "", err
		}
	}
	if err := planLegacyStyleRewrites(&plan, document, blocks, options.styleRewrites); err != nil {
		return "", err
	}
	if _, managed := blocks[preloadBlockName]; !managed && options.preload {
		if err := planPreloadInsertion(&plan, document, blocks, preload); err != nil {
			return "", err
		}
	}
	return plan.apply()
}

func planLegacyRuntimeRewrites(plan *htmlRewritePlan, document scannedHTML, blocks map[string]managedHTMLBlock, runtimePath string) error {
	for _, tag := range document.tags {
		if tag.closing || tag.name != "script" || tag.ordinaryTemplateDepth != 0 || managedBlockContains(blocks, tag.start) {
			continue
		}
		source, err := tag.attribute("src")
		if err != nil {
			return err
		}
		if source == nil || !source.hasValue {
			continue
		}
		kind, err := classifyScriptTag(plan.content, tag)
		if err != nil {
			return err
		}
		if kind != scriptKindClassic && kind != scriptKindModule {
			continue
		}
		value := plan.content[source.valueStart:source.valueEnd]
		match, ok := matchLegacyURL(value, htmlAttributeSourceBytes(plan.content, source), runtimeAssetName)
		if !ok {
			continue
		}
		if document.profile.complex {
			return document.profile.markerlessError("runtime", runtimeBlockName)
		}
		if tag.namespace != htmlNamespaceHTML {
			continue
		}
		replacement, err := encodeGeneratedHTMLAttributeValue(runtimePath, source.valueSyntax)
		if err != nil {
			return err
		}
		plan.add(htmlReplacement{
			start:       source.valueStart + match.start,
			end:         source.valueStart + match.end,
			value:       replacement + match.suffix,
			description: "legacy runtime script source",
		})
	}
	return nil
}

func planLegacyWASMRewrites(plan *htmlRewritePlan, document scannedHTML, blocks map[string]managedHTMLBlock, wasmPath string) error {
	for _, tag := range document.tags {
		if tag.closing || tag.name != "script" || tag.ordinaryTemplateDepth != 0 || managedBlockContains(blocks, tag.start) {
			continue
		}
		source, err := tag.attribute("src")
		if err != nil {
			return err
		}
		if source != nil {
			continue
		}
		kind, err := classifyScriptTag(plan.content, tag)
		if err != nil {
			return err
		}
		if kind != scriptKindClassic && kind != scriptKindModule {
			continue
		}
		match, ok := recognizeLegacyGoFrameBootstrap(plan.content, tag.rawStart, tag.rawEnd)
		if !ok {
			continue
		}
		if document.profile.complex {
			return document.profile.markerlessError("bootstrap", bootstrapBlockName)
		}
		if tag.namespace != htmlNamespaceHTML {
			continue
		}
		replacement, err := encodeGeneratedJavaScriptStringContents(wasmPath, match.quote)
		if err != nil {
			return err
		}
		plan.add(htmlReplacement{
			start:       match.urlStart,
			end:         match.urlEnd,
			value:       replacement + match.suffix,
			description: "legacy GoFrame bootstrap WASM URL",
		})
	}
	return nil
}

type scriptKind uint8

const (
	scriptKindData scriptKind = iota
	scriptKindClassic
	scriptKindModule
	scriptKindImportMap
	scriptKindSpeculationRules
)

func classifyScriptTag(content string, tag htmlTag) (scriptKind, error) {
	typeAttribute, err := tag.attribute("type")
	if err != nil {
		return scriptKindData, err
	}
	languageAttribute, err := tag.attribute("language")
	if err != nil {
		return scriptKindData, err
	}
	typeValue := semanticHTMLAttributeValue(content, typeAttribute)
	languageValue := semanticHTMLAttributeValue(content, languageAttribute)
	var typeString string
	switch {
	case typeAttribute != nil && typeValue == "":
		typeString = "text/javascript"
	case typeAttribute == nil && languageAttribute != nil && languageValue == "":
		typeString = "text/javascript"
	case typeAttribute == nil && languageAttribute == nil:
		typeString = "text/javascript"
	case typeAttribute != nil:
		typeString = trimHTMLSpace(typeValue)
	default:
		typeString = "text/" + languageValue
	}
	if javaScriptMIMETypeEssenceMatch(typeString) {
		return scriptKindClassic, nil
	}
	switch {
	case asciiFoldEqual(typeString, "module"):
		return scriptKindModule, nil
	case asciiFoldEqual(typeString, "importmap"):
		return scriptKindImportMap, nil
	case asciiFoldEqual(typeString, "speculationrules"):
		return scriptKindSpeculationRules, nil
	default:
		return scriptKindData, nil
	}
}

func javaScriptMIMETypeEssenceMatch(value string) bool {
	switch asciiLower(value) {
	case "application/ecmascript", "application/javascript", "application/x-ecmascript", "application/x-javascript",
		"text/ecmascript", "text/javascript", "text/javascript1.0", "text/javascript1.1",
		"text/javascript1.2", "text/javascript1.3", "text/javascript1.4", "text/javascript1.5",
		"text/jscript", "text/livescript", "text/x-ecmascript", "text/x-javascript":
		return true
	default:
		return false
	}
}

func planLegacyStyleRewrites(plan *htmlRewritePlan, document scannedHTML, blocks map[string]managedHTMLBlock, rewrites map[string]string) error {
	if len(rewrites) == 0 {
		return nil
	}
	_, preloadManaged := blocks[preloadBlockName]
	for _, tag := range document.tags {
		if tag.closing || tag.name != "link" || tag.ordinaryTemplateDepth != 0 || managedBlockContains(blocks, tag.start) {
			continue
		}
		href, err := tag.attribute("href")
		if err != nil {
			return err
		}
		rel, err := tag.attribute("rel")
		if err != nil {
			return err
		}
		as, err := tag.attribute("as")
		if err != nil {
			return err
		}
		if href == nil || !href.hasValue || rel == nil || !rel.hasValue {
			continue
		}
		relTokens := htmlSpaceTokenSet(semanticHTMLAttributeValue(plan.content, rel))
		stylesheet := relTokens["stylesheet"]
		stylePreload := relTokens["preload"] && as != nil && as.hasValue && asciiFoldEqual(trimHTMLSpace(semanticHTMLAttributeValue(plan.content, as)), "style")
		if !stylesheet && (!stylePreload || preloadManaged) {
			continue
		}
		value := plan.content[href.valueStart:href.valueEnd]
		reference, ok := preprocessLegacyURL(value, htmlAttributeSourceBytes(plan.content, href))
		if !ok {
			continue
		}
		base, ok := normalizeLegacyPackageURLPath(reference.base)
		if !ok {
			continue
		}
		destination, ok := rewrites[base]
		if !ok {
			continue
		}
		if tag.declarativeShadowMode != "" {
			return fmt.Errorf(
				"custom index stylesheet rewriting inside declarative Shadow DOM is not part of the preview markerless contract; use an external stable URL or remove asset-managed shadow-root styles",
			)
		}
		if document.profile.complex {
			return document.profile.stylesheetError()
		}
		if tag.namespace != htmlNamespaceHTML {
			continue
		}
		replacement, err := encodeGeneratedHTMLAttributeValue(destination, href.valueSyntax)
		if err != nil {
			return err
		}
		plan.add(htmlReplacement{
			start:       href.valueStart + reference.start,
			end:         href.valueStart + reference.end,
			value:       replacement + reference.suffix,
			description: "legacy stylesheet reference",
		})
	}
	return nil
}

func encodeGeneratedHTMLAttributeValue(value string, syntax htmlAttributeValueSyntax) (string, error) {
	if strings.IndexByte(value, 0) >= 0 {
		return "", fmt.Errorf("generated package URL contains a NUL byte")
	}
	var encoded strings.Builder
	encoded.Grow(len(value))
	for offset := 0; offset < len(value); offset++ {
		current := value[offset]
		switch {
		case current == '&':
			encoded.WriteString("&amp;")
		case syntax == htmlAttributeValueDoubleQuoted && current == '"':
			encoded.WriteString("&quot;")
		case syntax == htmlAttributeValueSingleQuoted && current == '\'':
			encoded.WriteString("&#39;")
		case syntax == htmlAttributeValueUnquoted:
			switch current {
			case ' ':
				encoded.WriteString("&#32;")
			case '\t':
				encoded.WriteString("&#9;")
			case '\n':
				encoded.WriteString("&#10;")
			case '\r':
				encoded.WriteString("&#13;")
			case '\f':
				encoded.WriteString("&#12;")
			case '"':
				encoded.WriteString("&quot;")
			case '\'':
				encoded.WriteString("&#39;")
			case '`':
				encoded.WriteString("&#96;")
			case '=':
				encoded.WriteString("&#61;")
			case '<':
				encoded.WriteString("&lt;")
			case '>':
				encoded.WriteString("&gt;")
			default:
				encoded.WriteByte(current)
			}
		case syntax == htmlAttributeValueNone:
			return "", fmt.Errorf("generated package URL has no HTML attribute value context")
		default:
			encoded.WriteByte(current)
		}
	}
	return encoded.String(), nil
}

func trimHTMLSpace(value string) string {
	start := 0
	for start < len(value) && isHTMLSpace(value[start]) {
		start++
	}
	end := len(value)
	for end > start && isHTMLSpace(value[end-1]) {
		end--
	}
	return value[start:end]
}

func splitHTMLSpaceTokens(value string) []string {
	var tokens []string
	for offset := 0; offset < len(value); {
		for offset < len(value) && isHTMLSpace(value[offset]) {
			offset++
		}
		start := offset
		for offset < len(value) && !isHTMLSpace(value[offset]) {
			offset++
		}
		if start < offset {
			tokens = append(tokens, value[start:offset])
		}
	}
	return tokens
}

func htmlSpaceTokenSet(value string) map[string]bool {
	result := map[string]bool{}
	for _, token := range splitHTMLSpaceTokens(value) {
		result[asciiLower(token)] = true
	}
	return result
}

func planPreloadInsertion(plan *htmlRewritePlan, document scannedHTML, blocks map[string]managedHTMLBlock, preload string) error {
	var closingHeads []htmlTag
	for _, tag := range document.tags {
		if tag.closing && tag.namespace == htmlNamespaceHTML && tag.name == "head" && tag.templateDepth == 0 && !managedBlockContains(blocks, tag.start) {
			closingHeads = append(closingHeads, tag)
		}
	}
	if len(closingHeads) == 0 {
		return fmt.Errorf("custom index cannot receive preload hints; add a closing </head> tag or an explicit goframe:preload block")
	}
	if len(closingHeads) > 1 {
		return fmt.Errorf("custom index has multiple structural </head> tags; keep one closing </head> tag or add an explicit goframe:preload block")
	}
	if document.profile.complex {
		return document.profile.preloadError()
	}
	if strings.HasSuffix(plan.content[:closingHeads[0].start], preload+"\n") {
		return nil
	}
	plan.add(htmlReplacement{
		start:       closingHeads[0].start,
		end:         closingHeads[0].start,
		value:       preload + "\n",
		description: "preload insertion",
	})
	return nil
}

type legacyURLReference struct {
	base   string
	suffix string
	start  int
	end    int
}

func matchLegacyURL(value string, units []sourceByte, expected string) (legacyURLReference, bool) {
	reference, ok := preprocessLegacyURL(value, units)
	if !ok {
		return legacyURLReference{}, false
	}
	normalized, ok := normalizeLegacyPackageURLPath(reference.base)
	if !ok || normalized != expected {
		return legacyURLReference{}, false
	}
	return reference, true
}

func normalizeLegacyPackageURLPath(value string) (string, bool) {
	if value == "" || value[0] == '/' || value[0] == '\\' || hasLegacyURLScheme(value) {
		return "", false
	}

	segments := make([]string, 0, strings.Count(value, "/")+strings.Count(value, "\\")+1)
	start := 0
	for offset := 0; offset <= len(value); offset++ {
		if offset < len(value) && value[offset] != '/' && value[offset] != '\\' {
			continue
		}
		segment := value[start:offset]
		switch {
		case singleDotURLPathSegment(segment):
			if offset == len(value) {
				segments = append(segments, "")
			}
		case doubleDotURLPathSegment(segment):
			if len(segments) == 0 {
				return "", false
			}
			segments = segments[:len(segments)-1]
			if offset == len(value) {
				segments = append(segments, "")
			}
		default:
			segments = append(segments, segment)
		}
		start = offset + 1
	}
	return strings.Join(segments, "/"), true
}

func singleDotURLPathSegment(value string) bool {
	return value == "." || asciiFoldEqual(value, "%2e")
}

func doubleDotURLPathSegment(value string) bool {
	return value == ".." || asciiFoldEqual(value, ".%2e") ||
		asciiFoldEqual(value, "%2e.") || asciiFoldEqual(value, "%2e%2e")
}

func hasLegacyURLScheme(value string) bool {
	if !isHTMLNameStart(value[0]) {
		return false
	}
	for offset := 1; offset < len(value); offset++ {
		switch current := value[offset]; {
		case current == ':':
			return true
		case current == '/' || current == '\\':
			return false
		case isASCIIAlphaNumeric(current) || current == '+' || current == '-' || current == '.':
			continue
		default:
			return false
		}
	}
	return false
}

func preprocessLegacyURL(value string, units []sourceByte) (legacyURLReference, bool) {
	start := 0
	for start < len(units) && units[start].value <= 0x20 {
		start++
	}
	end := len(units)
	for end > start && units[end-1].value <= 0x20 {
		end--
	}
	if start == end {
		return legacyURLReference{}, false
	}

	filtered := make([]sourceByte, 0, end-start)
	for _, unit := range units[start:end] {
		switch unit.value {
		case '\t', '\n', '\r':
			continue
		default:
			filtered = append(filtered, unit)
		}
	}
	if len(filtered) == 0 {
		return legacyURLReference{}, false
	}

	delimiter := len(filtered)
	var base strings.Builder
	for index, unit := range filtered {
		if unit.value == '?' || unit.value == '#' {
			delimiter = index
			break
		}
		base.WriteByte(unit.value)
	}
	rawStart := units[start].start
	rawEnd := units[end-1].end
	suffix := ""
	if delimiter < len(filtered) {
		suffix = value[filtered[delimiter].start:rawEnd]
	}
	return legacyURLReference{
		base:   base.String(),
		suffix: suffix,
		start:  rawStart,
		end:    rawEnd,
	}, true
}

type legacyBootstrapMatch struct {
	urlStart int
	urlEnd   int
	suffix   string
	quote    byte
}

type legacyBootstrapParser struct {
	content string
	offset  int
	end     int
}

func recognizeLegacyGoFrameBootstrap(content string, start, end int) (legacyBootstrapMatch, bool) {
	// Each branch is a complete historical GoFrame bootstrap shape. Extra
	// authored tokens make the script non-owned and therefore unchanged.
	for _, shape := range []func(*legacyBootstrapParser) (legacyBootstrapMatch, bool){
		parseGeneratedArrowBootstrap,
		parseLegacyFunctionBootstrap,
		parseLegacyLoadWrappedBootstrap,
	} {
		parser := &legacyBootstrapParser{content: content, offset: start, end: end}
		match, ok := shape(parser)
		if ok && parser.complete() {
			return match, true
		}
	}
	return legacyBootstrapMatch{}, false
}

func parseGeneratedArrowBootstrap(parser *legacyBootstrapParser) (legacyBootstrapMatch, bool) {
	if !parser.word("const") || !parser.word("go") || !parser.token("=") ||
		!parser.word("new") || !parser.word("Go") || !parser.token("(") ||
		!parser.token(")") || !parser.token(";") {
		return legacyBootstrapMatch{}, false
	}
	match, ok := parser.streamingCall()
	if !ok || !parser.token(".") || !parser.word("then") || !parser.token("(") ||
		!parser.token("(") || !parser.word("result") || !parser.token(")") ||
		!parser.token("=>") || !parser.word("go") || !parser.token(".") ||
		!parser.word("run") || !parser.token("(") || !parser.word("result") ||
		!parser.token(".") || !parser.word("instance") || !parser.token(")") ||
		!parser.token(")") || !parser.token(";") {
		return legacyBootstrapMatch{}, false
	}
	return match, true
}

func parseLegacyFunctionBootstrap(parser *legacyBootstrapParser) (legacyBootstrapMatch, bool) {
	if !parser.word("var") || !parser.word("go") || !parser.token("=") ||
		!parser.word("new") || !parser.word("Go") || !parser.token("(") ||
		!parser.token(")") || !parser.token(";") {
		return legacyBootstrapMatch{}, false
	}
	match, ok := parser.streamingCall()
	if !ok || !parser.token(".") || !parser.word("then") || !parser.token("(") ||
		!parser.word("function") || !parser.token("(") || !parser.word("result") ||
		!parser.token(")") || !parser.token("{") || !parser.word("go") ||
		!parser.token(".") || !parser.word("run") || !parser.token("(") ||
		!parser.word("result") || !parser.token(".") || !parser.word("instance") ||
		!parser.token(")") || !parser.token(";") || !parser.token("}") ||
		!parser.token(")") || !parser.token(";") {
		return legacyBootstrapMatch{}, false
	}
	return match, true
}

func parseLegacyLoadWrappedBootstrap(parser *legacyBootstrapParser) (legacyBootstrapMatch, bool) {
	if !parser.word("window") || !parser.token(".") || !parser.word("addEventListener") ||
		!parser.token("(") || !parser.token(`"load"`) || !parser.token(",") ||
		!parser.word("function") || !parser.token("(") || !parser.token(")") ||
		!parser.token("{") {
		return legacyBootstrapMatch{}, false
	}
	match, ok := parseLegacyFunctionBootstrap(parser)
	if !ok || !parser.token("}") || !parser.token(",") || !parser.token("{") ||
		!parser.word("once") || !parser.token(":") || !parser.word("true") ||
		!parser.token("}") || !parser.token(")") || !parser.token(";") {
		return legacyBootstrapMatch{}, false
	}
	return match, true
}

func (parser *legacyBootstrapParser) streamingCall() (legacyBootstrapMatch, bool) {
	if !parser.word("WebAssembly") || !parser.token(".") ||
		!parser.word("instantiateStreaming") || !parser.token("(") ||
		!parser.word("fetch") || !parser.token("(") {
		return legacyBootstrapMatch{}, false
	}
	match, ok := parser.legacyWASMURL()
	if !ok || !parser.token(")") || !parser.token(",") || !parser.word("go") ||
		!parser.token(".") || !parser.word("importObject") {
		return legacyBootstrapMatch{}, false
	}
	parser.optionalToken(",")
	if !parser.token(")") {
		return legacyBootstrapMatch{}, false
	}
	return match, true
}

func (parser *legacyBootstrapParser) legacyWASMURL() (legacyBootstrapMatch, bool) {
	parser.skipSpace()
	if parser.offset >= parser.end || parser.content[parser.offset] != '\'' && parser.content[parser.offset] != '"' {
		return legacyBootstrapMatch{}, false
	}
	quote := parser.content[parser.offset]
	valueStart := parser.offset + 1
	units := make([]sourceByte, 0, 32)
	for offset := valueStart; offset < parser.end; {
		switch parser.content[offset] {
		case '\n', '\r':
			return legacyBootstrapMatch{}, false
		case '\\':
			if offset+1 >= parser.end {
				return legacyBootstrapMatch{}, false
			}
			var decoded byte
			switch parser.content[offset+1] {
			case 't':
				decoded = '\t'
			case 'n':
				decoded = '\n'
			case 'r':
				decoded = '\r'
			default:
				return legacyBootstrapMatch{}, false
			}
			units = append(units, sourceByte{
				value: decoded,
				start: offset - valueStart,
				end:   offset + 2 - valueStart,
			})
			offset += 2
		case quote:
			value := parser.content[valueStart:offset]
			for _, legacy := range []string{"bundle.wasm", "main.wasm"} {
				reference, ok := matchLegacyURL(value, units, legacy)
				if !ok {
					continue
				}
				parser.offset = offset + 1
				return legacyBootstrapMatch{
					urlStart: valueStart + reference.start,
					urlEnd:   valueStart + reference.end,
					suffix:   reference.suffix,
					quote:    quote,
				}, true
			}
			return legacyBootstrapMatch{}, false
		default:
			units = append(units, sourceByte{
				value: parser.content[offset],
				start: offset - valueStart,
				end:   offset + 1 - valueStart,
			})
			offset++
		}
	}
	return legacyBootstrapMatch{}, false
}

func (parser *legacyBootstrapParser) word(value string) bool {
	parser.skipSpace()
	end := parser.offset + len(value)
	if end > parser.end || parser.content[parser.offset:end] != value {
		return false
	}
	if end < parser.end && isLegacyBootstrapIdentifierPart(parser.content[end]) {
		return false
	}
	parser.offset = end
	return true
}

func (parser *legacyBootstrapParser) token(value string) bool {
	parser.skipSpace()
	end := parser.offset + len(value)
	if end > parser.end || parser.content[parser.offset:end] != value {
		return false
	}
	parser.offset = end
	return true
}

func (parser *legacyBootstrapParser) optionalToken(value string) {
	parser.skipSpace()
	end := parser.offset + len(value)
	if end <= parser.end && parser.content[parser.offset:end] == value {
		parser.offset = end
	}
}

func (parser *legacyBootstrapParser) complete() bool {
	parser.skipSpace()
	return parser.offset == parser.end
}

func (parser *legacyBootstrapParser) skipSpace() {
	for parser.offset < parser.end && isHTMLSpace(parser.content[parser.offset]) {
		parser.offset++
	}
}

func isLegacyBootstrapIdentifierPart(value byte) bool {
	return value == '_' || value == '$' || value >= 'A' && value <= 'Z' ||
		value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}
