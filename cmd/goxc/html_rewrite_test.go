package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCustomIndexRewritePreservesAuthoredMarkerlessContent(t *testing.T) {
	source := `<!doctype html>
<html><head></head><body>
<script src="wasm_exec.js"></script>
<script>WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject)</script>
<!-- bundle.wasm must remain unchanged here. -->
<p>bundle.wasm</p>
<div data-example="bundle.wasm"></div>
<script>
const unrelated = "bundle.wasm";
const runtimeExample = "wasm_exec.js";
</script>
<script type="application/json">{"asset":"bundle.wasm","runtime":"wasm_exec.js"}</script>
<style>.example::after { content: "bundle.wasm"; }</style>
</body></html>`

	got, err := rewriteIndexHTML(source, htmlRewriteOptions{
		wasmPath:    "assets/bundle.12345678.wasm",
		runtimePath: "assets/wasm_exec.87654321.js",
	})
	if err != nil {
		t.Fatalf("rewriteIndexHTML() error: %v", err)
	}
	for _, authored := range []string{
		"<!-- bundle.wasm must remain unchanged here. -->",
		"<p>bundle.wasm</p>",
		`data-example="bundle.wasm"`,
		`const unrelated = "bundle.wasm";`,
		`const runtimeExample = "wasm_exec.js";`,
		`{"asset":"bundle.wasm","runtime":"wasm_exec.js"}`,
		`content: "bundle.wasm"`,
	} {
		if !strings.Contains(got, authored) {
			t.Errorf("authored content %q was rewritten", authored)
		}
	}
}

func TestCustomIndexRewriteRejectsOrphanManagedMarker(t *testing.T) {
	source := `<!-- goframe:runtime -->
<script src="wasm_exec.js"></script>`
	got, err := rewriteIndexHTML(source, htmlRewriteOptions{runtimePath: "assets/wasm_exec.87654321.js"})
	if err == nil {
		t.Fatalf("rewriteIndexHTML() = %q, want orphan-marker error", got)
	}
	if !strings.Contains(err.Error(), "orphan start") {
		t.Fatalf("rewriteIndexHTML() error = %v, want orphan start guidance", err)
	}
}

func TestCustomIndexPreloadUsesStructuralHeadClose(t *testing.T) {
	source := `<html><head><script>const example = "</head>";</script></head><body></body></html>`
	got, err := rewriteIndexHTML(source, htmlRewriteOptions{
		preload:     true,
		wasmPath:    "assets/bundle.wasm",
		runtimePath: "assets/wasm_exec.js",
	})
	if err != nil {
		t.Fatalf("rewriteIndexHTML() error: %v", err)
	}
	if strings.Contains(got, `const example = "<link`) {
		t.Fatal("preload markup was inserted into JavaScript string content")
	}
}

func TestCustomIndexForeignCDATAIsOpaque(t *testing.T) {
	for _, test := range []struct {
		name    string
		foreign string
	}{
		{
			name: "SVG",
			foreign: `<svg><g><![CDATA[
x >
<script src="wasm_exec.js?svg"></script>
<link rel="stylesheet" href="styles.css?svg">
</head>
<!-- goframe:runtime -->
]]></g></svg>`,
		},
		{
			name: "MathML",
			foreign: `<math><mrow><![CDATA[
x >
<script src="wasm_exec.js?math"></script>
<link rel="stylesheet" href="styles.css?math">
</head>
<!-- goframe:runtime -->
]]></mrow></math>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := htmlRewriteOptions{
				preload:     true,
				wasmPath:    "assets/bundle.11111111.wasm",
				runtimePath: "assets/wasm_exec.22222222.js",
				stylePaths:  []string{"assets/styles.33333333.css"},
				styleRewrites: map[string]string{
					"styles.css": "assets/styles.33333333.css",
				},
			}
			source := "<html><head>\n" + test.foreign + `
<script src="wasm_exec.js?real"></script>
<link rel="stylesheet" href="styles.css?real">
</head><body></body></html>`
			got, err := rewriteIndexHTML(source, options)
			if err != nil {
				t.Fatalf("rewriteIndexHTML() error: %v", err)
			}
			if !strings.Contains(got, test.foreign) {
				t.Fatalf("foreign CDATA bytes changed:\n%s", got)
			}
			for _, want := range []string{
				`src="assets/wasm_exec.22222222.js?real"`,
				`href="assets/styles.33333333.css?real"`,
			} {
				if !strings.Contains(got, want) {
					t.Errorf("real reference outside foreign CDATA missing %q:\n%s", want, got)
				}
			}
			preload := strings.Index(got, `<link rel="preload" href="assets/bundle.11111111.wasm"`)
			closingHead := strings.LastIndex(got, "</head>")
			if preload < 0 || closingHead < 0 || preload > closingHead {
				t.Fatalf("preload was not inserted before the real closing head:\n%s", got)
			}
		})
	}
}

func TestCustomIndexForeignIntegrationPointTransitions(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "SVG foreignObject",
			source: `<svg><FoReIgNoBjEcT><![CDATA[x > <script src="wasm_exec.js?cdata"></script>]]>
<script src="wasm_exec.js?child"></script></FoReIgNoBjEcT></svg>`,
			want: `src="assets/wasm_exec.22222222.js?child"`,
		},
		{
			name: "SVG desc",
			source: `<svg><desc><![CDATA[x > <script src="wasm_exec.js?cdata"></script>]]>
<script src="wasm_exec.js?child"></script></desc></svg>`,
			want: `src="assets/wasm_exec.22222222.js?child"`,
		},
		{
			name: "SVG title",
			source: `<svg><title><![CDATA[x > <script src="wasm_exec.js?cdata"></script>]]>
<script src="wasm_exec.js?child"></script></title></svg>`,
			want: `src="assets/wasm_exec.22222222.js?child"`,
		},
		{
			name: "MathML mi text integration point",
			source: `<math><mi><![CDATA[x > <script src="wasm_exec.js?cdata"></script>]]>
<script src="wasm_exec.js?child"></script></mi></math>`,
			want: `src="assets/wasm_exec.22222222.js?child"`,
		},
		{
			name: "MathML mo text integration point",
			source: `<math><mo><![CDATA[x > <script src="wasm_exec.js?cdata"></script>]]>
<script src="wasm_exec.js?child"></script></mo></math>`,
			want: `src="assets/wasm_exec.22222222.js?child"`,
		},
		{
			name: "MathML mn text integration point",
			source: `<math><mn><![CDATA[x > <script src="wasm_exec.js?cdata"></script>]]>
<script src="wasm_exec.js?child"></script></mn></math>`,
			want: `src="assets/wasm_exec.22222222.js?child"`,
		},
		{
			name: "MathML ms text integration point",
			source: `<math><ms><![CDATA[x > <script src="wasm_exec.js?cdata"></script>]]>
<script src="wasm_exec.js?child"></script></ms></math>`,
			want: `src="assets/wasm_exec.22222222.js?child"`,
		},
		{
			name: "MathML mtext integration point",
			source: `<math><mtext><![CDATA[x > <script src="wasm_exec.js?cdata"></script>]]>
<script src="wasm_exec.js?child"></script></mtext></math>`,
			want: `src="assets/wasm_exec.22222222.js?child"`,
		},
		{
			name: "MathML annotation HTML encoding",
			source: "<math><annotation-xml encoding=\"\tTEXT/HTML\r\"><![CDATA[x > <script src=\"wasm_exec.js?cdata\"></script>]]>\n" +
				"<script src=\"wasm_exec.js?child\"></script></annotation-xml></math>",
			want: `src="assets/wasm_exec.22222222.js?child"`,
		},
		{
			name: "MathML annotation XHTML encoding",
			source: `<math><annotation-xml encoding="application/xhtml+xml"><![CDATA[x > <script src="wasm_exec.js?cdata"></script>]]>
<script src="wasm_exec.js?child"></script></annotation-xml></math>`,
			want: `src="assets/wasm_exec.22222222.js?child"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := rewriteIndexForTest(t, test.source, htmlRewriteOptions{
				runtimePath: "assets/wasm_exec.22222222.js",
			})
			if !strings.Contains(got, `src="wasm_exec.js?cdata"`) {
				t.Fatalf("integration-point CDATA changed:\n%s", got)
			}
			if !strings.Contains(got, test.want) {
				t.Fatalf("HTML child below integration point was not rewritten:\n%s", got)
			}
		})
	}
}

