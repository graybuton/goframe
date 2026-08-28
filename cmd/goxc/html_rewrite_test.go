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
			source: "<math><annotation-xml encoding=\"TEXT/HTML\"><![CDATA[x > <script src=\"wasm_exec.js?cdata\"></script>]]>\n" +
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

func TestCustomIndexAnnotationXMLIntegrationPointExactEncoding(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	for _, test := range []struct {
		name        string
		encoding    string
		integration bool
	}{
		{name: "HTML", encoding: "text/html", integration: true},
		{name: "HTML ASCII case", encoding: "TEXT/HTML", integration: true},
		{name: "HTML named reference", encoding: "text&sol;html", integration: true},
		{name: "XHTML", encoding: "application/xhtml+xml", integration: true},
		{name: "XHTML ASCII case", encoding: "APPLICATION/XHTML+XML", integration: true},
		{name: "leading space", encoding: " text/html"},
		{name: "trailing space", encoding: "text/html "},
		{name: "leading newline", encoding: "\ntext/html"},
		{name: "numeric trailing space", encoding: "text/html&#x20;"},
		{name: "leading NBSP", encoding: " text/html"},
		{name: "MIME parameter", encoding: "text/html;charset=utf-8"},
		{name: "empty", encoding: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			attribute := ""
			if test.name != "empty" {
				attribute = ` encoding="` + test.encoding + `"`
			}
			source := `<math><annotation-xml` + attribute + `><script src="wasm_exec.js?annotation"></script></annotation-xml></math>`
			got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath})
			rewritten := strings.Contains(got, `src="`+runtimePath+`?annotation"`)
			if rewritten != test.integration {
				t.Fatalf("annotation integration rewrite = %v, want %v\ngot:  %q\nsource: %q", rewritten, test.integration, got, source)
			}
			if !test.integration && got != source {
				t.Fatalf("foreign annotation bytes changed\ngot:  %q\nwant: %q", got, source)
			}
		})
	}

	source := `<math><annotation-xml encoding="text/html"><svg><title><script src="wasm_exec.js?nested"></script></title></svg></annotation-xml></math>`
	got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath})
	if !strings.Contains(got, `src="`+runtimePath+`?nested"`) {
		t.Fatalf("nested SVG integration point did not expose its HTML child: %q", got)
	}
}

func TestCustomIndexCompleteTagNameState(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	for _, name := range []string{
		"title_extra",
		"title.extra",
		"title=extra",
		`title"extra`,
		"title'extra",
		"title@extra",
		"titleé",
		"title:extra",
		"title-extra",
		"title\x00extra",
	} {
		t.Run("false integration "+name, func(t *testing.T) {
			source := `<svg><` + name + `><script src="wasm_exec.js?foreign"></script></` + name + `></svg>`
			if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath}); got != source {
				t.Fatalf("tag name %q became an SVG integration point\ngot:  %q\nwant: %q", name, got, source)
			}
		})
	}

	for _, element := range []struct {
		name       string
		attributes string
	}{
		{name: "p_extra"},
		{name: "div.extra"},
		{name: "span=extra"},
		{name: "font_extra", attributes: ` color="red"`},
		{name: "body@extra"},
	} {
		t.Run("false breakout "+element.name, func(t *testing.T) {
			source := `<svg><` + element.name + element.attributes + `><script src="wasm_exec.js?foreign"></script></` + element.name + `></svg>`
			if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath}); got != source {
				t.Fatalf("tag name %q caused a foreign-content breakout\ngot:  %q\nwant: %q", element.name, got, source)
			}
		})
	}

	for _, element := range []struct {
		name       string
		attributes string
		breakout   bool
	}{
		{name: "title"},
		{name: "foreignObject"},
		{name: "desc"},
		{name: "p", breakout: true},
		{name: "div", breakout: true},
		{name: "font", attributes: ` color="red"`, breakout: true},
	} {
		t.Run("genuine control "+element.name, func(t *testing.T) {
			source := `<svg><` + element.name + element.attributes + `><script src="wasm_exec.js?html"></script></` + element.name + `></svg>`
			got, err := rewriteIndexHTML(source, htmlRewriteOptions{runtimePath: runtimePath})
			if element.breakout {
				if err == nil || got != "" || !strings.Contains(err.Error(), "foreign-content parser recovery") {
					t.Fatalf("genuine %q breakout = %q, %v, want managed-first failure", element.name, got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("genuine %q integration point error: %v", element.name, err)
			}
			if !strings.Contains(got, `src="`+runtimePath+`?html"`) {
				t.Fatalf("genuine %q control did not expose HTML content: %q", element.name, got)
			}
		})
	}

	source := `<div id="value"><input disabled/><script src="wasm_exec.js?attributes"></script></div>`
	want := `<div id="value"><input disabled/><script src="` + runtimePath + `?attributes"></script></div>`
	if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath}); got != want {
		t.Fatalf("ordinary attribute boundaries changed\ngot:  %q\nwant: %q", got, want)
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
			{name: "MIME parameters", value: "application/javascript; charset=utf-8"},
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

func TestCustomIndexForeignBreakoutRequiresManagedOwnership(t *testing.T) {
	for _, source := range []string{
		`<svg><p><script src="wasm_exec.js"></script>`,
		`<math><div><script src="wasm_exec.js"></script>`,
		`<svg></p><script src="wasm_exec.js"></script>`,
		`<svg><font color="red"><script src="wasm_exec.js"></script>`,
	} {
		got, err := rewriteIndexHTML(source, htmlRewriteOptions{runtimePath: "assets/wasm_exec.22222222.js"})
		if err == nil || got != "" || !strings.Contains(err.Error(), "foreign-content parser recovery") ||
			!strings.Contains(err.Error(), "goframe:runtime") {
			t.Fatalf("foreign breakout rewrite = %q, %v, want managed-first failure", got, err)
		}
	}

	balancedForeign := `<svg><g><script src="wasm_exec.js"></script></g></svg>`
	if got := rewriteIndexForTest(t, balancedForeign, htmlRewriteOptions{runtimePath: "assets/wasm_exec.22222222.js"}); got != balancedForeign {
		t.Fatalf("balanced foreign authored content changed\ngot:  %q\nwant: %q", got, balancedForeign)
	}
}

func TestCustomIndexScriptClassificationBrowserSemantics(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	javaScriptMIMETypes := []string{
		"application/ecmascript", "application/javascript", "application/x-ecmascript", "application/x-javascript",
		"text/ecmascript", "text/javascript", "text/javascript1.0", "text/javascript1.1",
		"text/javascript1.2", "text/javascript1.3", "text/javascript1.4", "text/javascript1.5",
		"text/jscript", "text/livescript", "text/x-ecmascript", "text/x-javascript",
	}
	for _, value := range javaScriptMIMETypes {
		for _, variant := range []string{value, strings.ToUpper(value)} {
			t.Run(variant, func(t *testing.T) {
				source := `<script type="` + variant + `" src="wasm_exec.js"></script>`
				got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath})
				if !strings.Contains(got, runtimePath) {
					t.Fatalf("JavaScript MIME type %q was not executable: %s", variant, got)
				}
			})
		}
	}

	for _, test := range []struct {
		name    string
		attrs   string
		rewrite bool
	}{
		{name: "attributes absent", rewrite: true},
		{name: "empty type", attrs: ` type=""`, rewrite: true},
		{name: "boolean type", attrs: ` type`, rewrite: true},
		{name: "space-only type", attrs: ` type=" "`},
		{name: "module", attrs: ` type=" module "`, rewrite: true},
		{name: "module parameters", attrs: ` type="module;foo"`},
		{name: "JavaScript parameters", attrs: ` type="text/javascript; charset=utf-8"`},
		{name: "unknown JavaScript version", attrs: ` type="text/javascript1.6"`},
		{name: "empty language", attrs: ` language=""`, rewrite: true},
		{name: "boolean language", attrs: ` language`, rewrite: true},
		{name: "language JavaScript", attrs: ` language="javascript"`, rewrite: true},
		{name: "language JavaScript 1.5", attrs: ` language="JavaScript1.5"`, rewrite: true},
		{name: "language is not trimmed", attrs: ` language=" javascript"`},
		{name: "language VBScript", attrs: ` language="vbscript"`},
		{name: "type overrides language", attrs: ` type="application/json" language="javascript"`},
		{name: "import map", attrs: ` type="importmap"`},
		{name: "speculation rules", attrs: ` type="speculationrules"`},
		{name: "data block", attrs: ` type="application/json"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := `<script` + test.attrs + ` src="wasm_exec.js"></script>`
			got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath})
			rewritten := strings.Contains(got, runtimePath)
			if rewritten != test.rewrite {
				t.Fatalf("script classification rewrite = %v, want %v\ngot: %s", rewritten, test.rewrite, got)
			}
		})
	}
}

func TestCustomIndexPlaintextBrowserSemantics(t *testing.T) {
	t.Run("opaque through EOF", func(t *testing.T) {
		source := `<html><head></head><body><plaintext>
<script src="wasm_exec.js"></script>
<link rel="stylesheet" href="styles.css">
</head>
<!-- goframe:runtime -->`
		got := rewriteIndexForTest(t, source, htmlRewriteOptions{
			runtimePath: "assets/wasm_exec.22222222.js",
			styleRewrites: map[string]string{
				"styles.css": "assets/styles.33333333.css",
			},
		})
		if got != source {
			t.Fatalf("plaintext content changed\ngot:  %q\nwant: %q", got, source)
		}
	})

	t.Run("real head close before plaintext", func(t *testing.T) {
		options := htmlRewriteOptions{preload: true, wasmPath: "assets/bundle.wasm", runtimePath: "assets/wasm_exec.js"}
		preload := preloadHTMLForTest(t, options)
		source := `<html><head></head><body><plaintext></head><script src="wasm_exec.js"></script>`
		got := rewriteIndexForTest(t, source, options)
		insertion := strings.Index(got, preload)
		realClose := strings.Index(got, "</head>")
		if insertion < 0 || realClose < 0 || insertion > realClose {
			t.Fatalf("preload was not inserted before the real head close: %s", got)
		}
		if strings.Count(got, preload) != 1 || !strings.HasSuffix(got, `<plaintext></head><script src="wasm_exec.js"></script>`) {
			t.Fatalf("plaintext bytes after the real head close changed: %s", got)
		}
	})

	t.Run("fake head close after plaintext", func(t *testing.T) {
		source := `<html><head><plaintext></head>`
		got, err := rewriteIndexHTML(source, htmlRewriteOptions{preload: true, wasmPath: "assets/bundle.wasm"})
		if err == nil || !strings.Contains(err.Error(), "closing </head>") {
			t.Fatalf("rewriteIndexHTML() = %q, %v, want missing structural head failure", got, err)
		}
		if got != "" {
			t.Fatalf("plaintext preload failure returned partial output %q", got)
		}
	})

	t.Run("foreign plaintext remains foreign", func(t *testing.T) {
		source := `<svg><plaintext><script src="wasm_exec.js"></script></plaintext></svg>`
		if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: "assets/wasm_exec.js"}); got != source {
			t.Fatalf("foreign plaintext changed document\ngot:  %q\nwant: %q", got, source)
		}
	})

	t.Run("EOF immediately after start tag", func(t *testing.T) {
		source := `<plaintext>`
		if got := rewriteIndexForTest(t, source, htmlRewriteOptions{}); got != source {
			t.Fatalf("plaintext EOF changed: %q", got)
		}
	})
}

func TestCustomIndexSelfClosingSolidusBrowserSemantics(t *testing.T) {
	for _, test := range []struct {
		name        string
		source      string
		selfClosing bool
		value       string
	}{
		{name: "compact boolean", source: `<input disabled/>`, selfClosing: true},
		{name: "spaced boolean", source: `<input disabled />`, selfClosing: true},
		{name: "quoted value", source: `<input value="x"/>`, selfClosing: true, value: "x"},
		{name: "unquoted slash", source: `<input value=x/>`, value: "x/"},
		{name: "foreign compact boolean", source: `<path disabled/>`, selfClosing: true},
		{name: "foreign unquoted slash", source: `<path value=x/>`, value: "x/"},
	} {
		t.Run(test.name, func(t *testing.T) {
			tag, ok, err := scanHTMLTag(test.source, 0)
			if err != nil {
				t.Fatalf("scanHTMLTag() error: %v", err)
			}
			if !ok || tag.selfClosing != test.selfClosing {
				t.Fatalf("scanHTMLTag() = %+v, %v, want selfClosing %v", tag, ok, test.selfClosing)
			}
			if test.value != "" {
				attribute, err := tag.attribute("value")
				if err != nil || attribute == nil {
					t.Fatalf("value attribute = %+v, %v", attribute, err)
				}
				if got := test.source[attribute.valueStart:attribute.valueEnd]; got != test.value {
					t.Fatalf("value = %q, want %q", got, test.value)
				}
			}
		})
	}

	t.Run("HTML non-void flag requires managed ownership after misnesting", func(t *testing.T) {
		source := `<svg><foreignObject><div/><![CDATA[x > <script src="wasm_exec.js?html"></script>]]></foreignObject></svg>`
		got, err := rewriteIndexHTML(source, htmlRewriteOptions{runtimePath: "assets/wasm_exec.22222222.js"})
		if err == nil || got != "" || !strings.Contains(err.Error(), "misnested") || !strings.Contains(err.Error(), "goframe:runtime") {
			t.Fatalf("self-closing HTML recovery = %q, %v, want managed-first failure", got, err)
		}
	})

	t.Run("foreign self-closing flag closes context", func(t *testing.T) {
		source := `<svg><path disabled/><![CDATA[x > <script src="wasm_exec.js?foreign"></script>]]></svg>`
		if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: "assets/wasm_exec.22222222.js"}); got != source {
			t.Fatalf("foreign self-closing tag exposed inert CDATA\ngot:  %q\nwant: %q", got, source)
		}
	})
}

func TestCustomIndexURLPreprocessingBrowserSemantics(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{name: "runtime spaces", source: `<script src=" wasm_exec.js "></script>`, want: `<script src=" ` + runtimePath + ` "></script>`},
		{name: "runtime tab and newline", source: "<script src=\"\twasm_exec.js\n\"></script>", want: "<script src=\"\t" + runtimePath + "\n\"></script>"},
		{name: "runtime internal newline", source: "<script src=\"wasm_\nexec.js\"></script>", want: `<script src="` + runtimePath + `"></script>`},
		{name: "runtime NBSP", source: `<script src=" wasm_exec.js "></script>`, want: `<script src=" wasm_exec.js "></script>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := rewriteIndexForTest(t, test.source, htmlRewriteOptions{runtimePath: runtimePath})
			if got != test.want {
				t.Fatalf("runtime URL rewrite mismatch\ngot:  %q\nwant: %q", got, test.want)
			}
		})
	}

	for _, test := range []struct {
		name    string
		href    string
		rewrite bool
	}{
		{name: "spaces", href: " styles.css ", rewrite: true},
		{name: "linefeed and tab", href: "\nstyles.css\t", rewrite: true},
		{name: "internal tab", href: "styles.\tcss", rewrite: true},
		{name: "NBSP", href: " styles.css "},
	} {
		t.Run("style "+test.name, func(t *testing.T) {
			source := `<link rel="stylesheet" href="` + test.href + `">`
			got := rewriteIndexForTest(t, source, htmlRewriteOptions{styleRewrites: map[string]string{
				"styles.css": "assets/styles.33333333.css",
			}})
			rewritten := strings.Contains(got, "assets/styles.33333333.css")
			if rewritten != test.rewrite {
				t.Fatalf("style URL rewrite = %v, want %v: %q", rewritten, test.rewrite, got)
			}
		})
	}

	t.Run("style source span", func(t *testing.T) {
		source := `<link rel="stylesheet" href=" styles.css?v=1#theme ">`
		want := `<link rel="stylesheet" href=" assets/styles.33333333.css?v=1#theme ">`
		got := rewriteIndexForTest(t, source, htmlRewriteOptions{styleRewrites: map[string]string{
			"styles.css": "assets/styles.33333333.css",
		}})
		if got != want {
			t.Fatalf("style source span mismatch\ngot:  %q\nwant: %q", got, want)
		}
	})

	t.Run("historical bootstrap source span", func(t *testing.T) {
		source := `<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch(" bundle.wasm?v=1#app "), go.importObject).then((result) => go.run(result.instance));</script>`
		want := `<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch(" assets/bundle.11111111.wasm?v=1#app "), go.importObject).then((result) => go.run(result.instance));</script>`
		got := rewriteIndexForTest(t, source, htmlRewriteOptions{wasmPath: "assets/bundle.11111111.wasm"})
		if got != want {
			t.Fatalf("bootstrap URL rewrite mismatch\ngot:  %q\nwant: %q", got, want)
		}
	})

	t.Run("historical bootstrap escaped whitespace", func(t *testing.T) {
		source := `<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch("\tbundle.wasm\n"), go.importObject).then((result) => go.run(result.instance));</script>`
		want := `<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch("\tassets/bundle.11111111.wasm\n"), go.importObject).then((result) => go.run(result.instance));</script>`
		got := rewriteIndexForTest(t, source, htmlRewriteOptions{wasmPath: "assets/bundle.11111111.wasm"})
		if got != want {
			t.Fatalf("escaped bootstrap URL rewrite mismatch\ngot:  %q\nwant: %q", got, want)
		}
	})

	for _, value := range []string{"/bundle.wasm", "https://example.test/bundle.wasm", "bundle.wasm.map", "my-bundle.wasm", " bundle.wasm "} {
		t.Run("bootstrap non-match "+value, func(t *testing.T) {
			source := `<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch("` + value + `"), go.importObject).then((result) => go.run(result.instance));</script>`
			if got := rewriteIndexForTest(t, source, htmlRewriteOptions{wasmPath: "assets/bundle.11111111.wasm"}); got != source {
				t.Fatalf("non-matching bootstrap URL changed\ngot:  %q\nwant: %q", got, source)
			}
		})
	}
}

func TestCustomIndexPercentEncodedLegacyURLMatching(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	for _, value := range []string{
		"%77asm_exec.js",
		"wasm%5Fexec.js",
		"%77%61sm_exec.js",
	} {
		t.Run("runtime "+value, func(t *testing.T) {
			source := `<script src="` + value + `?v=1#runtime"></script>`
			want := `<script src="` + runtimePath + `?v=1#runtime"></script>`
			if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath}); got != want {
				t.Fatalf("percent-encoded runtime rewrite mismatch\ngot:  %q\nwant: %q", got, want)
			}
		})
	}

	const stylePath = "assets/styles.33333333.css"
	for _, source := range []string{
		`<link rel="stylesheet" href="%73tyles.css?theme#main">`,
		`<link rel="preload" as="style" href="%73tyles.css?theme#main">`,
	} {
		want := strings.Replace(source, "%73tyles.css", stylePath, 1)
		if got := rewriteIndexForTest(t, source, htmlRewriteOptions{styleRewrites: map[string]string{
			"styles.css": stylePath,
		}}); got != want {
			t.Fatalf("percent-encoded style rewrite mismatch\ngot:  %q\nwant: %q", got, want)
		}
	}

	for _, value := range []string{"%62undle.wasm", "%6dain.wasm"} {
		t.Run("bootstrap "+value, func(t *testing.T) {
			source := `<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch("` + value + `?v=1#app"), go.importObject).then((result) => go.run(result.instance));</script>`
			want := strings.Replace(source, value, "assets/bundle.11111111.wasm", 1)
			if got := rewriteIndexForTest(t, source, htmlRewriteOptions{wasmPath: "assets/bundle.11111111.wasm"}); got != want {
				t.Fatalf("percent-encoded bootstrap rewrite mismatch\ngot:  %q\nwant: %q", got, want)
			}
		})
	}

	t.Run("segment-safe UTF-8", func(t *testing.T) {
		source := `<link rel="stylesheet" href="%E7%95%8C.css">`
		want := `<link rel="stylesheet" href="assets/%E7%95%8C.44444444.css">`
		if got := rewriteIndexForTest(t, source, htmlRewriteOptions{styleRewrites: map[string]string{
			"界.css": "assets/界.44444444.css",
		}}); got != want {
			t.Fatalf("percent-encoded UTF-8 rewrite mismatch\ngot:  %q\nwant: %q", got, want)
		}
	})

	for _, value := range []string{
		"%2577asm_exec.js",
		"%2Fwasm_exec.js",
		"%5Cwasm_exec.js",
		"%00wasm_exec.js",
		"%GGwasm_exec.js",
		"%",
	} {
		t.Run("unowned "+value, func(t *testing.T) {
			source := `<script src="` + value + `"></script>`
			if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath}); got != source {
				t.Fatalf("unsafe or ambiguous percent reference changed\ngot:  %q\nwant: %q", got, source)
			}
		})
	}
}

