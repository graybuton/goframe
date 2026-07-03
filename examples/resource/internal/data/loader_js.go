//go:build js && wasm

package data

import (
	"strings"
	"syscall/js"

	gf "github.com/graybuton/goframe/pkg/goframe"
)

const slowPrefix = "slow:"

func LoadIssues(key string, resolve func([]Issue), reject func(error)) gf.Cleanup {
	url := key
	delay := 0
	if strings.HasPrefix(key, slowPrefix) {
		url = strings.TrimPrefix(key, slowPrefix)
		delay = 320
	}

	active := true
	releasedTimer := false
	var timer js.Value
	var timerCallback js.Func
	var fetchCleanup gf.Cleanup

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
		text := ""
		if len(args) > 0 && args[0].Type() == js.TypeString {
			text = args[0].String()
		}
		releaseTimer()
		complete(text)
		return nil
	})

	fetchCleanup = gf.FetchText(url, func(text string) {
		if !active {
			return
		}
		if delay > 0 {
			timer = js.Global().Call("setTimeout", timerCallback, delay, text)
			return
		}
		releaseTimer()
		complete(text)
	}, func(err error) {
		if !active {
			return
		}
		active = false
		releaseTimer()
		reject(err)
	})

	return func() {
		if !active {
			return
		}
		active = false
		releaseTimer()
		if fetchCleanup != nil {
			fetchCleanup()
		}
	}
}
