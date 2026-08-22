//go:build js && wasm

package main

import (
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

func main() {
	errors := js.Global().Get("Array").New()
	js.Global().Set("__customIndexRuntimeErrors", errors)
	gf.SetErrorHandler(func(info gf.ErrorInfo) {
		errors.Call("push", info.Phase.String()+":"+info.Component+":"+info.Operation)
	})
	done := make(chan struct{})
	gf.Mount("root", App)
	<-done
}

func App() gf.Node {
	count, setCount := gf.UseState(0)
	return gf.El("main", gf.Props{"data-testid": "custom-index-app"},
		gf.El("h1", gf.Props{}, gf.Text("Custom index rewrite fixture")),
		gf.El("output", gf.Props{"data-testid": "custom-index-count"}, gf.Text(gf.ToString(count))),
		gf.El("button", gf.Props{
			"type":        "button",
			"data-testid": "custom-index-increment",
			"OnClick": func() {
				setCount(count + 1)
			},
		}, gf.Text("Increment")),
	)
}