func TestCustomIndexGeneratedPackagePathsBecomeBrowserURLs(t *testing.T) {
	options := htmlRewriteOptions{
		preload:     true,
		wasmPath:    "assets/bundle space#query?percent%界\x07.wasm",
		runtimePath: "assets/runtime space&quote\"apostrophe'界.js",
		stylePaths:  []string{"assets/style space&query?#percent%界.css"},
	}
	generated, err := generateIndexHTML(options)
	if err != nil {
		t.Fatalf("generateIndexHTML() error: %v", err)
	}
	for _, want := range []string{
		"assets/bundle%20space%23query%3Fpercent%25%E7%95%8C%07.wasm",
		"assets/runtime%20space%26quote%22apostrophe%27%E7%95%8C.js",
		"assets/style%20space%26query%3F%23percent%25%E7%95%8C.css",
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated index missing browser URL %q:\n%s", want, generated)
		}
	}
	for _, stale := range []string{"bundle space", "runtime space", "style space", `\\a`, `\\U`} {
		if strings.Contains(generated, stale) {
			t.Fatalf("generated index retained non-URL or Go-literal text %q:\n%s", stale, generated)
		}
	}

	runtimeSource := `<script src="wasm_exec.js?v=1#runtime"></script>`
	runtimeWant := `<script src="assets/runtime%20space%26quote%22apostrophe%27%E7%95%8C.js?v=1#runtime"></script>`
	markerlessOptions := options
	markerlessOptions.preload = false
	if got := rewriteIndexForTest(t, runtimeSource, markerlessOptions); got != runtimeWant {
		t.Fatalf("markerless runtime browser URL mismatch\ngot:  %q\nwant: %q", got, runtimeWant)
	}

	bootstrapSource := `<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch('bundle.wasm?v=1#app'), go.importObject).then((result) => go.run(result.instance));</script>`
	bootstrapWant := `<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch('assets/bundle%20space%23query%3Fpercent%25%E7%95%8C%07.wasm?v=1#app'), go.importObject).then((result) => go.run(result.instance));</script>`
	if got := rewriteIndexForTest(t, bootstrapSource, markerlessOptions); got != bootstrapWant {
		t.Fatalf("markerless bootstrap browser URL mismatch\ngot:  %q\nwant: %q", got, bootstrapWant)
	}

	styleSource := `<link rel=stylesheet href=style.css?theme#main>`
	styleWant := `<link rel=stylesheet href=assets/style%20space%26query%3F%23percent%25%E7%95%8C.css?theme#main>`
	if got := rewriteIndexForTest(t, styleSource, htmlRewriteOptions{styleRewrites: map[string]string{
		"style.css": options.stylePaths[0],
	}}); got != styleWant {
		t.Fatalf("markerless style browser URL mismatch\ngot:  %q\nwant: %q", got, styleWant)
	}
}

func TestCustomIndexGeneratedJavaScriptStringUsesECMAScriptEscapes(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		quote byte
		want  string
	}{
		{name: "NUL", value: "\x00", quote: '"', want: `\x00`},
		{name: "bell", value: "\x07", quote: '"', want: `\x07`},
		{name: "vertical tab", value: "\x0b", quote: '"', want: `\v`},
		{name: "delete", value: "\x7f", quote: '"', want: `\x7F`},
		{name: "C1 control", value: "\u0085", quote: '"', want: `\x85`},
		{name: "line separator", value: "\u2028", quote: '"', want: `\u2028`},
		{name: "paragraph separator", value: "\u2029", quote: '"', want: `\u2029`},
		{name: "non-BMP", value: "\U0001d11e", quote: '"', want: `\uD834\uDD1E`},
		{name: "double quote and backslash", value: "\"\\", quote: '"', want: `\"\\`},
		{name: "single quote and backslash", value: "'\\", quote: '\'', want: `\'\\`},
		{name: "script data less-than", value: `</script>`, quote: '"', want: `\u003C/script>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := encodeGeneratedJavaScriptStringContents(test.value, test.quote)
			if err != nil {
				t.Fatalf("encodeGeneratedJavaScriptStringContents() error: %v", err)
			}
			if got != test.want {
				t.Fatalf("JavaScript string contents = %q, want %q", got, test.want)
			}
			if strings.Contains(got, `\a`) || strings.Contains(got, `\U`) {
				t.Fatalf("JavaScript string contains a Go-only escape: %q", got)
			}
		})
	}
}

func TestCustomIndexUnquotedAttributeTagState(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	for _, test := range []struct {
		name      string
		source    string
		wantValue string
	}{
		{name: "quotation mark", source: `<div data-x=a"><script src="wasm_exec.js"></script>">`, wantValue: `a"`},
		{name: "apostrophe", source: `<div data-x=a'><script src="wasm_exec.js"></script>'>`, wantValue: `a'`},
		{name: "equals", source: `<div data-x=a=b><script src="wasm_exec.js"></script>`, wantValue: `a=b`},
		{name: "grave accent", source: "<div data-x=a`b><script src=\"wasm_exec.js\"></script>", wantValue: "a`b"},
		{name: "less-than", source: `<div data-x=a<b><script src="wasm_exec.js"></script>`, wantValue: `a<b`},
	} {
		t.Run(test.name, func(t *testing.T) {
			tag, ok, err := scanHTMLTag(test.source, 0)
			if err != nil || !ok {
				t.Fatalf("scanHTMLTag() = %+v, %v, %v", tag, ok, err)
			}
			wantEnd := strings.IndexByte(test.source, '>') + 1
			if tag.end != wantEnd {
				t.Fatalf("tag end = %d, want %d", tag.end, wantEnd)
			}
			attribute, err := tag.attribute("data-x")
			if err != nil || attribute == nil {
				t.Fatalf("data-x attribute = %+v, %v", attribute, err)
			}
			if got := semanticHTMLAttributeValue(test.source, attribute); got != test.wantValue {
				t.Fatalf("data-x = %q, want %q", got, test.wantValue)
			}
			want := strings.Replace(test.source, "wasm_exec.js", runtimePath, 1)
			if got := rewriteIndexForTest(t, test.source, htmlRewriteOptions{runtimePath: runtimePath}); got != want {
				t.Fatalf("runtime after parse-error attribute mismatch\ngot:  %q\nwant: %q", got, want)
			}
		})
	}

	for _, test := range []struct {
		name      string
		source    string
		wantValue string
	}{
		{name: "double quoted greater-than", source: `<div data-x="a>b"><script src="wasm_exec.js"></script>`, wantValue: `a>b`},
		{name: "single quoted greater-than", source: `<div data-x='a>b'><script src="wasm_exec.js"></script>`, wantValue: `a>b`},
	} {
		t.Run(test.name, func(t *testing.T) {
			tag, ok, err := scanHTMLTag(test.source, 0)
			if err != nil || !ok {
				t.Fatalf("scanHTMLTag() = %+v, %v, %v", tag, ok, err)
			}
			attribute, err := tag.attribute("data-x")
			if err != nil || attribute == nil || semanticHTMLAttributeValue(test.source, attribute) != test.wantValue {
				t.Fatalf("quoted data-x attribute = %+v, %v", attribute, err)
			}
			want := strings.Replace(test.source, "wasm_exec.js", runtimePath, 1)
			if got := rewriteIndexForTest(t, test.source, htmlRewriteOptions{runtimePath: runtimePath}); got != want {
				t.Fatalf("runtime after quoted greater-than mismatch\ngot:  %q\nwant: %q", got, want)
			}
		})
	}
}

func TestCustomIndexLeadingEqualsAttributeRecovery(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	type expectedAttribute struct {
		name     string
		value    string
		hasValue bool
	}
	for _, test := range []struct {
		name       string
		openingTag string
		attributes []expectedAttribute
	}{
		{name: "one equals", openingTag: `<div =x>`, attributes: []expectedAttribute{{name: "=x"}}},
		{name: "two equals", openingTag: `<div ==x>`, attributes: []expectedAttribute{{name: "=", value: "x", hasValue: true}}},
		{name: "equals in value", openingTag: `<div =x=y>`, attributes: []expectedAttribute{{name: "=x", value: "y", hasValue: true}}},
		{name: "equals before space", openingTag: `<div = >`, attributes: []expectedAttribute{{name: "="}}},
		{name: "following attribute", openingTag: `<div =x foo=bar>`, attributes: []expectedAttribute{{name: "=x"}, {name: "foo", value: "bar", hasValue: true}}},
		{name: "empty value", openingTag: `<div =x=>`, attributes: []expectedAttribute{{name: "=x", value: "", hasValue: true}}},
		{name: "compact solidus", openingTag: `<div =x/>`, attributes: []expectedAttribute{{name: "=x"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := test.openingTag + `<script src="wasm_exec.js"></script></div>`
			tag, ok, err := scanHTMLTag(source, 0)
			if err != nil || !ok {
				t.Fatalf("scanHTMLTag() = %+v, %v, %v", tag, ok, err)
			}
			if tag.end != len(test.openingTag) {
				t.Fatalf("tag end = %d, want %d", tag.end, len(test.openingTag))
			}
			if len(tag.attributes) != len(test.attributes) {
				t.Fatalf("attributes = %#v, want %#v", tag.attributes, test.attributes)
			}
			for index, want := range test.attributes {
				got := tag.attributes[index]
				if got.name != want.name || got.hasValue != want.hasValue {
					t.Fatalf("attribute %d = %#v, want name %q hasValue %t", index, got, want.name, want.hasValue)
				}
				wantSyntax := htmlAttributeValueNone
				if want.hasValue {
					wantSyntax = htmlAttributeValueUnquoted
				}
				if got.valueSyntax != wantSyntax {
					t.Fatalf("attribute %d syntax = %v, want %v", index, got.valueSyntax, wantSyntax)
				}
				if gotValue := semanticHTMLAttributeValue(source, &got); gotValue != want.value {
					t.Fatalf("attribute %d value = %q, want %q", index, gotValue, want.value)
				}
				if got.hasValue && source[got.valueStart:got.valueEnd] != want.value {
					t.Fatalf("attribute %d raw value = %q, want %q", index, source[got.valueStart:got.valueEnd], want.value)
				}
			}

			wantSource := strings.Replace(source, runtimeAssetName, runtimePath, 1)
			if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath}); got != wantSource {
				t.Fatalf("runtime after leading-equals recovery mismatch\ngot:  %q\nwant: %q", got, wantSource)
			}
		})
	}
}

func TestCustomIndexAttributeURLRewritePreservesBrowserSemantics(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		options      htmlRewriteOptions
		wantSource   string
		attribute    string
		wantSemantic string
		wantAttrs    int
	}{
		{
			name:   "double quoted active quote and ampersand",
			source: `<link rel="stylesheet" href="styles.css?v=1&copy;=x#theme">`,
			options: htmlRewriteOptions{styleRewrites: map[string]string{
				"styles.css": `assets/my " &copy; style.12345678.css`,
			}},
			wantSource:   `<link rel="stylesheet" href="assets/my%20%22%20%26copy%3B%20style.12345678.css?v=1&copy;=x#theme">`,
			attribute:    "href",
			wantSemantic: `assets/my%20%22%20%26copy%3B%20style.12345678.css?v=1©=x#theme`,
			wantAttrs:    2,
		},
		{
			name:   "single quoted active quote and ampersand",
			source: `<link rel='stylesheet' href='styles.css?v=1&copy;=x#theme'>`,
			options: htmlRewriteOptions{styleRewrites: map[string]string{
				"styles.css": "assets/my ' &copy; style.12345678.css",
			}},
			wantSource:   `<link rel='stylesheet' href='assets/my%20%27%20%26copy%3B%20style.12345678.css?v=1&copy;=x#theme'>`,
			attribute:    "href",
			wantSemantic: "assets/my%20%27%20%26copy%3B%20style.12345678.css?v=1©=x#theme",
			wantAttrs:    2,
		},
		{
			name:   "unquoted whitespace punctuation and ampersand",
			source: `<link rel=stylesheet href=my&#32;style.css?v=1&copy;=x#theme>`,
			options: htmlRewriteOptions{styleRewrites: map[string]string{
				"my style.css": "assets/my &copy; style\t\n\r\f\"'`=<>界.12345678.css",
			}},
			wantSource:   `<link rel=stylesheet href=assets/my%20%26copy%3B%20style%09%0A%0D%0C%22%27%60%3D%3C%3E%E7%95%8C.12345678.css?v=1&copy;=x#theme>`,
			attribute:    "href",
			wantSemantic: "assets/my%20%26copy%3B%20style%09%0A%0D%0C%22%27%60%3D%3C%3E%E7%95%8C.12345678.css?v=1©=x#theme",
			wantAttrs:    2,
		},
		{
			name:         "unquoted runtime uses shared encoder",
			source:       `<script src=wasm_exec.js?runtime></script>`,
			options:      htmlRewriteOptions{runtimePath: `assets/wasm &copy; "'` + "`" + `=<>.js`},
			wantSource:   `<script src=assets/wasm%20%26copy%3B%20%22%27%60%3D%3C%3E.js?runtime></script>`,
			attribute:    "src",
			wantSemantic: `assets/wasm%20%26copy%3B%20%22%27%60%3D%3C%3E.js?runtime`,
			wantAttrs:    1,
		},
		{
			name:   "unquoted style preload uses shared encoder",
			source: `<link rel=preload as=style href=my&#32;style.css#preload>`,
			options: htmlRewriteOptions{styleRewrites: map[string]string{
				"my style.css": "assets/my &copy; style.12345678.css",
			}},
			wantSource:   `<link rel=preload as=style href=assets/my%20%26copy%3B%20style.12345678.css#preload>`,
			attribute:    "href",
			wantSemantic: "assets/my%20%26copy%3B%20style.12345678.css#preload",
			wantAttrs:    3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := rewriteIndexForTest(t, test.source, test.options)
			if got != test.wantSource {
				t.Fatalf("rewritten source mismatch\ngot:  %q\nwant: %q", got, test.wantSource)
			}
			tag, ok, err := scanHTMLTag(got, 0)
			if err != nil || !ok {
				t.Fatalf("scanHTMLTag(rewritten) = %+v, %v, %v", tag, ok, err)
			}
			if len(tag.attributes) != test.wantAttrs {
				t.Fatalf("rewritten attribute count = %d, want %d: %q", len(tag.attributes), test.wantAttrs, got)
			}
			attribute, err := tag.attribute(test.attribute)
			if err != nil || attribute == nil {
				t.Fatalf("rewritten %s attribute = %+v, %v", test.attribute, attribute, err)
			}
			if value := semanticHTMLAttributeValue(got, attribute); value != test.wantSemantic {
				t.Fatalf("rewritten semantic value = %q, want %q", value, test.wantSemantic)
			}
		})
	}
}

func TestCustomIndexAttributeValueSyntax(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   htmlAttributeValueSyntax
	}{
		{name: "no value", source: `<input disabled>`, want: htmlAttributeValueNone},
		{name: "unquoted", source: `<input value=plain>`, want: htmlAttributeValueUnquoted},
		{name: "single quoted", source: `<input value='plain'>`, want: htmlAttributeValueSingleQuoted},
		{name: "double quoted", source: `<input value="plain">`, want: htmlAttributeValueDoubleQuoted},
	} {
		t.Run(test.name, func(t *testing.T) {
			tag, ok, err := scanHTMLTag(test.source, 0)
			if err != nil || !ok {
				t.Fatalf("scanHTMLTag() = %+v, %v, %v", tag, ok, err)
			}
			attribute := tag.attributes[0]
			if attribute.valueSyntax != test.want {
				t.Fatalf("value syntax = %v, want %v", attribute.valueSyntax, test.want)
			}
		})
	}
}

func TestCustomIndexAttributeValueEncoderMatrix(t *testing.T) {
	const destination = "assets/my space & &copy; \" ' ` =<>界.css"
	for _, test := range []struct {
		name   string
		syntax htmlAttributeValueSyntax
		open   string
		close  string
	}{
		{name: "double quoted", syntax: htmlAttributeValueDoubleQuoted, open: `<link href="`, close: `">`},
		{name: "single quoted", syntax: htmlAttributeValueSingleQuoted, open: `<link href='`, close: `'>`},
		{name: "unquoted", syntax: htmlAttributeValueUnquoted, open: `<link href=`, close: `>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := encodeGeneratedHTMLAttributeValue(destination, test.syntax)
			if err != nil {
				t.Fatalf("encodeGeneratedHTMLAttributeValue() error: %v", err)
			}
			source := test.open + encoded + test.close
			tag, ok, err := scanHTMLTag(source, 0)
			if err != nil || !ok {
				t.Fatalf("scanHTMLTag(encoded) = %+v, %v, %v", tag, ok, err)
			}
			if len(tag.attributes) != 1 {
				t.Fatalf("encoded attribute count = %d, want 1: %q", len(tag.attributes), source)
			}
			if got := semanticHTMLAttributeValue(source, &tag.attributes[0]); got != destination {
				t.Fatalf("semantic encoded value = %q, want %q", got, destination)
			}
		})
	}

	for _, syntax := range []htmlAttributeValueSyntax{
		htmlAttributeValueUnquoted,
		htmlAttributeValueSingleQuoted,
		htmlAttributeValueDoubleQuoted,
	} {
		if encoded, err := encodeGeneratedHTMLAttributeValue("assets/bad\x00.css", syntax); err == nil || encoded != "" {
			t.Fatalf("NUL encode with syntax %v = %q, %v, want error", syntax, encoded, err)
		}
	}
}

func TestCustomIndexLiteralNULAttributeSemantics(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	for _, test := range []struct {
		name   string
		source string
	}{
		{name: "double quoted leading", source: "<script src=\"\x00wasm_exec.js\"></script>"},
		{name: "single quoted leading", source: "<script src='\x00wasm_exec.js'></script>"},
		{name: "unquoted leading", source: "<script src=\x00wasm_exec.js></script>"},
		{name: "double quoted trailing", source: "<script src=\"wasm_exec.js\x00\"></script>"},
		{name: "single quoted trailing", source: "<script src='wasm_exec.js\x00'></script>"},
		{name: "unquoted trailing", source: "<script src=wasm_exec.js\x00></script>"},
		{name: "middle", source: "<script src=\"wasm\x00_exec.js\"></script>"},
		{name: "numeric decimal", source: `<script src="&#0;wasm_exec.js"></script>`},
		{name: "numeric hexadecimal", source: `<script src="&#x0;wasm_exec.js"></script>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := rewriteIndexForTest(t, test.source, htmlRewriteOptions{runtimePath: runtimePath}); got != test.source {
				t.Fatalf("NUL-containing runtime reference changed\ngot:  %q\nwant: %q", got, test.source)
			}
		})
	}

	for _, test := range []struct {
		name    string
		source  string
		options htmlRewriteOptions
	}{
		{
			name:    "script type",
			source:  "<script type=\"\x00text/javascript\" src=\"wasm_exec.js\"></script>",
			options: htmlRewriteOptions{runtimePath: runtimePath},
		},
		{
			name:    "script language",
			source:  "<script language=\"\x00javascript\" src=\"wasm_exec.js\"></script>",
			options: htmlRewriteOptions{runtimePath: runtimePath},
		},
		{
			name:   "link rel",
			source: "<link rel=\"\x00stylesheet\" href=\"styles.css\">",
			options: htmlRewriteOptions{styleRewrites: map[string]string{
				"styles.css": "assets/styles.33333333.css",
			}},
		},
		{
			name:   "link as",
			source: "<link rel=\"preload\" as=\"\x00style\" href=\"styles.css\">",
			options: htmlRewriteOptions{styleRewrites: map[string]string{
				"styles.css": "assets/styles.33333333.css",
			}},
		},
		{
			name:    "annotation XML encoding",
			source:  "<math><annotation-xml encoding=\"\x00text/html\"><script src=\"wasm_exec.js\"></script></annotation-xml></math>",
			options: htmlRewriteOptions{runtimePath: runtimePath},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := rewriteIndexForTest(t, test.source, test.options); got != test.source {
				t.Fatalf("NUL-containing semantic attribute changed\ngot:  %q\nwant: %q", got, test.source)
			}
		})
	}

	source := "<script s\x00rc=\"wasm_exec.js\"></script>"
	tag, ok, err := scanHTMLTag(source, 0)
	if err != nil || !ok {
		t.Fatalf("scanHTMLTag(NUL name) = %+v, %v, %v", tag, ok, err)
	}
	if attribute, err := tag.attribute("src"); err != nil || attribute != nil {
		t.Fatalf("NUL-containing attribute name resolved as src: %+v, %v", attribute, err)
	}
	if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath}); got != source {
		t.Fatalf("NUL-containing attribute name changed\ngot:  %q\nwant: %q", got, source)
	}

	mappedSource := `<script src="a` + "\x00" + `b"></script>`
	tag, ok, err = scanHTMLTag(mappedSource, 0)
	if err != nil || !ok {
		t.Fatalf("scanHTMLTag(mapped NUL) = %+v, %v, %v", tag, ok, err)
	}
	sourceAttribute, err := tag.attribute("src")
	if err != nil || sourceAttribute == nil {
		t.Fatalf("mapped NUL src attribute = %+v, %v", sourceAttribute, err)
	}
	units := htmlAttributeSourceBytes(mappedSource, sourceAttribute)
	wantValues := []byte("a\uFFFDb")
	if len(units) != len(wantValues) {
		t.Fatalf("mapped NUL unit count = %d, want %d", len(units), len(wantValues))
	}
	for index, want := range wantValues {
		if units[index].value != want {
			t.Fatalf("mapped NUL unit %d value = %#x, want %#x", index, units[index].value, want)
		}
		if index >= 1 && index <= 3 && (units[index].start != 1 || units[index].end != 2) {
			t.Fatalf("mapped NUL unit %d span = [%d,%d), want [1,2)", index, units[index].start, units[index].end)
		}
	}
}

