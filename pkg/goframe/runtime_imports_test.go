package goframe

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestProductionRuntimeAvoidsHeavyImports(t *testing.T) {
	forbidden := map[string]bool{
		"encoding/json": true,
		"fmt":           true,
		"log":           true,
		"net/http":      true,
		"reflect":       true,
		"regexp":        true,
		"runtime/debug": true,
		"unsafe":        true,
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(content), "goframe_debug") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Clean(name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, declaration := range file.Decls {
			imports, ok := declaration.(*ast.GenDecl)
			if !ok || imports.Tok != token.IMPORT {
				continue
			}
			for _, spec := range imports.Specs {
				path, err := strconv.Unquote(spec.(*ast.ImportSpec).Path.Value)
				if err != nil {
					t.Fatalf("unquote import in %s: %v", name, err)
				}
				if forbidden[path] {
					t.Fatalf("production runtime file %s imports %s", name, path)
				}
			}
		}
	}
}

func TestBrowserDebugFilesRequireDebugBuildTag(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.Contains(name, "_debug_") || !strings.HasSuffix(name, ".go") {
			continue
		}
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.HasSuffix(name, "_js.go") && !strings.Contains(string(content), "goframe_debug") {
			t.Fatalf("browser debug file %s is not gated by goframe_debug", name)
		}
	}
}

func TestProductionRuntimeDoesNotContainBrowserDebugGlobals(t *testing.T) {
	forbidden := []string{
		"goframeComponentRenderProbe",
		"goframeComponentPatchProbe",
		"goframeRenderProbe",
		"goframeDuplicateKeyWarnings",
		"goframeLifecycleWarnings",
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(content), "goframe_debug") {
			continue
		}
		for _, value := range forbidden {
			if strings.Contains(string(content), value) {
				t.Fatalf("production runtime file %s contains debug global %s", name, value)
			}
		}
	}
}
