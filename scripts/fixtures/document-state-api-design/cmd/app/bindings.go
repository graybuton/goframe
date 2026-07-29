package main

import (
	gf "github.com/graybuton/goframe/pkg/goframe"
	"github.com/graybuton/goframe/scripts/fixtures/document-state-api-design/internal/documentmeta"
)

type documentBindings struct {
	coordinator *documentmeta.Coordinator
	onSnapshot  func(documentmeta.Snapshot)
}

var documentBindingsContext = gf.CreateContext[*documentBindings](nil)

func provideDocumentBindings(bindings *documentBindings) {
	if bindings == nil || bindings.coordinator == nil || bindings.onSnapshot == nil {
		panic("document-state API design: document bindings are incomplete")
	}
	gf.ProvideContext(documentBindingsContext, bindings)
}

func requireDocumentBindings() *documentBindings {
	bindings := gf.UseContext(documentBindingsContext)
	if bindings == nil || bindings.coordinator == nil || bindings.onSnapshot == nil {
		panic("document-state API design: document bindings are missing")
	}
	return bindings
}

func publishCandidateTransition(
	candidate string,
	role string,
	transition documentmeta.Transition,
	onSnapshot func(documentmeta.Snapshot),
) {
	recordCandidateTransition(candidate, role, transition)
	if transition.Change != documentmeta.ChangeNone {
		onSnapshot(transition.Snapshot)
	}
}
