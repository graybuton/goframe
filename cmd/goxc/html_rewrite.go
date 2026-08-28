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
	finalSize := int64(len(plan.content))
	for index, replacement := range replacements {
		if replacement.start < 0 || replacement.end < replacement.start || replacement.end > len(plan.content) {
			return "", fmt.Errorf("custom index rewrite produced an invalid %s span", replacement.description)
		}
		finalSize += int64(len(replacement.value)) - int64(replacement.end-replacement.start)
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
	if len(replacements) == 0 {
		return plan.content, nil
	}
	if finalSize < 0 || uint64(finalSize) > uint64(^uint(0)>>1) {
		return "", fmt.Errorf("custom index rewrite result is too large")
	}

	var result strings.Builder
	result.Grow(int(finalSize))
	cursor := 0
	for _, replacement := range replacements {
		result.WriteString(plan.content[cursor:replacement.start])
		result.WriteString(replacement.value)
		cursor = replacement.end
	}
	result.WriteString(plan.content[cursor:])
	return result.String(), nil
}

type scannedHTML struct {
	comments []htmlComment
	tags     []htmlTag
	profile  htmlRewriteProfile
}

type htmlComment struct {
	start         int
	end           int
	eof           bool
	sourceContext managedSourceContext
}

type htmlTag struct {
	start                   int
	end                     int
	name                    string
	namespace               htmlNamespace
	closing                 bool
	selfClosing             bool
	attributes              []htmlAttribute
	templateDepth           int
	ordinaryTemplateDepth   int
	declarativeShadowMode   string
	foreignNamespaceCertain bool
	rawStart                int
	rawEnd                  int
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
	sourceStart             int
	foreignAncestor         bool
	htmlIntegrationPoint    bool
	mathMLTextIntegrationPt bool
	stableBrowserParent     bool
	previousSameName        int
}

type htmlScannerContext struct {
	elements  []htmlElementContext
	topByName map[string]int
	uncertain bool
	seenHead  bool
	seenBody  bool
}