func TestCustomIndexGeneratedHTMLURLSUseContextEncoders(t *testing.T) {
	options := htmlRewriteOptions{
		preload:     true,
		wasmPath:    `assets/bundle &copy;.wasm`,
		runtimePath: `assets/wasm " &copy;.js`,
		stylePaths:  []string{`assets/my &copy; style.css`},
	}
	generated, err := generateIndexHTML(options)
	if err != nil {
		t.Fatalf("generateIndexHTML() error: %v", err)
	}
	for _, want := range []string{
		`href="assets/bundle%20%26copy%3B.wasm"`,
		`href="assets/wasm%20%22%20%26copy%3B.js"`,
		`href="assets/my%20%26copy%3B%20style.css"`,
		`src="assets/wasm%20%22%20%26copy%3B.js"`,
		`fetch("assets/bundle%20%26copy%3B.wasm")`,
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated index missing %q:\n%s", want, generated)
		}
	}

	managed := rewriteIndexForTest(t, `<html><head><!-- goframe:preload --><!-- /goframe:preload --></head><body><!-- goframe:runtime --><!-- /goframe:runtime --><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></body></html>`, options)
	for _, want := range []string{
		`href="assets/bundle%20%26copy%3B.wasm"`,
		`href="assets/my%20%26copy%3B%20style.css"`,
		`src="assets/wasm%20%22%20%26copy%3B.js"`,
		`fetch("assets/bundle%20%26copy%3B.wasm")`,
	} {
		if !strings.Contains(managed, want) {
			t.Fatalf("managed index missing %q:\n%s", want, managed)
		}
	}

	ordinary, err := bootstrapHTML(htmlRewriteOptions{wasmPath: "assets/bundle.12345678.wasm"})
	if err != nil {
		t.Fatalf("bootstrapHTML() error: %v", err)
	}
	if !strings.Contains(ordinary, `fetch("assets/bundle.12345678.wasm")`) {
		t.Fatalf("ordinary bootstrap string changed: %s", ordinary)
	}

	for _, test := range []struct {
		name   string
		quote  string
		path   string
		wanted string
	}{
		{
			name:   "double quoted bootstrap",
			quote:  `"`,
			path:   `assets/my " <.wasm`,
			wanted: `fetch("assets/my%20%22%20%3C.wasm")`,
		},
		{
			name:   "single quoted bootstrap",
			quote:  `'`,
			path:   `assets/my ' <.wasm`,
			wanted: `fetch('assets/my%20%27%20%3C.wasm')`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := `<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch(` + test.quote + `bundle.wasm` + test.quote + `), go.importObject).then((result) => go.run(result.instance));</script>`
			got := rewriteIndexForTest(t, source, htmlRewriteOptions{wasmPath: test.path})
			if !strings.Contains(got, test.wanted) {
				t.Fatalf("bootstrap string context mismatch\ngot:  %q\nwant substring: %q", got, test.wanted)
			}
		})
	}
}

func TestCustomIndexGeneratedURLFailurePreservesFiles(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.html")
	destinationPath := filepath.Join(root, "destination.html")
	source := `<script src="wasm_exec.js"></script>`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destinationPath, []byte("destination sentinel\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := writeRewrittenIndex(sourcePath, destinationPath, htmlRewriteOptions{runtimePath: "assets/bad\x00.js"})
	if err == nil || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("writeRewrittenIndex() error = %v, want NUL rejection", err)
	}
	assertFileContent(t, sourcePath, source)
	assertFileContent(t, destinationPath, "destination sentinel\n")

	err = writeGeneratedIndex(destinationPath, htmlRewriteOptions{
		wasmPath:    "assets/bundle.wasm",
		runtimePath: "assets/wasm_exec.js",
		stylePaths:  []string{"assets/bad\x00.css"},
	})
	if err == nil || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("writeGeneratedIndex() error = %v, want NUL rejection", err)
	}
	assertFileContent(t, destinationPath, "destination sentinel\n")
}

func TestCustomIndexLegacyURLPathNormalization(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	for _, value := range []string{
		"wasm_exec.js",
		"./wasm_exec.js",
		"././wasm_exec.js",
		"assets/../wasm_exec.js",
		"%2e/wasm_exec.js",
		"assets/%2e%2e/wasm_exec.js",
		"assets/.%2e/wasm_exec.js",
		`.\wasm_exec.js`,
		`assets\..\wasm_exec.js`,
	} {
		t.Run("runtime positive "+value, func(t *testing.T) {
			source := `<script src="` + value + `?v=1#app"></script>`
			want := `<script src="` + runtimePath + `?v=1#app"></script>`
			if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath}); got != want {
				t.Fatalf("runtime path normalization mismatch\ngot:  %q\nwant: %q", got, want)
			}
		})
	}

	for _, value := range []string{
		"../wasm_exec.js",
		"foo/../../wasm_exec.js",
		"/wasm_exec.js",
		"//host/wasm_exec.js",
		"https://host/wasm_exec.js",
		"data:wasm_exec.js",
		"blob:wasm_exec.js",
		"C:/wasm_exec.js",
		`C:\wasm_exec.js`,
		"wasm_exec.js.map",
		"my-wasm_exec.js",
		"assets/%2f../wasm_exec.js",
		"assets/%5c../wasm_exec.js",
		"assets/%2e%2eX/wasm_exec.js",
		"wasm_exec.js/.",
		"wasm_exec.js/..",
	} {
		t.Run("runtime negative "+value, func(t *testing.T) {
			source := `<script src="` + value + `"></script>`
			if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath}); got != source {
				t.Fatalf("unowned runtime path changed\ngot:  %q\nwant: %q", got, source)
			}
		})
	}

	t.Run("semantic path and raw suffix spans", func(t *testing.T) {
		source := `<script src=" assets/..&sol;wasm&lowbar;exec.js?v=1&amp;x#app "></script>`
		want := `<script src=" ` + runtimePath + `?v=1&amp;x#app "></script>`
		if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath}); got != want {
			t.Fatalf("semantic path source-span mismatch\ngot:  %q\nwant: %q", got, want)
		}
	})

	const stylePath = "assets/styles.33333333.css"
	for _, value := range []string{
		"styles.css",
		"././styles.css",
		"assets/../styles.css",
		"assets/%2E%2E/styles.css",
	} {
		t.Run("style positive "+value, func(t *testing.T) {
			for _, link := range []string{
				`<link rel="stylesheet" href="` + value + `?v=1#theme">`,
				`<link rel="preload" as="style" href="` + value + `?v=1#theme">`,
			} {
				want := strings.Replace(link, value, stylePath, 1)
				got := rewriteIndexForTest(t, link, htmlRewriteOptions{styleRewrites: map[string]string{"styles.css": stylePath}})
				if got != want {
					t.Fatalf("style path normalization mismatch\ngot:  %q\nwant: %q", got, want)
				}
			}
		})
	}

	for _, value := range []string{
		"././bundle.wasm",
		"assets/../bundle.wasm",
		"assets/%2e%2e/main.wasm",
	} {
		t.Run("bootstrap positive "+value, func(t *testing.T) {
			source := `<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch("` + value + `?v=1#app"), go.importObject).then((result) => go.run(result.instance));</script>`
			want := strings.Replace(source, value, "assets/bundle.11111111.wasm", 1)
			got := rewriteIndexForTest(t, source, htmlRewriteOptions{wasmPath: "assets/bundle.11111111.wasm"})
			if got != want {
				t.Fatalf("bootstrap path normalization mismatch\ngot:  %q\nwant: %q", got, want)
			}
			if second := rewriteIndexForTest(t, got, htmlRewriteOptions{wasmPath: "assets/bundle.11111111.wasm"}); second != got {
				t.Fatalf("bootstrap rewrite is not idempotent\nfirst:  %q\nsecond: %q", got, second)
			}
		})
	}
}

func TestCustomIndexLegacyURLNormalizationDecodesPercentOnce(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	for _, value := range []string{
		"%2e/wasm_exec.js",
		"%2E/wasm_exec.js",
		"assets/%2e%2e/wasm_exec.js",
		"assets/.%2e/wasm_exec.js",
		"assets/%2e./wasm_exec.js",
		"&#37;2e/wasm_exec.js",
	} {
		t.Run("runtime once encoded "+value, func(t *testing.T) {
			source := `<script src="` + value + `?v=1#runtime"></script>`
			want := `<script src="` + runtimePath + `?v=1#runtime"></script>`
			if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath}); got != want {
				t.Fatalf("once-encoded runtime mismatch\ngot:  %q\nwant: %q", got, want)
			}
		})
	}

	const stylePath = "assets/styles.33333333.css"
	for _, value := range []string{
		"%2e/styles.css",
		"assets/%2E%2E/styles.css",
		"&#37;2e/styles.css",
	} {
		t.Run("style once encoded "+value, func(t *testing.T) {
			for _, link := range []string{
				`<link rel="stylesheet" href="` + value + `?v=1#theme">`,
				`<link rel="preload" as="style" href="` + value + `?v=1#theme">`,
			} {
				want := strings.Replace(link, value, stylePath, 1)
				got := rewriteIndexForTest(t, link, htmlRewriteOptions{styleRewrites: map[string]string{
					"styles.css": stylePath,
				}})
				if got != want {
					t.Fatalf("once-encoded style mismatch\ngot:  %q\nwant: %q", got, want)
				}
			}
		})
	}

	const wasmPath = "assets/bundle.11111111.wasm"
	for _, value := range []string{
		"%2e/bundle.wasm",
		"assets/%2E%2E/main.wasm",
		`\x252e/bundle.wasm`,
	} {
		t.Run("bootstrap once encoded "+value, func(t *testing.T) {
			source := `<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch("` + value + `?v=1#app"), go.importObject).then((result) => go.run(result.instance));</script>`
			want := strings.Replace(source, value, wasmPath, 1)
			if got := rewriteIndexForTest(t, source, htmlRewriteOptions{wasmPath: wasmPath}); got != want {
				t.Fatalf("once-encoded bootstrap mismatch\ngot:  %q\nwant: %q", got, want)
			}
		})
	}

	for _, value := range []string{
		"%252e/wasm_exec.js",
		"%252e%252e/wasm_exec.js",
		".%252e/wasm_exec.js",
		"%252e./wasm_exec.js",
		"%25%32%65/wasm_exec.js",
		"&#37;252e/wasm_exec.js",
	} {
		t.Run("runtime double encoded "+value, func(t *testing.T) {
			source := `<script src="` + value + `?v=1#authored"></script>`
			if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath}); got != source {
				t.Fatalf("double-encoded runtime was claimed\ngot:  %q\nwant: %q", got, source)
			}
		})
	}

	for _, value := range []string{
		"%252e/styles.css",
		"%25%32%65/styles.css",
	} {
		t.Run("style double encoded "+value, func(t *testing.T) {
			for _, link := range []string{
				`<link rel="stylesheet" href="` + value + `">`,
				`<link rel="preload" as="style" href="` + value + `">`,
			} {
				got := rewriteIndexForTest(t, link, htmlRewriteOptions{styleRewrites: map[string]string{
					"styles.css": stylePath,
				}})
				if got != link {
					t.Fatalf("double-encoded style was claimed\ngot:  %q\nwant: %q", got, link)
				}
			}
		})
	}

	for _, value := range []string{
		"%252e/bundle.wasm",
		"%25%32%65/main.wasm",
		`\x25252e/bundle.wasm`,
	} {
		t.Run("bootstrap double encoded "+value, func(t *testing.T) {
			source := `<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch("` + value + `"), go.importObject).then((result) => go.run(result.instance));</script>`
			if got := rewriteIndexForTest(t, source, htmlRewriteOptions{wasmPath: wasmPath}); got != source {
				t.Fatalf("double-encoded bootstrap was claimed\ngot:  %q\nwant: %q", got, source)
			}
		})
	}
}

func TestCustomIndexLegacyURLSchemeDetectionPrecedesPercentDecoding(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		want  string
		ok    bool
	}{
		{name: "literal drive scheme", value: "C:style.css"},
		{name: "literal lower drive scheme", value: "c:style.css"},
		{name: "literal HTTP scheme", value: "http:style.css"},
		{name: "literal HTTPS scheme", value: "https:style.css"},
		{name: "literal data scheme", value: "data:text/css,x"},
		{name: "literal custom scheme", value: "custom+scheme:asset.css"},
		{name: "encoded colon uppercase", value: "C%3Astyle.css", want: "C:style.css", ok: true},
		{name: "encoded colon lowercase", value: "C%3astyle.css", want: "C:style.css", ok: true},
		{name: "encoded leading character", value: "%43:style.css", want: "C:style.css", ok: true},
		{name: "encoded leading character and colon", value: "%43%3Astyle.css", want: "C:style.css", ok: true},
		{name: "encoded HTTP character", value: "h%74tp:style.css", want: "http:style.css", ok: true},
		{name: "encoded HTTP colon", value: "http%3Astyle.css", want: "http:style.css", ok: true},
		{name: "double encoded colon", value: "C%253Astyle.css", want: "C%3Astyle.css", ok: true},
		{name: "double encoded leading character and colon", value: "%2543%253Astyle.css", want: "%43%3Astyle.css", ok: true},
		{name: "triple encoded colon fragment", value: "%25253A", want: "%253A", ok: true},
		{name: "malformed short percent", value: "%"},
		{name: "malformed percent digits", value: "%GGstyle.css"},
		{name: "encoded slash", value: "assets%2Fstyle.css"},
		{name: "encoded backslash", value: "assets%5Cstyle.css"},
		{name: "encoded NUL", value: "style%00.css"},
		{name: "absolute", value: "/style.css"},
		{name: "authority", value: "//host/style.css"},
		{name: "root escape", value: "../style.css"},
		{name: "nested root escape", value: "assets/../../style.css"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := normalizeLegacyPackageURLPath(test.value)
			if ok != test.ok || got != test.want {
				t.Fatalf("normalizeLegacyPackageURLPath(%q) = %q, %v, want %q, %v", test.value, got, ok, test.want, test.ok)
			}
		})
	}

	const (
		logicalName = "C:style.css"
		stylePath   = "assets/C:style.12345678.css"
		styleURL    = "assets/C%3Astyle.12345678.css"
	)
	options := htmlRewriteOptions{styleRewrites: map[string]string{logicalName: stylePath}}
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "encoded colon preserves suffix",
			source: `<link rel="stylesheet" href="C%3Astyle.css?v=1#fragment">`,
			want:   `<link rel="stylesheet" href="` + styleURL + `?v=1#fragment">`,
		},
		{
			name:   "encoded leading character preserves raw suffix",
			source: `<link rel='stylesheet' href='%43%3Astyle.css?x=%253A#raw'>`,
			want:   `<link rel='stylesheet' href='` + styleURL + `?x=%253A#raw'>`,
		},
		{
			name:   "unquoted style preload",
			source: `<link rel=preload as=style href=C%3astyle.css?x=1#preload>`,
			want:   `<link rel=preload as=style href=` + styleURL + `?x=1#preload>`,
		},
		{
			name:   "HTML reference produces literal scheme",
			source: `<link rel="stylesheet" href="C&#58;style.css">`,
			want:   `<link rel="stylesheet" href="C&#58;style.css">`,
		},
		{
			name:   "hex HTML reference produces literal scheme",
			source: `<link rel="stylesheet" href="C&#x3A;style.css">`,
			want:   `<link rel="stylesheet" href="C&#x3A;style.css">`,
		},
		{
			name:   "HTML reference produces percent spelling",
			source: `<link rel="stylesheet" href="C&#37;3Astyle.css?v=1&amp;x#theme">`,
			want:   `<link rel="stylesheet" href="` + styleURL + `?v=1&amp;x#theme">`,
		},
		{
			name:   "literal scheme remains authored",
			source: `<link rel="stylesheet" href="C:style.css">`,
			want:   `<link rel="stylesheet" href="C:style.css">`,
		},
		{
			name:   "double encoding matches only literal percent name",
			source: `<link rel="stylesheet" href="C%253Astyle.css">`,
			want:   `<link rel="stylesheet" href="C%253Astyle.css">`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := rewriteIndexForTest(t, test.source, options)
			if got != test.want {
				t.Fatalf("scheme-order rewrite mismatch\ngot:  %q\nwant: %q", got, test.want)
			}
			if second := rewriteIndexForTest(t, got, options); second != got {
				t.Fatalf("scheme-order rewrite is not idempotent\nfirst:  %q\nsecond: %q", got, second)
			}
		})
	}

	t.Run("double encoding matches literal percent logical name", func(t *testing.T) {
		source := `<link rel="stylesheet" href="C%253Astyle.css">`
		want := `<link rel="stylesheet" href="assets/C%253Astyle.87654321.css">`
		got := rewriteIndexForTest(t, source, htmlRewriteOptions{styleRewrites: map[string]string{
			"C%3Astyle.css": "assets/C%3Astyle.87654321.css",
		}})
		if got != want {
			t.Fatalf("literal percent logical-name rewrite mismatch\ngot:  %q\nwant: %q", got, want)
		}
	})

	for _, test := range []struct {
		name    string
		source  string
		wantOK  bool
		wantRaw string
	}{
		{name: "ECMAScript escape produces percent spelling", source: `"\x43\x25\x33\x41style.css?v=1#app"`, wantOK: true, wantRaw: `?v=1#app`},
		{name: "ECMAScript escape produces literal scheme", source: `"\x43\x3Astyle.css"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			decoded, ok := decodeJavaScriptString(test.source, 0, len(test.source))
			if !ok {
				t.Fatal("decodeJavaScriptString() did not recognize static source")
			}
			raw := test.source[decoded.valueStart:decoded.valueEnd]
			match, gotOK := matchLegacyURL(raw, decoded.units, logicalName)
			if gotOK != test.wantOK {
				t.Fatalf("matchLegacyURL(%q) ok = %v, want %v; match = %+v", test.source, gotOK, test.wantOK, match)
			}
			if gotOK && match.suffix != test.wantRaw {
				t.Fatalf("matchLegacyURL(%q) suffix = %q, want %q", test.source, match.suffix, test.wantRaw)
			}
		})
	}
}

func TestCustomIndexNumericAttributeReferencesUseBrowserValues(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	for _, test := range []struct {
		name    string
		source  string
		options htmlRewriteOptions
		want    string
	}{
		{
			name:    "type and src",
			source:  `<script type="text&#x2f;javascript" src="&#x20;wasm_exec.js&#x20;"></script>`,
			options: htmlRewriteOptions{runtimePath: runtimePath},
			want:    `<script type="text&#x2f;javascript" src="&#x20;` + runtimePath + `&#x20;"></script>`,
		},
		{
			name:    "language",
			source:  `<script language="java&#x73;cript" src="wasm_exec.js"></script>`,
			options: htmlRewriteOptions{runtimePath: runtimePath},
			want:    `<script language="java&#x73;cript" src="` + runtimePath + `"></script>`,
		},
		{
			name:   "rel and href",
			source: `<link rel="style&#x73;heet" href="&#x20;styles.css&#x20;">`,
			options: htmlRewriteOptions{styleRewrites: map[string]string{
				"styles.css": "assets/styles.33333333.css",
			}},
			want: `<link rel="style&#x73;heet" href="&#x20;assets/styles.33333333.css&#x20;">`,
		},
		{
			name:   "style preload as",
			source: `<link rel="preload" as="st&#x79;le" href="styles.css">`,
			options: htmlRewriteOptions{styleRewrites: map[string]string{
				"styles.css": "assets/styles.33333333.css",
			}},
			want: `<link rel="preload" as="st&#x79;le" href="assets/styles.33333333.css">`,
		},
		{
			name:    "annotation XML encoding",
			source:  `<math><annotation-xml encoding="text&#x2f;html"><script src="wasm_exec.js"></script>`,
			options: htmlRewriteOptions{runtimePath: runtimePath},
			want:    `<math><annotation-xml encoding="text&#x2f;html"><script src="` + runtimePath + `"></script>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := rewriteIndexForTest(t, test.source, test.options)
			if got != test.want {
				t.Fatalf("numeric attribute references did not use browser values\ngot:  %q\nwant: %q", got, test.want)
			}
		})
	}
}

