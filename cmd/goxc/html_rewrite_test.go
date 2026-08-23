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
	}{
		{name: "title"},
		{name: "foreignObject"},
		{name: "desc"},
		{name: "p"},
		{name: "div"},
		{name: "font", attributes: ` color="red"`},
	} {
		t.Run("genuine control "+element.name, func(t *testing.T) {
			source := `<svg><` + element.name + element.attributes + `><script src="wasm_exec.js?html"></script></` + element.name + `></svg>`
			got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: runtimePath})
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

func TestCustomIndexForeignBreakoutBrowserSemantics(t *testing.T) {
	const runtimePath = "assets/wasm_exec.22222222.js"
	options := htmlRewriteOptions{runtimePath: runtimePath}
	breakoutNames := []string{
		"b", "big", "blockquote", "body", "br", "center", "code", "dd", "div", "dl", "dt",
		"em", "embed", "h1", "h2", "h3", "h4", "h5", "h6", "head", "hr", "i", "img",
		"li", "listing", "menu", "meta", "nobr", "ol", "p", "pre", "ruby", "s", "small",
		"span", "strong", "strike", "sub", "sup", "table", "tt", "u", "ul", "var",
	}
	for _, name := range breakoutNames {
		t.Run("SVG start "+name, func(t *testing.T) {
			source := `<svg><` + name + `><script src="wasm_exec.js?` + name + `"></script>`
			got := rewriteIndexForTest(t, source, options)
			want := `src="` + runtimePath + `?` + name + `"`
			if !strings.Contains(got, want) {
				t.Fatalf("foreign breakout <%s> did not reprocess the runtime script as HTML\ngot: %s", name, got)
			}
		})
	}

	for _, test := range []struct {
		name    string
		source  string
		rewrite bool
	}{
		{name: "MathML start", source: `<math><div><script src="wasm_exec.js?math"></script>`, rewrite: true},
		{name: "font color", source: `<svg><font color="red"><script src="wasm_exec.js?color"></script>`, rewrite: true},
		{name: "font face", source: `<svg><font face="sans"><script src="wasm_exec.js?face"></script>`, rewrite: true},
		{name: "font size", source: `<svg><font size="2"><script src="wasm_exec.js?size"></script>`, rewrite: true},
		{name: "font without trigger", source: `<svg><font><script src="wasm_exec.js?plain"></script>`},
		{name: "closing p", source: `<svg></p><script src="wasm_exec.js?end-p"></script>`, rewrite: true},
		{name: "closing br", source: `<math></br><script src="wasm_exec.js?end-br"></script>`, rewrite: true},
		{name: "ordinary SVG child", source: `<svg><g><script src="wasm_exec.js?g"></script></g></svg>`},
		{name: "HTML integration point", source: `<svg><foreignObject><p><script src="wasm_exec.js?integration"></script>`, rewrite: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := rewriteIndexForTest(t, test.source, options)
			rewritten := strings.Contains(got, runtimePath)
			if rewritten != test.rewrite {
				t.Fatalf("foreign-content classification rewrite = %v, want %v\ngot: %s", rewritten, test.rewrite, got)
			}
		})
	}

	t.Run("combined owned references", func(t *testing.T) {
		source := `<svg><p><link rel="stylesheet" href="styles.css"><script src="wasm_exec.js"></script><script>const go = new Go(); WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject).then((result) => go.run(result.instance));</script>`
		got := rewriteIndexForTest(t, source, htmlRewriteOptions{
			wasmPath:    "assets/bundle.11111111.wasm",
			runtimePath: runtimePath,
			styleRewrites: map[string]string{
				"styles.css": "assets/styles.33333333.css",
			},
		})
		for _, want := range []string{runtimePath, "assets/styles.33333333.css", "assets/bundle.11111111.wasm"} {
			if !strings.Contains(got, want) {
				t.Fatalf("breakout rewrite missing %q:\n%s", want, got)
			}
		}
	})
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
		source := `<html><head></head><body><plaintext></head><script src="wasm_exec.js"></script>`
		got := rewriteIndexForTest(t, source, options)
		insertion := strings.Index(got, preloadHTML(options))
		realClose := strings.Index(got, "</head>")
		if insertion < 0 || realClose < 0 || insertion > realClose {
			t.Fatalf("preload was not inserted before the real head close: %s", got)
		}
		if strings.Count(got, preloadHTML(options)) != 1 || !strings.HasSuffix(got, `<plaintext></head><script src="wasm_exec.js"></script>`) {
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

	t.Run("HTML non-void flag does not close context", func(t *testing.T) {
		source := `<svg><foreignObject><div/><![CDATA[x > <script src="wasm_exec.js?html"></script>]]></foreignObject></svg>`
		got := rewriteIndexForTest(t, source, htmlRewriteOptions{runtimePath: "assets/wasm_exec.22222222.js"})
		if !strings.Contains(got, `src="assets/wasm_exec.22222222.js?html"`) {
			t.Fatalf("self-closing HTML div incorrectly closed its HTML context: %s", got)
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
	want = want[:closingHead] + preloadHTML(options) + "\n" + want[closingHead:]

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
