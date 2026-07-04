package goframe

import "testing"

var benchmarkChildMatches []int

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

func BenchmarkMatchChildIndicesReorders(b *testing.B) {
	const size = 128

	tests := []struct {
		name   string
		old    []string
		new    []string
		checks map[int]int
	}{
		{
			name: "stable_keyed_order",
			old:  sequentialBenchmarkKeys(size),
			new:  sequentialBenchmarkKeys(size),
			checks: map[int]int{
				0:        0,
				size - 1: size - 1,
			},
		},
		{
			name: "reverse_keyed_order",
			old:  sequentialBenchmarkKeys(size),
			new:  reverseBenchmarkKeys(size),
			checks: map[int]int{
				0:        size - 1,
				size - 1: 0,
			},
		},
		{
			name: "rotate_left_keyed_order",
			old:  sequentialBenchmarkKeys(size),
			new:  rotateLeftBenchmarkKeys(size),
			checks: map[int]int{
				0:        1,
				size - 1: 0,
			},
		},
		{
			name: "rotate_right_keyed_order",
			old:  sequentialBenchmarkKeys(size),
			new:  rotateRightBenchmarkKeys(size),
			checks: map[int]int{
				0:        size - 1,
				size - 1: size - 2,
			},
		},
		{
			name: "insert_remove_keyed_order",
			old:  sequentialBenchmarkKeys(size),
			new:  insertRemoveBenchmarkKeys(size),
			checks: map[int]int{
				0:        noChildMatch,
				1:        1,
				size - 1: size - 1,
			},
		},
		{
			name: "mixed_keyed_unkeyed_order",
			old:  mixedBenchmarkKeys(size),
			new:  mixedReorderedBenchmarkKeys(size),
			checks: map[int]int{
				0: size - 2,
				1: 1,
				2: 0,
				3: 3,
			},
		},
	}

	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			matches := matchChildIndices(test.old, test.new)
			for index, want := range test.checks {
				if matches[index] != want {
					b.Fatalf("sanity match[%d] = %d, want %d", index, matches[index], want)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchmarkChildMatches = matchChildIndices(test.old, test.new)
			}
		})
	}
}

func sequentialBenchmarkKeys(size int) []string {
	keys := make([]string, size)
	for index := range keys {
		keys[index] = "item-" + ToString(index)
	}
	return keys
}

func reverseBenchmarkKeys(size int) []string {
	keys := make([]string, size)
	for index := range keys {
		keys[index] = "item-" + ToString(size-1-index)
	}
	return keys
}

func rotateLeftBenchmarkKeys(size int) []string {
	keys := make([]string, size)
	for index := range keys {
		keys[index] = "item-" + ToString((index+1)%size)
	}
	return keys
}

func rotateRightBenchmarkKeys(size int) []string {
	keys := make([]string, size)
	for index := range keys {
		keys[index] = "item-" + ToString((index+size-1)%size)
	}
	return keys
}

func insertRemoveBenchmarkKeys(size int) []string {
	keys := sequentialBenchmarkKeys(size)
	keys[0] = "new-item"
	return keys
}

func mixedBenchmarkKeys(size int) []string {
	keys := make([]string, size)
	for index := range keys {
		if index%2 == 0 {
			keys[index] = "item-" + ToString(index)
		}
	}
	return keys
}

func mixedReorderedBenchmarkKeys(size int) []string {
	keys := make([]string, size)
	for index := range keys {
		if index%2 == 0 {
			keys[index] = "item-" + ToString((index+size-2)%size)
		}
	}
	return keys
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
		a, _ := UseState(1)
		b, _ := UseState(2)
		c, _ := UseState(3)
		d, _ := UseState(4)
		if a+b+c+d == 0 {
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
