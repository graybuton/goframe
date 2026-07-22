package gox

import (
	"errors"
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalizeTextPreservesUnicodeBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "ASCII whitespace only", input: " \t ", want: ""},
		{name: "Unicode whitespace only", input: "\u00a0\u2003", want: ""},
		{name: "ordinary ASCII", input: "hello", want: "hello"},
		{name: "ASCII boundaries", input: " hello ", want: " hello "},
		{name: "leading NBSP", input: "\u00a0hello", want: " hello"},
		{name: "trailing NBSP", input: "hello\u00a0", want: "hello "},
		{name: "leading EM SPACE", input: "\u2003hello", want: " hello"},
		{name: "trailing narrow NBSP", input: "hello\u202f", want: "hello "},
		{name: "both Unicode boundaries", input: "\u3000hello\u2003", want: " hello "},
		{name: "internal Unicode whitespace", input: "hello\u2003world", want: "hello world"},
		{name: "Cyrillic", input: "Привет", want: "Привет"},
		{name: "emoji boundary", input: "🙂 hello 🙂", want: "🙂 hello 🙂"},
		{name: "trailing a with ogonek", input: "drogą", want: "drogą"},
		{name: "actual trailing NEL", input: "hello\u0085", want: "hello "},
		{name: "multiline Unicode", input: "\u00a0hello\u2003\nworld\u202f", want: "hello world"},
		{name: "invalid leading byte", input: string([]byte{0x85, 'x'}), want: string([]byte{0x85, 'x'})},
		{name: "invalid trailing byte", input: string([]byte{'x', 0x85}), want: string([]byte{'x', 0x85})},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := normalizeText(test.input)
			if got != test.want {
				t.Fatalf("normalizeText() bytes = % x (%q), want % x (%q)", []byte(got), got, []byte(test.want), test.want)
			}
		})
	}
}

func TestGeneratePreservesUnicodeTextBoundaries(t *testing.T) {
	source := []byte("package main\n\n" +
		"import gf \"github.com/graybuton/goframe/pkg/goframe\"\n\n" +
		"func Cyrillic() gf.Node {\n\treturn <p>\u00a0Привет\u00a0</p>\n}\n\n" +
		"func Polish() gf.Node {\n\treturn <p>drogą</p>\n}\n\n" +
		"func Emoji() gf.Node {\n\treturn <p>🙂</p>\n}\n")

	generated, err := Generate(source)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if !utf8.Valid(generated) {
		t.Fatalf("generated Go is not valid UTF-8: % x", generated)
	}
	if _, err := parser.ParseFile(gotoken.NewFileSet(), "unicode_text.gox.go", generated, parser.AllErrors); err != nil {
		t.Fatalf("generated Go does not parse: %v\n%s", err, generated)
	}

	text := string(generated)
	for _, want := range []string{
		`gf.Text(" Привет ")`,
		`gf.Text("drogą")`,
		`gf.Text("🙂")`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated source does not contain %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, `gf.Text("drogą ")`) {
		t.Fatalf("generated source invented trailing whitespace after ą:\n%s", text)
	}
	if got := strings.Count(text, `gf.El("p", nil,`); got != 3 {
		t.Fatalf("generated paragraph count = %d, want 3:\n%s", got, text)
	}

	invalidSource := append([]byte("package main\n\n"+
		"import gf \"github.com/graybuton/goframe/pkg/goframe\"\n\n"+
		"func Invalid() gf.Node {\n\treturn <p>x"), 0x85)
	invalidSource = append(invalidSource, []byte("</p>\n}\n")...)
	generated, err = Generate(invalidSource)
	if err != nil {
		t.Fatalf("Generate(invalid UTF-8) error: %v", err)
	}
	if !utf8.Valid(generated) {
		t.Fatalf("generated Go for invalid source text is not valid UTF-8: % x", generated)
	}
	if _, err := parser.ParseFile(gotoken.NewFileSet(), "invalid_text.gox.go", generated, parser.AllErrors); err != nil {
		t.Fatalf("generated Go for invalid source text does not parse: %v\n%s", err, generated)
	}
	invalidText := string(generated)
	if !strings.Contains(invalidText, `gf.Text("x\x85")`) {
		t.Fatalf("generated source did not preserve the invalid byte through quoting:\n%s", invalidText)
	}
	if strings.Contains(invalidText, `gf.Text("x\x85 ")`) {
		t.Fatalf("generated source invented trailing whitespace after an invalid byte:\n%s", invalidText)
	}
}

