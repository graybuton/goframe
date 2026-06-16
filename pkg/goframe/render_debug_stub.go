//go:build !goframe_debug

package goframe

func performanceNow() float64 {
	return 0
}

func reportRender(phase string, duration float64) {}
