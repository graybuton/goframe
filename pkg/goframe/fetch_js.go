//go:build js && wasm

package goframe

import (
	"errors"
	"strconv"
	"syscall/js"
)

// FetchText loads key with browser fetch and resolves the response body text.
//
// FetchText is an experimental browser/WASM ResourceLoader-compatible helper.
// Cleanup aborts the in-flight fetch and prevents later completion callbacks
// from resolving or rejecting the resource generation.
func FetchText(key string, resolve func(string), reject func(error)) Cleanup {
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
	fail := func(err error) {
		if !active {
			return
		}
		active = false
		reject(err)
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
		fail(errors.New("goframe: fetch failed"))
		return nil
	})
	responseThen = js.FuncOf(func(this js.Value, args []js.Value) any {
		if !active {
			releasePromiseFuncs()
			return nil
		}
		if len(args) == 0 {
			releasePromiseFuncs()
			fail(errors.New("goframe: fetch returned no response"))
			return nil
		}
		response := args[0]
		if !response.Get("ok").Bool() {
			status := response.Get("status").Int()
			releasePromiseFuncs()
			fail(errors.New("goframe: fetch returned HTTP " + strconv.Itoa(status)))
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