func TestCustomIndexForeignNamespaceControls(t *testing.T) {
	options := htmlRewriteOptions{runtimePath: "assets/wasm_exec.22222222.js"}
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "HTML namespace CDATA-like markup",
			source: `<div><![CDATA[x > <script src="wasm_exec.js?html"></script>]]></div>`,
			want:   `src="assets/wasm_exec.22222222.js?html"`,
		},
		{
			name:   "HTML child below SVG integration point",
			source: `<svg><foreignObject><div><![CDATA[x > <script src="wasm_exec.js?html"></script>]]></div></foreignObject></svg>`,
			want:   `src="assets/wasm_exec.22222222.js?html"`,
		},
		{
			name:   "self-closing foreign child retains SVG context",
			source: `<svg><path/><![CDATA[x > <script src="wasm_exec.js?cdata"></script>]]></svg><script src="wasm_exec.js?real"></script>`,
			want:   `src="assets/wasm_exec.22222222.js?real"`,
		},
		{
			name:   "closing integration point restores SVG context",
			source: `<svg><foreignObject><div>HTML child</div></foreignObject><![CDATA[x > <script src="wasm_exec.js?cdata"></script>]]></svg><script src="wasm_exec.js?real"></script>`,
			want:   `src="assets/wasm_exec.22222222.js?real"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := rewriteIndexForTest(t, test.source, options)
			if !strings.Contains(got, test.want) {
				t.Fatalf("namespace control missing %q:\n%s", test.want, got)
			}
			if strings.Contains(test.source, "?cdata") && !strings.Contains(got, `src="wasm_exec.js?cdata"`) {
				t.Fatalf("self-closing foreign child exposed CDATA content:\n%s", got)
			}
		})
	}

	annotation := `<math><annotation-xml encoding=" text/html"><script src="wasm_exec.js?foreign"></script></annotation-xml></math>`
	if got := rewriteIndexForTest(t, annotation, options); got != annotation {
		t.Fatalf("non-HTML-space annotation encoding created an HTML integration point\ngot:  %q\nwant: %q", got, annotation)
	}
}

func TestCustomIndexUnterminatedForeignCDATAFailsClosed(t *testing.T) {
	for _, source := range []string{
		`<svg><![CDATA[x >`,
		`<math><![CDATA[x >`,
	} {
		got, err := rewriteIndexHTML(source, htmlRewriteOptions{})
		if err == nil {
			t.Fatalf("rewriteIndexHTML(%q) = %q, want unterminated CDATA error", source, got)
		}
		if !strings.Contains(err.Error(), "unterminated") || !strings.Contains(err.Error(), "CDATA") {
			t.Fatalf("rewriteIndexHTML(%q) error = %v, want CDATA guidance", source, err)
		}
		if got != "" {
			t.Fatalf("rewriteIndexHTML(%q) returned partial output %q", source, got)
		}
	}
}

func TestCustomIndexHTMLWhitespaceClassification(t *testing.T) {
	t.Run("script type", func(t *testing.T) {
		for _, test := range []struct {
			name    string
			value   string
			rewrite bool
		}{
			{name: "empty", value: "", rewrite: true},
			{name: "ASCII space", value: " application/javascript ", rewrite: true},
			{name: "tab", value: "\ttext/javascript\t", rewrite: true},
			{name: "line feed", value: "\napplication/javascript\n", rewrite: true},
			{name: "form feed", value: "\fapplication/javascript\f", rewrite: true},
			{name: "carriage return", value: "\rapplication/javascript\r", rewrite: true},
			{name: "MIME parameters", value: "application/javascript; charset=utf-8", rewrite: true},
			{name: "NBSP prefix", value: "\u00a0application/javascript"},
			{name: "NBSP suffix", value: "application/javascript\u00a0"},
			{name: "EM SPACE", value: "\u2003text/javascript"},
			{name: "NARROW NBSP", value: "\u202fapplication/javascript"},
		} {
			t.Run(test.name, func(t *testing.T) {
				source := `<script type="` + test.value + `" src="wasm_exec.js"></script><p>authored</p>`
				want := source
				if test.rewrite {
					want = strings.Replace(want, `src="wasm_exec.js"`, `src="assets/wasm_exec.22222222.js"`, 1)
				}
				got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: "assets/wasm_exec.22222222.js"})
				if got != want {
					t.Fatalf("script type classification mismatch\ngot:  %q\nwant: %q", got, want)
				}
			})
		}
	})

	t.Run("link rel", func(t *testing.T) {
		for _, test := range []struct {
			name    string
			value   string
			rewrite bool
		}{
			{name: "ASCII space", value: "alternate stylesheet", rewrite: true},
			{name: "tab", value: "alternate\tstylesheet", rewrite: true},
			{name: "line feed", value: "preload\nstylesheet", rewrite: true},
			{name: "form feed", value: "alternate\fstylesheet", rewrite: true},
			{name: "carriage return", value: "alternate\rstylesheet", rewrite: true},
			{name: "NBSP", value: "alternate\u00a0stylesheet"},
			{name: "EM SPACE", value: "alternate\u2003stylesheet"},
			{name: "NARROW NBSP", value: "preload\u202fstylesheet"},
		} {
			t.Run(test.name, func(t *testing.T) {
				source := `<link rel="` + test.value + `" href="styles.css"><p>authored</p>`
				want := source
				if test.rewrite {
					want = strings.Replace(want, `href="styles.css"`, `href="assets/styles.33333333.css"`, 1)
				}
				got := rewriteIndexForTest(t, source, htmlRewriteOptions{styleRewrites: map[string]string{
					"styles.css": "assets/styles.33333333.css",
				}})
				if got != want {
					t.Fatalf("link rel classification mismatch\ngot:  %q\nwant: %q", got, want)
				}
			})
		}
	})

	t.Run("link as", func(t *testing.T) {
		for _, test := range []struct {
			name    string
			value   string
			rewrite bool
		}{
			{name: "plain", value: "style", rewrite: true},
			{name: "ASCII space", value: " style ", rewrite: true},
			{name: "NBSP prefix", value: "\u00a0style"},
			{name: "NBSP suffix", value: "style\u00a0"},
			{name: "EM SPACE", value: "\u2003style"},
			{name: "NARROW NBSP", value: "style\u202f"},
		} {
			t.Run(test.name, func(t *testing.T) {
				source := `<link rel="preload" as="` + test.value + `" href="styles.css"><p>authored</p>`
				want := source
				if test.rewrite {
					want = strings.Replace(want, `href="styles.css"`, `href="assets/styles.33333333.css"`, 1)
				}
				got := rewriteIndexForTest(t, source, htmlRewriteOptions{styleRewrites: map[string]string{
					"styles.css": "assets/styles.33333333.css",
				}})
				if got != want {
					t.Fatalf("link as classification mismatch\ngot:  %q\nwant: %q", got, want)
				}
			})
		}
	})
}

func TestCustomIndexHTMLSemanticsCombinedDocument(t *testing.T) {
	const nbsp = "\u00a0"
	options := htmlRewriteOptions{
		preload:     true,
		wasmPath:    "assets/bundle.11111111.wasm",
		runtimePath: "assets/wasm_exec.22222222.js",
		stylePaths:  []string{"assets/styles.33333333.css"},
		styleRewrites: map[string]string{
			"styles.css": "assets/styles.33333333.css",
		},
	}
	source := `<html><head>
<svg><![CDATA[x > <script src="wasm_exec.js?svg"></script><link rel="stylesheet" href="styles.css?svg"></head>]]></svg>
<math><![CDATA[x > <script src="wasm_exec.js?math"></script><link rel="stylesheet" href="styles.css?math"></head>]]></math>
<script src="wasm_exec.js?real"></script>
<script type="` + nbsp + `application/javascript" src="wasm_exec.js?nbsp"></script>
<link rel="stylesheet" href="styles.css?real">
<link rel="alternate` + nbsp + `stylesheet" href="styles.css?rel">
<link rel="preload" as="style" href="styles.css?preload">
<link rel="preload" as="` + nbsp + `style" href="styles.css?as">
</head><body></body></html>`
	want := strings.Replace(source, `src="wasm_exec.js?real"`, `src="assets/wasm_exec.22222222.js?real"`, 1)
	want = strings.Replace(want, `href="styles.css?real"`, `href="assets/styles.33333333.css?real"`, 1)
	want = strings.Replace(want, `href="styles.css?preload"`, `href="assets/styles.33333333.css?preload"`, 1)
	closingHead := strings.LastIndex(want, "</head>")
	if closingHead < 0 {
		t.Fatal("combined source has no structural closing head")
	}
	want = want[:closingHead] + preloadHTML(options) + "\n" + want[closingHead:]
	got := rewriteIndexForTest(t, source, options)
	if got != want {
		t.Fatalf("combined HTML semantics rewrite mismatch\ngot:  %q\nwant: %q", got, want)
	}
	if second := rewriteIndexForTest(t, got, options); second != got {
		t.Fatalf("combined HTML semantics rewrite is not idempotent\nfirst:  %q\nsecond: %q", got, second)
	}
}

func TestCustomIndexSimilarNamesRemainAuthored(t *testing.T) {
	source := `<p>my-bundle.wasm bundle.wasm.backup bundle.wasm.map main.wasm.example wasm_exec.js.map custom-wasm_exec.js</p>`
	got, err := rewriteIndexHTML(source, htmlRewriteOptions{
		wasmPath:    "assets/bundle.12345678.wasm",
		runtimePath: "assets/wasm_exec.87654321.js",
	})
	if err != nil {
		t.Fatalf("rewriteIndexHTML() error: %v", err)
	}
	if got != source {
		t.Fatalf("similar authored names were partially rewritten: %q", got)
	}
}

func TestCustomIndexMalformedManagedMarkerMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "start without end", source: `<!-- goframe:runtime --><script src="wasm_exec.js"></script>`},
		{name: "end without start", source: `<!-- /goframe:runtime --><script src="wasm_exec.js"></script>`},
		{name: "duplicate start", source: `<!-- goframe:runtime --><!-- goframe:runtime --><!-- /goframe:runtime -->`},
		{name: "duplicate end", source: `<!-- goframe:runtime --><!-- /goframe:runtime --><!-- /goframe:runtime -->`},
		{name: "two complete blocks", source: `<!-- goframe:runtime --><!-- /goframe:runtime --><!-- goframe:runtime --><!-- /goframe:runtime -->`},
		{name: "reversed order", source: `<!-- /goframe:runtime --><!-- goframe:runtime --><!-- /goframe:runtime -->`},
		{name: "nested blocks", source: `<!-- goframe:runtime --><!-- goframe:bootstrap --><!-- /goframe:bootstrap --><!-- /goframe:runtime -->`},
		{name: "interleaved blocks", source: `<!-- goframe:runtime --><!-- goframe:bootstrap --><!-- /goframe:runtime --><!-- /goframe:bootstrap -->`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := rewriteIndexHTML(test.source, htmlRewriteOptions{
				wasmPath:    "assets/bundle.wasm",
				runtimePath: "assets/wasm_exec.js",
			})
			if err == nil {
				t.Fatalf("rewriteIndexHTML() = %q, want malformed-marker error", got)
			}
			if !strings.Contains(err.Error(), "goframe:") {
				t.Fatalf("rewriteIndexHTML() error = %v, want managed-block guidance", err)
			}
		})
	}
}

func TestCustomIndexMarkerTextInsideRawElementsIsAuthored(t *testing.T) {
	source := `<script>const marker = "<!-- goframe:runtime --><!-- /goframe:runtime -->";</script><style>/* <!-- goframe:bootstrap --><!-- /goframe:bootstrap --> */</style><title><!-- goframe:preload --><!-- /goframe:preload --></title><textarea><!-- goframe:runtime --><!-- /goframe:runtime --></textarea>`
	got, err := rewriteIndexHTML(source, htmlRewriteOptions{
		wasmPath:    "assets/bundle.wasm",
		runtimePath: "assets/wasm_exec.js",
	})
	if err != nil {
		t.Fatalf("rewriteIndexHTML() error: %v", err)
	}
	if got != source {
		t.Fatalf("marker-looking raw text was treated as managed markup: %q", got)
	}
}

func TestCustomIndexManagedBlocksAreAuthoritativeAndIdempotent(t *testing.T) {
	source := `<!doctype html>
<html>
<head>
<!-- goframe:preload -->
authored preload interior
<!-- /goframe:preload -->
<link rel="stylesheet" href="styles.css">
</head>
<body>
<!-- goframe:runtime -->
<script src="wasm_exec.js"></script>
<!-- /goframe:runtime -->
<script src="wasm_exec.js" data-authored="outside"></script>
<!-- goframe:bootstrap -->
<script>fetch("bundle.wasm")</script>
<!-- /goframe:bootstrap -->
<script>fetch("bundle.wasm"); const docs = "bundle.wasm";</script>
</body>
</html>`
	options := htmlRewriteOptions{
		preload:     true,
		wasmPath:    "assets/bundle.12345678.wasm",
		runtimePath: "assets/wasm_exec.87654321.js",
		stylePaths:  []string{"assets/styles.abcdef01.css"},
		styleRewrites: map[string]string{
			"styles.css": "assets/styles.abcdef01.css",
		},
	}

	got := rewriteIndexForTest(t, source, options)
	for _, want := range []string{
		"<!-- goframe:preload -->",
		"<!-- /goframe:preload -->",
		`href="assets/bundle.12345678.wasm" as="fetch"`,
		`href="assets/styles.abcdef01.css" as="style"`,
		"<!-- goframe:runtime -->",
		`<script src="assets/wasm_exec.87654321.js"></script>`,
		"<!-- /goframe:runtime -->",
		"<!-- goframe:bootstrap -->",
		`fetch("assets/bundle.12345678.wasm")`,
		"<!-- /goframe:bootstrap -->",
		`<script src="wasm_exec.js" data-authored="outside"></script>`,
		`<script>fetch("bundle.wasm"); const docs = "bundle.wasm";</script>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("managed rewrite missing %q:\n%s", want, got)
		}
	}
	if second := rewriteIndexForTest(t, got, options); second != got {
		t.Fatalf("managed rewrite is not idempotent\nfirst:\n%s\nsecond:\n%s", got, second)
	}

	withoutPreload := options
	withoutPreload.preload = false
	empty := rewriteIndexForTest(t, source, withoutPreload)
	if !strings.Contains(empty, "<!-- goframe:preload -->\n\n<!-- /goframe:preload -->") {
		t.Fatalf("disabled preload did not retain empty managed delimiters:\n%s", empty)
	}
}

func TestCustomIndexLegacyRuntimeReferenceMatrix(t *testing.T) {
	source := `<!-- <script src="wasm_exec.js"></script> -->
<script defer src="wasm_exec.js"></script>
<SCRIPT data-order="first" SRC='./wasm_exec.js?v=1#runtime'></SCRIPT>
<script src=wasm_exec.js></script>
<script src="https://example.test/wasm_exec.js"></script>
<script src="//cdn.test/wasm_exec.js"></script>
<script src="/wasm_exec.js"></script>
<script src="data:text/javascript,wasm_exec.js"></script>
<script src="blob:wasm_exec.js"></script>
<script type="application/json" src="wasm_exec.js"></script>
<script src="wasm_exec.js.map"></script>
<script src="my-wasm_exec.js"></script>
<template><script src="wasm_exec.js"></script></template>`
	got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: "assets/wasm_exec.87654321.js"})

	for _, want := range []string{
		`src="assets/wasm_exec.87654321.js"`,
		`SRC='assets/wasm_exec.87654321.js?v=1#runtime'`,
		`src=assets/wasm_exec.87654321.js`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("runtime rewrite missing %q:\n%s", want, got)
		}
	}
	for _, preserved := range []string{
		`<!-- <script src="wasm_exec.js"></script> -->`,
		`src="https://example.test/wasm_exec.js"`,
		`src="//cdn.test/wasm_exec.js"`,
		`src="/wasm_exec.js"`,
		`src="data:text/javascript,wasm_exec.js"`,
		`src="blob:wasm_exec.js"`,
		`<script type="application/json" src="wasm_exec.js"></script>`,
		`src="wasm_exec.js.map"`,
		`src="my-wasm_exec.js"`,
		`<template><script src="wasm_exec.js"></script></template>`,
	} {
		if !strings.Contains(got, preserved) {
			t.Errorf("runtime authored context %q changed:\n%s", preserved, got)
		}
	}
}

