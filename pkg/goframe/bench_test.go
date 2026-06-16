package goframe

import "testing"

func BenchmarkDirtyQueuePruning(b *testing.B) {
	root := dirtyTestInstance("Root", nil)
	parent := dirtyTestInstance("Parent", root)
	child := dirtyTestInstance("Child", parent)
	grandchild := dirtyTestInstance("GrandChild", child)
	sibling := dirtyTestInstance("Sibling", root)
	dirty := []*componentInstance{child, parent, grandchild, sibling, child}

	b.ReportAllocs()
	for range b.N {
		result := pruneDirtyComponents(dirty)
		if len(result) != 2 {
			b.Fatalf("result length = %d", len(result))
		}
	}
}

func BenchmarkMatchChildIndicesKeyed(b *testing.B) {
	oldKeys := make([]string, 128)
	newKeys := make([]string, 128)
	for index := range oldKeys {
		oldKeys[index] = "item-" + ToString(index)
		newKeys[index] = "item-" + ToString(len(oldKeys)-1-index)
	}

	b.ReportAllocs()
	for range b.N {
		matches := matchChildIndices(oldKeys, newKeys)
		if matches[0] != len(oldKeys)-1 {
			b.Fatalf("first match = %d", matches[0])
		}
	}
}

func BenchmarkMatchChildIndicesUnkeyed(b *testing.B) {
	oldKeys := make([]string, 128)
	newKeys := make([]string, 128)

	b.ReportAllocs()
	for range b.N {
		matches := matchChildIndices(oldKeys, newKeys)
		if matches[127] != 127 {
			b.Fatalf("last match = %d", matches[127])
		}
	}
}

func BenchmarkSplitProps(b *testing.B) {
	props := Props{
		"ClassName":   "button primary",
		"ID":          "submit",
		"Value":       "task",
		"Placeholder": "What needs to be done?",
		"Disabled":    false,
		"Checked":     true,
		"data-testid": "todo-input",
		"OnInput":     func(InputEvent) {},
		"onClick":     func() {},
	}

	b.ReportAllocs()
	for range b.N {
		dom, events := splitProps(props)
		if len(dom) != 6 || len(events) != 2 {
			b.Fatalf("dom=%d events=%d", len(dom), len(events))
		}
	}
}

func BenchmarkEventNameNormalization(b *testing.B) {
	names := []string{"OnClick", "onInput", "OnSubmit", "class", "data-testid"}

	b.ReportAllocs()
	for range b.N {
		count := 0
		for _, name := range names {
			if _, ok := eventNameForProp(name); ok {
				count++
			}
		}
		if count != 3 {
			b.Fatalf("event count = %d", count)
		}
	}
}

func BenchmarkStateSlotAccess(b *testing.B) {
	instance := testComponentInstance("Slots", func() Node {
		a := UseState(1)
		b := UseState(2)
		c := UseState(3)
		d := UseState(4)
		if a.Get()+b.Get()+c.Get()+d.Get() == 0 {
			return Text("impossible")
		}
		return Empty()
	}, nil)
	renderComponentInstance(instance)

	b.ReportAllocs()
	for range b.N {
		renderComponentInstance(instance)
	}
}

func BenchmarkUnwrapKeyedNode(b *testing.B) {
	node := Key("outer", Key("inner", El("li", nil, Text("task"))))

	b.ReportAllocs()
	for range b.N {
		key, inner := unwrapNode(node)
		if key != "outer" {
			b.Fatalf("key = %q", key)
		}
		if _, ok := inner.(VNode); !ok {
			b.Fatalf("inner = %T", inner)
		}
	}
}
