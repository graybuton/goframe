package goframe

import (
	"reflect"
	"testing"
)

type currentPlacementChild struct {
	id      string
	pending bool
}

type currentPlacementStats struct {
	final    []string
	mounts   int
	removals int
	moves    int
}

func TestCurrentKeyedReorderPlacement(t *testing.T) {
	// These counts characterize current right-to-left placement behavior. They
	// are not asserted to be optimal; LIS work may intentionally update them.
	tests := []struct {
		name string
		old  []string
		new  []string
		want currentPlacementStats
	}{
		{
			name: "stable keyed order",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"a", "b", "c", "d"},
			want: currentPlacementStats{
				final: []string{"a", "b", "c", "d"},
			},
		},
		{
			name: "rotate left",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"b", "c", "d", "a"},
			want: currentPlacementStats{
				final: []string{"b", "c", "d", "a"},
				moves: 1,
			},
		},
		{
			name: "rotate right",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"d", "a", "b", "c"},
			want: currentPlacementStats{
				final: []string{"d", "a", "b", "c"},
				moves: 3,
			},
		},
		{
			name: "reverse",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"d", "c", "b", "a"},
			want: currentPlacementStats{
				final: []string{"d", "c", "b", "a"},
				moves: 3,
			},
		},
		{
			name: "insert beginning and end",
			old:  []string{"a", "b", "c"},
			new:  []string{"x", "a", "b", "c", "y"},
			want: currentPlacementStats{
				final:  []string{"x", "a", "b", "c", "y"},
				mounts: 2,
			},
		},
		{
			name: "remove middle",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"a", "c", "d"},
			want: currentPlacementStats{
				final:    []string{"a", "c", "d"},
				removals: 1,
			},
		},
		{
			name: "mixed insert remove reorder",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"c", "x", "a"},
			want: currentPlacementStats{
				final:    []string{"c", "x", "a"},
				mounts:   1,
				removals: 2,
				moves:    1,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := simulateCurrentPatchChildrenPlacement(t, test.old, test.new)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("placement stats = %#v, want %#v", got, test.want)
			}
		})
	}
}

func simulateCurrentPatchChildrenPlacement(t *testing.T, oldKeys, newKeys []string) currentPlacementStats {
	t.Helper()

	matches := matchChildIndices(oldKeys, newKeys)
	used := make([]bool, len(oldKeys))
	children := make([]currentPlacementChild, len(newKeys))
	for index, key := range newKeys {
		oldIndex := matches[index]
		if oldIndex == noChildMatch {
			children[index] = currentPlacementChild{id: key, pending: true}
			continue
		}
		used[oldIndex] = true
		children[index] = currentPlacementChild{id: oldKeys[oldIndex]}
	}

	stats := currentPlacementStats{
		final: make([]string, 0, len(newKeys)),
	}
	for index, key := range oldKeys {
		if used[index] {
			stats.final = append(stats.final, key)
			continue
		}
		stats.removals++
	}

	var reference string
	hasReference := false
	for index := len(children) - 1; index >= 0; index-- {
		child := children[index]
		if child.pending {
			stats.mounts++
			stats.final = insertBeforeCurrentPlacement(t, stats.final, child.id, reference, hasReference)
			reference = child.id
			hasReference = true
			continue
		}

		childIndex := indexOfCurrentPlacement(t, stats.final, child.id)
		hasNext := childIndex+1 < len(stats.final)
		nextIsReference := !hasNext && !hasReference
		if hasNext && hasReference && stats.final[childIndex+1] == reference {
			nextIsReference = true
		}
		if nextIsReference {
			reference = child.id
			hasReference = true
			continue
		}

		stats.moves++
		stats.final = append(stats.final[:childIndex], stats.final[childIndex+1:]...)
		stats.final = insertBeforeCurrentPlacement(t, stats.final, child.id, reference, hasReference)
		reference = child.id
		hasReference = true
	}

	return stats
}

func insertBeforeCurrentPlacement(t *testing.T, order []string, id, reference string, hasReference bool) []string {
	t.Helper()
	if !hasReference {
		return append(order, id)
	}
	index := indexOfCurrentPlacement(t, order, reference)
	order = append(order, "")
	copy(order[index+1:], order[index:])
	order[index] = id
	return order
}

func indexOfCurrentPlacement(t *testing.T, order []string, id string) int {
	t.Helper()
	for index, value := range order {
		if value == id {
			return index
		}
	}
	t.Fatalf("placement id %q missing from %v", id, order)
	return -1
}