func TestGenerateInsertsComponentDeclarationsUsingGoSyntax(t *testing.T) {
	tests := []struct {
		name            string
		source          string
		wantImportDecls int
	}{
		{
			name: "no imports",
			source: `package demo

func View() any {
	return <Button />
}
`,
		},
		{
			name: "single import",
			source: `package demo

import gf "github.com/graybuton/goframe/pkg/goframe"

func View() gf.Node {
	return <Button />
}
`,
			wantImportDecls: 1,
		},
		{
			name: "aliased single import",
			source: `package demo

import frame "github.com/graybuton/goframe/pkg/goframe"

func View() any {
	_ = frame.Node(nil)
	return <Button />
}
`,
			wantImportDecls: 1,
		},
		{
			name: "grouped import",
			source: `package demo

import (
	"fmt"
	gf "github.com/graybuton/goframe/pkg/goframe"
)

var _ = fmt.Sprintf

func View() gf.Node {
	return <Button />
}
`,
			wantImportDecls: 1,
		},
		{
			name: "multiple separate imports",
			source: `package demo

import "fmt"
import gf "github.com/graybuton/goframe/pkg/goframe"

var _ = fmt.Sprintf

func View() gf.Node {
	return <Button />
}
`,
			wantImportDecls: 2,
		},
		{
			name: "comments between imports",
			source: `package demo

import "fmt"

// Runtime import.
import gf "github.com/graybuton/goframe/pkg/goframe"

var _ = fmt.Sprintf

func View() gf.Node {
	return <Button />
}
`,
			wantImportDecls: 2,
		},
		{
			name: "comment between package and import",
			source: `package demo

// Runtime import.
import gf "github.com/graybuton/goframe/pkg/goframe"

func View() gf.Node {
	return <Button />
}
`,
			wantImportDecls: 1,
		},
		{
			name: "leading line comment containing package decoy",
			source: `// package fake

package demo

import gf "github.com/graybuton/goframe/pkg/goframe"

func View() gf.Node {
	return <Button />
}
`,
			wantImportDecls: 1,
		},
		{
			name: "leading block comment containing package decoy",
			source: `/*
package fake
*/

package demo

import gf "github.com/graybuton/goframe/pkg/goframe"

func View() gf.Node {
	return <Button />
}
`,
			wantImportDecls: 1,
		},
		{
			name: "grouped import with closing parenthesis in line comment",
			source: `package demo

import (
	"fmt" // ) is not the end of the import group
	gf "github.com/graybuton/goframe/pkg/goframe"
)

var _ = fmt.Sprintf

func View() gf.Node {
	return <Button />
}
`,
			wantImportDecls: 1,
		},
		{
			name: "grouped import with parentheses in block comment",
			source: `package demo

import (
	"fmt" /* ( ) ) */
	gf "github.com/graybuton/goframe/pkg/goframe"
)

var _ = fmt.Sprintf

func View() gf.Node {
	return <Button />
}
`,
			wantImportDecls: 1,
		},
		{
			name: "grouped import with parenthesis in import path",
			source: `package demo

import (
	_ "example.com/parenthesis)"
	gf "github.com/graybuton/goframe/pkg/goframe"
)

func View() gf.Node {
	return <Button />
}
`,
			wantImportDecls: 1,
		},
		{
			name: "cgo preamble",
			source: `package demo

/*
#include <stdlib.h>
*/
import "C"

import gf "github.com/graybuton/goframe/pkg/goframe"

func View() gf.Node {
	return <Button />
}
`,
			wantImportDecls: 2,
		},
		{
			name:            "semicolon separated declarations",
			source:          "package demo; import gf \"github.com/graybuton/goframe/pkg/goframe\"; func View() gf.Node { return <Button /> }\n",
			wantImportDecls: 1,
		},
		{
			name:            "CRLF source",
			source:          "package demo\r\n\r\nimport gf \"github.com/graybuton/goframe/pkg/goframe\"\r\n\r\nfunc View() gf.Node {\r\n\treturn <Button />\r\n}\r\n",
			wantImportDecls: 1,
		},
		{
			name: "Unicode comments",
			source: `// Пример компонента.
package demo

// Импорт среды выполнения.
import gf "github.com/graybuton/goframe/pkg/goframe"

func View() gf.Node {
	return <Button />
}
`,
			wantImportDecls: 1,
		},
		{
			name:            "raw string decoys",
			source:          "package demo\n\nimport gf \"github.com/graybuton/goframe/pkg/goframe\"\n\nvar Decoy = `package fake\nimport (\n)`\n\nfunc View() gf.Node {\n\treturn <Button />\n}\n",
			wantImportDecls: 1,
		},
		{
			name: "top level var before component function",
			source: `package demo

var Authored = 1

func View() any {
	return <Button />
}
`,
		},
		{
			name: "top level type before component function",
			source: `package demo

type Authored struct{}

func View() any {
	return <Button />
}
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := GenerateOptions{
				Filename:        "syntax_case.gox",
				PackageIdentity: "example.com/demo",
			}
			first, err := GenerateWithOptions([]byte(test.source), options)
			if err != nil {
				t.Fatalf("GenerateWithOptions() error: %v", err)
			}
			second, err := GenerateWithOptions([]byte(test.source), options)
			if err != nil {
				t.Fatalf("GenerateWithOptions(second) error: %v", err)
			}
			if string(first) != string(second) {
				t.Fatalf("generation is not deterministic:\nfirst:\n%s\nsecond:\n%s", first, second)
			}

			layout := assertGeneratedDeclarationLayout(t, "syntax_case.gox.go", first, "_goxComponent_syntax_case_Button")
			if layout.importDecls != test.wantImportDecls {
				t.Fatalf("import declarations = %d, want %d:\n%s", layout.importDecls, test.wantImportDecls, first)
			}
			if !declarationNamePresent(layout.file, "View") {
				t.Fatalf("authored View declaration is missing:\n%s", first)
			}
		})
	}
}

func TestGeneratePreservesAuthoredDeclarationDocumentation(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		declaration string
		wantComment string
	}{
		{
			name: "func",
			source: `package demo

// View renders the page.
func View() any {
	return <Button />
}
`,
			declaration: "View",
			wantComment: "// View renders the page.",
		},
		{
			name: "var",
			source: `package demo

// Banner stores authored state.
var Banner = "hello"

func View() any {
	return <Button />
}
`,
			declaration: "Banner",
			wantComment: "// Banner stores authored state.",
		},
		{
			name: "const",
			source: `package demo

// Kind identifies the view.
const Kind = "view"

func View() any {
	return <Button />
}
`,
			declaration: "Kind",
			wantComment: "// Kind identifies the view.",
		},
		{
			name: "type",
			source: `package demo

// Model stores view state.
type Model struct{}

func View() any {
	return <Button />
}
`,
			declaration: "Model",
			wantComment: "// Model stores view state.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			generated, err := GenerateWithOptions([]byte(test.source), GenerateOptions{
				Filename:        "documentation.gox",
				PackageIdentity: "example.com/demo",
			})
			if err != nil {
				t.Fatalf("GenerateWithOptions() error: %v", err)
			}
			layout := assertGeneratedDeclarationLayout(t, "documentation.gox.go", generated, "_goxComponent_documentation_Button")
			declaration := findNamedDeclaration(layout.file, test.declaration)
			if declaration == nil {
				t.Fatalf("authored declaration %q is missing:\n%s", test.declaration, generated)
			}
			doc := declarationDoc(declaration)
			if doc == nil || !commentGroupContains(doc, test.wantComment) {
				t.Fatalf("documentation for %q was not preserved: %#v\n%s", test.declaration, doc, generated)
			}
			if layout.generated.Doc != nil {
				t.Fatalf("generated declaration received authored documentation: %#v\n%s", layout.generated.Doc, generated)
			}
		})
	}
}

func TestGeneratePreservesEmbedDirectiveAttachment(t *testing.T) {
	source := []byte(`package demo

import (
	_ "embed"
	gf "github.com/graybuton/goframe/pkg/goframe"
)

//go:embed page.html
var pageHTML string

func View() gf.Node {
	return <Button />
}
`)
	generated, err := GenerateWithOptions(source, GenerateOptions{
		Filename:        "embed_directive.gox",
		PackageIdentity: "example.com/demo",
	})
	if err != nil {
		t.Fatalf("GenerateWithOptions() error: %v", err)
	}
	layout := assertGeneratedDeclarationLayout(t, "embed_directive.gox.go", generated, "_goxComponent_embed_directive_Button")
	authored := findNamedDeclaration(layout.file, "pageHTML")
	if authored == nil {
		t.Fatalf("pageHTML declaration is missing:\n%s", generated)
	}
	doc := declarationDoc(authored)
	if doc == nil || !commentGroupContains(doc, "//go:embed page.html") {
		t.Fatalf("go:embed directive is not attached to pageHTML: %#v\n%s", doc, generated)
	}
	if layout.generated.Pos() >= doc.Pos() {
		t.Fatalf("generated declaration was not inserted before go:embed directive:\n%s", generated)
	}
	if layout.generated.Doc != nil {
		t.Fatalf("generated declaration received go:embed directive: %#v\n%s", layout.generated.Doc, generated)
	}
}

func TestGeneratePreservesCgoPreamble(t *testing.T) {
	source := []byte(`package demo

/*
#include <stdlib.h>
*/
import "C"

import gf "github.com/graybuton/goframe/pkg/goframe"

func View() gf.Node {
	return <Button />
}
`)
	generated, err := GenerateWithOptions(source, GenerateOptions{
		Filename:        "cgo_preamble.gox",
		PackageIdentity: "example.com/demo",
	})
	if err != nil {
		t.Fatalf("GenerateWithOptions() error: %v", err)
	}
	layout := assertGeneratedDeclarationLayout(t, "cgo_preamble.gox.go", generated, "_goxComponent_cgo_preamble_Button")
	foundCgoPreamble := false
	for _, declaration := range layout.file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok || gen.Tok != gotoken.IMPORT {
			continue
		}
		for _, spec := range gen.Specs {
			importSpec := spec.(*ast.ImportSpec)
			path, unquoteErr := strconv.Unquote(importSpec.Path.Value)
			if unquoteErr == nil && path == "C" {
				foundCgoPreamble = commentGroupContains(gen.Doc, "#include <stdlib.h>") || commentGroupContains(importSpec.Doc, "#include <stdlib.h>")
			}
		}
	}
	if !foundCgoPreamble {
		t.Fatalf("cgo preamble is not attached to import C:\n%s", generated)
	}
}

func TestGenerateRejectsMalformedTransformedGoBeforeDeclarations(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "malformed package clause",
			source: `package

func View() any {
	return <Button />
}
`,
		},
		{
			name: "unterminated import group",
			source: `package demo

import (
	"fmt"

func View() any {
	return <Button />
}
`,
		},
		{
			name: "invalid import declaration",
			source: `package demo

import 123

func View() any {
	return <Button />
}
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := GenerateWithOptions([]byte(test.source), GenerateOptions{
				Filename:        "malformed_insertion.gox",
				PackageIdentity: "example.com/demo",
			})
			if err == nil {
				t.Fatal("GenerateWithOptions() returned nil error")
			}
			for _, want := range []string{
				"malformed_insertion.gox",
				"parse transformed Go source before generated declarations",
			} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q does not contain %q", err, want)
				}
			}
			if errors.Unwrap(err) == nil {
				t.Fatalf("error does not preserve the Go parser error: %v", err)
			}
		})
	}
}

