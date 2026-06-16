//go:build js && wasm

package main

import "syscall/js"

func setDocumentTitle(title string) {
	js.Global().Get("document").Set("title", title)
}
