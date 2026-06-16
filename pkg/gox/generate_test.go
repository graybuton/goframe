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