func TestCustomIndexScriptDataDoubleEscapedClosing(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	for _, test := range []struct {
		name   string
		source string
	}{
		{
			name:   "double escaped closing",
			source: `<script id="outer"><!--<script></script><script src="wasm_exec.js?inert"></script>--></script><script src="wasm_exec.js?real"></script>`,
		},
		{
			name:   "mixed case double escape",
			source: `<script><!--<ScRiPt></sCrIpT><script src="wasm_exec.js?inert"></script>--></script><script src="wasm_exec.js?real"></script>`,
		},
		{
			name:   "double escape end then escaped close",
			source: `<script><!--<script></script></script><script src="wasm_exec.js?real"></script>`,
		},
		{
			name:   "escaped close",
			source: `<script><!--</script><script src="wasm_exec.js?real"></script>`,
		},
		{
			name:   "NUL in escaped state",
			source: "<script><!--\x00<script></script><script src=\"wasm_exec.js?inert\"></script>--></script><script src=\"wasm_exec.js?real\"></script>",
		},
		{
			name:   "non-matching end tag name",
			source: `<script>const text = "</scriptx>";</script><script src="wasm_exec.js?real"></script>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			want := strings.Replace(test.source, `src="wasm_exec.js?real"`, `src="`+runtimePath+`?real"`, 1)
			got := rewriteIndexForTest(t, test.source, htmlRewriteOptions{runtimePath: runtimePath})
			if got != want {
				t.Fatalf("script-data closing mismatch\ngot:  %q\nwant: %q", got, want)
			}
		})
	}
}

func TestCustomIndexScriptDataMalformedEOF(t *testing.T) {
	for _, source := range []string{
		`<script><!--<script></script>`,
		`<script><!--<script></script><script src="wasm_exec.js"></script>`,
		"<script><!--\x00<script></script>",
	} {
		got, err := rewriteIndexHTML(source, htmlRewriteOptions{runtimePath: "assets/wasm_exec.js"})
		if err == nil || !strings.Contains(err.Error(), "no closing </script> tag") {
			t.Fatalf("rewriteIndexHTML(%q) = %q, %v, want script EOF error", source, got, err)
		}
		if got != "" {
			t.Fatalf("script EOF failure returned partial output %q", got)
		}
	}
}

func TestCustomIndexNamedCharacterReferencesUseBrowserValues(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	for _, test := range []struct {
		name    string
		source  string
		options htmlRewriteOptions
		want    string
	}{
		{
			name:    "runtime URL",
			source:  `<script src="wasm&lowbar;exec.js"></script>`,
			options: htmlRewriteOptions{runtimePath: runtimePath},
			want:    `<script src="` + runtimePath + `"></script>`,
		},
		{
			name:   "stylesheet URL",
			source: `<link rel="stylesheet" href="styles&period;css">`,
			options: htmlRewriteOptions{styleRewrites: map[string]string{
				"styles.css": "assets/styles.33333333.css",
			}},
			want: `<link rel="stylesheet" href="assets/styles.33333333.css">`,
		},
		{
			name:    "script type",
			source:  `<script type="text&sol;javascript" src="wasm_exec.js"></script>`,
			options: htmlRewriteOptions{runtimePath: runtimePath},
			want:    `<script type="text&sol;javascript" src="` + runtimePath + `"></script>`,
		},
		{
			name:    "script language remains non-executable",
			source:  `<script language="java&Tab;script" src="wasm_exec.js"></script>`,
			options: htmlRewriteOptions{runtimePath: runtimePath},
			want:    `<script language="java&Tab;script" src="wasm_exec.js"></script>`,
		},
		{
			name:   "stylesheet rel token",
			source: `<link rel="alternate&Tab;stylesheet" href="styles.css">`,
			options: htmlRewriteOptions{styleRewrites: map[string]string{
				"styles.css": "assets/styles.33333333.css",
			}},
			want: `<link rel="alternate&Tab;stylesheet" href="assets/styles.33333333.css">`,
		},
		{
			name:   "stylesheet preload as",
			source: `<link rel="preload" as="&NewLine;style&Tab;" href="styles.css">`,
			options: htmlRewriteOptions{styleRewrites: map[string]string{
				"styles.css": "assets/styles.33333333.css",
			}},
			want: `<link rel="preload" as="&NewLine;style&Tab;" href="assets/styles.33333333.css">`,
		},
		{
			name:   "NBSP is not an ASCII rel separator",
			source: `<link rel="alternate&nbsp;stylesheet" href="styles.css">`,
			options: htmlRewriteOptions{styleRewrites: map[string]string{
				"styles.css": "assets/styles.33333333.css",
			}},
			want: `<link rel="alternate&nbsp;stylesheet" href="styles.css">`,
		},
		{
			name:    "annotation XML encoding",
			source:  `<math><annotation-xml encoding="text&sol;html"><script src="wasm_exec.js"></script>`,
			options: htmlRewriteOptions{runtimePath: runtimePath},
			want:    `<math><annotation-xml encoding="text&sol;html"><script src="` + runtimePath + `"></script>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := rewriteIndexForTest(t, test.source, test.options)
			if got != test.want {
				t.Fatalf("named reference semantic mismatch\ngot:  %q\nwant: %q", got, test.want)
			}
		})
	}
}

func TestCustomIndexNamedCharacterReferenceSourceSpans(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "named filename bytes",
			source: `<script src="./wasm&lowbar;exec&period;js?v=1#app"></script>`,
			want:   `<script src="` + runtimePath + `?v=1#app"></script>`,
		},
		{
			name:   "named surrounding whitespace",
			source: `<script src="&Tab;wasm&lowbar;exec.js&NewLine;"></script>`,
			want:   `<script src="&Tab;` + runtimePath + `&NewLine;"></script>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := rewriteIndexForTest(t, test.source, htmlRewriteOptions{runtimePath: runtimePath})
			if got != test.want {
				t.Fatalf("named reference source span mismatch\ngot:  %q\nwant: %q", got, test.want)
			}
		})
	}
}

func TestCustomIndexTokenizerCombinedSourcePreservation(t *testing.T) {
	largeBody := strings.Repeat("x", 1<<20)
	inertScript := `<script id="inert" type="application/json"><!--<script>` + largeBody + `</script>` +
		`<script src="wasm_exec.js?inert"></script>` +
		`<link rel="stylesheet" href="styles.css?inert">` +
		`</head><!-- goframe:runtime -->--></script>`
	source := `<html><head>` + inertScript +
		`<div data-unknown="&unknown;" data-multi="&NotEqualTilde;"></div>` +
		`<script src="wasm&lowbar;exec.js?real"></script>` +
		`<link rel="stylesheet" href="styles&period;css?real">` +
		`<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch("bundle.wasm?real"), go.importObject).then((result) => go.run(result.instance));</script>` +
		`</head><body></body></html>`
	options := htmlRewriteOptions{
		preload:     true,
		wasmPath:    "assets/bundle.11111111.wasm",
		runtimePath: "assets/wasm_exec.22222222.js",
		stylePaths:  []string{"assets/styles.33333333.css"},
		styleRewrites: map[string]string{
			"styles.css": "assets/styles.33333333.css",
		},
	}

	want := strings.Replace(source, "wasm&lowbar;exec.js?real", options.runtimePath+"?real", 1)
	want = strings.Replace(want, "styles&period;css?real", options.styleRewrites["styles.css"]+"?real", 1)
	want = strings.Replace(want, "bundle.wasm?real", options.wasmPath+"?real", 1)
	closingHead := strings.LastIndex(want, "</head>")
	want = want[:closingHead] + preloadHTMLForTest(t, options) + "\n" + want[closingHead:]

	got := rewriteIndexForTest(t, source, options)
	if got != want {
		t.Fatalf("combined tokenizer rewrite changed bytes outside recognized spans")
	}
	if !strings.Contains(got, inertScript) {
		t.Fatal("combined tokenizer rewrite changed inert double-escaped script bytes")
	}
	if second := rewriteIndexForTest(t, got, options); second != got {
		t.Fatal("combined tokenizer rewrite is not idempotent")
	}
}

func TestCustomIndexNamedCharacterReferenceConsumption(t *testing.T) {
	for _, test := range []struct {
		name  string
		raw   string
		value string
	}{
		{name: "longest match", raw: "&notin;", value: "∉"},
		{name: "short match", raw: "&not;", value: "¬"},
		{name: "attribute alphanumeric exception", raw: "&notit;", value: "&notit;"},
		{name: "attribute amp exception", raw: "&ampfoo", value: "&ampfoo"},
		{name: "attribute equals exception", raw: "&copy=", value: "&copy="},
		{name: "semicolon match", raw: "&copy;", value: "©"},
		{name: "two code points", raw: "&NotEqualTilde;", value: "≂̸"},
		{name: "unknown", raw: "&unknown;", value: "&unknown;"},
		{name: "unknown legacy spelling", raw: "&lowbar", value: "&lowbar"},
		{name: "ambiguous prefix", raw: "&lowbarx", value: "&lowbarx"},
		{name: "ampersand", raw: "&", value: "&"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := `<div data-value="` + test.raw + `">`
			tag, ok, err := scanHTMLTag(source, 0)
			if err != nil || !ok {
				t.Fatalf("scanHTMLTag() = %+v, %v, %v", tag, ok, err)
			}
			attribute, err := tag.attribute("data-value")
			if err != nil || attribute == nil {
				t.Fatalf("data-value attribute = %+v, %v", attribute, err)
			}
			if got := semanticHTMLAttributeValue(source, attribute); got != test.value {
				t.Fatalf("semantic value = %q, want %q", got, test.value)
			}
			if test.name == "two code points" {
				for _, unit := range htmlAttributeSourceBytes(source, attribute) {
					if unit.start != 0 || unit.end != len(test.raw) {
						t.Fatalf("decoded byte source span = [%d,%d), want [0,%d)", unit.start, unit.end, len(test.raw))
					}
				}
			}
		})
	}
}

func TestCustomIndexRawElementAllocationStability(t *testing.T) {
	type scanner struct {
		name    string
		closing string
		scan    func(string) error
	}
	for _, scanner := range []scanner{
		{
			name:    "generic raw element",
			closing: "</style>",
			scan: func(content string) error {
				_, err := scanRawElementClose(content, 0, "style")
				return err
			},
		},
		{
			name:    "script data",
			closing: "</script>",
			scan: func(content string) error {
				_, err := scanScriptElementClose(content, 0)
				return err
			},
		},
	} {
		t.Run(scanner.name, func(t *testing.T) {
			allocations := func(size int) float64 {
				t.Helper()
				content := strings.Repeat("x", size) + scanner.closing
				return testing.AllocsPerRun(1, func() {
					if err := scanner.scan(content); err != nil {
						t.Fatal(err)
					}
				})
			}

			small := allocations(64 << 10)
			large := allocations(8 << 20)
			if large > small+8 {
				t.Fatalf("allocations grow with body length: 64 KiB %.0f, 8 MiB %.0f", small, large)
			}
			if large > 8 {
				t.Fatalf("8 MiB scan uses %.0f allocations, want at most 8", large)
			}
		})
	}
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
	want = want[:closingHead] + preloadHTMLForTest(t, options) + "\n" + want[closingHead:]
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

func TestCustomIndexParserPoppedHeadElementsPreserveManagedParent(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	const wasmPath = "assets/bundle.11111111.wasm"
	const stylePath = "assets/styles.33333333.css"
	options := htmlRewriteOptions{
		preload:     true,
		wasmPath:    wasmPath,
		runtimePath: runtimePath,
		stylePaths:  []string{stylePath},
	}

	preload := preloadHTMLForTest(t, options)
	runtime, err := runtimeHTML(options)
	if err != nil {
		t.Fatalf("runtimeHTML() error: %v", err)
	}
	bootstrap, err := bootstrapHTML(options)
	if err != nil {
		t.Fatalf("bootstrapHTML() error: %v", err)
	}

	for _, element := range []string{"basefont", "bgsound"} {
		t.Run(element, func(t *testing.T) {
			preloadBlock := managedMarkerText(preloadBlockName, true) + "authored preload" + managedMarkerText(preloadBlockName, false)
			runtimeBlock := managedMarkerText(runtimeBlockName, true) + "authored runtime" + managedMarkerText(runtimeBlockName, false)
			bootstrapBlock := managedMarkerText(bootstrapBlockName, true) + "authored bootstrap" + managedMarkerText(bootstrapBlockName, false)
			source := `<html><head><` + element + ` data-authored="preserve-me">` +
				`<meta name="before" content="preserve-me">` + preloadBlock + runtimeBlock + bootstrapBlock +
				`<meta name="after" content="preserve-me"></head><body></body></html>`

			document, err := scanCustomIndexHTML(source)
			if err != nil {
				t.Fatalf("scanCustomIndexHTML() error: %v", err)
			}
			if len(document.comments) != 6 {
				t.Fatalf("managed marker count = %d, want 6", len(document.comments))
			}
			for index, comment := range document.comments {
				if comment.sourceContext.parentName != "head" {
					t.Fatalf("managed marker %d parent = %q, want head", index, comment.sourceContext.parentName)
				}
			}

			want := strings.Replace(source, preloadBlock,
				managedMarkerText(preloadBlockName, true)+"\n"+preload+"\n"+managedMarkerText(preloadBlockName, false), 1)
			want = strings.Replace(want, runtimeBlock,
				managedMarkerText(runtimeBlockName, true)+"\n"+runtime+"\n"+managedMarkerText(runtimeBlockName, false), 1)
			want = strings.Replace(want, bootstrapBlock,
				managedMarkerText(bootstrapBlockName, true)+"\n"+bootstrap+"\n"+managedMarkerText(bootstrapBlockName, false), 1)
			got := rewriteIndexForTest(t, source, options)
			if got != want {
				t.Fatalf("managed rewrite changed bytes outside owned spans\ngot:  %q\nwant: %q", got, want)
			}
			if second := rewriteIndexForTest(t, got, options); second != got {
				t.Fatalf("managed rewrite is not idempotent\nfirst:  %q\nsecond: %q", got, second)
			}
		})
	}
}

func TestCustomIndexParserPoppedHeadElementsPreserveMarkerlessOwnership(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	const stylePath = "assets/styles.33333333.css"
	options := htmlRewriteOptions{
		runtimePath: runtimePath,
		styleRewrites: map[string]string{
			"styles.css": stylePath,
		},
	}

	for _, element := range []string{"basefont", "bgsound"} {
		t.Run(element, func(t *testing.T) {
			source := `<html><head><` + element + ` data-authored="preserve-me">` +
				`<meta name="before" content="preserve-me">` +
				`<script id="owned-runtime" src="wasm_exec.js?fixture=runtime#app"></script>` +
				`<link id="owned-style" rel="stylesheet" href="styles.css?fixture=style#theme">` +
				`<link id="owned-preload" rel="preload" as="style" href="styles.css?fixture=preload#theme">` +
				`<meta name="after" content="preserve-me"></head><body></body></html>`
			want := strings.Replace(source, "wasm_exec.js?fixture=runtime#app", runtimePath+"?fixture=runtime#app", 1)
			want = strings.Replace(want, "styles.css?fixture=style#theme", stylePath+"?fixture=style#theme", 1)
			want = strings.Replace(want, "styles.css?fixture=preload#theme", stylePath+"?fixture=preload#theme", 1)

			got := rewriteIndexForTest(t, source, options)
			if got != want {
				t.Fatalf("markerless rewrite changed bytes outside owned spans\ngot:  %q\nwant: %q", got, want)
			}
			if second := rewriteIndexForTest(t, got, options); second != got {
				t.Fatalf("markerless rewrite is not idempotent\nfirst:  %q\nsecond: %q", got, second)
			}
		})
	}
}

func TestCustomIndexHeadModeStackContract(t *testing.T) {
	const block = `<!-- goframe:preload --><!-- /goframe:preload -->`
	for _, test := range []struct {
		name          string
		beforeBlock   string
		wantParent    string
		managedAccept bool
		wantError     string
	}{
		{name: "html", beforeBlock: `<html data-duplicate="yes">`, wantParent: "html", wantError: "directly under <html>"},
		{name: "base", beforeBlock: `<base target="_blank">`, wantParent: "head", managedAccept: true},
		{name: "basefont", beforeBlock: `<basefont color="red">`, wantParent: "head", managedAccept: true},
		{name: "bgsound", beforeBlock: `<bgsound src="tone.wav">`, wantParent: "head", managedAccept: true},
		{name: "link", beforeBlock: `<link rel="author" href="author.html">`, wantParent: "head", managedAccept: true},
		{name: "meta", beforeBlock: `<meta name="fixture" content="yes">`, wantParent: "head", managedAccept: true},
		{name: "noframes", beforeBlock: `<noframes>authored</noframes>`, wantParent: "head", managedAccept: true},
		{name: "noscript", beforeBlock: `<noscript>authored</noscript>`, wantParent: "head", managedAccept: true},
		{name: "script", beforeBlock: `<script>const authored = true;</script>`, wantParent: "head", managedAccept: true},
		{name: "style", beforeBlock: `<style>body { color: black; }</style>`, wantParent: "head", managedAccept: true},
		{name: "template", beforeBlock: `<template></template>`, wantParent: "head", managedAccept: true},
		{name: "title", beforeBlock: `<title>Authored</title>`, wantParent: "head", managedAccept: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if !htmlStartTagStaysInHead(test.name) {
				t.Fatalf("htmlStartTagStaysInHead(%q) = false, want true", test.name)
			}
			source := `<html><head>` + test.beforeBlock + block + `</head><body></body></html>`
			document, err := scanCustomIndexHTML(source)
			if err != nil {
				t.Fatalf("scanCustomIndexHTML() error: %v", err)
			}
			if len(document.comments) != 2 {
				t.Fatalf("managed marker count = %d, want 2", len(document.comments))
			}
			for index, comment := range document.comments {
				if comment.sourceContext.parentName != test.wantParent {
					t.Fatalf("managed marker %d parent = %q, want %q", index, comment.sourceContext.parentName, test.wantParent)
				}
			}

			got, err := rewriteIndexHTML(source, htmlRewriteOptions{
				preload:  true,
				wasmPath: "assets/bundle.11111111.wasm",
			})
			if test.managedAccept {
				if err != nil {
					t.Fatalf("rewriteIndexHTML() error: %v", err)
				}
				if !strings.Contains(got, `rel="preload"`) {
					t.Fatalf("managed preload output missing:\n%s", got)
				}
				return
			}
			if err == nil || got != "" || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("duplicate document token rewrite = %q, %v, want managed placement failure", got, err)
			}
		})
	}
}

func TestCustomIndexManagedBlockStructuralContext(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	const wasmPath = "assets/bundle.11111111.wasm"
	for _, test := range []struct {
		name      string
		blockName string
		source    string
		want      string
	}{
		{
			name:      "SVG runtime",
			blockName: runtimeBlockName,
			source:    `<svg><!-- goframe:runtime --><!-- /goframe:runtime --></svg>`,
			want:      "SVG or MathML ancestry",
		},
		{
			name:      "MathML bootstrap",
			blockName: bootstrapBlockName,
			source:    `<math><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></math>`,
			want:      "SVG or MathML ancestry",
		},
		{
			name:      "SVG integration point descendant",
			blockName: runtimeBlockName,
			source:    `<svg><foreignObject><div><!-- goframe:runtime --><!-- /goframe:runtime --></div></foreignObject></svg>`,
			want:      "SVG or MathML ancestry",
		},
		{
			name:      "MathML integration point descendant",
			blockName: bootstrapBlockName,
			source:    `<math><annotation-xml encoding="text/html"><div><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></div></annotation-xml></math>`,
			want:      "SVG or MathML ancestry",
		},
		{
			name:      "head to body",
			blockName: runtimeBlockName,
			source:    `<html><head><!-- goframe:runtime --></head><body id="app"><!-- /goframe:runtime --></body></html>`,
			want:      "different structural contexts",
		},
		{
			name:      "sibling containers",
			blockName: runtimeBlockName,
			source:    `<body><div><!-- goframe:runtime --></div><div><!-- /goframe:runtime --></div></body>`,
			want:      "different structural contexts",
		},
		{
			name:      "document level",
			blockName: runtimeBlockName,
			source:    `<!-- goframe:runtime --><!-- /goframe:runtime -->`,
			want:      "document level",
		},
		{
			name:      "document to head",
			blockName: runtimeBlockName,
			source:    `<!-- goframe:runtime --><head><!-- /goframe:runtime --></head>`,
			want:      "document level",
		},
		{
			name:      "direct child of html",
			blockName: runtimeBlockName,
			source:    `<html><!-- goframe:runtime --><!-- /goframe:runtime --></html>`,
			want:      "directly under <html>",
		},
		{
			name:      "HTML to SVG",
			blockName: runtimeBlockName,
			source:    `<body><!-- goframe:runtime --><svg><!-- /goframe:runtime --></svg></body>`,
			want:      "SVG or MathML ancestry",
		},
		{
			name:      "SVG to HTML",
			blockName: runtimeBlockName,
			source:    `<svg><!-- goframe:runtime --></svg><body><!-- /goframe:runtime --></body>`,
			want:      "SVG or MathML ancestry",
		},
		{
			name:      "foreign breakout recovery",
			blockName: runtimeBlockName,
			source:    `<svg><p><!-- goframe:runtime --><!-- /goframe:runtime --></p></svg>`,
			want:      "structurally uncertain",
		},
		{
			name:      "outside to ordinary template",
			blockName: runtimeBlockName,
			source:    `<body><!-- goframe:runtime --><template><!-- /goframe:runtime --></template></body>`,
			want:      "<template>",
		},
		{
			name:      "same names different elements",
			blockName: bootstrapBlockName,
			source:    `<body><div><!-- goframe:bootstrap --></div><div><!-- /goframe:bootstrap --></div></body>`,
			want:      "different structural contexts",
		},
		{
			name:      "ownership affecting misnesting",
			blockName: runtimeBlockName,
			source:    `<body><div><!-- goframe:runtime --><span></div><!-- /goframe:runtime --></body>`,
			want:      "structurally uncertain",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := rewriteIndexHTML(test.source, htmlRewriteOptions{
				wasmPath:    wasmPath,
				runtimePath: runtimePath,
			})
			if err == nil {
				t.Fatalf("rewriteIndexHTML() = %q, want managed structural-context error", got)
			}
			if got != "" {
				t.Fatalf("managed structural-context failure returned partial output %q", got)
			}
			for _, want := range []string{"goframe:" + test.blockName, test.want, "directly under one concrete"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("rewriteIndexHTML() error = %v, want %q", err, want)
				}
			}
		})
	}

	for _, test := range []struct {
		name    string
		source  string
		options htmlRewriteOptions
		want    string
	}{
		{
			name:    "head preload with complete child",
			source:  `<html><head><!-- goframe:preload --><meta name="owned" content="yes"><!-- /goframe:preload --></head><body></body></html>`,
			options: htmlRewriteOptions{preload: true, wasmPath: wasmPath, runtimePath: runtimePath},
			want:    `<link rel="preload" href="` + wasmPath + `"`,
		},
		{
			name:    "body runtime",
			source:  `<html><body><!-- goframe:runtime --><!-- /goframe:runtime --></body></html>`,
			options: htmlRewriteOptions{runtimePath: runtimePath},
			want:    `<script src="` + runtimePath + `"></script>`,
		},
		{
			name:    "body bootstrap with complete child",
			source:  `<html><body><!-- goframe:bootstrap --><span>owned</span><!-- /goframe:bootstrap --></body></html>`,
			options: htmlRewriteOptions{wasmPath: wasmPath},
			want:    `fetch("` + wasmPath + `")`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := rewriteIndexForTest(t, test.source, test.options)
			if !strings.Contains(got, test.want) {
				t.Fatalf("safe managed block missing %q:\n%s", test.want, got)
			}
		})
	}
}

func TestCustomIndexManagedBlocksRequireDocumentContainerParent(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	const wasmPath = "assets/bundle.11111111.wasm"
	for _, test := range []struct {
		name    string
		source  string
		options htmlRewriteOptions
		want    string
	}{
		{
			name:    "runtime under div",
			source:  `<body><div><!-- goframe:runtime --><!-- /goframe:runtime --></div></body>`,
			options: htmlRewriteOptions{runtimePath: runtimePath},
			want:    "goframe:runtime blocks must be direct children of one concrete <head> or <body> element",
		},
		{
			name:    "bootstrap under heading",
			source:  `<body><h1><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></h1></body>`,
			options: htmlRewriteOptions{wasmPath: wasmPath},
			want:    "goframe:bootstrap blocks must be direct children of one concrete <head> or <body> element",
		},
		{
			name:    "heading implicitly closed",
			source:  `<body><h1><!-- goframe:bootstrap --><h2>owned</h2><!-- /goframe:bootstrap --></h1></body>`,
			options: htmlRewriteOptions{wasmPath: wasmPath},
			want:    "goframe:bootstrap blocks must be direct children of one concrete <head> or <body> element",
		},
		{
			name:    "preload under body",
			source:  `<body><!-- goframe:preload --><!-- /goframe:preload --></body>`,
			options: htmlRewriteOptions{preload: true, wasmPath: wasmPath, runtimePath: runtimePath},
			want:    "goframe:preload must be a direct child of <head>",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := rewriteIndexHTML(test.source, test.options)
			if err == nil {
				t.Fatalf("rewriteIndexHTML() = %q, want managed placement error", got)
			}
			if got != "" {
				t.Fatalf("managed placement failure returned partial output %q", got)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("rewriteIndexHTML() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestCustomIndexManagedBlockRejectsOrdinaryContainerParents(t *testing.T) {
	for _, test := range []struct {
		name       string
		parentName string
		source     string
	}{
		{name: "div", parentName: "div", source: `<body><div><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></div></body>`},
		{name: "section", parentName: "section", source: `<body><section><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></section></body>`},
		{name: "main", parentName: "main", source: `<body><main><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></main></body>`},
		{name: "article", parentName: "article", source: `<body><article><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></article></body>`},
		{name: "aside", parentName: "aside", source: `<body><aside><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></aside></body>`},
		{name: "nav", parentName: "nav", source: `<body><nav><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></nav></body>`},
		{name: "form", parentName: "form", source: `<body><form><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></form></body>`},
		{name: "paragraph", parentName: "p", source: `<body><p><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></p></body>`},
		{name: "heading", parentName: "h1", source: `<body><h1><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></h1></body>`},
		{name: "button", parentName: "button", source: `<body><button><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></button></body>`},
		{name: "anchor", parentName: "a", source: `<body><a href="#"><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></a></body>`},
		{name: "nobr", parentName: "nobr", source: `<body><nobr><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></nobr></body>`},
		{name: "list item", parentName: "li", source: `<body><ul><li><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></li></ul></body>`},
		{name: "description term", parentName: "dt", source: `<body><dl><dt><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></dt></dl></body>`},
		{name: "description detail", parentName: "dd", source: `<body><dl><dd><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></dd></dl></body>`},
		{name: "ruby text", parentName: "rt", source: `<body><ruby><rt><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></rt></ruby></body>`},
		{name: "custom element", parentName: "app-shell", source: `<body><app-shell><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></app-shell></body>`},
		{name: "heading implicit close", parentName: "h1", source: `<body><h1><!-- goframe:bootstrap --><h2>owned</h2><!-- /goframe:bootstrap --></h1></body>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			document, err := scanCustomIndexHTML(test.source)
			if err != nil {
				t.Fatalf("scanCustomIndexHTML() error: %v", err)
			}
			if len(document.comments) != 2 {
				t.Fatalf("scanCustomIndexHTML() comments = %d, want 2", len(document.comments))
			}
			startContext := document.comments[0].sourceContext
			endContext := document.comments[1].sourceContext
			if startContext.parentID != endContext.parentID || startContext.parentName != test.parentName || endContext.parentName != test.parentName {
				t.Fatalf("source contexts = %#v, %#v, want shared <%s> parent", startContext, endContext, test.parentName)
			}
			if !startContext.structurallyCertain || !endContext.structurallyCertain {
				t.Fatalf("source certainty = start %t, end %t, want both true", startContext.structurallyCertain, endContext.structurallyCertain)
			}

			got, err := rewriteIndexHTML(test.source, htmlRewriteOptions{wasmPath: "assets/bundle.11111111.wasm"})
			if err == nil {
				t.Fatalf("rewriteIndexHTML() = %q, want managed placement error", got)
			}
			if got != "" {
				t.Fatalf("managed structural-context failure returned partial output %q", got)
			}
			for _, want := range []string{"goframe:bootstrap blocks", "direct children", "<head> or <body>", test.parentName} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("rewriteIndexHTML() error = %v, want %q", err, want)
				}
			}
		})
	}

}