func TestCustomIndexMarkerlessNonBootstrapReferencesRemainAuthored(t *testing.T) {
	source := `<script>
fetch("bundle.wasm");
fetch ( './main.wasm?v=1#app' );
fetch /* compatibility */ ( "bundle.wasm#second", options );
// fetch("bundle.wasm")
/* fetch('main.wasm') */
const docs = "bundle.wasm";
const template = ` + "`fetch(\"bundle.wasm\")`" + `;
const pattern = /fetch\("bundle\.wasm"\)/;
if (dynamicURL) /fetch\("bundle.wasm"\)/.test(dynamicURL);
obj.fetch("bundle.wasm");
fetch(dynamicURL);
fetch("bundle.wasm.map");
</script>
<script type="application/json">{"asset":"bundle.wasm","call":"fetch(\\"bundle.wasm\\")"}</script>
<script type="importmap">{"imports":{"app":"bundle.wasm"}}</script>
<script type="speculationrules">{"prefetch":[{"source":"bundle.wasm"}]}</script>
<script type="text/plain">fetch("bundle.wasm")</script>
<template><script>fetch("bundle.wasm")</script></template>`
	got := rewriteIndexForTest(t, source, htmlRewriteOptions{wasmPath: "assets/bundle.12345678.wasm"})
	if got != source {
		t.Fatalf("non-bootstrap references changed\ngot:  %q\nwant: %q", got, source)
	}
}

