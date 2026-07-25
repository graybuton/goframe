//go:build js && wasm

package main

import (
	gf "github.com/graybuton/goframe/pkg/goframe"
	"github.com/graybuton/goframe/scripts/fixtures/document-state/internal/documentstate"
)

func main() {
	initDocumentStateEvidence()
	adapter, err := newBrowserDocumentAdapter()
	if err != nil {
		panic("document-state fixture: " + err.Error())
	}
	coordinator := documentstate.New(adapter.Baseline())
	gf.SetErrorHandler(recordDocumentStateError)

	done := make(chan struct{})
	gf.Mount("root", func() gf.Node {
		return App(AppProps{
			Adapter:     adapter,
			Coordinator: coordinator,
		})
	})
	<-done
}