func TestCustomIndexManagedBlockDocumentContainerPressure(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	const wasmPath = "assets/bundle.11111111.wasm"
	for _, test := range []struct {
		name    string
		source  string
		options htmlRewriteOptions
		want    string
	}{
		{name: "head preload", source: `<html><head><!-- goframe:preload --><!-- /goframe:preload --></head><body></body></html>`, options: htmlRewriteOptions{preload: true, wasmPath: wasmPath, runtimePath: runtimePath}, want: `rel="preload"`},
		{name: "head runtime", source: `<html><head><!-- goframe:runtime --><!-- /goframe:runtime --></head><body></body></html>`, options: htmlRewriteOptions{runtimePath: runtimePath}, want: runtimePath},
		{name: "head bootstrap", source: `<html><head><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></head><body></body></html>`, options: htmlRewriteOptions{wasmPath: wasmPath}, want: wasmPath},
		{name: "body runtime", source: `<html><head></head><body><!-- goframe:runtime --><!-- /goframe:runtime --></body></html>`, options: htmlRewriteOptions{runtimePath: runtimePath}, want: runtimePath},
		{name: "body bootstrap", source: `<html><head></head><body><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></body></html>`, options: htmlRewriteOptions{wasmPath: wasmPath}, want: wasmPath},
		{name: "body nested owned content", source: `<html><head></head><body><!-- goframe:bootstrap --><div><span>owned</span></div><!-- /goframe:bootstrap --></body></html>`, options: htmlRewriteOptions{wasmPath: wasmPath}, want: wasmPath},
	} {
		t.Run("accept "+test.name, func(t *testing.T) {
			got := rewriteIndexForTest(t, test.source, test.options)
			if !strings.Contains(got, test.want) {
				t.Fatalf("managed output missing %q:\n%s", test.want, got)
			}
		})
	}

	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{name: "body content closes head", source: `<html><head><!-- goframe:bootstrap --><div>owned</div><!-- /goframe:bootstrap --></head><body></body></html>`, want: "structurally uncertain"},
		{name: "duplicate head", source: `<html><head><head><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></head></head><body></body></html>`, want: "structurally uncertain"},
		{name: "duplicate body", source: `<html><head></head><body><body><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></body></body></html>`, want: "structurally uncertain"},
		{name: "repeated body crosses pair", source: `<html><head></head><body><!-- goframe:bootstrap --><body>owned<!-- /goframe:bootstrap --></body></html>`, want: "structurally uncertain"},
		{name: "repeated html parent", source: `<html><head></head><body><html><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></html></body></html>`, want: "directly under <html>"},
		{name: "frameset-sensitive body", source: `<html><head></head><body><!-- goframe:bootstrap --><frameset><frame></frameset><!-- /goframe:bootstrap --></body></html>`, want: "structurally uncertain"},
		{name: "explicit head close", source: `<html><head><!-- goframe:bootstrap --></head><!-- /goframe:bootstrap --><body></body></html>`, want: "directly under <html>"},
		{name: "explicit body close", source: `<html><head></head><body><!-- goframe:bootstrap --></body><!-- /goframe:bootstrap --></html>`, want: "directly under <html>"},
	} {
		t.Run("reject "+test.name, func(t *testing.T) {
			got, err := rewriteIndexHTML(test.source, htmlRewriteOptions{wasmPath: wasmPath})
			if err == nil || got != "" || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("rewriteIndexHTML() = %q, %v, want %q", got, err, test.want)
			}
		})
	}
}

func TestCustomIndexOwnedRuntimeBootstrapOrdering(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	const wasmPath = "assets/bundle.11111111.wasm"
	const historicalBootstrap = `<script>const go = new Go(); WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject).then((result) => go.run(result.instance));</script>`
	const managedRuntime = `<!-- goframe:runtime --><!-- /goframe:runtime -->`
	const managedBootstrap = `<!-- goframe:bootstrap --><!-- /goframe:bootstrap -->`
	options := htmlRewriteOptions{runtimePath: runtimePath, wasmPath: wasmPath}

	for _, test := range []struct {
		name   string
		source string
	}{
		{name: "managed before managed", source: `<body>` + managedRuntime + managedBootstrap + `</body>`},
		{name: "managed runtime before markerless bootstrap", source: `<body>` + managedRuntime + historicalBootstrap + `</body>`},
		{name: "markerless runtime before managed bootstrap", source: `<body><script src="wasm_exec.js"></script>` + managedBootstrap + `</body>`},
		{name: "both markerless", source: `<body><script src="wasm_exec.js"></script>` + historicalBootstrap + `</body>`},
		{name: "external runtime boundary", source: `<body><script src="https://cdn.example/runtime.js"></script>` + managedBootstrap + `</body>`},
	} {
		t.Run("safe "+test.name, func(t *testing.T) {
			got, err := rewriteIndexHTML(test.source, options)
			if err != nil {
				t.Fatalf("rewriteIndexHTML() error: %v", err)
			}
			if strings.Contains(test.source, runtimeAssetName) && !strings.Contains(got, runtimePath) {
				t.Fatalf("owned runtime was not rewritten:\n%s", got)
			}
			if (strings.Contains(test.source, bootstrapBlockName) || strings.Contains(test.source, "bundle.wasm")) && !strings.Contains(got, wasmPath) {
				t.Fatalf("owned bootstrap was not rewritten:\n%s", got)
			}
		})
	}

	for _, test := range []struct {
		name   string
		source string
	}{
		{name: "managed bootstrap before managed runtime", source: `<body>` + managedBootstrap + managedRuntime + `</body>`},
		{name: "managed bootstrap before markerless runtime", source: `<body>` + managedBootstrap + `<script src="wasm_exec.js"></script></body>`},
		{name: "markerless bootstrap before managed runtime", source: `<body>` + historicalBootstrap + managedRuntime + `</body>`},
		{name: "markerless bootstrap before markerless runtime", source: `<body>` + historicalBootstrap + `<script src="wasm_exec.js"></script></body>`},
		{name: "async markerless runtime", source: `<body><script async src="wasm_exec.js"></script>` + managedBootstrap + `</body>`},
		{name: "defer markerless runtime", source: `<body><script defer src="wasm_exec.js"></script>` + managedBootstrap + `</body>`},
		{name: "module markerless runtime", source: `<body><script type="module" src="wasm_exec.js"></script>` + managedBootstrap + `</body>`},
	} {
		t.Run("reject "+test.name, func(t *testing.T) {
			got, err := rewriteIndexHTML(test.source, options)
			if err == nil {
				t.Fatalf("rewriteIndexHTML() = %q, want owned runtime/bootstrap ordering error", got)
			}
			if got != "" {
				t.Fatalf("ordering failure returned partial output %q", got)
			}
			for _, want := range []string{
				"GoFrame-owned bootstrap may execute before an executable blocking runtime",
				"blocking classic runtime without nomodule/async/defer",
			} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("rewriteIndexHTML() error = %v, want %q", err, want)
				}
			}
		})
	}
}

func TestCustomIndexOwnedRuntimeExecutionClassification(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	const wasmPath = "assets/bundle.11111111.wasm"
	const managedBootstrap = `<!-- goframe:bootstrap --><!-- /goframe:bootstrap -->`
	options := htmlRewriteOptions{runtimePath: runtimePath, wasmPath: wasmPath}

	for _, test := range []struct {
		name       string
		attributes string
		wantSafe   bool
	}{
		{name: "classic", wantSafe: true},
		{name: "async", attributes: " async"},
		{name: "defer", attributes: " defer"},
		{name: "module", attributes: ` type="module"`},
		{name: "nomodule", attributes: " nomodule"},
		{name: "nomodule false", attributes: ` nomodule="false"`},
		{name: "uppercase nomodule", attributes: " NOMODULE"},
		{name: "valid event and for", attributes: ` for="&#x20;WiNdOw&#9;" event="&#10;OnLoAd()&#13;"`, wantSafe: true},
		{name: "valid onload without parentheses", attributes: ` event="ONLOAD" for="WINDOW"`, wantSafe: true},
		{name: "invalid event", attributes: ` for="window" event="onclick"`},
		{name: "invalid for", attributes: ` for="document" event="onload"`},
		{name: "boolean event and for", attributes: " event for"},
		{name: "non HTML space is not trimmed", attributes: " for=\"\u00a0window\u00a0\" event=onload"},
		{name: "event only keeps classic behavior", attributes: ` event="onclick"`, wantSafe: true},
		{name: "for only keeps classic behavior", attributes: ` for="document"`, wantSafe: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := `<body><script` + test.attributes + ` src="wasm_exec.js"></script>` + managedBootstrap + `</body>`
			got, err := rewriteIndexHTML(source, options)
			if test.wantSafe {
				if err != nil {
					t.Fatalf("rewriteIndexHTML() error: %v", err)
				}
				for _, want := range []string{runtimePath, wasmPath} {
					if !strings.Contains(got, want) {
						t.Fatalf("rewriteIndexHTML() missing %q:\n%s", want, got)
					}
				}
				return
			}
			if err == nil {
				t.Fatalf("rewriteIndexHTML() = %q, want executable blocking runtime error", got)
			}
			if got != "" {
				t.Fatalf("ordering failure returned partial output %q", got)
			}
			if !strings.Contains(err.Error(), "GoFrame-owned bootstrap may execute before") {
				t.Fatalf("rewriteIndexHTML() error = %v, want runtime ordering guidance", err)
			}
		})
	}
}

