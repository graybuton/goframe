package gox

import (
	"fmt"
	"strings"
)

// Parser builds a small GOX element tree from lexer tokens.
type Parser struct {
	lexer     *Lexer
	positions map[Node]int
}

// ParseElement parses one root GOX node and returns the number of consumed
// bytes. Any Go source after the root node is left untouched.
func ParseElement(input string) (Node, int, error) {
	return parseElementAt(input, "gox", 1, 1)
}

func parseElementAt(input, filename string, line, column int) (Node, int, error) {
	node, consumed, _, err := parseElementAtWithPositions(input, filename, line, column)
	return node, consumed, err
}

func parseElementAtWithPositions(input, filename string, line, column int) (Node, int, map[Node]int, error) {
	parser := &Parser{
		lexer:     newLexerAt(input, filename, line, column),
		positions: map[Node]int{},
	}
	start, err := parser.lexer.next()
	if err != nil {
		return nil, 0, nil, err
	}
	if start.kind != tokenOpenTag {
		return nil, 0, nil, parser.unexpected(start, "opening tag")
	}

	node, err := parser.parseOpenedNode()
	if err != nil {
		return nil, 0, nil, err
	}
	return node, parser.lexer.pos, parser.positions, nil
}

func (parser *Parser) parseOpenedNode() (Node, error) {
	name, err := parser.lexer.next()
	if err != nil {
		return nil, err
	}
	if name.kind == tokenTagEnd {
		fragment := &Fragment{}
		parser.positions[fragment] = name.offset
		children, err := parser.parseChildren("")
		if err != nil {
			return nil, err
		}
		fragment.Children = children
		return fragment, nil
	}
	if name.kind != tokenIdentifier {
		return nil, parser.unexpected(name, "tag name or > for fragment")
	}
	component := isComponent(name.value)
	if strings.Contains(name.value, ":") {
		return nil, parser.lexer.errorAt(name.offset, "namespace tags with ':' are not supported; use package-qualified component tags like <ui.Header>")
	}
	if strings.Contains(name.value, ".") {
		if strings.Count(name.value, ".") != 1 {
			return nil, parser.lexer.errorAt(name.offset, "qualified component tags support exactly packageAlias.Component: <%s>", name.value)
		}
		alias, selected, ok := splitQualifiedTag(name.value)
		if !ok || !validGoIdentifier(alias) || !validGoIdentifier(selected) {
			return nil, parser.lexer.errorAt(name.offset, "qualified component tags support exactly packageAlias.Component: <%s>", name.value)
		}
		if alias == "_" {
			return nil, parser.lexer.errorAt(name.offset, "package-qualified component tag cannot use blank or dot import alias: <%s>", name.value)
		}
		if !isExportedIdentifier(selected) {
			return nil, parser.lexer.errorAt(name.offset, "qualified component tag <%s> must select an exported component name", name.value)
		}
		component = true
	}
	if component && !validGoIdentifier(name.value) {
		if !strings.Contains(name.value, ".") {
			return nil, parser.lexer.errorAt(name.offset, "invalid component tag <%s>; component names must be Go identifiers", name.value)
		}
	}

	element := &Element{Tag: name.value}
	parser.positions[element] = name.offset
	seenKey := false
	for {
		next, err := parser.lexer.next()
		if err != nil {
			return nil, err
		}

		switch next.kind {
		case tokenTagEnd:
			children, err := parser.parseChildren(element.Tag)
			if err != nil {
				return nil, err
			}
			element.Children = children
			return element, nil
		case tokenSelfClose:
			return element, nil
		case tokenIdentifier:
			if component && next.value != "Key" && !validGoIdentifier(next.value) {
				return nil, parser.lexer.errorAt(next.offset, "component prop %q is not a valid Go field name", next.value)
			}
			if next.value == "Key" {
				if seenKey {
					return nil, parser.lexer.errorAt(next.offset, "gox: duplicate Key prop")
				}
				seenKey = true
			}
			attribute, err := parser.parseAttribute(next)
			if err != nil {
				return nil, err
			}
			element.Attributes = append(element.Attributes, attribute)
		case tokenExpression:
			if strings.HasPrefix(strings.TrimSpace(next.value), "...") {
				return nil, parser.lexer.errorAt(next.offset, "spread props are not supported; pass explicit props instead: {%s}", strings.TrimSpace(next.value))
			}
			return nil, parser.unexpected(next, "attribute, >, or />")
		default:
			return nil, parser.unexpected(next, "attribute, >, or />")
		}
	}
}

