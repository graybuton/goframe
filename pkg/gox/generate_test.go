package gox

import (
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
