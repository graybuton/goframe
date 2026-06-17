//go:build js && wasm

package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func main() {
	done := make(chan struct{})
	gf.Mount("root", App)
	<-done
}