func TestCustomIndexLegacyRuntimeOwnershipIsIndependentOfExecution(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	for _, attributes := range []string{
		" nomodule",
		` nomodule="false"`,
		" async",
		" defer",
		` type="module"`,
		` for="document" event="onclick"`,
	} {
		source := `<script` + attributes + ` src="wasm_exec.js"></script>`
		got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath})
		want := strings.Replace(source, "wasm_exec.js", runtimePath, 1)
		if got != want {
			t.Fatalf("runtime with attributes %q = %q, want URL-only rewrite %q", attributes, got, want)
		}
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
		name       string
		body       string
		legacyURL  string
		wantSuffix string
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
			legacyURL:  "./bundle.wasm?fixture=legacy#wasm",
			wantSuffix: "?fixture=legacy#wasm",
		},
		{
			name:      "v0.3.0-preview.1 dev callback bootstrap",
			body:      `var go = new Go(); WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject).then(function (result) { go.run(result.instance); });`,
			legacyURL: "bundle.wasm",
		},
		{
			name:       "v0.3.0-preview.1 load wrapped dev bootstrap",
			body:       `window.addEventListener("load", function () { var go = new Go(); WebAssembly.instantiateStreaming(fetch("./main.wasm#load"), go.importObject).then(function (result) { go.run(result.instance); }); }, { once: true });`,
			legacyURL:  "./main.wasm#load",
			wantSuffix: "#load",
		},
	}

	options := htmlRewriteOptions{wasmPath: "assets/bundle.12345678.wasm"}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if count := strings.Count(test.body, test.legacyURL); count != 1 {
				t.Fatalf("historical body legacy URL count = %d, want 1", count)
			}
			source := "<script>" + test.body + "</script>"
			want := strings.Replace(
				source,
				test.legacyURL,
				options.wasmPath+test.wantSuffix,
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

func TestCustomIndexHistoricalBootstrapECMAScriptTrivia(t *testing.T) {
	positive := []struct {
		name   string
		trivia string
	}{
		{name: "tab", trivia: "\t"},
		{name: "vertical tab", trivia: "\v"},
		{name: "form feed", trivia: "\f"},
		{name: "space", trivia: " "},
		{name: "no-break space", trivia: "\u00a0"},
		{name: "ogham space mark", trivia: "\u1680"},
		{name: "en quad", trivia: "\u2000"},
		{name: "em quad", trivia: "\u2001"},
		{name: "en space", trivia: "\u2002"},
		{name: "em space", trivia: "\u2003"},
		{name: "three-per-em space", trivia: "\u2004"},
		{name: "four-per-em space", trivia: "\u2005"},
		{name: "six-per-em space", trivia: "\u2006"},
		{name: "figure space", trivia: "\u2007"},
		{name: "punctuation space", trivia: "\u2008"},
		{name: "thin space", trivia: "\u2009"},
		{name: "hair space", trivia: "\u200a"},
		{name: "narrow no-break space", trivia: "\u202f"},
		{name: "medium mathematical space", trivia: "\u205f"},
		{name: "ideographic space", trivia: "\u3000"},
		{name: "byte order mark", trivia: "\ufeff"},
		{name: "line feed", trivia: "\n"},
		{name: "carriage return", trivia: "\r"},
		{name: "carriage return line feed", trivia: "\r\n"},
		{name: "line separator", trivia: "\u2028"},
		{name: "paragraph separator", trivia: "\u2029"},
		{name: "block comment", trivia: `/* fixture fetch("main.wasm") */`},
		{name: "multiline block comment", trivia: "/* fixture\u2028line */"},
		{name: "line comment line feed", trivia: "// fixture\n"},
		{name: "line comment line separator", trivia: "// fixture\u2028"},
	}

	const wasmPath = "assets/bundle.12345678.wasm"
	for _, test := range positive {
		t.Run("accept "+test.name, func(t *testing.T) {
			body := historicalArrowBootstrapWithTrivia(test.trivia, `"bundle.wasm"`)
			source := "<script>" + body + "</script>"
			want := strings.Replace(source, "bundle.wasm", wasmPath, 1)
			got := rewriteIndexForTest(t, source, htmlRewriteOptions{wasmPath: wasmPath})
			if got != want {
				t.Fatalf("trivia rewrite mismatch\ngot:  %q\nwant: %q", got, want)
			}
		})
	}

	for _, test := range []struct {
		name string
		body string
	}{
		{name: "legacy function with comments", body: historicalFunctionBootstrapWithTrivia("/* fixture */", `"bundle.wasm"`)},
		{name: "load wrapper with Unicode space", body: historicalLoadBootstrapWithTrivia("\u00a0", `"main.wasm"`)},
		{name: "load wrapper with line comments", body: historicalLoadBootstrapWithTrivia("// fixture\n", `"bundle.wasm"`)},
	} {
		t.Run("accept "+test.name, func(t *testing.T) {
			source := "<script>" + test.body + "</script>"
			want := strings.NewReplacer("bundle.wasm", wasmPath, "main.wasm", wasmPath).Replace(source)
			got := rewriteIndexForTest(t, source, htmlRewriteOptions{wasmPath: wasmPath})
			if got != want {
				t.Fatalf("historical shape trivia mismatch\ngot:  %q\nwant: %q", got, want)
			}
		})
	}

	for _, test := range []struct {
		name   string
		trivia string
	}{
		{name: "next line", trivia: "\u0085"},
		{name: "mongolian vowel separator", trivia: "\u180e"},
		{name: "zero width space", trivia: "\u200b"},
		{name: "invalid UTF-8", trivia: string([]byte{0xff})},
	} {
		t.Run("preserve "+test.name, func(t *testing.T) {
			source := "<script>" + historicalArrowBootstrapWithTrivia(test.trivia, `"bundle.wasm"`) + "</script>"
			if got := rewriteIndexForTest(t, source, htmlRewriteOptions{wasmPath: wasmPath}); got != source {
				t.Fatalf("unsupported trivia changed\ngot:  %q\nwant: %q", got, source)
			}
		})
	}

	base := historicalArrowBootstrapWithTrivia(" ", `"bundle.wasm"`)
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "line comment at EOF", body: base + "// trailing fixture"},
		{name: "unterminated block comment", body: base + "/* trailing fixture"},
		{name: "line terminator before arrow", body: strings.Replace(base, ") =>", ")\n=>", 1)},
		{name: "multiline comment before arrow", body: strings.Replace(base, ") =>", ")/*\n*/=>", 1)},
		{name: "escaped identifier", body: strings.Replace(base, "const", `c\u006Fnst`, 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := "<script>" + test.body + "</script>"
			got := rewriteIndexForTest(t, source, htmlRewriteOptions{wasmPath: wasmPath})
			if test.name == "line comment at EOF" {
				want := strings.Replace(source, "bundle.wasm", wasmPath, 1)
				if got != want {
					t.Fatalf("EOF line-comment rewrite mismatch\ngot:  %q\nwant: %q", got, want)
				}
				return
			}
			if got != source {
				t.Fatalf("unsupported candidate changed\ngot:  %q\nwant: %q", got, source)
			}
		})
	}
}

func TestCustomIndexHistoricalBootstrapStaticStringEscapes(t *testing.T) {
	const wasmPath = "assets/bundle.12345678.wasm"
	for _, test := range []struct {
		name      string
		quotedURL string
		rawPath   string
		rawSuffix string
	}{
		{name: "hex", quotedURL: `"bundle\x2ewasm"`, rawPath: `bundle\x2ewasm`},
		{name: "four digit Unicode", quotedURL: `"bundle\u002ewasm"`, rawPath: `bundle\u002ewasm`},
		{name: "code point Unicode", quotedURL: `"bundle\u{2e}wasm"`, rawPath: `bundle\u{2e}wasm`},
		{name: "identity escape", quotedURL: `"bundle\.wasm"`, rawPath: `bundle\.wasm`},
		{name: "single quoted main", quotedURL: `'main\x2ewasm'`, rawPath: `main\x2ewasm`},
		{name: "escaped query and fragment", quotedURL: `"bundle\x2ewasm\x3fmode=1\u0023app"`, rawPath: `bundle\x2ewasm`, rawSuffix: `\x3fmode=1\u0023app`},
		{name: "line feed continuation", quotedURL: "\"bundle.\\\nwasm\"", rawPath: "bundle.\\\nwasm"},
		{name: "carriage return line feed continuation", quotedURL: "\"bundle.\\\r\nwasm\"", rawPath: "bundle.\\\r\nwasm"},
		{name: "line separator continuation", quotedURL: "\"bundle.\\\u2028wasm\"", rawPath: "bundle.\\\u2028wasm"},
		{name: "paragraph separator continuation", quotedURL: "\"bundle.\\\u2029wasm\"", rawPath: "bundle.\\\u2029wasm"},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := historicalArrowBootstrapWithTrivia(" ", test.quotedURL)
			source := "<script>" + body + "</script>"
			want := strings.Replace(source, test.rawPath+test.rawSuffix, wasmPath+test.rawSuffix, 1)
			got := rewriteIndexForTest(t, source, htmlRewriteOptions{wasmPath: wasmPath})
			if got != want {
				t.Fatalf("escaped URL rewrite mismatch\ngot:  %q\nwant: %q", got, want)
			}
			if second := rewriteIndexForTest(t, got, htmlRewriteOptions{wasmPath: wasmPath}); second != got {
				t.Fatalf("escaped URL rewrite is not idempotent\nfirst:  %q\nsecond: %q", got, second)
			}
		})
	}

	for _, test := range []struct {
		name      string
		quotedURL string
	}{
		{name: "escaped backslash does not double decode", quotedURL: `"bundle\\x2ewasm"`},
		{name: "exact missing hex digits", quotedURL: `"bundle.wasm\x"`},
		{name: "exact short hex", quotedURL: `"bundle.wasm\x2"`},
		{name: "short hex", quotedURL: `"bundle\x2wasm"`},
		{name: "invalid hex", quotedURL: `"bundle\xGGwasm"`},
		{name: "exact missing Unicode digits", quotedURL: `"bundle.wasm\u"`},
		{name: "exact short Unicode", quotedURL: `"bundle.wasm\u123"`},
		{name: "empty Unicode", quotedURL: `"bundle\uwasm"`},
		{name: "short Unicode", quotedURL: `"bundle\u123wasm"`},
		{name: "invalid Unicode", quotedURL: `"bundle\u12GGwasm"`},
		{name: "empty code point", quotedURL: `"bundle\u{}wasm"`},
		{name: "out of range code point", quotedURL: `"bundle\u{110000}wasm"`},
		{name: "surrogate code point", quotedURL: `"bundle\u{D800}wasm"`},
		{name: "lone high surrogate", quotedURL: `"bundle\uD800wasm"`},
		{name: "lone low surrogate", quotedURL: `"bundle\uDC00wasm"`},
		{name: "octal one", quotedURL: `"bundle\1.wasm"`},
		{name: "octal zero seven", quotedURL: `"bundle\07.wasm"`},
		{name: "octal three digits", quotedURL: `"bundle\377.wasm"`},
		{name: "non octal eight", quotedURL: `"bundle\8.wasm"`},
		{name: "non octal nine", quotedURL: `"bundle\9.wasm"`},
		{name: "trailing backslash", quotedURL: `"bundle.wasm\`},
		{name: "unescaped active quote", quotedURL: `"bundle".wasm"`},
		{name: "raw line feed", quotedURL: "\"bundle.\nwasm\""},
		{name: "raw carriage return", quotedURL: "\"bundle.\rwasm\""},
		{name: "raw line separator", quotedURL: "\"bundle.\u2028wasm\""},
		{name: "raw paragraph separator", quotedURL: "\"bundle.\u2029wasm\""},
	} {
		t.Run("preserve "+test.name, func(t *testing.T) {
			source := "<script>" + historicalArrowBootstrapWithTrivia(" ", test.quotedURL) + "</script>"
			if got := rewriteIndexForTest(t, source, htmlRewriteOptions{wasmPath: wasmPath}); got != source {
				t.Fatalf("unsupported string changed\ngot:  %q\nwant: %q", got, source)
			}
		})
	}
}

func TestDecodeJavaScriptString(t *testing.T) {
	for _, test := range []struct {
		name    string
		source  string
		want    string
		wantOK  bool
		wantEnd int
	}{
		{name: "double quote", source: `"a\"b"`, want: `a"b`, wantOK: true},
		{name: "single quote", source: `'a\'b'`, want: `a'b`, wantOK: true},
		{name: "backslash", source: `"a\\b"`, want: `a\b`, wantOK: true},
		{name: "single escapes", source: `"\b\f\n\r\t\v"`, want: "\b\f\n\r\t\v", wantOK: true},
		{name: "null", source: `"\0"`, want: "\x00", wantOK: true},
		{name: "hex and Unicode", source: `"\x41\u0042\u{43}"`, want: "ABC", wantOK: true},
		{name: "identity", source: `"\."`, want: ".", wantOK: true},
		{name: "line continuations", source: "\"a\\\nb\\\rc\\\r\nd\\\u2028e\\\u2029f\"", want: "abcdef", wantOK: true},
		{name: "surrogate pair", source: `"\uD83D\uDE00"`, want: "😀", wantOK: true},
		{name: "raw non-BMP", source: `"😀"`, want: "😀", wantOK: true},
		{name: "maximum scalar", source: `"\u{10FFFF}"`, want: "\U0010ffff", wantOK: true},
		{name: "escaped line separators", source: `"\u2028\u2029"`, want: "\u2028\u2029", wantOK: true},
		{name: "trailing authored tokens", source: `"ok" trailing`, want: "ok", wantOK: true, wantEnd: 4},
		{name: "invalid UTF-8", source: string([]byte{'"', 0xff, '"'})},
		{name: "null followed by digit", source: `"\07"`},
		{name: "lone high surrogate", source: `"\uD83D"`},
		{name: "lone low surrogate", source: `"\uDE00"`},
		{name: "raw line separator", source: "\"a\u2028b\""},
	} {
		t.Run(test.name, func(t *testing.T) {
			decoded, ok := decodeJavaScriptString(test.source, 0, len(test.source))
			if ok != test.wantOK {
				t.Fatalf("decodeJavaScriptString() ok = %v, want %v; decoded = %+v", ok, test.wantOK, decoded)
			}
			if !ok {
				return
			}
			var value strings.Builder
			for _, unit := range decoded.units {
				value.WriteByte(unit.value)
			}
			if value.String() != test.want {
				t.Fatalf("decodeJavaScriptString() value = %q, want %q", value.String(), test.want)
			}
			wantEnd := test.wantEnd
			if wantEnd == 0 {
				wantEnd = len(test.source)
			}
			if decoded.end != wantEnd {
				t.Fatalf("decodeJavaScriptString() end = %d, want %d", decoded.end, wantEnd)
			}
		})
	}
}

func TestDecodeJavaScriptStringSourceSpans(t *testing.T) {
	for _, test := range []struct {
		name      string
		source    string
		semantic  string
		index     int
		wantStart int
		wantEnd   int
	}{
		{name: "hex escape", source: `"bundle\x2ewasm"`, semantic: "bundle.wasm", index: 6, wantStart: 6, wantEnd: 10},
		{name: "surrogate pair", source: `"\uD83D\uDE00"`, semantic: "😀", index: 0, wantStart: 0, wantEnd: 12},
		{name: "raw non-BMP", source: `"😀"`, semantic: "😀", index: 0, wantStart: 0, wantEnd: 4},
		{name: "after line continuation", source: "\"a\\\r\nb\"", semantic: "ab", index: 1, wantStart: 4, wantEnd: 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			decoded, ok := decodeJavaScriptString(test.source, 0, len(test.source))
			if !ok {
				t.Fatal("decodeJavaScriptString() did not recognize static string")
			}
			if len(decoded.units) != len(test.semantic) {
				t.Fatalf("semantic unit count = %d, want %d", len(decoded.units), len(test.semantic))
			}
			for index, want := range []byte(test.semantic) {
				if decoded.units[index].value != want {
					t.Fatalf("unit %d = %#x, want %#x", index, decoded.units[index].value, want)
				}
			}
			unit := decoded.units[test.index]
			if unit.start != test.wantStart || unit.end != test.wantEnd {
				t.Fatalf("mapped span = [%d,%d), want [%d,%d)", unit.start, unit.end, test.wantStart, test.wantEnd)
			}
		})
	}
}

func historicalArrowBootstrapWithTrivia(trivia, quotedURL string) string {
	return trivia + "const" + trivia + "go" + trivia + "=" + trivia + "new" + trivia + "Go" + trivia + "(" + trivia + ")" + trivia + ";" +
		trivia + "WebAssembly" + trivia + "." + trivia + "instantiateStreaming" + trivia + "(" + trivia + "fetch" + trivia + "(" + trivia + quotedURL + trivia + ")" + trivia + "," + trivia + "go" + trivia + "." + trivia + "importObject" + trivia + ")" + trivia + "." + trivia + "then" + trivia + "(" + trivia + "(" + trivia + "result" + trivia + ") =>" + trivia + "go" + trivia + "." + trivia + "run" + trivia + "(" + trivia + "result" + trivia + "." + trivia + "instance" + trivia + ")" + trivia + ")" + trivia + ";" + trivia
}

func historicalFunctionBootstrapWithTrivia(trivia, quotedURL string) string {
	return trivia + "var" + trivia + "go" + trivia + "=" + trivia + "new" + trivia + "Go" + trivia + "(" + trivia + ")" + trivia + ";" +
		trivia + "WebAssembly" + trivia + "." + trivia + "instantiateStreaming" + trivia + "(" + trivia + "fetch" + trivia + "(" + trivia + quotedURL + trivia + ")" + trivia + "," + trivia + "go" + trivia + "." + trivia + "importObject" + trivia + ")" + trivia + "." + trivia + "then" + trivia + "(" + trivia + "function" + trivia + "(" + trivia + "result" + trivia + ")" + trivia + "{" + trivia + "go" + trivia + "." + trivia + "run" + trivia + "(" + trivia + "result" + trivia + "." + trivia + "instance" + trivia + ")" + trivia + ";" + trivia + "}" + trivia + ")" + trivia + ";" + trivia
}

func historicalLoadBootstrapWithTrivia(trivia, quotedURL string) string {
	return trivia + "window" + trivia + "." + trivia + "addEventListener" + trivia + "(" + trivia + `"load"` + trivia + "," + trivia + "function" + trivia + "(" + trivia + ")" + trivia + "{" +
		historicalFunctionBootstrapWithTrivia(trivia, quotedURL) +
		"}" + trivia + "," + trivia + "{" + trivia + "once" + trivia + ":" + trivia + "true" + trivia + "}" + trivia + ")" + trivia + ";" + trivia
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
		{
			name:   "template URL",
			script: "const go = new Go(); WebAssembly.instantiateStreaming(fetch(`bundle.wasm`), go.importObject).then((result) => go.run(result.instance));",
		},
		{
			name:   "concatenated URL",
			script: `const go = new Go(); WebAssembly.instantiateStreaming(fetch("bundle." + "wasm"), go.importObject).then((result) => go.run(result.instance));`,
		},
		{
			name:   "variable URL",
			script: `const go = new Go(); WebAssembly.instantiateStreaming(fetch(url), go.importObject).then((result) => go.run(result.instance));`,
		},
		{
			name:   "escaped declaration identifier",
			script: `c\u006Fnst go = new Go(); WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject).then((result) => go.run(result.instance));`,
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

func TestCustomIndexHTMLCommentTokenizerStates(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	for _, test := range []struct {
		name    string
		comment string
	}{
		{name: "normal", comment: "<!--x-->"},
		{name: "end bang", comment: "<!--x--!>"},
		{name: "abrupt empty", comment: "<!-->"},
		{name: "abrupt dash", comment: "<!--->"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := test.comment + `<script src="wasm_exec.js?real"></script>`
			want := test.comment + `<script src="` + runtimePath + `?real"></script>`
			if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath}); got != want {
				t.Fatalf("comment close did not expose following runtime tag\ngot:  %q\nwant: %q", got, want)
			}
		})
	}

	for _, source := range []string{
		`<!-- marker-like <!-- goframe:runtime --> text --><script src="wasm_exec.js?real"></script>`,
		`<!-- <script src="wasm_exec.js?opaque"></script><link rel="stylesheet" href="styles.css"></head> --><script src="wasm_exec.js?real"></script>`,
	} {
		got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath})
		if strings.Contains(got, runtimePath+`?opaque`) {
			t.Fatalf("comment data became structural: %q", got)
		}
		if !strings.Contains(got, runtimePath+`?real`) {
			t.Fatalf("real runtime tag after comment was not rewritten: %q", got)
		}
	}

	markerLike := `<!-- goframe:runtime --!><script src="wasm_exec.js?real"></script>`
	if got := rewriteIndexForTest(t, markerLike, htmlRewriteOptions{runtimePath: runtimePath}); !strings.Contains(got, runtimePath+`?real`) {
		t.Fatalf("incorrectly closed marker-like comment became managed: %q", got)
	}

	eofComment := `<!-- <script src="wasm_exec.js?opaque"></script></head>`
	if got := rewriteIndexForTest(t, eofComment, htmlRewriteOptions{runtimePath: runtimePath}); got != eofComment {
		t.Fatalf("EOF comment changed\ngot:  %q\nwant: %q", got, eofComment)
	}
	document, err := scanCustomIndexHTML(eofComment)
	if err != nil {
		t.Fatalf("scanCustomIndexHTML(EOF comment) error: %v", err)
	}
	if len(document.comments) != 1 || !document.comments[0].eof || document.comments[0].end != len(eofComment) {
		t.Fatalf("EOF comment span = %#v, want one EOF-terminated full-source span", document.comments)
	}

	realHead := `<html><head></head><body><!-- </head>`
	got := rewriteIndexForTest(t, realHead, htmlRewriteOptions{
		preload:     true,
		wasmPath:    "assets/bundle.11111111.wasm",
		runtimePath: runtimePath,
	})
	if !strings.Contains(got, `<link rel="preload" href="assets/bundle.11111111.wasm"`) {
		t.Fatalf("real head close before EOF comment did not receive preload: %q", got)
	}

	fakeHead := `<html><head><!-- </head>`
	result, err := rewriteIndexHTML(fakeHead, htmlRewriteOptions{
		preload:     true,
		wasmPath:    "assets/bundle.11111111.wasm",
		runtimePath: runtimePath,
	})
	if err == nil || !strings.Contains(err.Error(), "closing </head>") {
		t.Fatalf("rewriteIndexHTML() = %q, %v, want structural head guidance", result, err)
	}
	if result != "" {
		t.Fatalf("EOF comment preload failure returned partial output %q", result)
	}
}