func TestCustomIndexLegacyWASMUnrelatedJavaScriptMatrix(t *testing.T) {
	tests := []struct {
		name   string
		script string
	}{
		{
			name:   "postfix increment division",
			script: "let x = 10;\nlet y = 2;\nconst result = x++ / y;",
		},
		{
			name:   "postfix decrement division",
			script: "let x = 10;\nlet y = 2;\nconst result = x-- / y;",
		},
		{
			name:   "parenthesized postfix division",
			script: "let x = 10;\nlet y = 2;\nconst result = (x++) / y;",
		},
		{
			name:   "indexed postfix division",
			script: "const values = [10];\nlet index = 0;\nconst divisor = 2;\nconst result = values[index]++ / divisor;",
		},
		{
			name:   "member postfix division",
			script: "const object = { value: 10 };\nconst divisor = 2;\nconst result = object.value-- / divisor;",
		},
		{
			name:   "chained division",
			script: "const a = 24;\nconst b = 3;\nconst c = 2;\nconst result = a / b / c;",
		},
		{
			name:   "division assignment",
			script: "let a = 10;\nconst b = 2;\na /= b;",
		},
		{
			name:   "unary plus division",
			script: "const a = 1;\nconst b = 4;\nconst c = 2;\nconst result = a + +b / c;",
		},
		{
			name: "authored legacy-looking strings",
			script: `const docs = "bundle.wasm";
const runtimeDocs = "wasm_exec.js";
let x = 10;
const y = 2;
const result = x++ / y;`,
		},
		{
			name: "authored legacy-looking regex",
			script: `const pattern = /fetch\("bundle\.wasm"\)/;
let x = 10;
const y = 2;
const result = x-- / y;`,
		},
		{
			name:   "authored legacy-looking template",
			script: "const template = `fetch(\"bundle.wasm\")`;\nconst values = [10];\nlet index = 0;\nconst divisor = 2;\nconst result = values[index]++ / divisor;",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := "<script>\n" + test.script + "\n</script>"
			got, err := rewriteIndexHTML(source, htmlRewriteOptions{
				wasmPath: "assets/bundle.12345678.wasm",
			})
			if err != nil {
				t.Fatalf("rewriteIndexHTML() rejected unrelated valid JavaScript: %v", err)
			}
			if got != source {
				t.Fatalf("unrelated JavaScript changed\ngot:  %q\nwant: %q", got, source)
			}
		})
	}
}

