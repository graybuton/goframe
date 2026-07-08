package gox

import (
	"fmt"
	"go/parser"
	"strings"
)

func splitRenderAndExpression(code string) (string, string, bool) {
	positions := topLevelOperatorPositions(code, "&&")
	for index := len(positions) - 1; index >= 0; index-- {
		position := positions[index]
		left := strings.TrimSpace(code[:position])
		right := strings.TrimSpace(code[position+2:])
		if left != "" && right != "" {
			return left, right, true
		}
	}
	return "", "", false
}

func splitTernaryExpression(code string) (string, string, string, bool, error) {
	question := firstTopLevelOperator(code, "?")
	if question < 0 {
		return "", "", "", false, nil
	}
	colon := firstTopLevelOperatorFrom(code, ":", question+1)
	if colon < 0 {
		return "", "", "", false, expressionError("gox: ternary render expression is missing ':'")
	}

	condition := strings.TrimSpace(code[:question])
	thenCode := strings.TrimSpace(code[question+1 : colon])
	elseCode := strings.TrimSpace(code[colon+1:])
	if condition == "" {
		return "", "", "", false, expressionError("gox: ternary render expression is missing condition")
	}
	if thenCode == "" || elseCode == "" {
		return "", "", "", false, expressionError("gox: ternary render expression requires two branches")
	}
	return condition, thenCode, elseCode, true, nil
}

func expressionError(message string) error {
	return simpleError(message)
}

func validateEmbeddedExpression(code string) error {
	if _, err := parser.ParseExpr(strings.TrimSpace(code)); err != nil {
		return fmt.Errorf("gox: invalid embedded expression: %v", err)
	}
	return nil
}

func unwrapOuterParens(code string) (string, bool) {
	code = strings.TrimSpace(code)
	if len(code) < 2 || code[0] != '(' {
		return "", false
	}
	close := matchingCloseParen(code, 0)
	if close != len(code)-1 {
		return "", false
	}
	return strings.TrimSpace(code[1:close]), true
}

func topLevelOperatorPositions(input, operator string) []int {
	var positions []int
	scanTopLevel(input, 0, func(index int) bool {
		if strings.HasPrefix(input[index:], operator) {
			positions = append(positions, index)
		}
		return true
	})
	return positions
}

func firstTopLevelOperator(input, operator string) int {
	return firstTopLevelOperatorFrom(input, operator, 0)
}

func firstTopLevelOperatorFrom(input, operator string, from int) int {
	found := -1
	scanTopLevel(input, from, func(index int) bool {
		if strings.HasPrefix(input[index:], operator) {
			found = index
			return false
		}
		return true
	})
	return found
}

func scanTopLevel(input string, from int, visit func(index int) bool) {
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	for index := from; index < len(input); {
		switch {
		case input[index] == '"' || input[index] == '\'' || input[index] == '`':
			index = skipQuotedInExpression(input, index)
			continue
		case strings.HasPrefix(input[index:], "//"):
			index += 2
			for index < len(input) && input[index] != '\n' {
				index++
			}
			continue
		case strings.HasPrefix(input[index:], "/*"):
			end := strings.Index(input[index+2:], "*/")
			if end < 0 {
				return
			}
			index += end + 4
			continue
		}

		switch input[index] {
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		default:
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				if !visit(index) {
					return
				}
			}
		}
		index++
	}
}

func matchingCloseParen(input string, open int) int {
	depth := 0
	for index := open; index < len(input); {
		switch {
		case input[index] == '"' || input[index] == '\'' || input[index] == '`':
			index = skipQuotedInExpression(input, index)
			continue
		case strings.HasPrefix(input[index:], "//"):
			index += 2
			for index < len(input) && input[index] != '\n' {
				index++
			}
			continue
		case strings.HasPrefix(input[index:], "/*"):
			end := strings.Index(input[index+2:], "*/")
			if end < 0 {
				return -1
			}
			index += end + 4
			continue
		}
		switch input[index] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return index
			}
		}
		index++
	}
	return -1
}

func skipQuotedInExpression(input string, start int) int {
	quote := input[start]
	index := start + 1
	for index < len(input) {
		if quote != '`' && input[index] == '\\' {
			index += 2
			continue
		}
		if input[index] == quote {
			return index + 1
		}
		index++
	}
	return len(input)
}

type simpleError string

func (err simpleError) Error() string {
	return string(err)
}
