//go:build js && wasm

package main

import (
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

func main() {
	global := js.Global()
	global.Set("goframeContextTopologyErrors", global.Get("Array").New())
	global.Set("goframeContextTopologySelectorCalls", 0)
	gf.SetErrorHandler(func(info gf.ErrorInfo) {
		report := global.Get("Object").New()
		report.Set("phase", info.Phase.String())
		report.Set("component", info.Component)
		report.Set("operation", info.Operation)
		report.Set("panic", gf.ToString(info.Panic))
		global.Get("goframeContextTopologyErrors").Call("push", report)
	})

	done := make(chan struct{})
	gf.Mount("root", App)
	<-done
}

func recordContextTopologySelectorCall() {
	global := js.Global()
	global.Set(
		"goframeContextTopologySelectorCalls",
		global.Get("goframeContextTopologySelectorCalls").Int()+1,
	)
}
