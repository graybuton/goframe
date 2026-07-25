//go:build js && wasm

package main

import (
	"errors"
	"fmt"
	"syscall/js"
)

type browserDocumentAdapter struct {
	title           js.Value
	description     js.Value
	viewport        js.Value
	viewportContent string
	baseline        serverBackedDocumentState
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
	viewports := document.Call(
		"querySelectorAll",
		`head meta[name="viewport"]`,
	)
	if viewports.Get("length").Int() != 1 {
		return nil, fmt.Errorf(
			"expected exactly one authored viewport element, found %d",
			viewports.Get("length").Int(),
		)
	}

	title := titles.Index(0)
	description := descriptions.Index(0)
	viewport := viewports.Index(0)
	return &browserDocumentAdapter{
		title:           title,
		description:     description,
		viewport:        viewport,
		viewportContent: viewport.Call("getAttribute", "content").String(),
		baseline: serverBackedDocumentState{
			Title:       title.Get("textContent").String(),
			Description: description.Call("getAttribute", "content").String(),
		},
	}, nil
}

func (adapter *browserDocumentAdapter) Baseline() serverBackedDocumentState {
	if adapter == nil {
		return serverBackedDocumentState{}
	}
	return adapter.baseline
}

func (adapter *browserDocumentAdapter) Apply(state serverBackedDocumentState) error {
	if adapter == nil {
		return errors.New("document adapter is nil")
	}
	document := js.Global().Get("document")
	titles := document.Call("querySelectorAll", "head title")
	if titles.Get("length").Int() != 1 ||
		!titles.Index(0).Equal(adapter.title) ||
		!adapter.title.Get("isConnected").Bool() {
		return errors.New("authored title element changed identity")
	}
	descriptions := document.Call(
		"querySelectorAll",
		`head meta[name="description"]`,
	)
	if descriptions.Get("length").Int() != 1 ||
		!descriptions.Index(0).Equal(adapter.description) ||
		!adapter.description.Get("isConnected").Bool() {
		return errors.New("authored description element changed identity")
	}
	viewports := document.Call(
		"querySelectorAll",
		`head meta[name="viewport"]`,
	)
	if viewports.Get("length").Int() != 1 ||
		!viewports.Index(0).Equal(adapter.viewport) ||
		!adapter.viewport.Get("isConnected").Bool() {
		return errors.New("authored viewport element changed identity")
	}
	if adapter.viewport.Call("getAttribute", "content").String() != adapter.viewportContent {
		return errors.New("authored viewport content changed")
	}

	if adapter.title.Get("textContent").String() != state.Title {
		adapter.title.Set("textContent", state.Title)
	}
	if adapter.description.Call("getAttribute", "content").String() != state.Description {
		adapter.description.Call("setAttribute", "content", state.Description)
	}
	return nil
}
