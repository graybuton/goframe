package main

import "fmt"

const managedDocumentParentID = -1

type managedSourceContext struct {
	parentID            int
	parentName          string
	namespace           htmlNamespace
	foreignAncestor     bool
	unsupported         string
	ordinaryTemplate    bool
	declarativeShadow   bool
	structurallyCertain bool
}

func (context htmlScannerContext) managedSourceContext() managedSourceContext {
	result := managedSourceContext{
		parentID:            managedDocumentParentID,
		namespace:           htmlNamespaceHTML,
		structurallyCertain: !context.uncertain,
	}
	if len(context.elements) == 0 {
		return result
	}
	parent := context.elements[len(context.elements)-1]
	result.parentID = parent.sourceStart
	result.parentName = parent.name
	result.namespace = parent.namespace
	result.foreignAncestor = parent.foreignAncestor
	result.structurallyCertain = result.structurallyCertain && parent.stableBrowserParent
	return result
}

func validateManagedBlockContexts(name string, start, end managedMarker) error {
	startContext := start.span.sourceContext
	endContext := end.span.sourceContext
	if reason := startContext.unsupportedReason(); reason != "" {
		return managedBlockContextError(name, reason, startContext, endContext)
	}
	if reason := endContext.unsupportedReason(); reason != "" {
		return managedBlockContextError(name, reason, startContext, endContext)
	}
	if !startContext.sameStructuralContext(endContext) {
		return managedBlockContextError(name, "different structural contexts", startContext, endContext)
	}
	if err := validateManagedBlockParent(name, startContext, endContext); err != nil {
		return err
	}
	return nil
}

func managedBlockContextError(name, reason string, start, end managedSourceContext) error {
	return fmt.Errorf(
		"goframe:%s managed block has %s (start context: %s; end context: %s); %s",
		name,
		reason,
		start.description(),
		end.description(),
		managedBlockPlacementGuidance(name),
	)
}

func validateManagedBlockParent(name string, start, end managedSourceContext) error {
	switch name {
	case preloadBlockName:
		if start.parentName == "head" {
			return nil
		}
		return fmt.Errorf(
			"custom index goframe:preload must be a direct child of <head> in the current preview contract (start context: %s; end context: %s)",
			start.description(),
			end.description(),
		)
	case runtimeBlockName, bootstrapBlockName:
		if start.parentName == "head" || start.parentName == "body" {
			return nil
		}
		return fmt.Errorf(
			"custom index goframe:%s blocks must be direct children of one concrete <head> or <body> element in the current preview contract (start context: %s; end context: %s)",
			name,
			start.description(),
			end.description(),
		)
	default:
		return fmt.Errorf("custom index has unsupported managed block goframe:%s", name)
	}
}

func managedBlockPlacementGuidance(name string) string {
	if name == preloadBlockName {
		return "place the complete pair directly under one concrete <head> element"
	}
	return "place the complete pair directly under one concrete <head> or <body> element"
}

func (context managedSourceContext) unsupportedReason() string {
	switch {
	case !context.structurallyCertain:
		return "a structurally uncertain source context"
	case context.parentID == managedDocumentParentID:
		return "a document level context without a concrete parent"
	case context.parentName == "html":
		return "a context directly under <html>"
	case context.foreignAncestor || context.namespace != htmlNamespaceHTML:
		return "SVG or MathML ancestry"
	case context.unsupported != "":
		return "an unsupported " + context.unsupported + " context"
	case context.ordinaryTemplate:
		return "an unsupported <template> context"
	case context.declarativeShadow:
		return "an unsupported declarative Shadow DOM context"
	default:
		return ""
	}
}

func (context managedSourceContext) sameStructuralContext(other managedSourceContext) bool {
	return context.parentID == other.parentID &&
		context.parentName == other.parentName &&
		context.namespace == other.namespace &&
		context.foreignAncestor == other.foreignAncestor &&
		context.unsupported == other.unsupported &&
		context.ordinaryTemplate == other.ordinaryTemplate &&
		context.declarativeShadow == other.declarativeShadow &&
		context.structurallyCertain == other.structurallyCertain
}

func (context managedSourceContext) description() string {
	if context.parentID == managedDocumentParentID {
		return "document level"
	}
	description := fmt.Sprintf("parent <%s> near byte %d", context.parentName, context.parentID)
	if context.foreignAncestor || context.namespace != htmlNamespaceHTML {
		description += " with SVG or MathML ancestry"
	}
	if context.unsupported != "" {
		description += " inside " + context.unsupported
	}
	if !context.structurallyCertain {
		description += " after structurally uncertain parsing"
	}
	return description
}