func (parser *Parser) parseAttribute(name token) (Attribute, error) {
	next, err := parser.lexer.next()
	if err != nil {
		return Attribute{}, err
	}
	if next.kind != tokenEquals {
		if name.value == "Key" {
			return Attribute{}, parser.lexer.errorAt(name.offset, "gox: Key requires a value")
		}
		return Attribute{}, parser.unexpected(next, "= after attribute "+name.value)
	}

	value, err := parser.lexer.next()
	if err != nil {
		return Attribute{}, err
	}
	switch value.kind {
	case tokenString:
		return Attribute{Name: name.value, Value: StringValue{Value: value.value}}, nil
	case tokenExpression:
		if strings.TrimSpace(value.value) == "" {
			if name.value == "Key" {
				return Attribute{}, parser.lexer.errorAt(value.offset, "gox: Key requires a value")
			}
			return Attribute{}, parser.lexer.errorAt(value.offset, "gox: empty expression for attribute %q", name.value)
		}
		return Attribute{Name: name.value, Value: ExpressionValue{Code: value.value}}, nil
	default:
		return Attribute{}, parser.unexpected(value, "quoted string or Go expression")
	}
}

func (parser *Parser) parseChildren(expectedTag string) ([]Node, error) {
	var children []Node
	for {
		next, err := parser.lexer.next()
		if err != nil {
			return nil, err
		}

		switch next.kind {
		case tokenText:
			child := &Text{Value: next.value}
			parser.positions[child] = next.offset
			children = append(children, child)
		case tokenExpression:
			if strings.TrimSpace(next.value) == "" {
				return nil, parser.lexer.errorAt(next.offset, "gox: empty child expression")
			}
			child := &Expression{Code: next.value}
			parser.positions[child] = next.offset
			children = append(children, child)
		case tokenOpenTag:
			child, err := parser.parseOpenedNode()
			if err != nil {
				return nil, err
			}
			children = append(children, child)
		case tokenCloseTag:
			closeName, err := parser.lexer.next()
			if err != nil {
				return nil, err
			}
			if expectedTag == "" {
				if closeName.kind == tokenTagEnd {
					return children, nil
				}
				if closeName.kind == tokenIdentifier {
					return nil, parser.lexer.errorAt(closeName.offset, "expected closing fragment </>, got </%s>", closeName.value)
				}
				return nil, parser.unexpected(closeName, "> to close fragment")
			}
			if closeName.kind == tokenTagEnd {
				return nil, parser.lexer.errorAt(closeName.offset, "expected closing tag </%s>, got </>", expectedTag)
			}
			if closeName.kind != tokenIdentifier {
				return nil, parser.unexpected(closeName, "closing tag name")
			}
			if closeName.value != expectedTag {
				return nil, parser.lexer.errorAt(closeName.offset, "expected closing tag </%s>, got </%s>", expectedTag, closeName.value)
			}
			end, err := parser.lexer.next()
			if err != nil {
				return nil, err
			}
			if end.kind != tokenTagEnd {
				return nil, parser.unexpected(end, ">")
			}
			return children, nil
		case tokenEOF:
			if expectedTag == "" {
				return nil, parser.lexer.errorAt(next.offset, "unclosed fragment; expected </>")
			}
			return nil, parser.lexer.errorAt(next.offset, "unclosed tag <%s>; expected </%s>", expectedTag, expectedTag)
		default:
			return nil, parser.unexpected(next, "text, expression, child, or closing tag")
		}
	}
}

func (parser *Parser) unexpected(got token, want string) error {
	line, column := lineColumn(parser.lexer.input, got.offset)
	if line == 1 {
		column += parser.lexer.columnOffset - 1
	}
	line += parser.lexer.lineOffset - 1
	return diagnosticError(parser.lexer.filename, line, column, fmt.Sprintf("expected %s", want), sourceLine(parser.lexer.input, got.offset))
}