func TestCustomIndexHistoricalBootstrapCorpus(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		legacyURL string
	}{
		{
			name: "v0.1.0-mvp10 markerless main bootstrap",
			body: `
        const go = new Go();
        WebAssembly.instantiateStreaming(fetch("main.wasm"), go.importObject)
            .then((result) => go.run(result.instance));
    `,
			legacyURL: "main.wasm",
		},
		{
			name: "current generated bundle bootstrap",
			body: `
    const go = new Go();
    WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject)
        .then((result) => go.run(result.instance));
`,
			legacyURL: "bundle.wasm",
		},
		{
			name: "committed single quote trailing comma fixture",
			body: `
        const go = new Go();
        WebAssembly.instantiateStreaming(
            fetch ( './bundle.wasm?fixture=legacy#wasm' ),
            go.importObject,
        ).then((result) => go.run(result.instance));
    `,
			legacyURL: "./bundle.wasm?fixture=legacy#wasm",
		},
		{
			name:      "v0.3.0-preview.1 dev callback bootstrap",
			body:      `var go = new Go(); WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject).then(function (result) { go.run(result.instance); });`,
			legacyURL: "bundle.wasm",
		},
		{
			name:      "v0.3.0-preview.1 load wrapped dev bootstrap",
			body:      `window.addEventListener("load", function () { var go = new Go(); WebAssembly.instantiateStreaming(fetch("./main.wasm#load"), go.importObject).then(function (result) { go.run(result.instance); }); }, { once: true });`,
			legacyURL: "./main.wasm#load",
		},
	}

	options := htmlRewriteOptions{wasmPath: "assets/bundle.12345678.wasm"}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if count := strings.Count(test.body, test.legacyURL); count != 1 {
				t.Fatalf("historical body legacy URL count = %d, want 1", count)
			}
			_, suffix := splitLegacyURL(test.legacyURL)
			source := "<script>" + test.body + "</script>"
			want := strings.Replace(
				source,
				test.legacyURL,
				options.wasmPath+suffix,
				1,
			)
			got := rewriteIndexForTest(t, source, options)
			if got != want {
				t.Fatalf("historical bootstrap rewrite mismatch\ngot:  %q\nwant: %q", got, want)
			}
			if second := rewriteIndexForTest(t, got, options); second != got {
				t.Fatalf("historical bootstrap rewrite is not idempotent\nfirst:  %q\nsecond: %q", got, second)
			}
		})
	}
}

