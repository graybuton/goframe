//go:build !goframe_debug

package goframe

func reportComponentRender(name string) {}

func reportComponentPatch(name string) {}

func reportComponentMemoSkip(name string) {}

func reportDuplicateSiblingNodeKeys(nodes []Node, owner string) {}

func reportDuplicateSiblingKeys(keys []string, owner string) {}
