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

func TestKeyedReorderPlacement(t *testing.T) {
	// Before counts characterize the previous stable-suffix placement behavior.
	// After counts assert the current LIS-aware keyed placement behavior.
	tests := []struct {
		name   string
		old    []string
		new    []string
		before currentPlacementStats
		after  currentPlacementStats
	}{
		{
			name: "stable keyed order",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"a", "b", "c", "d"},
			before: currentPlacementStats{
				final: []string{"a", "b", "c", "d"},
			},
			after: currentPlacementStats{
				final: []string{"a", "b", "c", "d"},
			},
		},
		{
			name: "rotate left",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"b", "c", "d", "a"},
			before: currentPlacementStats{
				final: []string{"b", "c", "d", "a"},
				moves: 1,
			},
			after: currentPlacementStats{
				final: []string{"b", "c", "d", "a"},
				moves: 1,
			},
		},
		{
			name: "rotate right",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"d", "a", "b", "c"},
			before: currentPlacementStats{
				final: []string{"d", "a", "b", "c"},
				moves: 1,
			},
			after: currentPlacementStats{
				final: []string{"d", "a", "b", "c"},
				moves: 1,
			},
		},
		{
			name: "reverse",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"d", "c", "b", "a"},
			before: currentPlacementStats{
				final: []string{"d", "c", "b", "a"},
				moves: 3,
			},
			after: currentPlacementStats{
				final: []string{"d", "c", "b", "a"},
				moves: 3,
			},
		},
		{
			name: "move middle item forward",
			old:  []string{"a", "b", "c", "d", "e"},
			new:  []string{"a", "d", "b", "c", "e"},
			before: currentPlacementStats{
				final: []string{"a", "d", "b", "c", "e"},
				moves: 1,
			},
			after: currentPlacementStats{
				final: []string{"a", "d", "b", "c", "e"},
				moves: 1,
			},
		},
		{
			name: "move middle item backward",
			old:  []string{"a", "b", "c", "d", "e"},
			new:  []string{"a", "c", "d", "b", "e"},
			before: currentPlacementStats{
				final: []string{"a", "c", "d", "b", "e"},
				moves: 2,
			},
			after: currentPlacementStats{
				final: []string{"a", "c", "d", "b", "e"},
				moves: 1,
			},
		},
		{
			name: "insert beginning and end",
			old:  []string{"a", "b", "c"},
			new:  []string{"x", "a", "b", "c", "y"},
			before: currentPlacementStats{
				final:  []string{"new-0:x", "a", "b", "c", "new-4:y"},
				mounts: 2,
			},
			after: currentPlacementStats{
				final:  []string{"new-0:x", "a", "b", "c", "new-4:y"},
				mounts: 2,
			},
		},
		{
			name: "remove middle",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"a", "c", "d"},
			before: currentPlacementStats{
				final:    []string{"a", "c", "d"},
				removals: 1,
			},
			after: currentPlacementStats{
				final:    []string{"a", "c", "d"},
				removals: 1,
			},
		},
		{
			name: "mixed keyed and unkeyed reorder",
			old:  []string{"a", "", "b", ""},
			new:  []string{"b", "", "a", ""},
			before: currentPlacementStats{
				final: []string{"b", "old-1", "a", "old-3"},
				moves: 2,
			},
			after: currentPlacementStats{
				final: []string{"b", "old-1", "a", "old-3"},
				moves: 2,
			},
		},
		{
			name: "duplicate new key mounts duplicate",
			old:  []string{"a", "b"},
			new:  []string{"a", "a", "b"},
			before: currentPlacementStats{
				final:  []string{"a", "new-1:a", "b"},
				mounts: 1,
			},
			after: currentPlacementStats{
				final:  []string{"a", "new-1:a", "b"},
				mounts: 1,
			},
		},
		{
			name: "mixed insert remove reorder",
			old:  []string{"a", "b", "c", "d"},
			new:  []string{"c", "x", "a"},
			before: currentPlacementStats{
				final:    []string{"c", "new-1:x", "a"},
				mounts:   1,
				removals: 2,
				moves:    1,
			},
			after: currentPlacementStats{
				final:    []string{"c", "new-1:x", "a"},
				mounts:   1,
				removals: 2,
				moves:    1,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := simulateCurrentPatchChildrenPlacement(t, test.old, test.new, stableSuffixChildPlacementFlags)
			if !reflect.DeepEqual(before, test.before) {
				t.Fatalf("previous placement stats = %#v, want %#v", before, test.before)
			}
			after := simulateCurrentPatchChildrenPlacement(t, test.old, test.new, stableChildPlacements)
			if !reflect.DeepEqual(after, test.after) {
				t.Fatalf("placement stats = %#v, want %#v", after, test.after)
			}
		})
	}
}