func TestCustomIndexHistoricalBootstrapMultipleScripts(t *testing.T) {
	source := `<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch("main.wasm"), go.importObject).then((result) => go.run(result.instance));</script>
<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch('./bundle.wasm?v=1#app'), go.importObject,).then((result) => go.run(result.instance));</script>`
	want := `<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch("assets/bundle.12345678.wasm"), go.importObject).then((result) => go.run(result.instance));</script>
<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch('assets/bundle.12345678.wasm?v=1#app'), go.importObject,).then((result) => go.run(result.instance));</script>`
	got := rewriteIndexForTest(t, source, htmlRewriteOptions{
		wasmPath: "assets/bundle.12345678.wasm",
	})
	if got != want {
		t.Fatalf("multiple bootstrap rewrite mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestCustomIndexMarkerlessIncompleteJavaScriptIsPreserved(t *testing.T) {
	tests := []struct {
		name   string
		script string
	}{
		{name: "quoted string", script: `const example = "fetch('bundle.wasm')`},
		{name: "template literal", script: "const example = `fetch(\"bundle.wasm\")"},
		{name: "regular expression", script: `const example = /fetch\("bundle\.wasm"\)`},
		{name: "block comment", script: `/* fetch("bundle.wasm")`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := "<script>" + test.script + "</script>"
			got, err := rewriteIndexHTML(source, htmlRewriteOptions{
				wasmPath: "assets/bundle.12345678.wasm",
			})
			if err != nil {
				t.Fatalf("rewriteIndexHTML() treated authored JavaScript as a package error: %v", err)
			}
			if got != source {
				t.Fatalf("incomplete authored JavaScript changed\ngot:  %q\nwant: %q", got, source)
			}
		})
	}
}

func TestCustomIndexLegacyWASMArbitraryJavaScriptIsPreserved(t *testing.T) {
	tests := []struct {
		name   string
		script string
	}{
		{
			name:   "nested template",
			script: "const value = `outer ${`fetch(\"bundle.wasm\")`}`;",
		},
		{
			name:   "conditional nested template",
			script: "const value = `outer ${condition ? `fetch(\"bundle.wasm\")` : \"x\"}`;",
		},
		{
			name: "deep nested template",
			script: "const value = `outer ${\n" +
				"    (() => `nested ${`fetch(\"main.wasm\")`}`)()\n" +
				"}`;",
		},
		{
			name:   "regex after statement block",
			script: "if (ready) {}\n/fetch(\"bundle.wasm\")/.test(value);",
		},
		{
			name:   "regex after catch block",
			script: "try {} catch {}\n/fetch(\"main.wasm\")/.test(value);",
		},
		{
			name:   "regex after arrow block",
			script: "const fn = () => {};\n/fetch(\"bundle.wasm\")/.test(value);",
		},
		{
			name:   "regex after labeled block",
			script: "label: {}\n/fetch(\"bundle.wasm\")/.test(value);",
		},
		{
			name: "class private field and static block",
			script: `class Example {
    #value = "fetch(\"bundle.wasm\")";
    static {
        const pattern = /fetch\("bundle\.wasm"\)/;
    }
}`,
		},
		{
			name:   "optional calls",
			script: `const result = object?.method?.("bundle.wasm");`,
		},
		{
			name:   "nullish assignment",
			script: `const result = value ??= "fetch(\"bundle.wasm\")";`,
		},
		{
			name:   "dynamic import",
			script: `await import("./bundle.wasm");`,
		},
		{
			name:   "arbitrary direct fetch",
			script: `fetch("bundle.wasm");`,
		},
		{
			name: "unsupported let bootstrap",
			script: `let go = new Go();
WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject)
    .then((result) => go.run(result.instance));`,
		},
		{
			name: "absolute bootstrap URL",
			script: `const go = new Go();
WebAssembly.instantiateStreaming(fetch("https://example.test/bundle.wasm"), go.importObject)
    .then((result) => go.run(result.instance));`,
		},
		{
			name: "authored async runtime loader",
			script: `const runtimeScript = document.createElement("script");
runtimeScript.src = "wasm_exec.js";
runtimeScript.onload = async () => {
    const go = new Go();
    const result = await WebAssembly.instantiateStreaming(fetch("main.wasm"), go.importObject);
    go.run(result.instance);
};
document.head.appendChild(runtimeScript);`,
		},
		{
			name: "extra authored statement around bootstrap",
			script: `const docs = "custom";
const go = new Go();
WebAssembly.instantiateStreaming(
    fetch("bundle.wasm"),
    go.importObject,
).then((result) => go.run(result.instance));`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := "<script>\n" + test.script + "\n</script>"
			got, err := rewriteIndexHTML(source, htmlRewriteOptions{
				wasmPath: "assets/bundle.12345678.wasm",
			})
			if err != nil {
				t.Fatalf("rewriteIndexHTML() treated authored JavaScript as a package error: %v", err)
			}
			if got != source {
				t.Fatalf("authored JavaScript changed\ngot:  %q\nwant: %q", got, source)
			}
		})
	}
}

