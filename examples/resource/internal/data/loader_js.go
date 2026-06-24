//go:build js && wasm

package data

import (
	"strings"
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

const slowPrefix = "slow:"

type LoadError string

func (err LoadError) Error() string {
	return string(err)
}

func LoadIssues(key string, resolve func([]Issue), reject func(error)) gf.Cleanup {
	url := key
	delay := 0
	if strings.HasPrefix(key, slowPrefix) {
		url = strings.TrimPrefix(key, slowPrefix)
		delay = 320
	}

	active := true
	releasedPromiseFuncs := false
	releasedTimer := false
	var timer js.Value
	var timerCallback js.Func
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
	releaseTimer := func() {
		if releasedTimer {
			return
		}
		releasedTimer = true
		if timer.Type() == js.TypeNumber {
			js.Global().Call("clearTimeout", timer)
		}
		timerCallback.Release()
	}
	complete := func(text string) {
		if !active {
			return
		}
		issues, err := ParseIssues(text)
		active = false
		if err != nil {
			reject(err)
			return
		}
		resolve(issues)
	}

	timerCallback = js.FuncOf(func(this js.Value, args []js.Value) any {
		releaseTimer()
		complete(args[0].String())
		return nil
	})
	textThen = js.FuncOf(func(this js.Value, args []js.Value) any {
		text := ""
		if len(args) > 0 && args[0].Type() == js.TypeString {
			text = args[0].String()
		}
		releasePromiseFuncs()
		if !active {
			return nil
		}
		if delay > 0 {
			timer = js.Global().Call("setTimeout", timerCallback, delay, text)
			return nil
		}
		releaseTimer()
		complete(text)
		return nil
	})
	catchFunc = js.FuncOf(func(this js.Value, args []js.Value) any {
		releasePromiseFuncs()
		releaseTimer()
		if active {
			active = false
			reject(LoadError("fetch failed"))
		}
		return nil
	})
	responseThen = js.FuncOf(func(this js.Value, args []js.Value) any {
		if !active {
			releasePromiseFuncs()
			releaseTimer()
			return nil
		}
		if len(args) == 0 {
			active = false
			releasePromiseFuncs()
			releaseTimer()
			reject(LoadError("fetch returned no response"))
			return nil
		}
		response := args[0]
		if !response.Get("ok").Bool() {
			active = false
			releasePromiseFuncs()
			releaseTimer()
			reject(LoadError("fetch returned a non-ok response"))
			return nil
		}
		response.Call("text").Call("then", textThen).Call("catch", catchFunc)
		return nil
	})

	controller := js.Global().Get("AbortController").New()
	options := js.Global().Get("Object").New()
	options.Set("signal", controller.Get("signal"))
	js.Global().Call("fetch", url, options).Call("then", responseThen).Call("catch", catchFunc)

	return func() {
		if !active {
			return
		}
		active = false
		releaseTimer()
		controller.Call("abort")
	}
}
