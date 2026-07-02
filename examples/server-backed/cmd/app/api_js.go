//go:build js && wasm

package main

import (
	"strconv"
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

func loadGreeting(key string, resolve func(string), reject func(error)) gf.Cleanup {
	active := true
	releasedPromiseFuncs := false
	var responseThen js.Func
	var textThen js.Func
	var catchFunc js.Func

	releasePromiseFuncs := func() {
		if releasedPromiseFuncs {
			return
		}
		releasedPromiseFuncs = true
		responseThen.Release()
		textThen.Release()
		catchFunc.Release()
	}
	complete := func(text string) {
		if !active {
			return
		}
		active = false
		resolve(text)
	}

	textThen = js.FuncOf(func(this js.Value, args []js.Value) any {
		text := ""
		if len(args) > 0 && args[0].Type() == js.TypeString {
			text = args[0].String()
		}
		releasePromiseFuncs()
		complete(text)
		return nil
	})
	catchFunc = js.FuncOf(func(this js.Value, args []js.Value) any {
		releasePromiseFuncs()
		if active {
			active = false
			reject(apiError("fetch failed"))
		}
		return nil
	})
	responseThen = js.FuncOf(func(this js.Value, args []js.Value) any {
		if !active {
			releasePromiseFuncs()
			return nil
		}
		if len(args) == 0 {
			active = false
			releasePromiseFuncs()
			reject(apiError("fetch returned no response"))
			return nil
		}
		response := args[0]
		if !response.Get("ok").Bool() {
			status := response.Get("status").Int()
			active = false
			releasePromiseFuncs()
			reject(apiError("backend returned HTTP " + strconv.Itoa(status)))
			return nil
		}
		response.Call("text").Call("then", textThen).Call("catch", catchFunc)
		return nil
	})

	controller := js.Global().Get("AbortController").New()
	options := js.Global().Get("Object").New()
	options.Set("signal", controller.Get("signal"))
	js.Global().Call("fetch", key, options).Call("then", responseThen).Call("catch", catchFunc)

	return func() {
		if !active {
			return
		}
		active = false
		controller.Call("abort")
	}
}
