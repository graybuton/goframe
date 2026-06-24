package gox

import (
	"errors"
	"go/parser"
	gotoken "go/token"
	"strings"
	"testing"
)

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
