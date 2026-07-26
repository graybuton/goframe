//go:build !js || !wasm

package main

type browserDocumentAdapter struct {
	baseline serverBackedDocumentState
}

func newBrowserDocumentAdapter() (*browserDocumentAdapter, error) {
	return &browserDocumentAdapter{}, nil
}

func (adapter *browserDocumentAdapter) Baseline() serverBackedDocumentState {
	if adapter == nil {
		return serverBackedDocumentState{}
	}
	return adapter.baseline
}

func (adapter *browserDocumentAdapter) Apply(serverBackedDocumentState) error {
	return nil
}
