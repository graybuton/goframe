package gox

import (
	"go/parser"
	gotoken "go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const maxFuzzInputSize = 4096

func FuzzGenerate(f *testing.F) {
	addGOXFileSeeds(f, "testdata/*.gox")
	addGOXFileSeeds(f, "testdata/errors/*.gox")
	for _, seed := range []string{
		`package main

func View() any {
	return <main><h1>Hello</h1></main>
}
`,
		`package main

func View(show bool) any {
	return <>{show && <span>Visible</span>}</>
}
`,
		`package main

func View(ok bool) any {
	return <section>{ok ? <p>Yes</p> : <p>No</p>}</section>
}
`,
		`package main

func View(id string) any {
	return <Button Key={id} Label="Save" />
}
`,
		`package main

import ui "github.com/example/app/internal/ui"

func View() any {
	return <ui.Shell Title="Dashboard"><p>Body</p></ui.Shell>
}
`,
		`package main

func View() any {
	return <input type="text" value={value} />
}
`,
		`// package fake

package main

func View() any {
	return <Button />
}
`,
		`package main

import "fmt"
import gf "github.com/graybuton/goframe/pkg/goframe"

var _ = fmt.Sprintf

func View() gf.Node {
	return <Button />
}
`,
		`package main

import (
	"fmt" // ) is not the end of the import group
	gf "github.com/graybuton/goframe/pkg/goframe"
)

var _ = fmt.Sprintf

func View() gf.Node {
	return <Button />
}
`,
		`package main

/*
#include <stdlib.h>
*/
import "C"

import gf "github.com/graybuton/goframe/pkg/goframe"

func View() gf.Node {
	return <Button />
}
`,
		`package main

// View renders the page.
func View() any {
	return <Button />
}
`,
		`package main

import A_B "example.com/components/A_B"

func View() any {
	return <><A_B_C /><A_B.C /></>
}
`,
		`package main

func View() any {
	return <><A_B /><A-B /><A.B /><A/B /></>
}
`,
		`package main

var _goxComponent_input_Button = 1

func View() any {
	return <Button />
}
`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > maxFuzzInputSize {
			t.Skip("bounded fuzz input size")
		}
		generated, err := GenerateWithOptions([]byte(source), GenerateOptions{
			Filename:        "fuzz/input.gox",
			PackageIdentity: "github.com/graybuton/goframe/fuzz",
		})
		if err != nil {
			return
		}
		if !hasPackageDeclaration(source) {
			return
		}
		if _, err := parser.ParseFile(gotoken.NewFileSet(), "fuzz_output.go", generated, parser.AllErrors); err != nil {
			t.Fatalf("generated Go does not parse: %v\n%s", err, generated)
		}
	})
}

func FuzzParseElement(f *testing.F) {
	for _, seed := range []string{
		`<div />`,
		`<main class="page"><p>Hello {name}</p></main>`,
		`<><span>One</span><span>Two</span></>`,
		`<Button Key={id} Label="Save" />`,
		`<List>{gf.Map(items, func(item Item) gf.Node { return <Row Key={item.ID} Item={item} /> })}</List>`,
		`<section>{visible && <p>Visible</p>}</section>`,
		`<section>{ok ? <p>Yes</p> : <p>No</p>}</section>`,
		`<ui.Shell Title="Dashboard"><gf.RouterLink To="/issues">Issues</gf.RouterLink></ui.Shell>`,
		`<input type="text" />`,
		`<ui:Shell />`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, snippet string) {
		if len(snippet) > maxFuzzInputSize {
			t.Skip("bounded fuzz input size")
		}
		node, consumed, err := ParseElement(snippet)
		if err != nil {
			return
		}
		if consumed <= 0 || consumed > len(snippet) {
			t.Fatalf("consumed bytes = %d for input length %d", consumed, len(snippet))
		}
		generated, err := Codegen(node)
		if err != nil {
			return
		}
		if strings.TrimSpace(generated) == "" {
			t.Fatalf("Codegen returned empty output for %q", snippet)
		}
		if !strings.Contains(snippet, "{") {
			if _, err := parser.ParseExpr(generated); err != nil {
				t.Fatalf("generated expression does not parse: %v\n%s", err, generated)
			}
		}
	})
}

func addGOXFileSeeds(f *testing.F, pattern string) {
	files, err := filepath.Glob(pattern)
	if err != nil {
		f.Fatal(err)
	}
	for _, path := range files {
		source, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		if len(source) <= maxFuzzInputSize {
			f.Add(string(source))
		}
	}
}

func hasPackageDeclaration(source string) bool {
	for _, line := range strings.Split(source, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "package ") {
			return true
		}
	}
	return false
}
