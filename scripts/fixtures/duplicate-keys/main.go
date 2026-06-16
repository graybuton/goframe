package main

import gf "github.com/graybuton/goframe/pkg/goframe"

func App() gf.Node {
	return gf.El("main", gf.Props{"id": "duplicate-key-fixture"},
		gf.El("ul", nil,
			gf.Key("same", gf.El("li", nil, gf.Text("first"))),
			gf.Key("same", gf.El("li", nil, gf.Text("second"))),
		),
	)
}

func main() {
	done := make(chan struct{})
	gf.Mount("root", App)
	<-done
}
