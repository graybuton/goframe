//go:build !js || !wasm

package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func loadGreeting(key string, resolve func(string), reject func(error)) gf.Cleanup {
	reject(apiError("browser fetch is available only in browser/WASM builds"))
	return nil
}
