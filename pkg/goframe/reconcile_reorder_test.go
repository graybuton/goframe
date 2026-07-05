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
	// These counts characterize current stable-placement-aware right-to-left
	// behavior. They are not asserted to be theoretically optimal; future
	// placement work may intentionally update them.
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
				moves: 1,
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
	stableStart := stableChildPlacementStart(matches)
	for index := len(children) - 1; index >= 0; index-- {
		child := children[index]
		if child.pending {
			stats.mounts++
			stats.final = insertBeforeCurrentPlacement(t, stats.final, child.id, reference, hasReference)
			reference = child.id
			hasReference = true
			continue
		}
		if index >= stableStart {
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

func TestStableChildPlacements(t *testing.T) {
	tests := []struct {
		name    string
		matches []int
		want    []bool
	}{
		{
			name:    "empty",
			matches: nil,
		},
		{
			name:    "all mounts",
			matches: []int{noChildMatch, noChildMatch},
		},
		{
			name:    "stable",
			matches: []int{0, 1, 2, 3},
		},
		{
			name:    "insert around stable",
			matches: []int{noChildMatch, 0, 1, 2, noChildMatch},
		},
		{
			name:    "rotate left",
			matches: []int{1, 2, 3, 0},
		},
		{
			name:    "rotate right",
			matches: []int{3, 0, 1, 2},
			want:    []bool{false, true, true, true},
		},
		{
			name:    "reverse",
			matches: []int{3, 2, 1, 0},
		},
		{
			name:    "mixed",
			matches: []int{2, noChildMatch, 0},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := stableChildPlacementFlags(test.matches)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("stable placements = %v, want %v", got, test.want)
			}
			for index, stable := range got {
				if stable && test.matches[index] == noChildMatch {
					t.Fatalf("noChildMatch at %d marked stable", index)
				}
			}
		})
	}
}

func stableChildPlacementFlags(matches []int) []bool {
	stableStart := stableChildPlacementStart(matches)
	if stableStart == len(matches) {
		return nil
	}
	flags := make([]bool, len(matches))
	for index := stableStart; index < len(matches); index++ {
		if matches[index] != noChildMatch {
			flags[index] = true
		}
	}
	return flags
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