func TestCustomIndexBogusCommentSemantics(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	const stylePath = "assets/styles.33333333.css"
	for _, test := range []struct {
		name    string
		source  string
		options htmlRewriteOptions
		old     string
		new     string
	}{
		{
			name:    "processing instruction looking",
			source:  `<?x "><script src="wasm_exec.js"></script>">`,
			options: htmlRewriteOptions{runtimePath: runtimePath},
			old:     "wasm_exec.js",
			new:     runtimePath,
		},
		{
			name:   "invalid markup declaration",
			source: `<!unknown "><link rel="stylesheet" href="styles.css">">`,
			options: htmlRewriteOptions{styleRewrites: map[string]string{
				"styles.css": stylePath,
			}},
			old: "styles.css",
			new: stylePath,
		},
		{
			name:    "HTML namespace CDATA looking",
			source:  `<![CDATA["><script src="wasm_exec.js"></script>"]>`,
			options: htmlRewriteOptions{runtimePath: runtimePath},
			old:     "wasm_exec.js",
			new:     runtimePath,
		},
		{
			name:    "invalid end tag open",
			source:  `</?x "><script src="wasm_exec.js"></script>">`,
			options: htmlRewriteOptions{runtimePath: runtimePath},
			old:     "wasm_exec.js",
			new:     runtimePath,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			want := strings.Replace(test.source, test.old, test.new, 1)
			if got := rewriteIndexForTest(t, test.source, test.options); got != want {
				t.Fatalf("bogus comment boundary mismatch\ngot:  %q\nwant: %q", got, want)
			}
		})
	}

	for _, source := range []string{`<?x "opaque`, `<!unknown "opaque`, `<![CDATA["opaque`, `</?x "opaque`} {
		if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath}); got != source {
			t.Fatalf("EOF bogus comment changed\ngot:  %q\nwant: %q", got, source)
		}
	}

	doctype := `<!DOCTYPE html PUBLIC "x>y"><script src="wasm_exec.js"></script>`
	want := strings.Replace(doctype, "wasm_exec.js", runtimePath, 1)
	if got := rewriteIndexForTest(t, doctype, htmlRewriteOptions{runtimePath: runtimePath}); got != want {
		t.Fatalf("DOCTYPE path changed\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCustomIndexDoctypeTokenizerBoundary(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	const stylePath = "assets/styles.33333333.css"
	tests := []struct {
		name    string
		source  string
		options htmlRewriteOptions
		old     string
		new     string
	}{
		{name: "normal", source: `<!doctype html><script src="wasm_exec.js"></script>`, options: htmlRewriteOptions{runtimePath: runtimePath}, old: runtimeAssetName, new: runtimePath},
		{name: "PUBLIC double quoted", source: `<!DOCTYPE html PUBLIC "id"><script src="wasm_exec.js"></script>`, options: htmlRewriteOptions{runtimePath: runtimePath}, old: runtimeAssetName, new: runtimePath},
		{name: "PUBLIC single quoted", source: `<!DOCTYPE html PUBLIC 'id'><script src="wasm_exec.js"></script>`, options: htmlRewriteOptions{runtimePath: runtimePath}, old: runtimeAssetName, new: runtimePath},
		{name: "SYSTEM double quoted", source: `<!DOCTYPE html SYSTEM "id"><script src="wasm_exec.js"></script>`, options: htmlRewriteOptions{runtimePath: runtimePath}, old: runtimeAssetName, new: runtimePath},
		{name: "SYSTEM single quoted", source: `<!DOCTYPE html SYSTEM 'id'><script src="wasm_exec.js"></script>`, options: htmlRewriteOptions{runtimePath: runtimePath}, old: runtimeAssetName, new: runtimePath},
		{name: "abrupt PUBLIC double quoted", source: `<!DOCTYPE html PUBLIC "x><script src="wasm_exec.js"></script>">`, options: htmlRewriteOptions{runtimePath: runtimePath}, old: runtimeAssetName, new: runtimePath},
		{name: "abrupt PUBLIC single quoted", source: `<!DOCTYPE html PUBLIC 'x><script src="wasm_exec.js"></script>'>`, options: htmlRewriteOptions{runtimePath: runtimePath}, old: runtimeAssetName, new: runtimePath},
		{name: "abrupt SYSTEM double quoted style", source: `<!DOCTYPE html SYSTEM "x><link rel="stylesheet" href="styles.css">">`, options: htmlRewriteOptions{styleRewrites: map[string]string{"styles.css": stylePath}}, old: "styles.css", new: stylePath},
		{name: "abrupt SYSTEM single quoted", source: `<!DOCTYPE html SYSTEM 'x><script src="wasm_exec.js"></script>'>`, options: htmlRewriteOptions{runtimePath: runtimePath}, old: runtimeAssetName, new: runtimePath},
		{name: "bare DOCTYPE", source: `<!DOCTYPE><script src="wasm_exec.js"></script>`, options: htmlRewriteOptions{runtimePath: runtimePath}, old: runtimeAssetName, new: runtimePath},
		{name: "PUBLIC without identifier", source: `<!DOCTYPE html PUBLIC><script src="wasm_exec.js"></script>`, options: htmlRewriteOptions{runtimePath: runtimePath}, old: runtimeAssetName, new: runtimePath},
		{name: "SYSTEM without identifier", source: `<!DOCTYPE html SYSTEM><script src="wasm_exec.js"></script>`, options: htmlRewriteOptions{runtimePath: runtimePath}, old: runtimeAssetName, new: runtimePath},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := strings.Replace(test.source, test.old, test.new, 1)
			got, err := rewriteIndexHTML(test.source, test.options)
			if err != nil {
				t.Fatalf("rewriteIndexHTML() error: %v", err)
			}
			if got != want {
				t.Fatalf("DOCTYPE boundary mismatch\ngot:  %q\nwant: %q", got, want)
			}
		})
	}

	for _, source := range []string{
		`<!DOCTYPE html PUBLIC "opaque <script src=wasm_exec.js"`,
		`<!DOCTYPE html SYSTEM 'opaque <link rel=stylesheet href=styles.css'`,
		`<!DOCTYPE html PUBLIC "<!-- goframe:runtime -->`,
	} {
		t.Run("EOF "+source, func(t *testing.T) {
			got, err := rewriteIndexHTML(source, htmlRewriteOptions{
				wasmPath:    "assets/bundle.11111111.wasm",
				runtimePath: runtimePath,
				styleRewrites: map[string]string{
					"styles.css": stylePath,
				},
			})
			if err != nil {
				t.Fatalf("EOF DOCTYPE became a package error: %v", err)
			}
			if got != source {
				t.Fatalf("EOF DOCTYPE changed\ngot:  %q\nwant: %q", got, source)
			}
		})
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

func TestPackageCustomIndexLiteralNULReferencePreserved(t *testing.T) {
	appDir := t.TempDir()
	writeMinimalPackageApp(t, appDir)
	source := "<!doctype html><html><body><div id=\"root\"></div>" +
		"<script src=\"\x00wasm_exec.js\"></script>" +
		"<script src=\"wasm_exec.js\"></script></body></html>"
	writeTestFile(t, appDir, indexHTMLAssetName, source)

	outDir := filepath.Join(t.TempDir(), "package")
	if err := packageApp(packageOptions{
		appDir: appDir, compiler: "go", outDir: outDir, compress: map[string]bool{},
	}); err != nil {
		t.Fatalf("packageApp() error: %v", err)
	}
	assertFileContent(t, filepath.Join(appDir, indexHTMLAssetName), source)
	packaged, err := os.ReadFile(filepath.Join(outDir, indexHTMLAssetName))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(source, `<script src="wasm_exec.js"></script>`, `<script src="assets/wasm_exec.js"></script>`, 1)
	if string(packaged) != want {
		t.Fatalf("packaged NUL source mismatch\ngot:  %q\nwant: %q", packaged, want)
	}
	if _, err := inspectPackageGraph(outDir); err != nil {
		t.Fatalf("inspectPackageGraph() error: %v", err)
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

func TestPackageCustomIndexBrowserSemanticsFailuresAreAtomic(t *testing.T) {
	for _, test := range []struct {
		name    string
		source  string
		preload bool
		want    string
	}{
		{
			name:    "plaintext hides closing head",
			source:  `<html><head><plaintext></head>`,
			preload: true,
			want:    "closing </head>",
		},
		{
			name:   "malformed solidus",
			source: `<html><head></head><body><input disabled//></body></html>`,
			want:   "malformed solidus",
		},
		{
			name:   "unterminated script data",
			source: `<html><head></head><body><script><!--<script></script>`,
			want:   "no closing </script> tag",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			appDir := t.TempDir()
			writeMinimalPackageApp(t, appDir)
			writeTestFile(t, appDir, indexHTMLAssetName, test.source)

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
					appDir:   appDir,
					compiler: "go",
					outDir:   outDir,
					preload:  test.preload,
					compress: map[string]bool{},
				})
			})
			if packageErr == nil || !strings.Contains(packageErr.Error(), test.want) {
				t.Fatalf("packageApp() error = %v, want %q", packageErr, test.want)
			}
			if strings.Contains(output, "packaged ") {
				t.Fatalf("failed package emitted success output: %q", output)
			}
			if got := snapshotInspectTree(t, outDir); !reflect.DeepEqual(got, before) {
				t.Fatalf("rewrite failure changed previous package\nbefore: %#v\nafter:  %#v", before, got)
			}
			assertFileContent(t, filepath.Join(appDir, indexHTMLAssetName), test.source)
			markerAfter, err := os.ReadFile(filepath.Join(outDir, packageMetadataName))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(markerAfter, markerBefore) {
				t.Fatalf("rewrite failure changed the previous completion marker\nbefore: %q\nafter:  %q", markerBefore, markerAfter)
			}
			entries, err := os.ReadDir(temporaryRoot)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "goxc-package-") {
					t.Fatalf("rewrite failure retained temporary staging %s", entry.Name())
				}
			}
		})
	}
}

func TestCustomIndexForeignMarkerlessLookalikesAreExcludedBeforeHTMLOwnership(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	const wasmPath = "assets/bundle.11111111.wasm"
	const stylePath = "assets/styles.33333333.css"
	const bootstrap = `const go = new Go(); WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject).then((result) => go.run(result.instance));`
	options := htmlRewriteOptions{
		runtimePath: runtimePath,
		wasmPath:    wasmPath,
		styleRewrites: map[string]string{
			"styles.css": stylePath,
		},
	}

	for _, test := range []struct {
		name   string
		source string
	}{
		{
			name:   "complex profile SVG runtime",
			source: `<table></table><svg><script src="wasm_exec.js"></script></svg>`,
		},
		{
			name:   "complex profile MathML runtime",
			source: `<table></table><math><script src="wasm_exec.js"></script></math>`,
		},
		{
			name:   "active base and complex profile SVG runtime",
			source: `<base href="/redirected/"><table></table><svg><script src="wasm_exec.js"></script></svg>`,
		},
		{
			name:   "complex profile SVG bootstrap",
			source: `<table></table><svg><script>` + bootstrap + `</script></svg>`,
		},
		{
			name:   "active base SVG bootstrap",
			source: `<base href="/redirected/"><svg><script>` + bootstrap + `</script></svg>`,
		},
		{
			name:   "complex profile SVG stylesheet",
			source: `<table></table><svg><link rel="stylesheet" href="styles.css"></link></svg>`,
		},
		{
			name:   "active base SVG stylesheet",
			source: `<base href="/redirected/"><svg><link rel="stylesheet" href="styles.css"></link></svg>`,
		},
		{
			name:   "complex profile MathML style preload",
			source: `<table></table><math><link rel="preload" as="style" href="styles.css"></link></math>`,
		},
		{
			name:   "active base MathML style preload",
			source: `<base href="/redirected/"><math><link rel="preload" as="style" href="styles.css"></link></math>`,
		},
		{
			name:   "foreign runtime duplicate src",
			source: `<table></table><svg><script src="wasm_exec.js" src="other.js"></script></svg>`,
		},
		{
			name:   "foreign runtime duplicate type",
			source: `<table></table><math><script type="text/javascript" type="application/json" src="wasm_exec.js"></script></math>`,
		},
		{
			name:   "foreign stylesheet duplicate attributes",
			source: `<table></table><svg><link rel="stylesheet" rel="icon" as="style" as="script" href="styles.css" href="other.css"></link></svg>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := rewriteIndexHTML(test.source, options)
			if err != nil {
				t.Fatalf("rewriteIndexHTML() rejected foreign authored content: %v", err)
			}
			if got != test.source {
				t.Fatalf("foreign authored content changed\ngot:  %q\nwant: %q", got, test.source)
			}
		})
	}

	for _, test := range []struct {
		name      string
		source    string
		operation string
	}{
		{
			name:      "real HTML runtime",
			source:    `<table></table><svg><script src="wasm_exec.js"></script></svg><script src="wasm_exec.js"></script>`,
			operation: "goframe:runtime",
		},
		{
			name:      "real HTML bootstrap",
			source:    `<table></table><svg><script>` + bootstrap + `</script></svg><script>` + bootstrap + `</script>`,
			operation: "goframe:bootstrap",
		},
		{
			name:      "real HTML stylesheet",
			source:    `<table></table><svg><link rel="stylesheet" href="styles.css"></link></svg><link rel="stylesheet" href="styles.css">`,
			operation: "stylesheet",
		},
		{
			name:      "real HTML style preload",
			source:    `<table></table><math><link rel="preload" as="style" href="styles.css"></link></math><link rel="preload" as="style" href="styles.css">`,
			operation: "stylesheet",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := rewriteIndexHTML(test.source, options)
			if err == nil {
				t.Fatalf("rewriteIndexHTML() = %q, want real HTML markerless-profile failure", got)
			}
			if got != "" || !strings.Contains(err.Error(), "<table>") || !strings.Contains(err.Error(), test.operation) {
				t.Fatalf("rewriteIndexHTML() = %q, %v, want real HTML fail-closed guidance", got, err)
			}
		})
	}

	for _, test := range []struct {
		name   string
		source string
		old    string
		new    string
	}{
		{
			name:   "SVG runtime",
			source: `<svg><foreignObject><script src="wasm_exec.js"></script></foreignObject></svg>`,
			old:    runtimeAssetName,
			new:    runtimePath,
		},
		{
			name:   "MathML runtime",
			source: `<math><annotation-xml encoding="text/html"><script src="wasm_exec.js"></script></annotation-xml></math>`,
			old:    runtimeAssetName,
			new:    runtimePath,
		},
		{
			name:   "SVG bootstrap",
			source: `<svg><foreignObject><script>` + bootstrap + `</script></foreignObject></svg>`,
			old:    `bundle.wasm`,
			new:    wasmPath,
		},
		{
			name:   "MathML stylesheet",
			source: `<math><annotation-xml encoding="text/html"><link rel="stylesheet" href="styles.css"></annotation-xml></math>`,
			old:    `styles.css`,
			new:    stylePath,
		},
		{
			name:   "SVG style preload",
			source: `<svg><foreignObject><link rel="preload" as="style" href="styles.css"></foreignObject></svg>`,
			old:    `styles.css`,
			new:    stylePath,
		},
	} {
		t.Run("HTML integration point "+test.name, func(t *testing.T) {
			want := strings.Replace(test.source, test.old, test.new, 1)
			if got := rewriteIndexForTest(t, test.source, options); got != want {
				t.Fatalf("HTML integration-point mismatch\ngot:  %q\nwant: %q", got, want)
			}
		})
	}
}

func TestCustomIndexManagedFirstMarkerlessProfile(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	tests := []struct {
		name      string
		source    string
		options   htmlRewriteOptions
		construct string
		guidance  string
	}{
		{
			name:      "select",
			source:    `<select><svg><script src="wasm_exec.js"></script></svg></select>`,
			options:   htmlRewriteOptions{runtimePath: runtimePath},
			construct: "<select>",
			guidance:  "goframe:runtime",
		},
		{
			name:      "select in table",
			source:    `<table><select><svg><script src="wasm_exec.js"></script></svg></select></table>`,
			options:   htmlRewriteOptions{runtimePath: runtimePath},
			construct: "<table>",
			guidance:  "goframe:runtime",
		},
		{
			name:      "table",
			source:    `<table><tr><td><script src="wasm_exec.js"></script></td></tr></table>`,
			options:   htmlRewriteOptions{runtimePath: runtimePath},
			construct: "<table>",
			guidance:  "goframe:runtime",
		},
		{
			name:   "table stylesheet",
			source: `<table><tr><td><link rel="stylesheet" href="styles.css"></td></tr></table>`,
			options: htmlRewriteOptions{styleRewrites: map[string]string{
				"styles.css": "assets/styles.33333333.css",
			}},
			construct: "<table>",
			guidance:  "stylesheet",
		},
		{
			name:      "select historical bootstrap",
			source:    `<select><script>const go = new Go(); WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject).then((result) => go.run(result.instance));</script></select>`,
			options:   htmlRewriteOptions{wasmPath: "assets/bundle.11111111.wasm"},
			construct: "<select>",
			guidance:  "goframe:bootstrap",
		},
		{
			name:      "frameset",
			source:    `<frameset><script src="wasm_exec.js"></script></frameset>`,
			options:   htmlRewriteOptions{runtimePath: runtimePath},
			construct: "<frameset>",
			guidance:  "goframe:runtime",
		},
		{
			name:      "noscript",
			source:    `<noscript>fallback</noscript><script src="wasm_exec.js"></script>`,
			options:   htmlRewriteOptions{runtimePath: runtimePath},
			construct: "<noscript>",
			guidance:  "goframe:runtime",
		},
		{
			name:      "ownership affecting misnesting",
			source:    `<div><span></div><script src="wasm_exec.js"></script>`,
			options:   htmlRewriteOptions{runtimePath: runtimePath},
			construct: "misnested",
			guidance:  "goframe:runtime",
		},
		{
			name:      "structural preload in select document",
			source:    `<html><head></head><body><select><option>x</option></select></body></html>`,
			options:   htmlRewriteOptions{preload: true, wasmPath: "assets/bundle.11111111.wasm"},
			construct: "<select>",
			guidance:  "goframe:preload",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := rewriteIndexHTML(test.source, test.options)
			if err == nil {
				t.Fatalf("rewriteIndexHTML() = %q, want managed-first compatibility error", got)
			}
			if got != "" {
				t.Fatalf("rewriteIndexHTML() returned partial output %q", got)
			}
			for _, want := range []string{test.construct, test.guidance} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("rewriteIndexHTML() error = %v, want %q", err, want)
				}
			}
		})
	}
}

func TestCustomIndexManagedFirstSafeBlocks(t *testing.T) {
	options := htmlRewriteOptions{
		preload:     true,
		wasmPath:    "assets/bundle.11111111.wasm",
		runtimePath: "assets/wasm_exec.22222222.js",
	}
	source := `<!doctype html><html><head>
<!-- goframe:preload --><!-- /goframe:preload -->
</head><body>
<!-- goframe:runtime --><!-- /goframe:runtime -->
<!-- goframe:bootstrap --><!-- /goframe:bootstrap -->
<select><option>authored</option></select>
</body></html>`
	got := rewriteIndexForTest(t, source, options)
	for _, want := range []string{options.wasmPath, options.runtimePath, "<select><option>authored</option></select>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("safe managed complex document missing %q:\n%s", want, got)
		}
	}
	baseOnly := `<base href="/nested/"><p>authored</p>`
	if got := rewriteIndexForTest(t, baseOnly, htmlRewriteOptions{}); got != baseOnly {
		t.Fatalf("active-base document without an owned rewrite changed\ngot:  %q\nwant: %q", got, baseOnly)
	}

	for _, test := range []struct {
		name      string
		source    string
		construct string
	}{
		{
			name:      "select",
			source:    `<select><!-- goframe:runtime --><!-- /goframe:runtime --></select>`,
			construct: "<select>",
		},
		{
			name:      "ordinary template",
			source:    `<template><!-- goframe:runtime --><!-- /goframe:runtime --></template>`,
			construct: "<template>",
		},
		{
			name:      "declarative shadow template",
			source:    `<template shadowrootmode="open"><!-- goframe:runtime --><!-- /goframe:runtime --></template>`,
			construct: "declarative Shadow DOM",
		},
		{
			name:      "noscript",
			source:    `<noscript><!-- goframe:runtime --><!-- /goframe:runtime --></noscript>`,
			construct: "<noscript>",
		},
	} {
		t.Run("reject nested "+test.name, func(t *testing.T) {
			got, err := rewriteIndexHTML(test.source, htmlRewriteOptions{runtimePath: options.runtimePath})
			if err == nil {
				t.Fatalf("rewriteIndexHTML() = %q, want managed placement error", got)
			}
			if got != "" || !strings.Contains(err.Error(), "goframe:runtime") || !strings.Contains(err.Error(), test.construct) {
				t.Fatalf("rewriteIndexHTML() = %q, %v, want managed placement guidance", got, err)
			}
		})
	}

	for _, name := range []string{preloadBlockName, bootstrapBlockName} {
		source := `<select>` + managedMarkerText(name, true) + managedMarkerText(name, false) + `</select>`
		got, err := rewriteIndexHTML(source, options)
		if err == nil || got != "" || !strings.Contains(err.Error(), "goframe:"+name) || !strings.Contains(err.Error(), "<select>") {
			t.Fatalf("nested goframe:%s block = %q, %v, want placement failure", name, got, err)
		}
	}
}

func TestCustomIndexActiveBaseRejectsManagedRelativeURLs(t *testing.T) {
	options := htmlRewriteOptions{
		preload:     true,
		wasmPath:    "assets/bundle.11111111.wasm",
		runtimePath: "assets/wasm_exec.22222222.js",
		stylePaths:  []string{"assets/styles.33333333.css"},
	}
	tests := []struct {
		name      string
		source    string
		options   htmlRewriteOptions
		operation string
	}{
		{
			name: "runtime",
			source: `<!doctype html><html><head><base href="/redirected/"></head><body>
<!-- goframe:runtime --><!-- /goframe:runtime -->
</body></html>`,
			options:   htmlRewriteOptions{runtimePath: options.runtimePath},
			operation: "goframe:runtime",
		},
		{
			name: "bootstrap",
			source: `<!doctype html><html><head><base href="subdirectory/"></head><body>
<!-- goframe:bootstrap --><!-- /goframe:bootstrap -->
</body></html>`,
			options:   htmlRewriteOptions{wasmPath: options.wasmPath},
			operation: "goframe:bootstrap",
		},
		{
			name: "preload",
			source: `<!doctype html><html><head><base href="https://example.invalid/deployment/">
<!-- goframe:preload --><!-- /goframe:preload -->
</head><body></body></html>`,
			options:   options,
			operation: "goframe:preload",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := rewriteIndexHTML(test.source, test.options)
			if err == nil {
				t.Fatalf("rewriteIndexHTML() succeeded with %d output bytes under an active base:\n%s", len(got), got)
			}
			if got != "" {
				t.Fatalf("rewriteIndexHTML() returned partial output %q", got)
			}
			for _, want := range []string{test.operation, "active <base href>", "package-relative URLs"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("rewriteIndexHTML() error = %v, want %q", err, want)
				}
			}
		})
	}
}

func TestCustomIndexActiveBaseValueMatrix(t *testing.T) {
	for _, value := range []string{
		"/other/",
		"subdirectory/",
		"https://example.invalid/path/",
		"//example.invalid/path/",
		"",
		".",
		"./",
		"#fragment",
		"?query",
	} {
		t.Run(value, func(t *testing.T) {
			source := `<html><head><base href="` + value + `"></head><body><!-- goframe:runtime --><!-- /goframe:runtime --></body></html>`
			got, err := rewriteIndexHTML(source, htmlRewriteOptions{runtimePath: "assets/wasm_exec.22222222.js"})
			if err == nil || got != "" || !strings.Contains(err.Error(), "active <base href>") {
				t.Fatalf("rewriteIndexHTML(%q) = %q, %v, want active-base rejection", source, got, err)
			}
		})
	}
}

func TestCustomIndexActiveBaseContextAndNoOutputControls(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	managedRuntime := `<body><!-- goframe:runtime --><!-- /goframe:runtime --></body>`
	for _, test := range []struct {
		name   string
		source string
	}{
		{
			name:   "target only",
			source: `<base target="_blank">` + managedRuntime,
		},
		{
			name:   "ordinary template base",
			source: `<template><base href="/inert/"></template>` + managedRuntime,
		},
		{
			name:   "declarative shadow base",
			source: `<host-element><template shadowrootmode="open"><base href="/shadow/"></template></host-element>` + managedRuntime,
		},
		{
			name:   "SVG base lookalike",
			source: `<svg><base href="/foreign/"></base></svg>` + managedRuntime,
		},
		{
			name:   "MathML base lookalike",
			source: `<math><base href="/foreign/"></base></math>` + managedRuntime,
		},
		{
			name:   "comment base lookalike",
			source: `<!-- <base href="/comment/"> -->` + managedRuntime,
		},
		{
			name:   "script base lookalike",
			source: `<script>const example = '<base href="/script/">';</script>` + managedRuntime,
		},
		{
			name:   "noscript base lookalike",
			source: `<noscript><base href="/noscript/"></noscript>` + managedRuntime,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := rewriteIndexForTest(t, test.source, htmlRewriteOptions{runtimePath: runtimePath})
			if !strings.Contains(got, `<script src="`+runtimePath+`"></script>`) {
				t.Fatalf("managed runtime output missing:\n%s", got)
			}
		})
	}

	t.Run("ordinary template base does not block markerless output", func(t *testing.T) {
		source := `<template><base href="/inert/"></template><script src="wasm_exec.js"></script>`
		want := `<template><base href="/inert/"></template><script src="` + runtimePath + `"></script>`
		if got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath}); got != want {
			t.Fatalf("ordinary template base changed markerless ownership\ngot:  %q\nwant: %q", got, want)
		}
	})

	t.Run("active base with no owned operation", func(t *testing.T) {
		source := `<!doctype html><base href="/redirected/"><script src="https://cdn.example/runtime.js"></script><link rel="stylesheet" href="https://cdn.example/styles.css"><script>externalLoader("https://cdn.example/app.wasm")</script>`
		options := htmlRewriteOptions{
			wasmPath:    "assets/bundle.11111111.wasm",
			runtimePath: runtimePath,
			styleRewrites: map[string]string{
				"styles.css": "assets/styles.33333333.css",
			},
		}
		if got := rewriteIndexForTest(t, source, options); got != source {
			t.Fatalf("authored external document changed\ngot:  %q\nwant: %q", got, source)
		}
	})

	t.Run("disabled managed preload is empty and allowed", func(t *testing.T) {
		source := `<html><head><base href="/redirected/"><!-- goframe:preload -->authored<!-- /goframe:preload --></head><body></body></html>`
		got := rewriteIndexForTest(t, source, htmlRewriteOptions{
			wasmPath:    "assets/bundle.11111111.wasm",
			runtimePath: runtimePath,
		})
		if !strings.Contains(got, "<!-- goframe:preload -->\n\n<!-- /goframe:preload -->") {
			t.Fatalf("disabled managed preload was not emptied:\n%s", got)
		}
	})

	t.Run("active base foreign owned lookalikes remain authored", func(t *testing.T) {
		source := `<base href="/redirected/"><svg><script src="wasm_exec.js"></script><link rel="stylesheet" href="styles.css"></link></svg>`
		options := htmlRewriteOptions{
			runtimePath: runtimePath,
			styleRewrites: map[string]string{
				"styles.css": "assets/styles.33333333.css",
			},
		}
		if got := rewriteIndexForTest(t, source, options); got != source {
			t.Fatalf("foreign lookalikes changed\ngot:  %q\nwant: %q", got, source)
		}
	})
}

func TestCustomIndexActiveBaseMultiplicity(t *testing.T) {
	const managedRuntime = `<!-- goframe:runtime --><!-- /goframe:runtime -->`
	for _, test := range []struct {
		name   string
		source string
	}{
		{
			name:   "body base",
			source: `<html><head></head><body><base href="/body/">` + managedRuntime + `</body></html>`,
		},
		{
			name:   "two active bases",
			source: `<html><head><base href="/first/"><base href="/second/"></head><body>` + managedRuntime + `</body></html>`,
		},
		{
			name:   "target then active",
			source: `<html><head><base target="_blank"><base href="/active/"></head><body>` + managedRuntime + `</body></html>`,
		},
		{
			name:   "uppercase href",
			source: `<html><head><BASE HREF="/active/"></head><body>` + managedRuntime + `</body></html>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := rewriteIndexHTML(test.source, htmlRewriteOptions{runtimePath: "assets/wasm_exec.22222222.js"})
			if err == nil || got != "" || !strings.Contains(err.Error(), "active <base href>") {
				t.Fatalf("rewriteIndexHTML() = %q, %v, want deterministic active-base failure", got, err)
			}
		})
	}
}