type generatedDeclarationLayout struct {
	file        *ast.File
	generated   *ast.GenDecl
	importDecls int
}

func assertGeneratedDeclarationLayout(t *testing.T, filename string, generated []byte, wantName string) generatedDeclarationLayout {
	t.Helper()
	file, err := parser.ParseFile(
		gotoken.NewFileSet(),
		filename,
		generated,
		parser.ParseComments|parser.AllErrors|parser.SkipObjectResolution,
	)
	if err != nil {
		t.Fatalf("generated Go does not parse: %v\n%s", err, generated)
	}

	layout := generatedDeclarationLayout{file: file}
	firstNonImport := -1
	generatedIndex := -1
	generatedNames := 0
	for index, declaration := range file.Decls {
		if gen, ok := declaration.(*ast.GenDecl); ok && gen.Tok == gotoken.IMPORT {
			layout.importDecls++
			continue
		}
		if firstNonImport < 0 {
			firstNonImport = index
		}
		if gen, ok := declaration.(*ast.GenDecl); ok && declarationNamePresentInGenDecl(gen, wantName) {
			layout.generated = gen
			generatedIndex = index
			generatedNames++
		}
	}
	if generatedNames != 1 {
		t.Fatalf("generated declaration count for %q = %d, want 1:\n%s", wantName, generatedNames, generated)
	}
	if generatedIndex != firstNonImport {
		t.Fatalf("generated declaration index = %d, first non-import declaration index = %d:\n%s", generatedIndex, firstNonImport, generated)
	}
	return layout
}

