//go:build js && wasm

package main

import (
	"errors"
	"strconv"
	"strings"
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

func postSavedGreeting(name string, resolve func(string), reject func(error)) gf.Cleanup {
	active := true
	releasedPromiseFuncs := false
	responseOK := false
	responseStatus := 0
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
			text = strings.TrimSpace(args[0].String())
		}
		releasePromiseFuncs()
		if !responseOK {
			if text == "" {
				text = "saved greeting request returned HTTP " + strconv.Itoa(responseStatus)
			}
			fail(errors.New(text))
			return nil
		}
		complete(text)
		return nil
	})
	catchFunc = js.FuncOf(func(this js.Value, args []js.Value) any {
		releasePromiseFuncs()
		fail(errors.New("saved greeting request failed"))
		return nil
	})
	responseThen = js.FuncOf(func(this js.Value, args []js.Value) any {
		if !active {
			releasePromiseFuncs()
			return nil
		}
		if len(args) == 0 {
			releasePromiseFuncs()
			fail(errors.New("saved greeting request returned no response"))
			return nil
		}
		response := args[0]
		responseOK = response.Get("ok").Bool()
		responseStatus = response.Get("status").Int()
		response.Call("text").Call("then", textThen).Call("catch", catchFunc)
		return nil
	})

	controller := js.Global().Get("AbortController").New()
	headers := js.Global().Get("Headers").New()
	headers.Call("set", "Content-Type", "application/x-www-form-urlencoded")
	form := js.Global().Get("URLSearchParams").New()
	form.Call("set", "name", name)
	options := js.Global().Get("Object").New()
	options.Set("method", "POST")
	options.Set("headers", headers)
	options.Set("body", form.Call("toString"))
	options.Set("signal", controller.Get("signal"))
	js.Global().Call("fetch", savedGreetingAPIPath, options).Call("then", responseThen).Call("catch", catchFunc)

	return func() {
		if !active {
			return
		}
		active = false
		controller.Call("abort")
	}
}
