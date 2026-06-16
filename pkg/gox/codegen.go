package gox

import (
	"bytes"
	"fmt"
	"html"
	"strconv"
	"strings"
	"unicode"
)

// Codegen turns a parsed GOX node into calls to the goframe runtime. The MVP
// expects the runtime package to be imported with the name gf.
func Codegen(node Node) (string, error) {
	var output bytes.Buffer
	if err := writeNode(&output, node, 0); err != nil {
		return "", err
	}
	return output.String(), nil
}

func writeNode(output *bytes.Buffer, node Node, depth int) error {
	switch node := node.(type) {
	case *Element:
		if isComponent(node.Tag) {
			return writeComponent(output, node, depth)
		}
		return writeElement(output, node, depth)
	case *Fragment:
		return writeFragment(output, node.Children, depth)
	case *Text:
		value := normalizeText(node.Value)
		if value == "" {
			return nil
		}
		fmt.Fprintf(output, "gf.Text(%s)", strconv.Quote(html.UnescapeString(value)))
		return nil
	case *Expression:
		if strings.TrimSpace(node.Code) == "" {
			return fmt.Errorf("gox: empty child expression")
		}
		fmt.Fprintf(output, "gf.Child(%s)", node.Code)
		return nil
	default:
		return fmt.Errorf("gox: unsupported AST node %T", node)
	}
}

func writeComponent(output *bytes.Buffer, component *Element, depth int) error {
	if !validGoIdentifier(component.Tag) {
		return fmt.Errorf("gox: invalid component name %q", component.Tag)
	}

	fmt.Fprintf(output, "gf.Component(%q, %sProps{\n", component.Tag, component.Tag)
	for _, attribute := range component.Attributes {
		if !validGoIdentifier(attribute.Name) {
			return fmt.Errorf("gox: component prop %q must be a Go field name", attribute.Name)
		}
		writeIndent(output, depth+1)
		fmt.Fprintf(output, "%s: ", attribute.Name)
		if err := writeAttributeValue(output, attribute); err != nil {
			return err
		}
		output.WriteString(",\n")
	}
	if hasRenderableChildren(component.Children) {
		writeIndent(output, depth+1)
		output.WriteString("Children: []gf.Node{\n")
		if err := writeChildren(output, component.Children, depth+2); err != nil {
			return err
		}
		writeIndent(output, depth+1)
		output.WriteString("},\n")
	}
	writeIndent(output, depth)
	fmt.Fprintf(output, "}, %s)", component.Tag)
	return nil
}

func writeElement(output *bytes.Buffer, element *Element, depth int) error {
	fmt.Fprintf(output, "gf.El(%q, ", element.Tag)
	if len(element.Attributes) == 0 {
		output.WriteString("nil")
	} else {
		output.WriteString("gf.Props{\n")
		for _, attribute := range element.Attributes {
			writeIndent(output, depth+1)
			fmt.Fprintf(output, "%q: ", attribute.Name)
			if err := writeAttributeValue(output, attribute); err != nil {
				return err
			}
			output.WriteString(",\n")
		}
		writeIndent(output, depth)
		output.WriteString("}")
	}

	if hasRenderableChildren(element.Children) {
		output.WriteString(",\n")
		if err := writeChildren(output, element.Children, depth+1); err != nil {
			return err
		}
	} else {
		output.WriteString(",\n")
	}
	writeIndent(output, depth)
	output.WriteString(")")
	return nil
}

func writeFragment(output *bytes.Buffer, children []Node, depth int) error {
	output.WriteString("gf.Fragment(")
	if hasRenderableChildren(children) {
		output.WriteString("\n")
		if err := writeChildren(output, children, depth+1); err != nil {
			return err
		}
	}
	writeIndent(output, depth)
	output.WriteString(")")
	return nil
}

func writeChildren(output *bytes.Buffer, children []Node, depth int) error {
	for _, child := range children {
		var childOutput bytes.Buffer
		if err := writeNode(&childOutput, child, depth); err != nil {
			return err
		}
		if childOutput.Len() == 0 {
			continue
		}
		writeIndent(output, depth)
		output.Write(childOutput.Bytes())
		output.WriteString(",\n")
	}
	return nil
}

func writeAttributeValue(output *bytes.Buffer, attribute Attribute) error {
	switch value := attribute.Value.(type) {
	case StringValue:
		output.WriteString(strconv.Quote(html.UnescapeString(value.Value)))
	case ExpressionValue:
		if strings.TrimSpace(value.Code) == "" {
			return fmt.Errorf("gox: empty expression for attribute %q", attribute.Name)
		}
		output.WriteString(value.Code)
	case BoolValue:
		output.WriteString("true")
	default:
		return fmt.Errorf("gox: unsupported value for attribute %q", attribute.Name)
	}
	return nil
}

func hasRenderableChildren(children []Node) bool {
	for _, child := range children {
		if text, ok := child.(*Text); !ok || normalizeText(text.Value) != "" {
			return true
		}
	}
	return false
}

func isComponent(tag string) bool {
	if tag == "" {
		return false
	}
	return unicode.IsUpper([]rune(tag)[0])
}

func validGoIdentifier(value string) bool {
	characters := []rune(value)
	if len(characters) == 0 || !unicode.IsLetter(characters[0]) && characters[0] != '_' {
		return false
	}
	for _, character := range characters[1:] {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '_' {
			return false
		}
	}
	return true
}

func writeIndent(output *bytes.Buffer, depth int) {
	output.WriteString(strings.Repeat("\t", depth))
}

func normalizeText(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	if strings.ContainsAny(value, "\r\n") {
		return strings.Join(strings.Fields(value), " ")
	}

	leading := unicode.IsSpace(rune(value[0]))
	trailing := unicode.IsSpace(rune(value[len(value)-1]))
	normalized := strings.Join(strings.Fields(value), " ")
	if leading {
		normalized = " " + normalized
	}
	if trailing {
		normalized += " "
	}
	return normalized
}
