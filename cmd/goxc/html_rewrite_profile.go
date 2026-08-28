package main

import (
	"fmt"
	"strings"
)

type htmlRewriteProfile struct {
	complex          bool
	construct        string
	activeBaseHref   bool
	activeBaseOffset int
}

func (profile *htmlRewriteProfile) markComplex(construct string) {
	if profile.complex {
		return
	}
	profile.complex = true
	profile.construct = construct
}

func (profile *htmlRewriteProfile) markActiveBaseHref(offset int) {
	if profile.activeBaseHref {
		return
	}
	profile.activeBaseHref = true
	profile.activeBaseOffset = offset
}

func (profile htmlRewriteProfile) requireRelativePackageURLSafety(operation, alternative string) error {
	if !profile.activeBaseHref {
		return nil
	}
	return fmt.Errorf(
		"custom index %s cannot use package-relative URLs with an active <base href> near byte %d; %s",
		operation,
		profile.activeBaseOffset,
		alternative,
	)
}

func (profile htmlRewriteProfile) markerlessError(operation, managedBlock string) error {
	return fmt.Errorf(
		"custom index markerless %s rewriting is unavailable with %s; move the integration to a safe top-level goframe:%s block",
		operation,
		profile.construct,
		managedBlock,
	)
}

func (profile htmlRewriteProfile) stylesheetError() error {
	return fmt.Errorf(
		"custom index markerless stylesheet rewriting is unavailable with %s; use an external stable URL or a simple-profile document",
		profile.construct,
	)
}

func (profile htmlRewriteProfile) preloadError() error {
	return fmt.Errorf(
		"custom index preload insertion is unavailable with %s; use an explicit top-level goframe:preload block",
		profile.construct,
	)
}

type htmlProfileElement struct {
	name                  string
	tagIndex              int
	previousSameName      int
	previousUnsupported   string
	previousShadowMode    string
	previousTemplateDepth int
}

type htmlProfileContext struct {
	elements              []htmlProfileElement
	topByName             map[string]int
	unsupportedEnclosing  string
	declarativeShadowMode string
	ordinaryTemplateDepth int
}

func classifyHTMLRewriteProfile(content string, document *scannedHTML) {
	context := htmlProfileContext{topByName: map[string]int{}}
	tagIndex := 0
	commentIndex := 0
	for tagIndex < len(document.tags) || commentIndex < len(document.comments) {
		if commentIndex < len(document.comments) &&
			(tagIndex == len(document.tags) || document.comments[commentIndex].start < document.tags[tagIndex].start) {
			comment := &document.comments[commentIndex]
			comment.sourceContext.unsupported = context.unsupportedEnclosing
			comment.sourceContext.ordinaryTemplate = context.ordinaryTemplateDepth != 0
			comment.sourceContext.declarativeShadow = context.declarativeShadowMode != ""
			commentIndex++
			continue
		}

		tag := &document.tags[tagIndex]
		tag.declarativeShadowMode = context.declarativeShadowMode
		tag.ordinaryTemplateDepth = context.ordinaryTemplateDepth
		tag.foreignNamespaceCertain = tag.namespace != htmlNamespaceHTML && context.unsupportedEnclosing == ""
		if potentiallyActiveBaseHref(*tag) {
			document.profile.markActiveBaseHref(tag.start)
		}
		if tag.closing {
			openingIndex, matched := context.close(tag.name, &document.profile)
			if matched && tag.name == "script" && document.tags[openingIndex].rawStart == 0 {
				document.tags[openingIndex].rawStart = document.tags[openingIndex].end
				document.tags[openingIndex].rawEnd = tag.start
			}
			tagIndex++
			continue
		}

		construct, shadowMode, ordinaryTemplate := htmlProfileStartTag(content, *tag)
		if construct != "" {
			document.profile.markComplex(construct)
		}
		if htmlProfileElementStaysOpen(*tag) {
			context.push(tag.name, tagIndex, managedUnsupportedConstruct(construct, ordinaryTemplate), shadowMode, ordinaryTemplate)
		}
		tagIndex++
	}

	appendNoscriptManagedMarkers(content, document)
}

func potentiallyActiveBaseHref(tag htmlTag) bool {
	return !tag.closing && tag.namespace == htmlNamespaceHTML && tag.name == "base" &&
		hasHTMLAttribute(tag, "href") && tag.ordinaryTemplateDepth == 0 && tag.declarativeShadowMode == ""
}

