package main

import gf "github.com/jin-wu/goframe/pkg/goframe"

func main() {
	done := make(chan struct{})
	gf.Mount("root", App)
	<-done
}
