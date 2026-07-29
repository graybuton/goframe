//go:build js && wasm

package main

import (
	"strings"
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
	"github.com/graybuton/goframe/scripts/fixtures/document-state-api-design/internal/documentmeta"
)

func main() {
	mode := strings.TrimPrefix(js.Global().Get("location").Get("hash").String(), "#/")
	if !validDesignMode(mode) {
		mode = "control"
	}
	initDocumentAPIDesignEvidence(mode)
	adapter, err := newBrowserDocumentAdapter()
	if err != nil {
		panic("document-state API design fixture: " + err.Error())
	}
	coordinator := documentmeta.New(adapter.Baseline())
	recordCoordinatorStatistics(coordinator.Stats())
	control := documentmeta.NewStringOwners(coordinator)
	gf.SetErrorHandler(recordDocumentAPIDesignError)

	done := make(chan struct{})
	gf.Mount("root", func() gf.Node {
		return App(AppProps{
			Mode:        mode,
			Adapter:     adapter,
			Coordinator: coordinator,
			Control:     control,
		})
	})
	<-done
}

func validDesignMode(mode string) bool {
	switch mode {
	case "control", "hook", "component", "handle":
		return true
	default:
		return false
	}
}