func htmlProfileStartTag(content string, tag htmlTag) (construct, shadowMode string, ordinaryTemplate bool) {
	if tag.namespace == htmlNamespaceHTML && tag.name == "template" {
		if value, ok := firstHTMLAttributeValue(content, tag, "shadowrootmode"); ok {
			switch {
			case asciiFoldEqual(value, "open"):
				return "declarative Shadow DOM", "open", false
			case asciiFoldEqual(value, "closed"):
				return "declarative Shadow DOM", "closed", false
			}
		}
		return "", "", true
	}
	if tag.namespace != htmlNamespaceHTML {
		return "", "", false
	}
	switch tag.name {
	case "select", "option", "optgroup",
		"table", "caption", "colgroup", "col", "tbody", "thead", "tfoot", "tr", "td", "th",
		"frameset", "frame", "noscript":
		return "<" + tag.name + ">", "", false
	default:
		return "", "", false
	}
}

func managedUnsupportedConstruct(construct string, ordinaryTemplate bool) string {
	if ordinaryTemplate {
		return "<template>"
	}
	if construct != "" {
		return construct
	}
	return ""
}

func hasHTMLAttribute(tag htmlTag, name string) bool {
	for _, attribute := range tag.attributes {
		if attribute.name == name {
			return true
		}
	}
	return false
}

func htmlProfileElementStaysOpen(tag htmlTag) bool {
	if tag.namespace == htmlNamespaceHTML {
		return !voidHTMLElement(tag.name)
	}
	return !tag.selfClosing
}

func (context *htmlProfileContext) push(name string, tagIndex int, unsupported, shadowMode string, ordinaryTemplate bool) {
	previousSameName := -1
	if previous, ok := context.topByName[name]; ok {
		previousSameName = previous
	}
	element := htmlProfileElement{
		name:                  name,
		tagIndex:              tagIndex,
		previousSameName:      previousSameName,
		previousUnsupported:   context.unsupportedEnclosing,
		previousShadowMode:    context.declarativeShadowMode,
		previousTemplateDepth: context.ordinaryTemplateDepth,
	}
	context.elements = append(context.elements, element)
	context.topByName[name] = len(context.elements) - 1
	if unsupported != "" {
		context.unsupportedEnclosing = unsupported
	}
	if shadowMode != "" {
		context.declarativeShadowMode = shadowMode
	}
	if ordinaryTemplate {
		context.ordinaryTemplateDepth++
	}
}

func (context *htmlProfileContext) close(name string, profile *htmlRewriteProfile) (int, bool) {
	index, ok := context.topByName[name]
	if !ok {
		profile.markComplex("unmatched closing </" + name + ">")
		return 0, false
	}
	openingTagIndex := context.elements[index].tagIndex
	if index != len(context.elements)-1 {
		profile.markComplex("misnested tags")
	}
	for remove := len(context.elements) - 1; remove >= index; remove-- {
		element := context.elements[remove]
		if element.previousSameName < 0 {
			delete(context.topByName, element.name)
		} else {
			context.topByName[element.name] = element.previousSameName
		}
		context.unsupportedEnclosing = element.previousUnsupported
		context.declarativeShadowMode = element.previousShadowMode
		context.ordinaryTemplateDepth = element.previousTemplateDepth
	}
	context.elements = context.elements[:index]
	return openingTagIndex, true
}

func appendNoscriptManagedMarkers(content string, document *scannedHTML) {
	for _, tag := range document.tags {
		if tag.closing || tag.name != "noscript" || tag.rawStart >= tag.rawEnd {
			continue
		}
		for _, name := range []string{preloadBlockName, runtimeBlockName, bootstrapBlockName} {
			for _, start := range []bool{true, false} {
				marker := managedMarkerText(name, start)
				search := tag.rawStart
				for search < tag.rawEnd {
					relative := strings.Index(content[search:tag.rawEnd], marker)
					if relative < 0 {
						break
					}
					markerStart := search + relative
					document.comments = append(document.comments, htmlComment{
						start: markerStart,
						end:   markerStart + len(marker),
						sourceContext: managedSourceContext{
							parentID:            tag.start,
							parentName:          tag.name,
							namespace:           tag.namespace,
							unsupported:         "<noscript>",
							structurallyCertain: true,
						},
					})
					search = markerStart + len(marker)
				}
			}
		}
	}
}
