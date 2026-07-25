//go:build !js || !wasm

package main

import (
	gf "github.com/graybuton/goframe/pkg/goframe"
	"github.com/graybuton/goframe/scripts/fixtures/document-state/internal/documentstate"
)

type browserDocumentAdapter struct {
	baseline documentstate.State
}

func newBrowserDocumentAdapter() (*browserDocumentAdapter, error) {
	return &browserDocumentAdapter{}, nil
}

func (adapter *browserDocumentAdapter) Baseline() documentstate.State {
	if adapter == nil {
		return documentstate.State{}
	}
	return adapter.baseline
}

func (adapter *browserDocumentAdapter) Apply(documentstate.State) error {
	return nil
}

func initDocumentStateEvidence() {}

func recordDocumentStateTransition(documentstate.Transition) {}

func recordDocumentScopeMount() {}

func recordDocumentScopeUnmount() {}

func recordDocumentStateError(gf.ErrorInfo) {}
