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

func TestCustomIndexLegacyWASMFetchMatrix(t *testing.T) {
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

	for _, want := range []string{
		`fetch("assets/bundle.12345678.wasm")`,
		`fetch ( 'assets/bundle.12345678.wasm?v=1#app' )`,
		`fetch /* compatibility */ ( "assets/bundle.12345678.wasm#second", options )`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("WASM rewrite missing %q:\n%s", want, got)
		}
	}
	for _, preserved := range []string{
		`// fetch("bundle.wasm")`,
		`/* fetch('main.wasm') */`,
		`const docs = "bundle.wasm";`,
		"const template = `fetch(\"bundle.wasm\")`;",
		`/fetch\("bundle\.wasm"\)/`,
		`if (dynamicURL) /fetch\("bundle.wasm"\)/.test(dynamicURL);`,
		`obj.fetch("bundle.wasm")`,
		`fetch(dynamicURL)`,
		`fetch("bundle.wasm.map")`,
		`<script type="application/json">{"asset":"bundle.wasm"`,
		`<script type="importmap">{"imports":{"app":"bundle.wasm"}}</script>`,
		`<script type="speculationrules">{"prefetch":[{"source":"bundle.wasm"}]}</script>`,
		`<script type="text/plain">fetch("bundle.wasm")</script>`,
		`<template><script>fetch("bundle.wasm")</script></template>`,
	} {
		if !strings.Contains(got, preserved) {
			t.Errorf("WASM authored context %q changed:\n%s", preserved, got)
		}
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
		"<script> fetch (\"bundle.wasm#boot\") </script>\r\n<p> authored bundle.wasm 界 </p>\r\n</BODY>\r\n</HTML>"
	want := "<!DoCtYpE html>\r\n<HTML lang='界'>\r\n<HEAD data-doc=\"bundle.wasm\">\r\n" +
		"<link href='assets/styles.33333333.css?v=1#x' data-order=first rel='stylesheet'>\r\n</HEAD>\r\n" +
		"<BODY>\r\n<script data-x=1 src='assets/wasm_exec.22222222.js?v=2'></script>\r\n" +
		"<script> fetch (\"assets/bundle.11111111.wasm#boot\") </script>\r\n<p> authored bundle.wasm 界 </p>\r\n</BODY>\r\n</HTML>"
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
