//go:build goframe_debug && (!js || !wasm)

package goframe

func reportStateSetAfterUnmount(owner string) {}

func reportStateSetDuringRender(owner, renderer string) {}

func reportEffectUpdateLoopGuard() {}

func shouldStopEffectUpdateLoop() bool {
	return true
}
