package ui

import gf "github.com/graybuton/goframe/pkg/goframe"

type LayoutProps struct {
	Title       string
	Count       int
	OnIncrement func()
	Children    []gf.Node
}

type HeaderProps struct {
	Title string
}
