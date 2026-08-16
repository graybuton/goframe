package main

import "testing"

func TestBrowserAssetExtensionClassification(t *testing.T) {
	tests := []struct {
		name string
		kind browserAssetKind
	}{
		{name: "bundle.wasm", kind: browserAssetWASM},
		{name: "bundle.WASM", kind: browserAssetWASM},
		{name: "bundle.WaSm", kind: browserAssetWASM},
		{name: "runtime.js", kind: browserAssetJavaScript},
		{name: "runtime.JS", kind: browserAssetJavaScript},
		{name: "runtime.Js", kind: browserAssetJavaScript},
		{name: "theme.css", kind: browserAssetCSS},
		{name: "theme.CSS", kind: browserAssetCSS},
		{name: "theme.CsS", kind: browserAssetCSS},
		{name: "index.html", kind: browserAssetHTML},
		{name: "index.HTML", kind: browserAssetHTML},
		{name: "index.HtMl", kind: browserAssetHTML},
		{name: "bundle.waſm", kind: browserAssetUnknown},
		{name: "runtime.jſ", kind: browserAssetUnknown},
		{name: "theme.csſ", kind: browserAssetUnknown},
		{name: "bundle.wasmx", kind: browserAssetUnknown},
		{name: "theme.css.gz", kind: browserAssetUnknown},
		{name: "README", kind: browserAssetUnknown},
		{name: "asset.界", kind: browserAssetUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyBrowserAsset(test.name); got != test.kind {
				t.Fatalf("classifyBrowserAsset(%q) = %d, want %d", test.name, got, test.kind)
			}
		})
	}

	if equalFoldASCII("ſ", "s") {
		t.Fatal("equalFoldASCII() folded non-ASCII input")
	}
	if allocations := testing.AllocsPerRun(1000, func() {
		_ = classifyBrowserAsset("assets/THEME.CsS")
	}); allocations != 0 {
		t.Fatalf("classifyBrowserAsset() allocations = %v, want 0", allocations)
	}
}