func declarationNamePresent(file *ast.File, name string) bool {
	return findNamedDeclaration(file, name) != nil
}

func findNamedDeclaration(file *ast.File, name string) ast.Decl {
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			if declaration.Name.Name == name {
				return declaration
			}
		case *ast.GenDecl:
			if declarationNamePresentInGenDecl(declaration, name) {
				return declaration
			}
		}
	}
	return nil
}

func declarationNamePresentInGenDecl(declaration *ast.GenDecl, name string) bool {
	for _, spec := range declaration.Specs {
		switch spec := spec.(type) {
		case *ast.ValueSpec:
			for _, identifier := range spec.Names {
				if identifier.Name == name {
					return true
				}
			}
		case *ast.TypeSpec:
			if spec.Name.Name == name {
				return true
			}
		}
	}
	return false
}

func declarationDoc(declaration ast.Decl) *ast.CommentGroup {
	switch declaration := declaration.(type) {
	case *ast.FuncDecl:
		return declaration.Doc
	case *ast.GenDecl:
		return declaration.Doc
	default:
		return nil
	}
}

func commentGroupContains(group *ast.CommentGroup, text string) bool {
	if group == nil {
		return false
	}
	for _, comment := range group.List {
		if strings.Contains(comment.Text, text) {
			return true
		}
	}
	return false
}

func TestInsertGeneratedDeclarationsAfterPackageAndImportsOnly(t *testing.T) {
	const declarations = "var _goxComponent_helper_Button = 1\n"
	tests := []struct {
		name            string
		input           string
		wantImportDecls int
		trailingComment string
	}{
		{name: "package only without trailing newline", input: "package demo"},
		{name: "package with trailing comment", input: "package demo\n\n// trailing package comment\n", trailingComment: "// trailing package comment"},
		{name: "single import", input: "package demo\n\nimport \"fmt\"\n", wantImportDecls: 1},
		{name: "multiple imports and trailing comment", input: "package demo\n\nimport \"fmt\"\nimport \"strings\"\n\n// trailing import comment\n", wantImportDecls: 2, trailingComment: "// trailing import comment"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := insertGeneratedDeclarations("package_only.gox", test.input, declarations)
			if err != nil {
				t.Fatalf("insertGeneratedDeclarations() error: %v", err)
			}
			layout := assertGeneratedDeclarationLayout(t, "package_only.gox.go", []byte(output), "_goxComponent_helper_Button")
			if layout.importDecls != test.wantImportDecls {
				t.Fatalf("import declarations = %d, want %d:\n%s", layout.importDecls, test.wantImportDecls, output)
			}
			if test.trailingComment != "" {
				commentIndex := strings.Index(output, test.trailingComment)
				declarationIndex := strings.Index(output, "_goxComponent_helper_Button")
				if commentIndex < 0 || declarationIndex < commentIndex {
					t.Fatalf("trailing comment was not preserved before the appended declaration:\n%s", output)
				}
			}
		})
	}
}

func TestGenerateCounter(t *testing.T) {
	source := []byte(`package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func App() gf.Node {
	count, setCount := gf.UseState(0)
	return (
		<div class="app">
			<h1>Counter: {count}</h1>
			<button onClick={func() {
				setCount(count + 1)
			}}>Increment</button>
		</div>
	)
}
`)

	generated, err := Generate(source)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if _, err := parser.ParseFile(gotoken.NewFileSet(), "app.gox.go", generated, parser.AllErrors); err != nil {
		t.Fatalf("generated Go does not parse: %v\n%s", err, generated)
	}

	text := string(generated)
	for _, want := range []string{
		`gf.El("div", gf.Props{`,
		`"class": "app"`,
		`gf.Child(count)`,
		`"onClick": func()`,
		`gf.Text("Increment")`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated source does not contain %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "return (") {
		t.Fatalf("invalid multiline return parentheses were not removed:\n%s", text)
	}
}

func TestGenerateIgnoresGoComparison(t *testing.T) {
	source := []byte(`package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func App() gf.Node {
	count, max := 1, 2
	if count<max {
		count++
	}
	return <p>{count}</p>
}
`)

	generated, err := Generate(source)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if !strings.Contains(string(generated), "if count < max") {
		t.Fatalf("Go comparison was not preserved:\n%s", generated)
	}
}

