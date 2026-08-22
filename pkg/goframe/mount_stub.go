//go:build !js || !wasm

package goframe

// Mount is only available in a js/wasm build.
func Mount(rootID string, app func() Node) {
	panicRuntimeInvariant("goframe: Mount requires GOOS=js GOARCH=wasm")
}