func TestCustomIndexActiveBaseRejectsMarkerlessRelativeURLs(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		options   htmlRewriteOptions
		operation string
	}{
		{
			name:      "runtime",
			source:    `<base href="/redirected/"><script src="wasm_exec.js"></script>`,
			options:   htmlRewriteOptions{runtimePath: "assets/wasm_exec.22222222.js"},
			operation: "runtime",
		},
		{
			name:      "bootstrap",
			source:    `<base href="/redirected/"><script>const go = new Go(); WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject).then((result) => go.run(result.instance));</script>`,
			options:   htmlRewriteOptions{wasmPath: "assets/bundle.11111111.wasm"},
			operation: "bootstrap",
		},
		{
			name:   "stylesheet",
			source: `<base href="/redirected/"><link rel="stylesheet" href="styles.css">`,
			options: htmlRewriteOptions{styleRewrites: map[string]string{
				"styles.css": "assets/styles.33333333.css",
			}},
			operation: "stylesheet",
		},
		{
			name:   "style preload",
			source: `<base href="/redirected/"><link rel="preload" as="style" href="styles.css">`,
			options: htmlRewriteOptions{styleRewrites: map[string]string{
				"styles.css": "assets/styles.33333333.css",
			}},
			operation: "style preload",
		},
		{
			name:      "structural preload",
			source:    `<html><head><base href="/redirected/"></head><body></body></html>`,
			options:   htmlRewriteOptions{preload: true, wasmPath: "assets/bundle.11111111.wasm", runtimePath: "assets/wasm_exec.22222222.js"},
			operation: "preload",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := rewriteIndexHTML(test.source, test.options)
			if err == nil || got != "" {
				t.Fatalf("rewriteIndexHTML() = %q, %v, want active-base failure", got, err)
			}
			for _, want := range []string{test.operation, "active <base href>", "package-relative URLs"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("rewriteIndexHTML() error = %v, want %q", err, want)
				}
			}
		})
	}
}

func TestPackageCustomIndexActiveBaseManagedFailureIsAtomic(t *testing.T) {
	appDir := t.TempDir()
	writeMinimalPackageApp(t, appDir)
	source := `<!doctype html><html><head><base href="/redirected/"></head><body><div id="root"></div>
<!-- goframe:runtime --><!-- /goframe:runtime -->
</body></html>`
	writeTestFile(t, appDir, indexHTMLAssetName, source)

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
			appDir: appDir, compiler: "go", outDir: outDir, assetHash: true, compress: map[string]bool{},
		})
	})
	if packageErr == nil || !strings.Contains(packageErr.Error(), "active <base href>") || !strings.Contains(packageErr.Error(), "goframe:runtime") {
		t.Fatalf("packageApp() error = %v, want active-base runtime failure; output = %q", packageErr, output)
	}
	if strings.Contains(output, "packaged ") {
		t.Fatalf("failed package emitted success output: %q", output)
	}
	if got := snapshotInspectTree(t, outDir); !reflect.DeepEqual(got, before) {
		t.Fatalf("active-base failure changed previous package\nbefore: %#v\nafter:  %#v", before, got)
	}
	assertFileContent(t, filepath.Join(appDir, indexHTMLAssetName), source)
	markerAfter, err := os.ReadFile(filepath.Join(outDir, packageMetadataName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(markerAfter, markerBefore) {
		t.Fatalf("active-base failure changed completion marker\nbefore: %q\nafter:  %q", markerBefore, markerAfter)
	}
	entries, err := os.ReadDir(temporaryRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "goxc-package-") {
			t.Fatalf("active-base failure retained temporary stage %s", entry.Name())
		}
	}
}

func TestCustomIndexDeclarativeShadowStylesheetProfile(t *testing.T) {
	options := htmlRewriteOptions{styleRewrites: map[string]string{
		"styles.css": "assets/styles.33333333.css",
	}}
	for _, mode := range []string{"open", "closed", "OPEN"} {
		t.Run(mode, func(t *testing.T) {
			source := `<host-element><template shadowrootmode="` + mode + `"><link rel="stylesheet" href="styles.css"></template></host-element>`
			got, err := rewriteIndexHTML(source, options)
			if err == nil {
				t.Fatalf("rewriteIndexHTML() = %q, want declarative Shadow DOM stylesheet error", got)
			}
			if got != "" || !strings.Contains(err.Error(), "declarative Shadow DOM") || !strings.Contains(err.Error(), "stylesheet") {
				t.Fatalf("rewriteIndexHTML() = %q, %v, want shadow stylesheet guidance", got, err)
			}
		})
	}

	for _, test := range []struct {
		name   string
		source string
	}{
		{
			name:   "invalid mode is ordinary template",
			source: `<host-element><template shadowrootmode=" open "><link rel="stylesheet" href="styles.css"></template></host-element>`,
		},
		{
			name:   "ordinary template",
			source: `<template><template><link rel="stylesheet" href="styles.css"></template></template>`,
		},
		{
			name:   "external shadow stylesheet",
			source: `<host-element><template shadowrootmode="open"><link rel="stylesheet" href="https://cdn.example/styles.css"></template></host-element>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := rewriteIndexForTest(t, test.source, options); got != test.source {
				t.Fatalf("authored template bytes changed\ngot:  %q\nwant: %q", got, test.source)
			}
		})
	}
}

func TestPackageCustomIndexManagedFirstFailureIsAtomic(t *testing.T) {
	appDir := t.TempDir()
	writeMinimalPackageApp(t, appDir)
	source := `<html><head></head><body><select><svg><script src="wasm_exec.js"></script></svg></select></body></html>`
	writeTestFile(t, appDir, indexHTMLAssetName, source)

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
	if packageErr == nil || !strings.Contains(packageErr.Error(), "<select>") {
		t.Fatalf("packageApp() error = %v, want select profile rejection", packageErr)
	}
	if strings.Contains(output, "packaged ") {
		t.Fatalf("failed package emitted success output: %q", output)
	}
	if got := snapshotInspectTree(t, outDir); !reflect.DeepEqual(got, before) {
		t.Fatalf("managed-first failure changed previous package\nbefore: %#v\nafter:  %#v", before, got)
	}
	assertFileContent(t, filepath.Join(appDir, indexHTMLAssetName), source)
	markerAfter, err := os.ReadFile(filepath.Join(outDir, packageMetadataName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(markerAfter, markerBefore) {
		t.Fatalf("managed-first failure changed completion marker\nbefore: %q\nafter:  %q", markerBefore, markerAfter)
	}
	entries, err := os.ReadDir(temporaryRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "goxc-package-") {
			t.Fatalf("managed-first failure retained temporary stage %s", entry.Name())
		}
	}
}

func TestCustomIndexScannerClosingLookupIsConstant(t *testing.T) {
	for _, depth := range []int{1, 1_000, 50_000} {
		var context htmlScannerContext
		for range depth {
			context.push("", htmlTag{name: "div", namespace: htmlNamespaceHTML})
		}
		resolution := context.resolveClosingTag("missing")
		if resolution.matched || resolution.lookupCount != 1 {
			t.Fatalf("depth %d unmatched resolution = %+v, want one indexed lookup", depth, resolution)
		}
		if len(context.elements) != depth {
			t.Fatalf("depth %d unmatched lookup changed stack length to %d", depth, len(context.elements))
		}
	}

	var context htmlScannerContext
	for _, name := range []string{"div", "span", "div"} {
		context.push("", htmlTag{name: name, namespace: htmlNamespaceHTML})
	}
	span := context.resolveClosingTag("span")
	context.close(span)
	if len(context.elements) != 1 || context.elements[0].name != "div" {
		t.Fatalf("matched suffix close retained %#v, want outer div only", context.elements)
	}
	if got := context.resolveClosingTag("div"); !got.matched || got.index != 0 || got.lookupCount != 1 {
		t.Fatalf("restored same-name index = %+v, want outer div", got)
	}
	if got := context.resolveClosingTag("span"); got.matched || got.lookupCount != 1 {
		t.Fatalf("removed span index = %+v, want one unmatched lookup", got)
	}
}

func TestCustomIndexRewritePlanOutputCompatibility(t *testing.T) {
	plan := htmlRewritePlan{content: "0123456789"}
	plan.add(htmlReplacement{start: 8, end: 9, value: "eight", description: "late"})
	plan.add(htmlReplacement{start: 2, end: 4, value: "AB", description: "early"})
	plan.add(htmlReplacement{start: 6, end: 6, value: "-", description: "insert"})
	got, err := plan.apply()
	if err != nil {
		t.Fatalf("htmlRewritePlan.apply() error: %v", err)
	}
	if want := "01AB45-67eight9"; got != want {
		t.Fatalf("htmlRewritePlan.apply() = %q, want %q", got, want)
	}

	overlap := htmlRewritePlan{content: "abcdef", replacements: []htmlReplacement{
		{start: 1, end: 4, value: "x", description: "first"},
		{start: 3, end: 5, value: "y", description: "second"},
	}}
	if got, err := overlap.apply(); err == nil || got != "" || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("overlapping plan = %q, %v, want overlap error", got, err)
	}
}

func TestPackageCustomIndexManagedStructureFailureIsAtomic(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "foreign runtime block",
			source: `<html><head></head><body><svg><!-- goframe:runtime --><!-- /goframe:runtime --></svg></body></html>`,
			want:   "SVG or MathML ancestry",
		},
		{
			name:   "head body crossing",
			source: `<html><head><!-- goframe:runtime --></head><body id="app"><!-- /goframe:runtime --></body></html>`,
			want:   "different structural contexts",
		},
		{
			name:   "runtime under div",
			source: `<html><head></head><body><div><!-- goframe:runtime --><!-- /goframe:runtime --></div></body></html>`,
			want:   "goframe:runtime blocks must be direct children",
		},
		{
			name:   "bootstrap under heading",
			source: `<html><head></head><body><h1><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></h1></body></html>`,
			want:   "goframe:bootstrap blocks must be direct children",
		},
		{
			name:   "preload under body",
			source: `<html><head></head><body><!-- goframe:preload --><!-- /goframe:preload --></body></html>`,
			want:   "goframe:preload must be a direct child of <head>",
		},
		{
			name:   "duplicate html token in head",
			source: `<html><head><html><!-- goframe:preload --><!-- /goframe:preload --></head><body></body></html>`,
			want:   "directly under <html>",
		},
		{
			name:   "heading implicit close",
			source: `<html><head></head><body><h1><!-- goframe:bootstrap --><h2>owned</h2><!-- /goframe:bootstrap --></h1></body></html>`,
			want:   "goframe:bootstrap blocks must be direct children",
		},
		{
			name:   "reversed runtime bootstrap",
			source: `<html><head></head><body><!-- goframe:bootstrap --><!-- /goframe:bootstrap --><!-- goframe:runtime --><!-- /goframe:runtime --></body></html>`,
			want:   "GoFrame-owned bootstrap may execute before an executable blocking runtime",
		},
		{
			name:   "nomodule runtime before bootstrap",
			source: `<html><head></head><body><script nomodule src="wasm_exec.js"></script><!-- goframe:bootstrap --><!-- /goframe:bootstrap --></body></html>`,
			want:   "GoFrame-owned bootstrap may execute before",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			appDir := t.TempDir()
			writeMinimalPackageApp(t, appDir)
			writeTestFile(t, appDir, indexHTMLAssetName, test.source)

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
			if packageErr == nil || !strings.Contains(packageErr.Error(), test.want) {
				t.Fatalf("packageApp() error = %v, want %q; output = %q", packageErr, test.want, output)
			}
			if strings.Contains(output, "packaged ") {
				t.Fatalf("failed package emitted success output: %q", output)
			}
			if got := snapshotInspectTree(t, outDir); !reflect.DeepEqual(got, before) {
				t.Fatalf("managed-structure failure changed previous package\nbefore: %#v\nafter:  %#v", before, got)
			}
			assertFileContent(t, filepath.Join(appDir, indexHTMLAssetName), test.source)
			markerAfter, err := os.ReadFile(filepath.Join(outDir, packageMetadataName))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(markerAfter, markerBefore) {
				t.Fatalf("managed-structure failure changed completion marker\nbefore: %q\nafter:  %q", markerBefore, markerAfter)
			}
			entries, err := os.ReadDir(temporaryRoot)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "goxc-package-") {
					t.Fatalf("managed-structure failure retained temporary stage %s", entry.Name())
				}
			}
		})
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

func preloadHTMLForTest(t *testing.T, options htmlRewriteOptions) string {
	t.Helper()
	got, err := preloadHTML(options)
	if err != nil {
		t.Fatalf("preloadHTML() error: %v", err)
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
