//go:build goframe_debug && (!js || !wasm)

package goframe

func performanceNow() float64 {
	return 0
}

func reportRender(phase string, duration float64) {}
