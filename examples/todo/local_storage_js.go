//go:build js && wasm

package main

import "syscall/js"

func localStorageGet(key string) (string, bool) {
	storage := js.Global().Get("localStorage")
	if storage.IsUndefined() || storage.IsNull() {
		return "", false
	}
	value := storage.Call("getItem", key)
	if value.IsUndefined() || value.IsNull() {
		return "", false
	}
	return value.String(), true
}

func localStorageSet(key string, value string) {
	storage := js.Global().Get("localStorage")
	if storage.IsUndefined() || storage.IsNull() {
		return
	}
	storage.Call("setItem", key, value)
}
