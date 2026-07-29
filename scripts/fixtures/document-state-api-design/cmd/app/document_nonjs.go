//go:build !js || !wasm

package main

import (
	gf "github.com/graybuton/goframe/pkg/goframe"
	"github.com/graybuton/goframe/scripts/fixtures/document-state-api-design/internal/documentmeta"
)

type browserDocumentAdapter struct {
	baseline documentmeta.Metadata
}

func newBrowserDocumentAdapter() (*browserDocumentAdapter, error) {
	return &browserDocumentAdapter{}, nil
}

func (adapter *browserDocumentAdapter) Baseline() documentmeta.Metadata {
	if adapter == nil {
		return documentmeta.Metadata{}
	}
	return adapter.baseline
}

func (adapter *browserDocumentAdapter) Apply(documentmeta.Metadata) error {
	return nil
}

func initDocumentAPIDesignEvidence(string) {}

func recordCandidateTransition(string, string, documentmeta.Transition) {}

func recordCandidateRender(string, string, int) {}

func recordCandidateOwnerRender(string, string, uint64) {}

func recordComponentOwnerMount(string, uint64) {}

func recordComponentOwnerUnmount(string, uint64) {}

func recordHandleForward(uint64) {}

func recordHandleDuplicateCoalesced(uint64) {}

func recordHandleCreation(string) {}

func recordPublicationCreation(string, bool) {}

func recordHandlePublicationState(string, string, uint64, int) {}

func recordCoordinatorStatistics(documentmeta.Statistics) {}

func recordScopeMount() {}

func recordScopeUnmount() {}

func recordDocumentAPIDesignError(gf.ErrorInfo) {}