func simulateCurrentPatchChildrenPlacement(t *testing.T, oldKeys, newKeys []string, stable func([]int, []string) []bool) currentPlacementStats {
	t.Helper()

	oldIDs := make([]string, len(oldKeys))
	for index, key := range oldKeys {
		oldIDs[index] = currentPlacementID("old", index, key)
	}

	matches := matchChildIndices(oldKeys, newKeys)
	used := make([]bool, len(oldKeys))
	children := make([]currentPlacementChild, len(newKeys))
	for index, key := range newKeys {
		oldIndex := matches[index]
		if oldIndex == noChildMatch {
			children[index] = currentPlacementChild{id: currentPlacementID("new", index, key), pending: true}
			continue
		}
		used[oldIndex] = true
		children[index] = currentPlacementChild{id: oldIDs[oldIndex]}
	}

	stats := currentPlacementStats{
		final: make([]string, 0, len(newKeys)),
	}
	for index := range oldKeys {
		if used[index] {
			stats.final = append(stats.final, oldIDs[index])
			continue
		}
		stats.removals++
	}

	var reference string
	hasReference := false
	stablePlacements := stable(matches, newKeys)
	for index := len(children) - 1; index >= 0; index-- {
		child := children[index]
		if child.pending {
			stats.mounts++
			stats.final = insertBeforeCurrentPlacement(t, stats.final, child.id, reference, hasReference)
			reference = child.id
			hasReference = true
			continue
		}
		if stablePlacements != nil && stablePlacements[index] {
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

func currentPlacementID(prefix string, index int, key string) string {
	if key == "" {
		return prefix + "-" + ToString(index)
	}
	if prefix == "old" {
		return key
	}
	return prefix + "-" + ToString(index) + ":" + key
}

func TestStableChildPlacements(t *testing.T) {
	tests := []struct {
		name    string
		keys    []string
		matches []int
		want    []bool
	}{
		{
			name:    "empty",
			matches: nil,
		},
		{
			name:    "all mounts",
			keys:    []string{"a", "b"},
			matches: []int{noChildMatch, noChildMatch},
		},
		{
			name:    "stable",
			keys:    []string{"a", "b", "c", "d"},
			matches: []int{0, 1, 2, 3},
			want:    []bool{true, true, true, true},
		},
		{
			name:    "insert around stable",
			keys:    []string{"x", "a", "b", "c", "y"},
			matches: []int{noChildMatch, 0, 1, 2, noChildMatch},
			want:    []bool{false, true, true, true, false},
		},
		{
			name:    "rotate left",
			keys:    []string{"b", "c", "d", "a"},
			matches: []int{1, 2, 3, 0},
			want:    []bool{true, true, true, false},
		},
		{
			name:    "rotate right",
			keys:    []string{"d", "a", "b", "c"},
			matches: []int{3, 0, 1, 2},
			want:    []bool{false, true, true, true},
		},
		{
			name:    "reverse",
			keys:    []string{"d", "c", "b", "a"},
			matches: []int{3, 2, 1, 0},
			want:    []bool{true, false, false, false},
		},
		{
			name:    "mixed",
			keys:    []string{"c", "x", "a"},
			matches: []int{2, noChildMatch, 0},
			want:    []bool{true, false, false},
		},
		{
			name:    "unkeyed ignored",
			keys:    []string{"b", "", "a", ""},
			matches: []int{2, 1, 0, 3},
			want:    []bool{true, false, false, false},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := stableChildPlacements(test.matches, test.keys)
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

func stableSuffixChildPlacementFlags(matches []int, keys []string) []bool {
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
