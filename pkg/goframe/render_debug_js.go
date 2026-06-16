//go:build js && wasm && goframe_debug

package goframe

import "syscall/js"

func performanceNow() float64 {
	performance := js.Global().Get("performance")
	if performance.IsUndefined() || performance.IsNull() {
		return 0
	}
	return performance.Call("now").Float()
}

func reportRender(phase string, duration float64) {
	probe := js.Global().Get("goframeRenderProbe")
	if probe.Type() == js.TypeFunction {
		probe.Invoke(phase, duration)
	}
}
