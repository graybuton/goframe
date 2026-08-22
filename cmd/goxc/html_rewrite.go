package main

import (
	"fmt"
	"sort"
	"strings"
)

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
}

type htmlComment struct {
	start         int
	end           int
	templateDepth int
}

type htmlTag struct {
	start         int
	end           int
	name          string
	closing       bool
	selfClosing   bool
	attributes    []htmlAttribute
	templateDepth int
	rawStart      int
	rawEnd        int
}

type htmlAttribute struct {
	name       string
	valueStart int
	valueEnd   int
	hasValue   bool
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
	templateDepth := 0
	for offset := 0; offset < len(content); {
		if content[offset] != '<' {
			offset++
			continue
		}
		if strings.HasPrefix(content[offset:], "<!--") {
			end := strings.Index(content[offset+4:], "-->")
			if end < 0 {
				return scannedHTML{}, fmt.Errorf("custom index has an unterminated HTML comment; close it with -->")
			}
			end += offset + 7
			document.comments = append(document.comments, htmlComment{
				start:         offset,
				end:           end,
				templateDepth: templateDepth,
			})
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
		if tag.closing && tag.name == "template" {
			if templateDepth == 0 {
				return scannedHTML{}, fmt.Errorf("custom index has a closing </template> without a matching opening tag")
			}
			templateDepth--
		}
		tag.templateDepth = templateDepth
		if !tag.closing && opaqueHTMLTextElement(tag.name) {
			closing, err := scanRawElementClose(content, tag.end, tag.name)
			if err != nil {
				return scannedHTML{}, err
			}
			tag.rawStart = tag.end
			tag.rawEnd = closing.start
			document.tags = append(document.tags, tag)
			closing.templateDepth = templateDepth
			document.tags = append(document.tags, closing)
			offset = closing.end
			continue
		}
		document.tags = append(document.tags, tag)
		if !tag.closing && tag.name == "template" {
			templateDepth++
		}
		offset = tag.end
	}
	if templateDepth != 0 {
		return scannedHTML{}, fmt.Errorf("custom index has an opening <template> without a matching closing tag")
	}
	return document, nil
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
	for offset < len(content) && isHTMLNameByte(content[offset]) {
		offset++
	}
	name := asciiLower(content[nameStart:offset])
	tagEnd, err := scanHTMLConstructEnd(content, offset)
	if err != nil {
		return htmlTag{}, false, fmt.Errorf("custom index <%s> tag is unterminated: %w", name, err)
	}
	limit := tagEnd - 1
	last := limit - 1
	for last >= offset && isHTMLSpace(content[last]) {
		last--
	}
	selfClosing := last >= offset && content[last] == '/' &&
		(last == offset || isHTMLSpace(content[last-1]) || content[last-1] == '\'' || content[last-1] == '"')
	if selfClosing {
		limit = last
	}
	attributes, err := scanHTMLAttributes(content, offset, limit, name)
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

func scanHTMLAttributes(content string, start, end int, tagName string) ([]htmlAttribute, error) {
	var attributes []htmlAttribute
	for offset := start; offset < end; {
		for offset < end && isHTMLSpace(content[offset]) {
			offset++
		}
		if offset >= end {
			break
		}
		nameStart := offset
		for offset < end && !isHTMLSpace(content[offset]) && content[offset] != '=' && content[offset] != '/' && content[offset] != '>' {
			offset++
		}
		if nameStart == offset {
			return nil, fmt.Errorf("custom index <%s> has a malformed attribute near byte %d", tagName, offset)
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
				return nil, fmt.Errorf("custom index <%s> attribute %s has no value", tagName, attribute.name)
			}
			if content[offset] == '\'' || content[offset] == '"' {
				quote := content[offset]
				offset++
				attribute.valueStart = offset
				for offset < end && content[offset] != quote {
					offset++
				}
				if offset >= end {
					return nil, fmt.Errorf("custom index <%s> attribute %s has an unterminated quoted value", tagName, attribute.name)
				}
				attribute.valueEnd = offset
				offset++
			} else {
				attribute.valueStart = offset
				for offset < end && !isHTMLSpace(content[offset]) && content[offset] != '>' {
					offset++
				}
				attribute.valueEnd = offset
				if attribute.valueStart == attribute.valueEnd {
					return nil, fmt.Errorf("custom index <%s> attribute %s has an empty unquoted value", tagName, attribute.name)
				}
			}
		}
		attributes = append(attributes, attribute)
	}
	return attributes, nil
}

func scanRawElementClose(content string, start int, name string) (htmlTag, error) {
	needle := "</" + name
	for offset := start; offset+len(needle) <= len(content); offset++ {
		if !asciiFoldEqual(content[offset:offset+len(needle)], needle) {
			continue
		}
		afterName := offset + len(needle)
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

func isHTMLNameByte(value byte) bool {
	return isHTMLNameStart(value) || value >= '0' && value <= '9' || value == '-' || value == ':'
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

func asciiFoldEqual(first, second string) bool {
	return len(first) == len(second) && asciiLower(first) == asciiLower(second)
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
		if comment.templateDepth != 0 {
			continue
		}
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

	managedValues := map[string]string{
		preloadBlockName:   preloadHTML(options),
		runtimeBlockName:   runtimeHTML(options),
		bootstrapBlockName: bootstrapHTML(options),
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
		if err := planPreloadInsertion(&plan, document, blocks, preloadHTML(options)); err != nil {
			return "", err
		}
	}
	return plan.apply()
}

func planLegacyRuntimeRewrites(plan *htmlRewritePlan, document scannedHTML, blocks map[string]managedHTMLBlock, runtimePath string) error {
	for _, tag := range document.tags {
		if tag.closing || tag.name != "script" || tag.templateDepth != 0 || managedBlockContains(blocks, tag.start) {
			continue
		}
		source, err := tag.attribute("src")
		if err != nil {
			return err
		}
		if source == nil || !source.hasValue {
			continue
		}
		typeAttribute, err := tag.attribute("type")
		if err != nil {
			return err
		}
		if !executableScriptType(plan.content, typeAttribute) {
			continue
		}
		value := plan.content[source.valueStart:source.valueEnd]
		suffix, ok := legacyURLSuffix(value, runtimeAssetName)
		if !ok {
			continue
		}
		plan.add(htmlReplacement{
			start:       source.valueStart,
			end:         source.valueEnd,
			value:       runtimePath + suffix,
			description: "legacy runtime script source",
		})
	}
	return nil
}

func planLegacyWASMRewrites(plan *htmlRewritePlan, document scannedHTML, blocks map[string]managedHTMLBlock, wasmPath string) error {
	for _, tag := range document.tags {
		if tag.closing || tag.name != "script" || tag.templateDepth != 0 || managedBlockContains(blocks, tag.start) {
			continue
		}
		source, err := tag.attribute("src")
		if err != nil {
			return err
		}
		if source != nil {
			continue
		}
		typeAttribute, err := tag.attribute("type")
		if err != nil {
			return err
		}
		if !executableScriptType(plan.content, typeAttribute) {
			continue
		}
		match, ok := recognizeLegacyGoFrameBootstrap(plan.content, tag.rawStart, tag.rawEnd)
		if !ok {
			continue
		}
		plan.add(htmlReplacement{
			start:       match.urlStart,
			end:         match.urlEnd,
			value:       wasmPath + match.suffix,
			description: "legacy GoFrame bootstrap WASM URL",
		})
	}
	return nil
}

func executableScriptType(content string, typeAttribute *htmlAttribute) bool {
	if typeAttribute == nil || !typeAttribute.hasValue {
		return true
	}
	value := asciiLower(strings.TrimSpace(content[typeAttribute.valueStart:typeAttribute.valueEnd]))
	if separator := strings.IndexByte(value, ';'); separator >= 0 {
		value = strings.TrimSpace(value[:separator])
	}
	switch value {
	case "", "module", "text/javascript", "application/javascript", "text/ecmascript", "application/ecmascript", "text/jscript":
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
		if tag.closing || tag.name != "link" || tag.templateDepth != 0 || managedBlockContains(blocks, tag.start) {
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
		relTokens := asciiTokenSet(plan.content[rel.valueStart:rel.valueEnd])
		stylesheet := relTokens["stylesheet"]
		stylePreload := relTokens["preload"] && as != nil && as.hasValue && asciiFoldEqual(strings.TrimSpace(plan.content[as.valueStart:as.valueEnd]), "style")
		if !stylesheet && (!stylePreload || preloadManaged) {
			continue
		}
		value := plan.content[href.valueStart:href.valueEnd]
		base, suffix := splitLegacyURL(value)
		base = strings.TrimPrefix(base, "./")
		destination, ok := rewrites[base]
		if !ok {
			continue
		}
		plan.add(htmlReplacement{
			start:       href.valueStart,
			end:         href.valueEnd,
			value:       destination + suffix,
			description: "legacy stylesheet reference",
		})
	}
	return nil
}

func asciiTokenSet(value string) map[string]bool {
	result := map[string]bool{}
	for _, token := range strings.Fields(value) {
		result[asciiLower(token)] = true
	}
	return result
}

func planPreloadInsertion(plan *htmlRewritePlan, document scannedHTML, blocks map[string]managedHTMLBlock, preload string) error {
	var closingHeads []htmlTag
	for _, tag := range document.tags {
		if tag.closing && tag.name == "head" && tag.templateDepth == 0 && !managedBlockContains(blocks, tag.start) {
			closingHeads = append(closingHeads, tag)
		}
	}
	if len(closingHeads) == 0 {
		return fmt.Errorf("custom index cannot receive preload hints; add a closing </head> tag or an explicit goframe:preload block")
	}
	if len(closingHeads) > 1 {
		return fmt.Errorf("custom index has multiple structural </head> tags; keep one closing </head> tag or add an explicit goframe:preload block")
	}
	plan.add(htmlReplacement{
		start:       closingHeads[0].start,
		end:         closingHeads[0].start,
		value:       preload + "\n",
		description: "preload insertion",
	})
	return nil
}

func legacyURLSuffix(value, expected string) (string, bool) {
	base, suffix := splitLegacyURL(value)
	if strings.TrimPrefix(base, "./") != expected {
		return "", false
	}
	return suffix, true
}

func splitLegacyURL(value string) (string, string) {
	if index := strings.IndexAny(value, "?#"); index >= 0 {
		return value[:index], value[index:]
	}
	return value, ""
}

type legacyBootstrapMatch struct {
	urlStart int
	urlEnd   int
	suffix   string
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
	for offset := valueStart; offset < parser.end; offset++ {
		switch parser.content[offset] {
		case '\\', '\n', '\r':
			return legacyBootstrapMatch{}, false
		case quote:
			value := parser.content[valueStart:offset]
			for _, legacy := range []string{"bundle.wasm", "main.wasm"} {
				suffix, ok := legacyURLSuffix(value, legacy)
				if !ok {
					continue
				}
				parser.offset = offset + 1
				return legacyBootstrapMatch{
					urlStart: valueStart,
					urlEnd:   offset,
					suffix:   suffix,
				}, true
			}
			return legacyBootstrapMatch{}, false
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
