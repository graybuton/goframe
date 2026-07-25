//go:build js && wasm

package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func main() {
	adapter, err := newBrowserDocumentAdapter()
	if err != nil {
		panic("server-backed document state: " + err.Error())
	}
	coordinator := newServerBackedDocumentCoordinator(adapter.Baseline())
	done := make(chan struct{})
	gf.Mount("root", func() gf.Node {
		return App(AppProps{
			Adapter:     adapter,
			Coordinator: coordinator,
		})
	})
	<-done
}
