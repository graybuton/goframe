//go:build !js || !wasm

package goframe

import "errors"

// FetchText loads key with browser fetch and resolves the response body text.
//
// FetchText is available only in browser/WASM builds. Host builds reject with a
// clear error so packages can compile and tests can exercise fallback paths.
func FetchText(key string, resolve func(string), reject func(error)) Cleanup {
	if reject != nil {
		reject(errors.New("goframe: FetchText is only available in js/wasm browser builds"))
	}
	return func() {}
}
