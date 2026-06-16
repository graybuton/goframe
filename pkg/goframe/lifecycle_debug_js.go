//go:build js && wasm && goframe_debug

package goframe

import "syscall/js"

func reportStateSetAfterUnmount(owner string) {
	reportLifecycleWarning("goframe: State.Set called after component unmount: " + owner)
}

func reportStateSetDuringRender(owner, renderer string) {
	reportLifecycleWarning("goframe: State.Set called during render of " + renderer + " for " + owner)
}

func reportEffectUpdateLoopGuard() {
	reportLifecycleWarning("goframe: effect update loop guard stopped pending updates")
}

func shouldStopEffectUpdateLoop() bool {
	return true
}

func reportLifecycleWarning(message string) {
	warnings := js.Global().Get("goframeLifecycleWarnings")
	if warnings.IsUndefined() || warnings.IsNull() {
		warnings = js.Global().Get("Array").New()
		js.Global().Set("goframeLifecycleWarnings", warnings)
	}
	warnings.Call("push", message)

	console := js.Global().Get("console")
	if !console.IsUndefined() && !console.IsNull() && console.Get("warn").Type() == js.TypeFunction {
		console.Call("warn", message)
	}
}
