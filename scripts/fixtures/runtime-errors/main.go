//go:build js && wasm

package main

import (
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

type cleanupPanelProps struct{}

func App() gf.Node {
	count, setCount := gf.UseState(0)
	showCleanup, setShowCleanup := gf.UseState(true)

	gf.UseEffect(func() gf.Cleanup {
		panic("effect setup boom")
	})

	children := []gf.Node{
		gf.El("p", gf.Props{"id": "runtime-error-count"}, gf.Text(gf.ToString(count))),
		gf.El("button", gf.Props{
			"id": "event-panic",
			"OnClick": func() {
				panic("event boom")
			},
		}, gf.Text("Trigger event panic")),
		gf.El("button", gf.Props{
			"id": "increment",
			"OnClick": func() {
				setCount(count + 1)
			},
		}, gf.Text("Increment")),
		gf.El("button", gf.Props{
			"id": "toggle-cleanup",
			"OnClick": func() {
				setShowCleanup(!showCleanup)
			},
		}, gf.Text("Toggle cleanup panel")),
	}
	if showCleanup {
		children = append(children, gf.Component("CleanupPanel", cleanupPanelProps{}, CleanupPanel))
	}
	return gf.El("main", gf.Props{"id": "runtime-error-fixture"}, children...)
}

func CleanupPanel(cleanupPanelProps) gf.Node {
	gf.UseEffect(func() gf.Cleanup {
		return func() {
			panic("effect cleanup boom")
		}
	})
	gf.UseUnmount(func() {
		panic("unmount cleanup boom")
	})
	return gf.El("section", gf.Props{"id": "cleanup-panel"}, gf.Text("cleanup panel"))
}

func main() {
	js.Global().Set("goframeRuntimeErrorReports", js.Global().Get("Array").New())
	gf.SetErrorHandler(func(info gf.ErrorInfo) {
		report := js.Global().Get("Object").New()
		report.Set("phase", info.Phase.String())
		report.Set("component", info.Component)
		report.Set("operation", info.Operation)
		report.Set("panic", gf.ToString(info.Panic))
		js.Global().Get("goframeRuntimeErrorReports").Call("push", report)
	})

	done := make(chan struct{})
	gf.Mount("root", App)
	<-done
}