func TestCustomIndexLegacyWASMMixedPageRewritesOnlyCompleteBootstrap(t *testing.T) {
	source := `<script>const value = ` + "`outer ${`fetch(\"bundle.wasm\")`}`" + `;</script>
<script>if (ready) {}
/fetch("bundle.wasm")/.test(value);</script>
<script>fetch("bundle.wasm");</script>
<script>
    const go = new Go();
    WebAssembly.instantiateStreaming(fetch("bundle.wasm?v=1#app"), go.importObject)
        .then((result) => go.run(result.instance));
</script>`
	want := `<script>const value = ` + "`outer ${`fetch(\"bundle.wasm\")`}`" + `;</script>
<script>if (ready) {}
/fetch("bundle.wasm")/.test(value);</script>
<script>fetch("bundle.wasm");</script>
<script>
    const go = new Go();
    WebAssembly.instantiateStreaming(fetch("assets/bundle.12345678.wasm?v=1#app"), go.importObject)
        .then((result) => go.run(result.instance));
</script>`

	got, err := rewriteIndexHTML(source, htmlRewriteOptions{
		wasmPath: "assets/bundle.12345678.wasm",
	})
	if err != nil {
		t.Fatalf("rewriteIndexHTML() error: %v", err)
	}
	if got != want {
		t.Fatalf("mixed-page rewrite mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestCustomIndexLegacyStylesheetReferenceMatrix(t *testing.T) {
	source := `<link rel="stylesheet" href="styles.css">
<LINK HREF='./styles/theme.css?v=1#dark' REL='alternate STYLESHEET'>
<link as=STYLE href=styles.css#preload rel="modulepreload preload">
<a href="styles.css">documentation</a>
<div data-href="styles.css"></div>
<link rel="icon" href="styles.css">
<link rel="preload" as="script" href="styles.css">
<link rel="stylesheet" href="https://example.test/styles.css">
<link rel=stylesheet href=styles.css/>
<template><link rel="stylesheet" href="styles.css"></template>`
	got := rewriteIndexForTest(t, source, htmlRewriteOptions{styleRewrites: map[string]string{
		"styles.css":       "assets/styles.11111111.css",
		"styles/theme.css": "assets/styles/theme.22222222.css",
	}})

	for _, want := range []string{
		`href="assets/styles.11111111.css"`,
		`HREF='assets/styles/theme.22222222.css?v=1#dark'`,
		`href=assets/styles.11111111.css#preload`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("style rewrite missing %q:\n%s", want, got)
		}
	}
	for _, preserved := range []string{
		`<a href="styles.css">documentation</a>`,
		`data-href="styles.css"`,
		`<link rel="icon" href="styles.css">`,
		`<link rel="preload" as="script" href="styles.css">`,
		`href="https://example.test/styles.css"`,
		`<link rel=stylesheet href=styles.css/>`,
		`<template><link rel="stylesheet" href="styles.css"></template>`,
	} {
		if !strings.Contains(got, preserved) {
			t.Errorf("style authored context %q changed:\n%s", preserved, got)
		}
	}
}

func TestCustomIndexManagedPreloadDoesNotOwnExternalStyleSheet(t *testing.T) {
	source := `<html><head>
<!-- goframe:preload --><!-- /goframe:preload -->
<link rel="preload" as="style" href="styles.css">
<link rel="stylesheet" href="styles.css">
</head></html>`
	got := rewriteIndexForTest(t, source, htmlRewriteOptions{
		preload:     true,
		wasmPath:    "assets/bundle.wasm",
		runtimePath: "assets/wasm_exec.js",
		stylePaths:  []string{"assets/styles.11111111.css"},
		styleRewrites: map[string]string{
			"styles.css": "assets/styles.11111111.css",
		},
	})
	if !strings.Contains(got, `<link rel="preload" as="style" href="styles.css">`) {
		t.Fatalf("managed preload rewrote an authored external preload:\n%s", got)
	}
	if !strings.Contains(got, `<link rel="stylesheet" href="assets/styles.11111111.css">`) {
		t.Fatalf("stylesheet reference outside preload block was not rewritten:\n%s", got)
	}
}

func TestCustomIndexPreloadInsertionMatrix(t *testing.T) {
	options := htmlRewriteOptions{
		preload:     true,
		wasmPath:    "assets/bundle.wasm",
		runtimePath: "assets/wasm_exec.js",
	}
	for _, test := range []struct {
		name   string
		source string
		close  string
	}{
		{name: "normal head", source: `<html><head></head><body></body></html>`, close: "</head>"},
		{name: "uppercase head", source: `<HTML><HEAD></HEAD><BODY></BODY></HTML>`, close: "</HEAD>"},
		{name: "comment false close", source: `<html><head><!-- </head> --></head></html>`, close: "</head>"},
		{name: "script false close", source: `<html><head><script>const close = "</head>";</script></head></html>`, close: "</head>"},
		{name: "style false close", source: `<html><head><style>/* </head> */</style></head></html>`, close: "</head>"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := rewriteIndexForTest(t, test.source, options)
			insertion := strings.Index(got, `<link rel="preload" href="assets/bundle.wasm"`)
			structuralClose := strings.LastIndex(got, test.close)
			if insertion < 0 || structuralClose < 0 || insertion > structuralClose {
				t.Fatalf("preload insertion is not before structural head close:\n%s", got)
			}
		})
	}

	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{name: "missing head close", source: `<html><head><title>x</title>`, want: "add a closing </head>"},
		{name: "multiple head closes", source: `<html><head></head></head></html>`, want: "multiple structural </head>"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := rewriteIndexHTML(test.source, options)
			if err == nil {
				t.Fatalf("rewriteIndexHTML() = %q, want error", got)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("rewriteIndexHTML() error = %v, want %q", err, test.want)
			}
		})
	}

	withoutPreload := options
	withoutPreload.preload = false
	if got := rewriteIndexForTest(t, `<html><head>`, withoutPreload); got != `<html><head>` {
		t.Fatalf("disabled preload changed a document without closing head: %q", got)
	}
}

func TestCustomIndexBytePreservation(t *testing.T) {
	source := "<!DoCtYpE html>\r\n<HTML lang='界'>\r\n<HEAD data-doc=\"bundle.wasm\">\r\n" +
		"<link href='styles.css?v=1#x' data-order=first rel='stylesheet'>\r\n</HEAD>\r\n" +
		"<BODY>\r\n<script data-x=1 src='./wasm_exec.js?v=2'></script>\r\n" +
		"<script> const go = new Go(); WebAssembly.instantiateStreaming(fetch(\"bundle.wasm#boot\"), go.importObject).then((result) => go.run(result.instance)); </script>\r\n" +
		"<p> authored bundle.wasm 界 </p>\r\n</BODY>\r\n</HTML>"
	want := "<!DoCtYpE html>\r\n<HTML lang='界'>\r\n<HEAD data-doc=\"bundle.wasm\">\r\n" +
		"<link href='assets/styles.33333333.css?v=1#x' data-order=first rel='stylesheet'>\r\n</HEAD>\r\n" +
		"<BODY>\r\n<script data-x=1 src='assets/wasm_exec.22222222.js?v=2'></script>\r\n" +
		"<script> const go = new Go(); WebAssembly.instantiateStreaming(fetch(\"assets/bundle.11111111.wasm#boot\"), go.importObject).then((result) => go.run(result.instance)); </script>\r\n" +
		"<p> authored bundle.wasm 界 </p>\r\n</BODY>\r\n</HTML>"
	got := rewriteIndexForTest(t, source, htmlRewriteOptions{
		wasmPath:    "assets/bundle.11111111.wasm",
		runtimePath: "assets/wasm_exec.22222222.js",
		styleRewrites: map[string]string{
			"styles.css": "assets/styles.33333333.css",
		},
	})
	if got != want {
		t.Fatalf("source-preserving rewrite mismatch\ngot:  %q\nwant: %q", got, want)
	}
	if strings.HasSuffix(source, "\n") || strings.HasSuffix(got, "\n") {
		t.Fatal("rewrite changed trailing-newline absence")
	}
	if second := rewriteIndexForTest(t, got, htmlRewriteOptions{
		wasmPath:    "assets/bundle.11111111.wasm",
		runtimePath: "assets/wasm_exec.22222222.js",
		styleRewrites: map[string]string{
			"styles.css": "assets/styles.33333333.css",
		},
	}); second != got {
		t.Fatalf("markerless rewrite is not idempotent\nfirst:  %q\nsecond: %q", got, second)
	}

	trailingNewline := "<!doctype html>\n<html><head></head><body>authored</body></html>\n"
	if preserved := rewriteIndexForTest(t, trailingNewline, htmlRewriteOptions{}); preserved != trailingNewline {
		t.Fatalf("rewrite changed a trailing newline: %q", preserved)
	}
}