type htmlClosingResolution struct {
	index       int
	namespace   htmlNamespace
	matched     bool
	lookupCount int
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
				start:         offset,
				end:           end,
				eof:           eof,
				sourceContext: context.managedSourceContext(),
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
		if strings.HasPrefix(content[offset:], "<?") {
			offset = scanHTMLBogusCommentEnd(content, offset+2)
			continue
		}
		if strings.HasPrefix(content[offset:], "<!") && !asciiFoldPrefixAt(content, offset+2, "doctype") {
			offset = scanHTMLBogusCommentEnd(content, offset+2)
			continue
		}
		if strings.HasPrefix(content[offset:], "<!") {
			offset = scanHTMLDoctypeEnd(content, offset+2)
			continue
		}
		if strings.HasPrefix(content[offset:], "</") && offset+2 < len(content) &&
			content[offset+2] != '>' && !isHTMLNameStart(content[offset+2]) {
			offset = scanHTMLBogusCommentEnd(content, offset+2)
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
		var closing htmlClosingResolution
		if context.applyForeignBreakout(tag) {
			document.profile.markComplex("foreign-content parser recovery")
			context.uncertain = true
			if tag.closing {
				tag.namespace = htmlNamespaceHTML
				closing = context.resolveClosingTag(tag.name)
			} else {
				tag.namespace = namespaceForHTMLStartTag(tag.name)
			}
		} else if tag.closing {
			closing = context.resolveClosingTag(tag.name)
			tag.namespace = closing.namespace
		} else {
			tag.namespace = context.startTagNamespace(tag.name)
		}
		if tag.closing && (!closing.matched || closing.index != len(context.elements)-1) {
			context.uncertain = true
		}
		if !tag.closing && tag.namespace == htmlNamespaceHTML {
			context.markManagedDocumentContainerInstability(tag.name)
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
			context.close(closing)
		} else if tag.namespace == htmlNamespaceHTML {
			if htmlStartTagRemainsOpenInScanner(tag.name) {
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
	keep := len(context.elements)
	for keep > 0 {
		current := context.elements[keep-1]
		if current.namespace == htmlNamespaceHTML || current.htmlIntegrationPoint || current.mathMLTextIntegrationPt {
			break
		}
		keep--
	}
	context.truncate(keep)
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
	if context.topByName == nil {
		context.topByName = map[string]int{}
	}
	previousSameName := -1
	if previous, ok := context.topByName[tag.name]; ok {
		previousSameName = previous
	}
	foreignAncestor := tag.namespace != htmlNamespaceHTML
	if len(context.elements) != 0 && context.elements[len(context.elements)-1].foreignAncestor {
		foreignAncestor = true
	}
	stableBrowserParent := true
	if tag.namespace == htmlNamespaceHTML {
		switch tag.name {
		case "head":
			stableBrowserParent = !context.seenHead && !context.seenBody && context.canStartDocumentContainer("head")
			context.seenHead = true
		case "body":
			stableBrowserParent = !context.seenBody && context.canStartDocumentContainer("body")
			context.seenBody = true
		}
	}
	element := htmlElementContext{
		name:                tag.name,
		namespace:           tag.namespace,
		sourceStart:         tag.start,
		foreignAncestor:     foreignAncestor,
		stableBrowserParent: stableBrowserParent,
		previousSameName:    previousSameName,
	}
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
	context.topByName[tag.name] = len(context.elements) - 1
}

func (context htmlScannerContext) canStartDocumentContainer(name string) bool {
	if len(context.elements) == 0 {
		return true
	}
	parent := context.elements[len(context.elements)-1]
	if parent.namespace != htmlNamespaceHTML {
		return false
	}
	if parent.name == "html" {
		return true
	}
	return name == "body" && parent.name == "head"
}

// Managed blocks only use head and body as browser structural parents. A body
// content token ends head insertion even when the source omits </head>.
func (context *htmlScannerContext) markManagedDocumentContainerInstability(startName string) {
	if htmlStartTagStaysInHead(startName) {
		return
	}
	for index := len(context.elements) - 1; index >= 0; index-- {
		element := context.elements[index]
		if element.name != "head" || element.namespace != htmlNamespaceHTML {
			continue
		}
		for unstable := index; unstable < len(context.elements); unstable++ {
			context.elements[unstable].stableBrowserParent = false
		}
		return
	}
}

func htmlStartTagStaysInHead(name string) bool {
	switch name {
	case "html", "base", "basefont", "bgsound", "link", "meta", "noframes", "noscript", "script", "style", "template", "title":
		return true
	default:
		return false
	}
}

func (context htmlScannerContext) resolveClosingTag(name string) htmlClosingResolution {
	resolution := htmlClosingResolution{namespace: context.currentNamespace(), lookupCount: 1}
	index, ok := context.topByName[name]
	if !ok {
		return resolution
	}
	resolution.index = index
	resolution.namespace = context.elements[index].namespace
	resolution.matched = true
	return resolution
}

func (context *htmlScannerContext) close(resolution htmlClosingResolution) {
	if resolution.matched {
		context.truncate(resolution.index)
	}
}

func (context *htmlScannerContext) truncate(length int) {
	for index := len(context.elements) - 1; index >= length; index-- {
		element := context.elements[index]
		if element.previousSameName < 0 {
			delete(context.topByName, element.name)
		} else {
			context.topByName[element.name] = element.previousSameName
		}
	}
	context.elements = context.elements[:length]
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

// Some obsolete HTML elements are not void, but browser tree construction
// inserts and immediately pops them. They therefore cannot become the source
// scanner parent of following content.
func htmlStartTagRemainsOpenInScanner(name string) bool {
	if voidHTMLElement(name) {
		return false
	}
	switch name {
	case "basefont", "bgsound":
		return false
	default:
		return true
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

func scanHTMLDoctypeEnd(content string, start int) int {
	if end := strings.IndexByte(content[start:], '>'); end >= 0 {
		return start + end + 1
	}
	return len(content)
}

func scanHTMLBogusCommentEnd(content string, start int) int {
	if end := strings.IndexByte(content[start:], '>'); end >= 0 {
		return start + end + 1
	}
	return len(content)
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
	tag := htmlTag{start: start, name: name, closing: closing}
	if err := scanHTMLTagAttributes(content, offset, &tag); err != nil {
		return htmlTag{}, false, err
	}
	return tag, true, nil
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

type htmlTagAttributeState uint8

const (
	htmlBeforeAttributeName htmlTagAttributeState = iota
	htmlAttributeName
	htmlAfterAttributeName
	htmlBeforeAttributeValue
	htmlTagAttributeValueDoubleQuoted
	htmlTagAttributeValueSingleQuoted
	htmlTagAttributeValueUnquoted
	htmlAfterAttributeValueQuoted
	htmlSelfClosingStartTag
)

// scanHTMLTagAttributes owns both normal-tag termination and attribute value
// syntax. Quotes receive delimiter semantics only after '=' selects a quoted
// value state; inside an unquoted value they remain literal parse-error bytes.
func scanHTMLTagAttributes(content string, start int, tag *htmlTag) error {
	state := htmlBeforeAttributeName
	offset := start
	nameStart := 0
	attributeIndex := -1

	finishAttributeName := func(end int) {
		tag.attributes = append(tag.attributes, htmlAttribute{
			name: semanticHTMLTagName(content[nameStart:end]),
		})
		attributeIndex = len(tag.attributes) - 1
	}
	finishTag := func(end int, selfClosing bool) {
		tag.end = end
		tag.selfClosing = selfClosing
	}

	for offset < len(content) {
		current := content[offset]
		switch state {
		case htmlBeforeAttributeName:
			switch {
			case isHTMLSpace(current):
				offset++
			case current == '/':
				state = htmlSelfClosingStartTag
				offset++
			case current == '>':
				finishTag(offset+1, false)
				return nil
			case current == '=':
				nameStart = offset
				state = htmlAttributeName
				offset++
			default:
				nameStart = offset
				state = htmlAttributeName
			}

		case htmlAttributeName:
			switch {
			case isHTMLSpace(current):
				finishAttributeName(offset)
				state = htmlAfterAttributeName
				offset++
			case current == '/':
				finishAttributeName(offset)
				state = htmlSelfClosingStartTag
				offset++
			case current == '=':
				finishAttributeName(offset)
				tag.attributes[attributeIndex].hasValue = true
				state = htmlBeforeAttributeValue
				offset++
			case current == '>':
				finishAttributeName(offset)
				finishTag(offset+1, false)
				return nil
			default:
				offset++
			}

		case htmlAfterAttributeName:
			switch {
			case isHTMLSpace(current):
				offset++
			case current == '/':
				state = htmlSelfClosingStartTag
				offset++
			case current == '=':
				tag.attributes[attributeIndex].hasValue = true
				state = htmlBeforeAttributeValue
				offset++
			case current == '>':
				finishTag(offset+1, false)
				return nil
			default:
				state = htmlBeforeAttributeName
			}

		case htmlBeforeAttributeValue:
			attribute := &tag.attributes[attributeIndex]
			switch {
			case isHTMLSpace(current):
				offset++
			case current == '"':
				attribute.valueSyntax = htmlAttributeValueDoubleQuoted
				attribute.valueStart = offset + 1
				state = htmlTagAttributeValueDoubleQuoted
				offset++
			case current == '\'':
				attribute.valueSyntax = htmlAttributeValueSingleQuoted
				attribute.valueStart = offset + 1
				state = htmlTagAttributeValueSingleQuoted
				offset++
			case current == '>':
				attribute.valueSyntax = htmlAttributeValueUnquoted
				attribute.valueStart = offset
				attribute.valueEnd = offset
				finishTag(offset+1, false)
				return nil
			default:
				attribute.valueSyntax = htmlAttributeValueUnquoted
				attribute.valueStart = offset
				state = htmlTagAttributeValueUnquoted
			}

		case htmlTagAttributeValueDoubleQuoted:
			if current == '"' {
				tag.attributes[attributeIndex].valueEnd = offset
				state = htmlAfterAttributeValueQuoted
			}
			offset++

		case htmlTagAttributeValueSingleQuoted:
			if current == '\'' {
				tag.attributes[attributeIndex].valueEnd = offset
				state = htmlAfterAttributeValueQuoted
			}
			offset++

		case htmlTagAttributeValueUnquoted:
			switch {
			case isHTMLSpace(current):
				tag.attributes[attributeIndex].valueEnd = offset
				state = htmlBeforeAttributeName
				offset++
			case current == '>':
				tag.attributes[attributeIndex].valueEnd = offset
				finishTag(offset+1, false)
				return nil
			default:
				offset++
			}

		case htmlAfterAttributeValueQuoted:
			switch {
			case isHTMLSpace(current):
				state = htmlBeforeAttributeName
				offset++
			case current == '/':
				state = htmlSelfClosingStartTag
				offset++
			case current == '>':
				finishTag(offset+1, false)
				return nil
			default:
				state = htmlBeforeAttributeName
			}

		case htmlSelfClosingStartTag:
			if current == '>' {
				finishTag(offset+1, true)
				return nil
			}
			return fmt.Errorf("custom index <%s> has a malformed solidus near byte %d", tag.name, offset-1)
		}
	}

	if state == htmlTagAttributeValueDoubleQuoted || state == htmlTagAttributeValueSingleQuoted {
		return fmt.Errorf(
			"custom index <%s> attribute %s has an unterminated quoted value",
			tag.name,
			tag.attributes[attributeIndex].name,
		)
	}
	return fmt.Errorf("custom index <%s> tag is unterminated", tag.name)
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
	pairs := map[string][2]managedMarker{}
	for _, comment := range document.comments {
		raw := content[comment.start:comment.end]
		for _, name := range names {
			for _, start := range []bool{true, false} {
				if raw == managedMarkerText(name, start) {
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
		pairs[name] = [2]managedMarker{starts[0], ends[0]}
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
	for _, name := range names {
		pair, ok := pairs[name]
		if !ok {
			continue
		}
		if err := validateManagedBlockContexts(name, pair[0], pair[1]); err != nil {
			return nil, err
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
	managedOperations := map[string]struct {
		name        string
		alternative string
	}{
		preloadBlockName: {
			name:        "goframe:preload output",
			alternative: "remove the base element or disable GoFrame-managed preload output",
		},
		runtimeBlockName: {
			name:        "goframe:runtime output",
			alternative: "remove the base element or provide a deployment-safe external runtime integration outside GoFrame ownership",
		},
		bootstrapBlockName: {
			name:        "goframe:bootstrap output",
			alternative: "remove the base element or use an external deployment-safe loader outside GoFrame ownership",
		},
	}
	var ownedRuntimes []ownedRuntimeIntegration
	var ownedBootstraps []ownedBootstrapIntegration
	for _, name := range []string{preloadBlockName, runtimeBlockName, bootstrapBlockName} {
		block, ok := blocks[name]
		if !ok {
			continue
		}
		if managedValues[name] != "" {
			operation := managedOperations[name]
			if err := document.profile.requireRelativePackageURLSafety(operation.name, operation.alternative); err != nil {
				return "", err
			}
		}
		plan.add(htmlReplacement{
			start:       block.start,
			end:         block.end,
			value:       managedMarkerText(name, true) + "\n" + managedValues[name] + "\n" + managedMarkerText(name, false),
			description: "goframe:" + name + " managed block",
		})
		switch {
		case name == runtimeBlockName && managedValues[name] != "":
			ownedRuntimes = append(ownedRuntimes, ownedRuntimeIntegration{
				offset:    block.start,
				execution: ownedRuntimeParserBlocking,
			})
		case name == bootstrapBlockName && managedValues[name] != "":
			ownedBootstraps = append(ownedBootstraps, ownedBootstrapIntegration{offset: block.start})
		}
	}

	if _, managed := blocks[runtimeBlockName]; !managed {
		legacyRuntimes, err := planLegacyRuntimeRewrites(&plan, document, blocks, options.runtimePath)
		if err != nil {
			return "", err
		}
		ownedRuntimes = append(ownedRuntimes, legacyRuntimes...)
	}
	if _, managed := blocks[bootstrapBlockName]; !managed {
		legacyBootstraps, err := planLegacyWASMRewrites(&plan, document, blocks, options.wasmPath)
		if err != nil {
			return "", err
		}
		ownedBootstraps = append(ownedBootstraps, legacyBootstraps...)
	}
	if err := validateOwnedRuntimeBootstrapOrder(ownedRuntimes, ownedBootstraps); err != nil {
		return "", err
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

type ownedRuntimeExecution uint8

const (
	ownedRuntimeUnavailable ownedRuntimeExecution = iota
	ownedRuntimeNonBlocking
	ownedRuntimeParserBlocking
)

type ownedRuntimeIntegration struct {
	offset    int
	execution ownedRuntimeExecution
}

type ownedBootstrapIntegration struct {
	offset int
}

func validateOwnedRuntimeBootstrapOrder(runtimes []ownedRuntimeIntegration, bootstraps []ownedBootstrapIntegration) error {
	if len(runtimes) == 0 || len(bootstraps) == 0 {
		return nil
	}
	for _, bootstrap := range bootstraps {
		ordered := false
		for _, runtime := range runtimes {
			if runtime.execution == ownedRuntimeParserBlocking && runtime.offset < bootstrap.offset {
				ordered = true
				break
			}
		}
		if !ordered {
			return fmt.Errorf(
				"custom index GoFrame-owned bootstrap may execute before an executable blocking runtime; use a blocking classic runtime without nomodule/async/defer or use one external loader that owns both steps",
			)
		}
	}
	return nil
}

func planLegacyRuntimeRewrites(plan *htmlRewritePlan, document scannedHTML, blocks map[string]managedHTMLBlock, runtimePath string) ([]ownedRuntimeIntegration, error) {
	var runtimeURL string
	var integrations []ownedRuntimeIntegration
	for _, tag := range document.tags {
		if tag.closing || tag.name != "script" || (tag.namespace != htmlNamespaceHTML && tag.foreignNamespaceCertain) ||
			tag.ordinaryTemplateDepth != 0 || managedBlockContains(blocks, tag.start) {
			continue
		}
		source, err := tag.attribute("src")
		if err != nil {
			return nil, err
		}
		if source == nil || !source.hasValue {
			continue
		}
		kind, err := classifyScriptTag(plan.content, tag)
		if err != nil {
			return nil, err
		}
		if kind != scriptKindClassic && kind != scriptKindModule {
			continue
		}
		value := plan.content[source.valueStart:source.valueEnd]
		match, ok := matchLegacyURL(value, htmlAttributeSourceBytes(plan.content, source), runtimeAssetName)
		if !ok {
			continue
		}
		if err := document.profile.requireRelativePackageURLSafety(
			"markerless runtime rewrite",
			"remove the base element or provide a deployment-safe external runtime integration outside GoFrame ownership",
		); err != nil {
			return nil, err
		}
		if document.profile.complex {
			return nil, document.profile.markerlessError("runtime", runtimeBlockName)
		}
		if runtimeURL == "" {
			runtimeURL, err = encodePackagePathAsBrowserURL(runtimePath)
			if err != nil {
				return nil, err
			}
		}
		replacement, err := encodeGeneratedHTMLAttributeValue(runtimeURL, source.valueSyntax)
		if err != nil {
			return nil, err
		}
		plan.add(htmlReplacement{
			start:       source.valueStart + match.start,
			end:         source.valueStart + match.end,
			value:       replacement + match.suffix,
			description: "legacy runtime script source",
		})
		execution, err := classifyOwnedRuntimeExecution(plan.content, tag, kind)
		if err != nil {
			return nil, err
		}
		integrations = append(integrations, ownedRuntimeIntegration{offset: tag.start, execution: execution})
	}
	return integrations, nil
}

func classifyOwnedRuntimeExecution(content string, tag htmlTag, kind scriptKind) (ownedRuntimeExecution, error) {
	if hasHTMLAttribute(tag, "nomodule") {
		return ownedRuntimeUnavailable, nil
	}
	if kind != scriptKindClassic || hasHTMLAttribute(tag, "async") || hasHTMLAttribute(tag, "defer") {
		return ownedRuntimeNonBlocking, nil
	}

	eventAttribute, err := tag.attribute("event")
	if err != nil {
		return ownedRuntimeUnavailable, err
	}
	forAttribute, err := tag.attribute("for")
	if err != nil {
		return ownedRuntimeUnavailable, err
	}
	if eventAttribute == nil || forAttribute == nil {
		return ownedRuntimeParserBlocking, nil
	}

	event := trimHTMLSpace(semanticHTMLAttributeValue(content, eventAttribute))
	target := trimHTMLSpace(semanticHTMLAttributeValue(content, forAttribute))
	if asciiFoldEqual(target, "window") && (asciiFoldEqual(event, "onload") || asciiFoldEqual(event, "onload()")) {
		return ownedRuntimeParserBlocking, nil
	}
	return ownedRuntimeUnavailable, nil
}

func planLegacyWASMRewrites(plan *htmlRewritePlan, document scannedHTML, blocks map[string]managedHTMLBlock, wasmPath string) ([]ownedBootstrapIntegration, error) {
	var wasmURL string
	var integrations []ownedBootstrapIntegration
	for _, tag := range document.tags {
		if tag.closing || tag.name != "script" || (tag.namespace != htmlNamespaceHTML && tag.foreignNamespaceCertain) ||
			tag.ordinaryTemplateDepth != 0 || managedBlockContains(blocks, tag.start) {
			continue
		}
		source, err := tag.attribute("src")
		if err != nil {
			return nil, err
		}
		if source != nil {
			continue
		}
		kind, err := classifyScriptTag(plan.content, tag)
		if err != nil {
			return nil, err
		}
		if kind != scriptKindClassic && kind != scriptKindModule {
			continue
		}
		match, ok := recognizeLegacyGoFrameBootstrap(plan.content, tag.rawStart, tag.rawEnd)
		if !ok {
			continue
		}
		if err := document.profile.requireRelativePackageURLSafety(
			"markerless bootstrap rewrite",
			"remove the base element or use an external deployment-safe loader outside GoFrame ownership",
		); err != nil {
			return nil, err
		}
		if document.profile.complex {
			return nil, document.profile.markerlessError("bootstrap", bootstrapBlockName)
		}
		if wasmURL == "" {
			wasmURL, err = encodePackagePathAsBrowserURL(wasmPath)
			if err != nil {
				return nil, err
			}
		}
		replacement, err := encodeGeneratedJavaScriptStringContents(wasmURL, match.quote)
		if err != nil {
			return nil, err
		}
		plan.add(htmlReplacement{
			start:       match.urlStart,
			end:         match.urlEnd,
			value:       replacement + match.suffix,
			description: "legacy GoFrame bootstrap WASM URL",
		})
		integrations = append(integrations, ownedBootstrapIntegration{offset: tag.start})
	}
	return integrations, nil
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
		if tag.closing || tag.name != "link" || (tag.namespace != htmlNamespaceHTML && tag.foreignNamespaceCertain) ||
			tag.ordinaryTemplateDepth != 0 || managedBlockContains(blocks, tag.start) {
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
		operation := "markerless stylesheet rewrite"
		if stylePreload && !stylesheet {
			operation = "markerless style preload rewrite"
		}
		if tag.declarativeShadowMode != "" {
			return fmt.Errorf(
				"custom index stylesheet rewriting inside declarative Shadow DOM is not part of the preview markerless contract; use an external stable URL or remove asset-managed shadow-root styles",
			)
		}
		if err := document.profile.requireRelativePackageURLSafety(
			operation,
			"remove the base element or use a deployment-safe external stylesheet outside GoFrame ownership",
		); err != nil {
			return err
		}
		if document.profile.complex {
			return document.profile.stylesheetError()
		}
		destinationURL, err := encodePackagePathAsBrowserURL(destination)
		if err != nil {
			return err
		}
		replacement, err := encodeGeneratedHTMLAttributeValue(destinationURL, href.valueSyntax)
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
	if err := document.profile.requireRelativePackageURLSafety(
		"preload insertion",
		"remove the base element or disable GoFrame-managed preload output",
	); err != nil {
		return err
	}
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
	value, ok := decodeLegacyURLPathOnce(value)
	if !ok || value == "" || value[0] == '/' || value[0] == '\\' {
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
		case segment == ".":
			if offset == len(value) {
				segments = append(segments, "")
			}
		case segment == "..":
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
		!parser.tokenWithoutLineTerminator("=>") || !parser.word("go") || !parser.token(".") ||
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
	if !parser.optionalToken(",") {
		return legacyBootstrapMatch{}, false
	}
	if !parser.token(")") {
		return legacyBootstrapMatch{}, false
	}
	return match, true
}

func (parser *legacyBootstrapParser) legacyWASMURL() (legacyBootstrapMatch, bool) {
	if _, ok := parser.skipTrivia(); !ok {
		return legacyBootstrapMatch{}, false
	}
	decoded, ok := decodeJavaScriptString(parser.content, parser.offset, parser.end)
	if !ok {
		return legacyBootstrapMatch{}, false
	}
	value := parser.content[decoded.valueStart:decoded.valueEnd]
	for _, legacy := range []string{"bundle.wasm", "main.wasm"} {
		reference, ok := matchLegacyURL(value, decoded.units, legacy)
		if !ok {
			continue
		}
		parser.offset = decoded.end
		return legacyBootstrapMatch{
			urlStart: decoded.valueStart + reference.start,
			urlEnd:   decoded.valueStart + reference.end,
			suffix:   reference.suffix,
			quote:    decoded.quote,
		}, true
	}
	return legacyBootstrapMatch{}, false
}

func (parser *legacyBootstrapParser) word(value string) bool {
	if _, ok := parser.skipTrivia(); !ok {
		return false
	}
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
	if _, ok := parser.skipTrivia(); !ok {
		return false
	}
	return parser.tokenAtCurrentOffset(value)
}

func (parser *legacyBootstrapParser) tokenWithoutLineTerminator(value string) bool {
	// The generated arrow is the only supported historical shape with a
	// restricted-production boundary.
	trivia, ok := parser.skipTrivia()
	if !ok || trivia.sawLineTerminator {
		return false
	}
	return parser.tokenAtCurrentOffset(value)
}

func (parser *legacyBootstrapParser) tokenAtCurrentOffset(value string) bool {
	end := parser.offset + len(value)
	if end > parser.end || parser.content[parser.offset:end] != value {
		return false
	}
	parser.offset = end
	return true
}

func (parser *legacyBootstrapParser) optionalToken(value string) bool {
	if _, ok := parser.skipTrivia(); !ok {
		return false
	}
	end := parser.offset + len(value)
	if end <= parser.end && parser.content[parser.offset:end] == value {
		parser.offset = end
	}
	return true
}

func (parser *legacyBootstrapParser) complete() bool {
	_, ok := parser.skipTrivia()
	return ok && parser.offset == parser.end
}

func isLegacyBootstrapIdentifierPart(value byte) bool {
	return value == '_' || value == '$' || value >= 'A' && value <= 'Z' ||
		value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}
