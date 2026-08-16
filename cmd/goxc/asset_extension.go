package main

import "path"

type browserAssetKind uint8

const (
	browserAssetUnknown browserAssetKind = iota
	browserAssetWASM
	browserAssetJavaScript
	browserAssetCSS
	browserAssetHTML
)

func classifyBrowserAsset(name string) browserAssetKind {
	switch {
	case hasASCIIExtension(name, ".wasm"):
		return browserAssetWASM
	case hasASCIIExtension(name, ".js"):
		return browserAssetJavaScript
	case hasASCIIExtension(name, ".css"):
		return browserAssetCSS
	case hasASCIIExtension(name, ".html"):
		return browserAssetHTML
	default:
		return browserAssetUnknown
	}
}

func hasASCIIExtension(name, expected string) bool {
	return equalFoldASCII(path.Ext(name), expected)
}

func equalFoldASCII(actual, expected string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := 0; index < len(actual); index++ {
		left := actual[index]
		right := expected[index]
		if left >= 'A' && left <= 'Z' {
			left += 'a' - 'A'
		}
		if right >= 'A' && right <= 'Z' {
			right += 'a' - 'A'
		}
		if left != right {
			return false
		}
	}
	return true
}