func TestCustomIndexMalformedSyntaxFailsClosed(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{name: "comment", source: `<!-- open`, want: "unterminated HTML comment"},
		{name: "tag", source: `<script src="wasm_exec.js"`, want: "unterminated"},
		{name: "script", source: `<script>fetch("bundle.wasm")`, want: "no closing </script>"},
		{name: "template", source: `<template><p>x</p>`, want: "opening <template>"},
		{name: "quoted attribute", source: `<script src="wasm_exec.js></script>`, want: "unterminated"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := rewriteIndexHTML(test.source, htmlRewriteOptions{})
			if err == nil {
				t.Fatalf("rewriteIndexHTML() = %q, want malformed syntax error", got)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("rewriteIndexHTML() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestWriteRewrittenIndexFailurePreservesFiles(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.html")
	destinationPath := filepath.Join(root, "destination.html")
	source := `<!-- goframe:runtime --><script src="wasm_exec.js"></script>`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destinationPath, []byte("destination sentinel\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := writeRewrittenIndex(sourcePath, destinationPath, htmlRewriteOptions{runtimePath: "assets/wasm_exec.js"})
	if err == nil || !strings.Contains(err.Error(), "orphan start") {
		t.Fatalf("writeRewrittenIndex() error = %v, want managed-marker failure", err)
	}
	assertFileContent(t, sourcePath, source)
	assertFileContent(t, destinationPath, "destination sentinel\n")
}

func TestPackageCustomIndexRewriteFailurePreservesPublishedPackage(t *testing.T) {
	appDir := t.TempDir()
	writeMinimalPackageApp(t, appDir)
	malformed := `<!doctype html><html><body><!-- goframe:runtime --><script src="wasm_exec.js"></script></body></html>`
	writeTestFile(t, appDir, indexHTMLAssetName, malformed)

	outDir := filepath.Join(t.TempDir(), "package")
	writeCompleteCurrentPackage(t, outDir)
	before := snapshotInspectTree(t, outDir)
	var packageErr error
	output := captureStdout(t, func() {
		packageErr = packageApp(packageOptions{
			appDir: appDir, compiler: "go", outDir: outDir, compress: map[string]bool{},
		})
	})
	if packageErr == nil || !strings.Contains(packageErr.Error(), "orphan start") {
		t.Fatalf("packageApp() error = %v, want managed-marker failure", packageErr)
	}
	if strings.Contains(output, "packaged ") {
		t.Fatalf("failed package emitted success output: %q", output)
	}
	if got := snapshotInspectTree(t, outDir); !reflect.DeepEqual(got, before) {
		t.Fatalf("rewrite failure changed previous package\nbefore: %#v\nafter:  %#v", before, got)
	}
	assertFileContent(t, filepath.Join(appDir, indexHTMLAssetName), malformed)
	if ownership := inspectPackageOwnership(outDir); ownership.State != packageOwnedCurrent {
		t.Fatalf("previous package ownership = %+v, want current", ownership)
	}
}

func TestPackageCustomIndexUnterminatedForeignCDATAFailureIsAtomic(t *testing.T) {
	appDir := t.TempDir()
	writeMinimalPackageApp(t, appDir)
	malformed := "<!doctype html><html><body><svg><![CDATA[x >\n"
	writeTestFile(t, appDir, indexHTMLAssetName, malformed)

	temporaryRoot := t.TempDir()
	t.Setenv("TMPDIR", temporaryRoot)
	outDir := filepath.Join(t.TempDir(), "package")
	writeCompleteCurrentPackage(t, outDir)
	before := snapshotInspectTree(t, outDir)
	markerBefore, err := os.ReadFile(filepath.Join(outDir, packageMetadataName))
	if err != nil {
		t.Fatal(err)
	}
	var packageErr error
	output := captureStdout(t, func() {
		packageErr = packageApp(packageOptions{
			appDir: appDir, compiler: "go", outDir: outDir, compress: map[string]bool{},
		})
	})
	if packageErr == nil || !strings.Contains(packageErr.Error(), "unterminated") || !strings.Contains(packageErr.Error(), "CDATA") {
		t.Fatalf("packageApp() error = %v, want unterminated CDATA failure", packageErr)
	}
	if strings.Contains(output, "packaged ") {
		t.Fatalf("failed package emitted success output: %q", output)
	}
	if got := snapshotInspectTree(t, outDir); !reflect.DeepEqual(got, before) {
		t.Fatalf("CDATA rewrite failure changed previous package\nbefore: %#v\nafter:  %#v", before, got)
	}
	assertFileContent(t, filepath.Join(appDir, indexHTMLAssetName), malformed)
	markerAfter, err := os.ReadFile(filepath.Join(outDir, packageMetadataName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(markerAfter, markerBefore) {
		t.Fatalf("CDATA rewrite failure changed the previous completion marker\nbefore: %q\nafter:  %q", markerBefore, markerAfter)
	}
	entries, err := os.ReadDir(temporaryRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "goxc-package-") {
			t.Fatalf("CDATA rewrite failure retained temporary staging %s", entry.Name())
		}
	}
}

func rewriteIndexForTest(t *testing.T, source string, options htmlRewriteOptions) string {
	t.Helper()
	got, err := rewriteIndexHTML(source, options)
	if err != nil {
		t.Fatalf("rewriteIndexHTML() error: %v", err)
	}
	return got
}

func TestCustomIndexRewriteDoesNotEmitPartialOutputOnError(t *testing.T) {
	var output bytes.Buffer
	result, err := rewriteIndexHTML(`<!-- goframe:bootstrap -->`, htmlRewriteOptions{wasmPath: "assets/bundle.wasm"})
	output.WriteString(result)
	if err == nil {
		t.Fatal("rewriteIndexHTML() accepted orphan bootstrap marker")
	}
	if output.Len() != 0 {
		t.Fatalf("failed rewrite returned partial output: %q", output.String())
	}
}
