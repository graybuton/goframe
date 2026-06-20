//go:build js && wasm

package goframe

import "syscall/js"

func routerCurrentTarget() string {
	location := js.Global().Get("location")
	if location.IsUndefined() || location.IsNull() {
		return "/"
	}
	return normalizeRouteTarget(location.Get("hash").String())
}

func routerSubscribeHashChange(callback func(string)) Cleanup {
	var listener js.Func
	listener = js.FuncOf(func(this js.Value, args []js.Value) any {
		defer func() {
			if recovered := recover(); recovered != nil {
				reportRecoveredRuntimeError(ErrorInfo{
					Phase:     ErrorPhaseEvent,
					Operation: "hashchange",
				}, recovered)
			}
		}()
		callback(routerCurrentTarget())
		return nil
	})
	js.Global().Call("addEventListener", "hashchange", listener)
	return func() {
		js.Global().Call("removeEventListener", "hashchange", listener)
		listener.Release()
	}
}

// Navigate changes the browser hash route.
func Navigate(to string) {
	location := js.Global().Get("location")
	if location.IsUndefined() || location.IsNull() {
		return
	}
	location.Set("hash", HashHref(to))
}
