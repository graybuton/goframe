package main

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/graybuton/goframe/pkg/gox"
)

func TestDocumentMetadataAPIShapeGOXProjectionsGenerate(t *testing.T) {
	source, err := os.ReadFile("projections.gox")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := gox.GenerateNamed("projections.gox", source)
	if err != nil {
		t.Fatalf("GenerateNamed() error: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "projections.gox.go", generated, parser.AllErrors); err != nil {
		t.Fatalf("generated Go parse error: %v", err)
	}
	text := string(generated)
	for _, call := range []string{
		"gf.UseDocumentMetadata(",
		"gf.DocumentMetadataComponent",
		"gf.UseDocumentMetadataOwner()",
		"gf.UseOwnedDocumentMetadata(",
		"gf.UseDocumentMetadataHandoffExperiment(",
	} {
		if !strings.Contains(text, call) {
			t.Fatalf("generated projection does not contain %q", call)
		}
	}
}
