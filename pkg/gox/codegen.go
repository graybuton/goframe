package gox

import (
	"bytes"
	"fmt"
	"html"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

// Codegen turns a parsed GOX node into calls to the goframe runtime. The MVP
// expects the runtime package to be imported with the name gf.
func Codegen(node Node) (string, error) {
	ctx := newCodegenContext("gox", "inline", false)
	return ctx.codegen(node)
}

type codegenContext struct {
	packageName           string
	sourceName            string
	declareComponentTypes bool
	componentTypes        map[string]string
	componentOrder        []string
}

func newCodegenContext(packageName string, sourceName string, declareComponentTypes bool) *codegenContext {
	if packageName == "" {
		packageName = "main"
	}
	sourceName = strings.TrimSuffix(filepath.Base(sourceName), filepath.Ext(sourceName))
	if sourceName == "" || sourceName == "." {
		sourceName = "gox"
	}
	return &codegenContext{
		packageName:           packageName,
		sourceName:            sanitizeIdentifierPart(sourceName),
		declareComponentTypes: declareComponentTypes,
		componentTypes:        make(map[string]string),
	}
}

func (ctx *codegenContext) codegen(node Node) (string, error) {
	var output bytes.Buffer
	if err := ctx.writeNode(&output, node, 0); err != nil {
		return "", err
	}
	return output.String(), nil
}

func (ctx *codegenContext) componentTypeExpression(tag string) string {
	id := ctx.packageName + "." + tag
	if !ctx.declareComponentTypes {
		return fmt.Sprintf("gf.NewComponentType(%q, %q)", id, tag)
	}
	if name, ok := ctx.componentTypes[tag]; ok {
		return name
	}
	name := "_goxComponent_" + ctx.sourceName + "_" + sanitizeIdentifierPart(tag)
	ctx.componentTypes[tag] = name
	ctx.componentOrder = append(ctx.componentOrder, tag)
	return name
}

func (ctx *codegenContext) declarations() string {
	if len(ctx.componentOrder) == 0 {
		return ""
	}
	var output bytes.Buffer
	output.WriteString("var (\n")
	for _, tag := range ctx.componentOrder {
		name := ctx.componentTypes[tag]
		id := ctx.packageName + "." + tag
		fmt.Fprintf(&output, "\t%s = gf.NewComponentType(%q, %q)\n", name, id, tag)
	}
	output.WriteString(")\n")
	return output.String()
}

func (ctx *codegenContext) writeNode(output *bytes.Buffer, node Node, depth int) error {
	switch node := node.(type) {
	case *Element:
		if isComponent(node.Tag) {
			return ctx.writeComponent(output, node, depth)
		}
		return ctx.writeElement(output, node, depth)
	case *Fragment:
		return ctx.writeFragment(output, node.Children, depth)
	case *Text:
		value := normalizeText(node.Value)
		if value == "" {
			return nil
		}
		fmt.Fprintf(output, "gf.Text(%s)", strconv.Quote(html.UnescapeString(value)))
		return nil
	case *Expression:
		code := strings.TrimSpace(node.Code)
		if code == "" {
			return fmt.Errorf("gox: empty child expression")
		}
		return ctx.writeChildExpression(output, code, depth)
	default:
		return fmt.Errorf("gox: unsupported AST node %T", node)
	}
}

func (ctx *codegenContext) writeComponent(output *bytes.Buffer, component *Element, depth int) error {
	if !validGoIdentifier(component.Tag) {
		return fmt.Errorf("gox: invalid component name %q", component.Tag)
	}
	key, attributes, err := splitKeyAttribute(component.Attributes)
	if err != nil {
		return err
	}

	var body bytes.Buffer

	fmt.Fprintf(&body, "gf.ComponentT(%s, %sProps{\n", ctx.componentTypeExpression(component.Tag), component.Tag)
	for _, attribute := range attributes {
		if !validGoIdentifier(attribute.Name) {
			return fmt.Errorf("gox: component prop %q must be a Go field name", attribute.Name)
		}
		writeIndent(&body, depth+1)
		fmt.Fprintf(&body, "%s: ", attribute.Name)
		if err := writeAttributeValue(&body, attribute); err != nil {
			return err
		}
		body.WriteString(",\n")
	}
	if hasRenderableChildren(component.Children) {
		writeIndent(&body, depth+1)
		body.WriteString("Children: []gf.Node{\n")
		if err := ctx.writeChildren(&body, component.Children, depth+2); err != nil {
			return err
		}
		writeIndent(&body, depth+1)
		body.WriteString("},\n")
	}
	writeIndent(&body, depth)
	fmt.Fprintf(&body, "}, %s)", component.Tag)
	return writeKeyed(output, key, body.Bytes(), depth)
}

func (ctx *codegenContext) writeElement(output *bytes.Buffer, element *Element, depth int) error {
	key, attributes, err := splitKeyAttribute(element.Attributes)
	if err != nil {
		return err
	}

	var body bytes.Buffer
	fmt.Fprintf(&body, "gf.El(%q, ", element.Tag)
	if len(attributes) == 0 {
		body.WriteString("nil")
	} else {
		body.WriteString("gf.Props{\n")
		for _, attribute := range attributes {
			writeIndent(&body, depth+1)
			fmt.Fprintf(&body, "%q: ", attribute.Name)
			if err := writeAttributeValue(&body, attribute); err != nil {
				return err
			}
			body.WriteString(",\n")
		}
		writeIndent(&body, depth)
		body.WriteString("}")
	}

	if hasRenderableChildren(element.Children) {
		body.WriteString(",\n")
		if err := ctx.writeChildren(&body, element.Children, depth+1); err != nil {
			return err
		}
	} else {
		body.WriteString(",\n")
	}
	writeIndent(&body, depth)
	body.WriteString(")")
	return writeKeyed(output, key, body.Bytes(), depth)
}

func (ctx *codegenContext) writeFragment(output *bytes.Buffer, children []Node, depth int) error {
	output.WriteString("gf.Fragment(")
	if hasRenderableChildren(children) {
		output.WriteString("\n")
		if err := ctx.writeChildren(output, children, depth+1); err != nil {
			return err
		}
	}
	writeIndent(output, depth)
	output.WriteString(")")
	return nil
}

func (ctx *codegenContext) writeChildren(output *bytes.Buffer, children []Node, depth int) error {
	for _, child := range children {
		var childOutput bytes.Buffer
		if err := ctx.writeNode(&childOutput, child, depth); err != nil {
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

func (ctx *codegenContext) writeChildExpression(output *bytes.Buffer, code string, depth int) error {
	if rendered, ok, err := ctx.lowerRenderExpression(code, depth); err != nil {
		return err
	} else if ok {
		output.WriteString(rendered)
		return nil
	}

	rewritten, err := ctx.rewriteMarkupInGo(code)
	if err != nil {
		return err
	}
	fmt.Fprintf(output, "gf.Child(%s)", rewritten)
	return nil
}

func (ctx *codegenContext) lowerRenderExpression(code string, depth int) (string, bool, error) {
	if condition, thenCode, elseCode, ok, err := splitTernaryExpression(code); err != nil {
		return "", false, err
	} else if ok {
		thenNode, thenOK, err := ctx.codegenNodeExpression(thenCode, depth)
		if err != nil {
			return "", false, err
		}
		elseNode, elseOK, err := ctx.codegenNodeExpression(elseCode, depth)
		if err != nil {
			return "", false, err
		}
		if !thenOK || !elseOK {
			return "", false, fmt.Errorf("gox: ternary render branches must be GOX nodes")
		}
		return "gf.IfElse(" + strings.TrimSpace(condition) + ", " + thenNode + ", " + elseNode + ")", true, nil
	}

	if condition, nodeCode, ok := splitRenderAndExpression(code); ok {
		node, nodeOK, err := ctx.codegenNodeExpression(nodeCode, depth)
		if err != nil {
			return "", false, err
		}
		if nodeOK {
			return "gf.If(" + strings.TrimSpace(condition) + ", " + node + ")", true, nil
		}
	}
	return "", false, nil
}

func (ctx *codegenContext) codegenNodeExpression(code string, depth int) (string, bool, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", false, nil
	}
	if inner, ok := unwrapOuterParens(code); ok {
		return ctx.codegenNodeExpression(inner, depth)
	}
	if code[0] != '<' {
		return "", false, nil
	}

	node, consumed, err := ParseElement(code)
	if err != nil {
		return "", false, err
	}
	if strings.TrimSpace(code[consumed:]) != "" {
		return "", false, fmt.Errorf("gox: unexpected text after GOX node expression")
	}
	var output bytes.Buffer
	if err := ctx.writeNode(&output, node, depth); err != nil {
		return "", false, err
	}
	return output.String(), true, nil
}

func (ctx *codegenContext) rewriteMarkupInGo(code string) (string, error) {
	var output bytes.Buffer
	cursor := 0
	searchFrom := 0
	replaced := false
	for {
		start := findMarkupStart(code, searchFrom)
		if start < 0 {
			break
		}
		node, consumed, err := ParseElement(code[start:])
		if err != nil {
			return "", err
		}
		generated, err := ctx.codegen(node)
		if err != nil {
			return "", err
		}
		replaceStart, replaceEnd := unwrapReturnParentheses(code, start, start+consumed)
		output.WriteString(code[cursor:replaceStart])
		output.WriteString(generated)
		cursor = replaceEnd
		searchFrom = cursor
		replaced = true
	}
	if !replaced {
		return code, nil
	}
	output.WriteString(code[cursor:])
	return output.String(), nil
}

func splitKeyAttribute(attributes []Attribute) (string, []Attribute, error) {
	filtered := make([]Attribute, 0, len(attributes))
	key := ""
	for _, attribute := range attributes {
		if attribute.Name != "Key" {
			filtered = append(filtered, attribute)
			continue
		}
		if key != "" {
			return "", nil, fmt.Errorf("gox: duplicate Key prop")
		}
		keyCode, err := keyCode(attribute.Value)
		if err != nil {
			return "", nil, err
		}
		key = keyCode
	}
	return key, filtered, nil
}

func keyCode(value AttributeValue) (string, error) {
	switch value := value.(type) {
	case StringValue:
		return strconv.Quote(html.UnescapeString(value.Value)), nil
	case ExpressionValue:
		code := strings.TrimSpace(value.Code)
		if code == "" {
			return "", fmt.Errorf("gox: Key requires a value")
		}
		return "gf.ToString(" + code + ")", nil
	case BoolValue:
		return "", fmt.Errorf("gox: Key requires a value")
	default:
		return "", fmt.Errorf("gox: unsupported Key value")
	}
}

func writeKeyed(output *bytes.Buffer, key string, body []byte, depth int) error {
	if key == "" {
		output.Write(body)
		return nil
	}
	fmt.Fprintf(output, "gf.Key(%s,\n", key)
	writeIndent(output, depth+1)
	output.Write(body)
	output.WriteString(",\n")
	writeIndent(output, depth)
	output.WriteString(")")
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

func sanitizeIdentifierPart(value string) string {
	var output strings.Builder
	for index, character := range value {
		if unicode.IsLetter(character) || character == '_' || index > 0 && unicode.IsDigit(character) {
			output.WriteRune(character)
			continue
		}
		output.WriteByte('_')
	}
	if output.Len() == 0 {
		return "gox"
	}
	result := output.String()
	if first := []rune(result)[0]; !unicode.IsLetter(first) && first != '_' {
		return "_" + result
	}
	return result
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
