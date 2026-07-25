//go:build js && wasm

package main

import (
	"errors"
	"fmt"
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
	"github.com/graybuton/goframe/scripts/fixtures/document-state/internal/documentstate"
)

type browserDocumentAdapter struct {
	title       js.Value
	description js.Value
	unrelated   js.Value
	baseline    documentstate.State
	current     documentstate.State
}

func newBrowserDocumentAdapter() (*browserDocumentAdapter, error) {
	document := js.Global().Get("document")
	if document.IsUndefined() || document.IsNull() {
		return nil, errors.New("document is unavailable")
	}
	titles := document.Call("querySelectorAll", "head title")
	if titles.Get("length").Int() != 1 {
		return nil, fmt.Errorf(
			"expected exactly one authored title element, found %d",
			titles.Get("length").Int(),
		)
	}
	descriptions := document.Call(
		"querySelectorAll",
		`head meta[name="description"]`,
	)
	if descriptions.Get("length").Int() != 1 {
		return nil, fmt.Errorf(
			"expected exactly one authored description element, found %d",
			descriptions.Get("length").Int(),
		)
	}
	unrelated := document.Call(
		"querySelector",
		`head meta[name="fixture-unrelated"]`,
	)
	if unrelated.IsUndefined() || unrelated.IsNull() {
		return nil, errors.New("authored unrelated metadata is missing")
	}

	title := titles.Index(0)
	description := descriptions.Index(0)
	baseline := documentstate.State{
		Title:       title.Get("textContent").String(),
		Description: description.Call("getAttribute", "content").String(),
	}
	return &browserDocumentAdapter{
		title:       title,
		description: description,
		unrelated:   unrelated,
		baseline:    baseline,
		current:     baseline,
	}, nil
}

func (adapter *browserDocumentAdapter) Baseline() documentstate.State {
	if adapter == nil {
		return documentstate.State{}
	}
	return adapter.baseline
}

func (adapter *browserDocumentAdapter) Apply(state documentstate.State) error {
	if adapter == nil {
		return errors.New("document adapter is nil")
	}
	if !adapter.title.Get("isConnected").Bool() {
		return errors.New("authored title element is no longer connected")
	}
	if !adapter.description.Get("isConnected").Bool() {
		return errors.New("authored description element is no longer connected")
	}
	if adapter.unrelated.Call("getAttribute", "content").String() != "preserve-me" {
		return errors.New("unrelated authored metadata changed")
	}

	previous := adapter.current
	if adapter.title.Get("textContent").String() != state.Title {
		adapter.title.Set("textContent", state.Title)
	}
	if adapter.description.Call("getAttribute", "content").String() != state.Description {
		adapter.description.Call("setAttribute", "content", state.Description)
	}
	adapter.current = state
	if previous != adapter.baseline && state == adapter.baseline {
		incrementDocumentStateEvidence("baselineRestorations")
	}
	return nil
}

func initDocumentStateEvidence() {
	evidence := js.Global().Get("Object").New()
	for _, field := range []string{
		"routeMetadataCommits",
		"nestedOwnerActivations",
		"nestedOwnerReleases",
		"ownerUpdates",
		"ownerRemovals",
		"baselineRestorations",
		"scopeMounts",
		"scopeUnmounts",
		"errorBoundaryCaptures",
	} {
		evidence.Set(field, 0)
	}
	evidence.Set("ownershipEvents", js.Global().Get("Array").New())
	evidence.Set("runtimeErrors", js.Global().Get("Array").New())
	js.Global().Set("goframeDocumentStateEvidence", evidence)
}

func recordDocumentStateTransition(transition documentstate.Transition) {
	if transition.Change == documentstate.ChangeNone {
		return
	}
	switch {
	case transition.Owner == "route":
		incrementDocumentStateEvidence("routeMetadataCommits")
	case transition.Owner == "editor" && transition.Change == documentstate.ChangeAdded:
		incrementDocumentStateEvidence("nestedOwnerActivations")
	case transition.Owner == "editor" && transition.Change == documentstate.ChangeRemoved:
		incrementDocumentStateEvidence("nestedOwnerReleases")
	}
	if transition.Change == documentstate.ChangeUpdated {
		incrementDocumentStateEvidence("ownerUpdates")
	}
	if transition.Change == documentstate.ChangeRemoved {
		incrementDocumentStateEvidence("ownerRemovals")
	}

	event := js.Global().Get("Object").New()
	event.Set("owner", transition.Owner)
	event.Set("change", transition.Change.String())
	event.Set("activeOwner", transition.Snapshot.ActiveOwner)
	event.Set("title", transition.Snapshot.State.Title)
	event.Set("description", transition.Snapshot.State.Description)
	documentStateEvidence().Get("ownershipEvents").Call("push", event)
}

func recordDocumentScopeMount() {
	incrementDocumentStateEvidence("scopeMounts")
}

func recordDocumentScopeUnmount() {
	incrementDocumentStateEvidence("scopeUnmounts")
}

func recordDocumentStateError(info gf.ErrorInfo) {
	incrementDocumentStateEvidence("errorBoundaryCaptures")
	report := js.Global().Get("Object").New()
	report.Set("phase", info.Phase.String())
	report.Set("component", info.Component)
	report.Set("operation", info.Operation)
	report.Set("panic", gf.ToString(info.Panic))
	documentStateEvidence().Get("runtimeErrors").Call("push", report)
}

func incrementDocumentStateEvidence(field string) {
	evidence := documentStateEvidence()
	evidence.Set(field, evidence.Get(field).Int()+1)
}

func documentStateEvidence() js.Value {
	return js.Global().Get("goframeDocumentStateEvidence")
}
