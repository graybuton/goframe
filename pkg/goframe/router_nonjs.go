//go:build !js || !wasm

package goframe

func routerCurrentTarget() string {
	return "/"
}

func routerSubscribeHashChange(func(string)) Cleanup {
	return nil
}

// Navigate is a no-op outside browser/WASM builds.
func Navigate(string) {}
