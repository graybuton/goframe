//go:build js && wasm

package main

import (
	"strings"
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
	"github.com/graybuton/goframe/scripts/fixtures/history-routing/internal/historyroute"
)

func browserHistoryBasePath() string {
	document := js.Global().Get("document")
	if document.IsUndefined() || document.IsNull() {
		return "/"
	}
	baseURI := document.Get("baseURI")
	if baseURI.IsUndefined() || baseURI.IsNull() || baseURI.String() == "" {
		return "/"
	}
	parsed := js.Global().Get("URL").New(baseURI.String())
	base, err := historyroute.NormalizeBase(parsed.Get("pathname").String())
	if err != nil {
		return "/"
	}
	return base
}

func browserHistoryCurrentTarget(base string) string {
	location := js.Global().Get("location")
	if location.IsUndefined() || location.IsNull() {
		return "/"
	}
	target, inside, err := historyroute.TargetFromLocation(
		base,
		location.Get("pathname").String(),
		strings.TrimPrefix(location.Get("search").String(), "?"),
	)
	if err != nil || !inside {
		return "/"
	}
	return target
}

func browserHistoryPush(base, target string) string {
	return browserHistoryChange("pushState", base, target)
}

func browserHistoryReplace(base, target string) string {
	return browserHistoryChange("replaceState", base, target)
}

func browserHistoryChange(method, base, target string) string {
	target = historyroute.NormalizeTarget(target)
	location, err := historyroute.LocationForTarget(base, target)
	if err != nil {
		panic("history fixture: construct browser location: " + err.Error())
	}
	js.Global().Get("history").Call(method, js.Null(), "", location)
	return target
}

func browserHistorySubscribe(base string, callback func(string)) gf.Cleanup {
	var listener js.Func
	listener = js.FuncOf(func(this js.Value, args []js.Value) any {
		callback(browserHistoryCurrentTarget(base))
		return nil
	})
	js.Global().Call("addEventListener", "popstate", listener)
	return func() {
		js.Global().Call("removeEventListener", "popstate", listener)
		listener.Release()
	}
}
