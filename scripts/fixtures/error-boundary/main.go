//go:build js && wasm

package main

import (
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

func main() {
	initBoundaryProbe()
	gf.SetErrorHandler(func(info gf.ErrorInfo) {
		report := js.Global().Get("Object").New()
		report.Set("phase", info.Phase.String())
		report.Set("component", info.Component)
		report.Set("operation", info.Operation)
		report.Set("panic", gf.ToString(info.Panic))
		js.Global().Get("goframeErrorBoundaryReports").Call("push", report)
	})

	done := make(chan struct{})
	gf.Mount("root", App)
	<-done
}