func TestGenerateRejectsKeywordComponentProps(t *testing.T) {
	keywords := []string{
		"break", "default", "func", "interface", "select",
		"case", "defer", "go", "map", "struct",
		"chan", "else", "goto", "package", "switch",
		"const", "fallthrough", "if", "range", "type",
		"continue", "for", "import", "return", "var",
	}
	for _, keyword := range keywords {
		t.Run(keyword, func(t *testing.T) {
			source := []byte(`package main

func View() any {
	return <Button ` + keyword + `="x" />
}
`)
			_, err := GenerateNamed("keyword_component_prop.gox", source)
			if err == nil {
				t.Fatal("GenerateNamed() returned nil error")
			}
			want := `component prop "` + keyword + `" is not a valid Go field name`
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error = %q, want %q", err.Error(), want)
			}
		})
	}
}

func TestGenerateAllowsDOMTypeAttributeAndComponentPseudoProps(t *testing.T) {
	source := []byte(`package main

import gf "github.com/graybuton/goframe/pkg/goframe"

type ButtonProps struct {
	Label string
}

func Button(props ButtonProps) gf.Node {
	return <button type="button">{props.Label}</button>
}

func View() gf.Node {
	return (
		<form>
			<input type="text" />
			<Button Key="save" Label="Save" />
		</form>
	)
}
`)
	generated, err := Generate(source)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if _, err := parser.ParseFile(gotoken.NewFileSet(), "valid_props.gox.go", generated, parser.AllErrors); err != nil {
		t.Fatalf("generated Go does not parse: %v\n%s", err, generated)
	}
	text := string(generated)
	for _, want := range []string{
		`"type": "text"`,
		`"type": "button"`,
		`Label: "Save"`,
		`gf.Key("save"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated source does not contain %q:\n%s", want, text)
		}
	}
}

func TestGenerateFileToWithOptionsWritesExplicitOutput(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "src", "view.gox")
	outputPath := filepath.Join(root, "generated", "view.gox.go")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	source := []byte(`package ui

import gf "github.com/graybuton/goframe/pkg/goframe"

func View() gf.Node {
	return <Card />
}
`)
	if err := os.WriteFile(sourcePath, source, 0o644); err != nil {
		t.Fatal(err)
	}

	wrote, err := GenerateFileToWithOptions(sourcePath, outputPath, GenerateOptions{
		Filename:        "internal/ui/view.gox",
		PackageIdentity: "github.com/example/app/internal/ui",
	})
	if err != nil {
		t.Fatalf("GenerateFileToWithOptions() error: %v", err)
	}
	if wrote != outputPath {
		t.Fatalf("generated path = %q, want %q", wrote, outputPath)
	}
	generated, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read generated output: %v", err)
	}
	text := string(generated)
	for _, want := range []string{
		generatedHeader,
		`gf.NewComponentType("github.com/example/app/internal/ui.Card", "Card")`,
		`gf.ComponentT(_goxComponent_view_Card`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated output missing %q:\n%s", want, text)
		}
	}
}

func TestFindFilesReturnsStableGOXFiles(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{
		"b.gox",
		"a.gox",
		"nested/c.gox",
		"nested/ignore.txt",
	} {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("package main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := FindFiles(root)
	if err != nil {
		t.Fatalf("FindFiles(directory) error: %v", err)
	}
	want := []string{
		filepath.Join(root, "a.gox"),
		filepath.Join(root, "b.gox"),
		filepath.Join(root, "nested", "c.gox"),
	}
	if strings.Join(files, "\n") != strings.Join(want, "\n") {
		t.Fatalf("FindFiles() = %#v, want %#v", files, want)
	}

	direct, err := FindFiles(filepath.Join(root, "a.gox"))
	if err != nil {
		t.Fatalf("FindFiles(file) error: %v", err)
	}
	if len(direct) != 1 || direct[0] != filepath.Join(root, "a.gox") {
		t.Fatalf("FindFiles(file) = %#v", direct)
	}
	if _, err := FindFiles(filepath.Join(root, "nested", "ignore.txt")); err == nil {
		t.Fatal("FindFiles(non-gox file) returned nil error")
	}
}

func TestCodegenComponentUsesInlineComponentType(t *testing.T) {
	node, _, err := ParseElement(`<Button Label="Save" />`)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := Codegen(node)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`gf.ComponentT(gf.NewComponentType("gox.Button", "Button"), ButtonProps{`,
		`Label: "Save"`,
		`}, Button)`,
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated component does not contain %q:\n%s", want, generated)
		}
	}
}

func TestGenerateDeclaresComponentTypeOncePerTag(t *testing.T) {
	source := []byte(`package demo

import gf "github.com/graybuton/goframe/pkg/goframe"

type ButtonProps struct { Label string }

func Button(props ButtonProps) gf.Node {
	return <button>{props.Label}</button>
}

func View() gf.Node {
	return (
		<div>
			<Button Label="One" />
			<Button Label="Two" />
		</div>
	)
}
`)

	generated, err := GenerateNamed("testdata/repeated_button.gox", source)
	if err != nil {
		t.Fatalf("GenerateNamed() error: %v", err)
	}
	text := string(generated)
	if got := strings.Count(text, `gf.NewComponentType("demo.Button", "Button")`); got != 1 {
		t.Fatalf("component type declarations = %d, want 1:\n%s", got, text)
	}
	if got := strings.Count(text, `gf.ComponentT(_goxComponent_repeated_button_Button`); got != 2 {
		t.Fatalf("ComponentT uses = %d, want 2:\n%s", got, text)
	}
	if typeIndex := strings.Index(text, `_goxComponent_repeated_button_Button = gf.NewComponentType`); typeIndex < 0 {
		t.Fatalf("missing component type declaration:\n%s", text)
	} else if importIndex := strings.Index(text, `import gf "github.com/graybuton/goframe/pkg/goframe"`); importIndex < 0 || typeIndex < importIndex {
		t.Fatalf("component type declaration should be after import:\n%s", text)
	}
}

func TestGenerateWithOptionsUsesPackageIdentity(t *testing.T) {
	source := []byte(`package ui

import gf "github.com/graybuton/goframe/pkg/goframe"

func View() gf.Node {
	return <Button Label="Save" />
}
`)

	generated, err := GenerateWithOptions(source, GenerateOptions{
		Filename:        "internal/ui/view.gox",
		PackageIdentity: "github.com/example/app/internal/ui",
	})
	if err != nil {
		t.Fatalf("GenerateWithOptions() error: %v", err)
	}
	text := string(generated)
	if !strings.Contains(text, `gf.NewComponentType("github.com/example/app/internal/ui.Button", "Button")`) {
		t.Fatalf("generated source does not use package identity:\n%s", text)
	}
	if strings.Contains(text, `gf.NewComponentType("ui.Button", "Button")`) {
		t.Fatalf("generated source fell back to package name identity:\n%s", text)
	}
}

func TestGenerateAllocatesDistinctComponentIdentifiersForCollidingTags(t *testing.T) {
	sources := []string{
		`package demo

import A_B "example.com/components/A_B"

func View() any {
	return <><A_B_C /><A_B.C /></>
}
`,
		`package demo

import A_B "example.com/components/A_B"

func View() any {
	return <><A_B.C /><A_B_C /></>
}
`,
	}

	var first map[string]string
	for index, source := range sources {
		generated := generateDeterministically(t, "view.gox", source)
		identifiers := generatedComponentIdentifiers(t, generated)
		if identifiers["A_B_C"] == identifiers["A_B.C"] {
			t.Fatalf("colliding tags use the same generated identifier %q:\n%s", identifiers["A_B_C"], generated)
		}
		if index == 0 {
			first = identifiers
			continue
		}
		for tag, want := range first {
			if got := identifiers[tag]; got != want {
				t.Fatalf("identifier for %q changed with traversal order: got %q, want %q", tag, got, want)
			}
		}
	}
}

func TestGenerateAvoidsSameFileAuthoredIdentifierCollisions(t *testing.T) {
	const authoredName = "_goxComponent_view_Button"
	tests := []struct {
		name        string
		declaration string
	}{
		{name: "variable", declaration: "var " + authoredName + " = 1"},
		{name: "constant", declaration: "const " + authoredName + " = 1"},
		{name: "type", declaration: "type " + authoredName + " struct{}"},
		{name: "function", declaration: "func " + authoredName + "() {}"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := "package app\n\n" + test.declaration + `

func View() any {
	return <Button />
}
`
			generated := generateDeterministically(t, "view.gox", source)
			if got := generatedComponentIdentifiers(t, generated)["Button"]; got == authoredName {
				t.Fatalf("generated identifier collides with authored %s %q:\n%s", test.name, authoredName, generated)
			}
		})
	}
}

func generateDeterministically(t *testing.T, filename, source string) []byte {
	t.Helper()
	options := GenerateOptions{
		Filename:        filename,
		PackageIdentity: "example.com/collision",
	}
	first, err := GenerateWithOptions([]byte(source), options)
	if err != nil {
		t.Fatalf("GenerateWithOptions(%s) error: %v", filename, err)
	}
	second, err := GenerateWithOptions([]byte(source), options)
	if err != nil {
		t.Fatalf("GenerateWithOptions(%s, second) error: %v", filename, err)
	}
	if string(first) != string(second) {
		t.Fatalf("GenerateWithOptions(%s) is not deterministic:\nfirst:\n%s\nsecond:\n%s", filename, first, second)
	}
	if _, err := parser.ParseFile(gotoken.NewFileSet(), filename+".go", first, parser.AllErrors); err != nil {
		t.Fatalf("generated Go for %s does not parse: %v\n%s", filename, err, first)
	}
	return first
}

func generatedComponentIdentifiers(t *testing.T, generated []byte) map[string]string {
	t.Helper()
	file, err := parser.ParseFile(gotoken.NewFileSet(), "generated.gox.go", generated, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse generated Go: %v\n%s", err, generated)
	}
	identifiers := make(map[string]string)
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok || gen.Tok != gotoken.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || len(value.Values) != 1 {
				continue
			}
			call, ok := value.Values[0].(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				continue
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "NewComponentType" {
				continue
			}
			debugName, ok := call.Args[1].(*ast.BasicLit)
			if !ok || debugName.Kind != gotoken.STRING {
				continue
			}
			name, err := strconv.Unquote(debugName.Value)
			if err != nil {
				t.Fatalf("unquote component debug name %q: %v", debugName.Value, err)
			}
			identifiers[name] = value.Names[0].Name
		}
	}
	return identifiers
}

func TestGenerateWithOptionsDifferentPackageIdentities(t *testing.T) {
	source := []byte(`package ui

import gf "github.com/graybuton/goframe/pkg/goframe"

func View() gf.Node {
	return <Header />
}
`)

	first, err := GenerateWithOptions(source, GenerateOptions{
		Filename:        "internal/ui/header.gox",
		PackageIdentity: "github.com/example/app/internal/ui",
	})
	if err != nil {
		t.Fatalf("GenerateWithOptions(first) error: %v", err)
	}
	second, err := GenerateWithOptions(source, GenerateOptions{
		Filename:        "internal/other/header.gox",
		PackageIdentity: "github.com/example/app/internal/other",
	})
	if err != nil {
		t.Fatalf("GenerateWithOptions(second) error: %v", err)
	}
	if string(first) == string(second) {
		t.Fatalf("different package identities produced identical output:\n%s", first)
	}
	if !strings.Contains(string(first), `github.com/example/app/internal/ui.Header`) {
		t.Fatalf("first output missing ui identity:\n%s", first)
	}
	if !strings.Contains(string(second), `github.com/example/app/internal/other.Header`) {
		t.Fatalf("second output missing other identity:\n%s", second)
	}
}

func TestGenerateQualifiedComponentExplicitAlias(t *testing.T) {
	source := []byte(`package demo

import (
	ui "github.com/example/app/internal/ui"
	gf "github.com/graybuton/goframe/pkg/goframe"
)

func View() gf.Node {
	return <ui.Header Title="Hello" />
}
`)

	generated, err := GenerateNamed("qualified_explicit.gox", source)
	if err != nil {
		t.Fatalf("GenerateNamed() error: %v", err)
	}
	text := string(generated)
	for _, want := range []string{
		`gf.NewComponentType("github.com/example/app/internal/ui.Header", "ui.Header")`,
		`gf.ComponentT(_goxComponent_qualified_explicit_ui_Header, ui.HeaderProps{`,
		`Title: "Hello"`,
		`}, ui.Header)`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated source does not contain %q:\n%s", want, text)
		}
	}
}

func TestGenerateQualifiedComponentImplicitAlias(t *testing.T) {
	source := []byte(`package demo

import (
	"github.com/example/app/internal/ui"
	gf "github.com/graybuton/goframe/pkg/goframe"
)

func View() gf.Node {
	return <ui.Header Title="Hello" />
}
`)

	generated, err := GenerateNamed("qualified_implicit.gox", source)
	if err != nil {
		t.Fatalf("GenerateNamed() error: %v", err)
	}
	text := string(generated)
	if !strings.Contains(text, `gf.NewComponentType("github.com/example/app/internal/ui.Header", "ui.Header")`) {
		t.Fatalf("generated source does not use implicit import alias identity:\n%s", text)
	}
}

func TestGenerateQualifiedComponentImportAliasDoesNotDefineIdentity(t *testing.T) {
	source := []byte(`package demo

import (
	widgets "github.com/example/app/internal/ui"
	gf "github.com/graybuton/goframe/pkg/goframe"
)

func View() gf.Node {
	return <widgets.Header Title="Hello" />
}
`)

	generated, err := GenerateNamed("qualified_alias_rename.gox", source)
	if err != nil {
		t.Fatalf("GenerateNamed() error: %v", err)
	}
	text := string(generated)
	if !strings.Contains(text, `gf.NewComponentType("github.com/example/app/internal/ui.Header", "widgets.Header")`) {
		t.Fatalf("generated source does not preserve import-path identity across alias rename:\n%s", text)
	}
	if strings.Contains(text, `gf.NewComponentType("widgets.Header"`) {
		t.Fatalf("generated source used local alias as component identity:\n%s", text)
	}
}

func TestGenerateWithOptionsFilenameDoesNotDefineRuntimeIdentity(t *testing.T) {
	source := []byte(`package ui

import gf "github.com/graybuton/goframe/pkg/goframe"

func View() gf.Node {
	return <Header />
}
`)

	first, err := GenerateWithOptions(source, GenerateOptions{
		Filename:        "internal/ui/header.gox",
		PackageIdentity: "github.com/example/app/internal/ui",
	})
	if err != nil {
		t.Fatalf("GenerateWithOptions(first) error: %v", err)
	}
	second, err := GenerateWithOptions(source, GenerateOptions{
		Filename:        "internal/ui/alt_header.gox",
		PackageIdentity: "github.com/example/app/internal/ui",
	})
	if err != nil {
		t.Fatalf("GenerateWithOptions(second) error: %v", err)
	}
	const want = `gf.NewComponentType("github.com/example/app/internal/ui.Header", "Header")`
	if strings.Count(string(first), want) != 1 || strings.Count(string(second), want) != 1 {
		t.Fatalf("generated output missing stable runtime identity:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if string(first) == string(second) {
		t.Fatalf("different filenames should still produce distinct generated variable names")
	}
}

func TestGenerateQualifiedComponentVersionedImportPathsDiffer(t *testing.T) {
	source := []byte(`package demo

import (
	v1 "github.com/example/widgets"
	v2 "github.com/example/widgets/v2"
	gf "github.com/graybuton/goframe/pkg/goframe"
)

func View() gf.Node {
	return (
		<div>
			<v1.Card />
			<v2.Card />
		</div>
	)
}
`)

	generated, err := GenerateNamed("qualified_versions.gox", source)
	if err != nil {
		t.Fatalf("GenerateNamed() error: %v", err)
	}
	text := string(generated)
	for _, want := range []string{
		`gf.NewComponentType("github.com/example/widgets.Card", "v1.Card")`,
		`gf.NewComponentType("github.com/example/widgets/v2.Card", "v2.Card")`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated source does not contain %q:\n%s", want, text)
		}
	}
}

func TestGenerateQualifiedComponentCollisions(t *testing.T) {
	source := []byte(`package demo

import (
	layout "github.com/example/app/internal/layout"
	pages "github.com/example/app/internal/pages"
	gf "github.com/graybuton/goframe/pkg/goframe"
)

func View() gf.Node {
	return (
		<div>
			<layout.Header />
			<pages.Header />
		</div>
	)
}
`)

	generated, err := GenerateNamed("qualified_headers.gox", source)
	if err != nil {
		t.Fatalf("GenerateNamed() error: %v", err)
	}
	text := string(generated)
	for _, want := range []string{
		`gf.NewComponentType("github.com/example/app/internal/layout.Header", "layout.Header")`,
		`gf.NewComponentType("github.com/example/app/internal/pages.Header", "pages.Header")`,
		`_goxComponent_qualified_headers_layout_Header`,
		`_goxComponent_qualified_headers_pages_Header`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated source does not contain %q:\n%s", want, text)
		}
	}
}

func TestGenerateQualifiedRouterLink(t *testing.T) {
	source := []byte(`package demo

import gf "github.com/graybuton/goframe/pkg/goframe"

func View() gf.Node {
	return <gf.RouterLink To="/issues">Issues</gf.RouterLink>
}
`)

	generated, err := GenerateNamed("qualified_router_link.gox", source)
	if err != nil {
		t.Fatalf("GenerateNamed() error: %v", err)
	}
	text := string(generated)
	for _, want := range []string{
		`gf.NewComponentType("github.com/graybuton/goframe/pkg/goframe.RouterLink", "gf.RouterLink")`,
		`gf.ComponentT(_goxComponent_qualified_router_link_gf_RouterLink, gf.RouterLinkProps{`,
		`Children: []gf.Node{`,
		`gf.Text("Issues")`,
		`}, gf.RouterLink)`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated source does not contain %q:\n%s", want, text)
		}
	}
}

func TestGenerateQualifiedNestedCallbackReturn(t *testing.T) {
	source := []byte(`package demo

import (
	rows "github.com/example/app/internal/rows"
	gf "github.com/graybuton/goframe/pkg/goframe"
)

type Item struct {
	ID string
}

func View(items []Item) gf.Node {
	return (
		<ul>
			{gf.Map(items, func(item Item) gf.Node {
				return <rows.ItemRow Key={item.ID} Item={item} />
			})}
		</ul>
	)
}
`)

	generated, err := GenerateNamed("qualified_nested_callback.gox", source)
	if err != nil {
		t.Fatalf("GenerateNamed() error: %v", err)
	}
	text := string(generated)
	for _, want := range []string{
		`gf.NewComponentType("github.com/example/app/internal/rows.ItemRow", "rows.ItemRow")`,
		`gf.Key(gf.ToString(item.ID),`,
		`gf.ComponentT(_goxComponent_qualified_nested_callback_rows_ItemRow, rows.ItemRowProps{`,
		`}, rows.ItemRow)`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated source does not contain %q:\n%s", want, text)
		}
	}
}

func TestGenerateQualifiedUnknownAliasDiagnostic(t *testing.T) {
	source := []byte(`package demo

func View() any {
	return <ui.Header />
}
`)
	_, err := GenerateNamed("examples/broken/qualified.gox", source)
	if err == nil {
		t.Fatal("GenerateNamed() returned nil error")
	}
	for _, want := range []string{
		`examples/broken/qualified.gox:4:`,
		`unknown package alias "ui" in qualified component <ui.Header>`,
		`<ui.Header />`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
}

func TestParseElementReportsMismatchedTag(t *testing.T) {
	_, _, err := ParseElement("<div><span></div>")
	if err == nil || !strings.Contains(err.Error(), "expected closing tag </span>, got </div>") {
		t.Fatalf("error = %v, want mismatched closing tag error", err)
	}
}

func TestParseElementReportsFragmentClosingMismatch(t *testing.T) {
	for _, test := range []struct {
		source string
		want   string
	}{
		{"<><p></p></main>", "expected closing fragment </>, got </main>"},
		{"<main></>", "expected closing tag </main>, got </>"},
	} {
		_, _, err := ParseElement(test.source)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("ParseElement(%q) error = %v, want %q", test.source, err, test.want)
		}
	}
}

func TestGenerateNamedReportsFilenameAndGlobalLine(t *testing.T) {
	source := []byte(`package main

func App() any {
	return (
		<div>
			<span></main>
		</div>
	)
}
`)
	_, err := GenerateNamed("examples/broken/app.gox", source)
	if err == nil {
		t.Fatal("GenerateNamed() returned nil error")
	}
	for _, want := range []string{
		"examples/broken/app.gox:6:",
		"expected closing tag </span>, got </main>",
		"<span></main>",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
}

func TestGenerateNamedReturnsDiagnosticError(t *testing.T) {
	source := []byte(`package main

func App() any {
	return <main>{}</main>
}
`)
	_, err := GenerateNamed("examples/broken/app.gox", source)
	if err == nil {
		t.Fatal("GenerateNamed() returned nil error")
	}
	var diagnostic DiagnosticError
	if !errors.As(err, &diagnostic) {
		t.Fatalf("error %T is not DiagnosticError: %v", err, err)
	}
	if diagnostic.Diagnostic.Filename != "examples/broken/app.gox" {
		t.Fatalf("diagnostic filename = %q", diagnostic.Diagnostic.Filename)
	}
	if diagnostic.Diagnostic.Line != 4 || diagnostic.Diagnostic.Column == 0 {
		t.Fatalf("diagnostic location = %d:%d", diagnostic.Diagnostic.Line, diagnostic.Diagnostic.Column)
	}
	if !strings.Contains(diagnostic.Diagnostic.Message, "empty child expression") {
		t.Fatalf("diagnostic message = %q", diagnostic.Diagnostic.Message)
	}
}
