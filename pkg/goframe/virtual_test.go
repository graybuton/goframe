package goframe

import "testing"

func TestCalculateVirtualRangeEmpty(t *testing.T) {
	got := calculateVirtualRange(0, 100, 20, 2, 0)
	if got != (VirtualRange{}) {
		t.Fatalf("range = %#v, want empty", got)
	}
}

func TestCalculateVirtualRangeShortList(t *testing.T) {
	got := calculateVirtualRange(3, 200, 40, 4, 0)
	want := VirtualRange{Start: 0, End: 3, TopSpacer: 0, BottomSpacer: 0, TotalHeight: 120}
	if got != want {
		t.Fatalf("range = %#v, want %#v", got, want)
	}
}

func TestCalculateVirtualRangeTop(t *testing.T) {
	got := calculateVirtualRange(100, 100, 20, 2, 0)
	want := VirtualRange{Start: 0, End: 7, TopSpacer: 0, BottomSpacer: 1860, TotalHeight: 2000}
	if got != want {
		t.Fatalf("range = %#v, want %#v", got, want)
	}
}

func TestCalculateVirtualRangeMiddle(t *testing.T) {
	got := calculateVirtualRange(100, 100, 20, 2, 440)
	want := VirtualRange{Start: 20, End: 29, TopSpacer: 400, BottomSpacer: 1420, TotalHeight: 2000}
	if got != want {
		t.Fatalf("range = %#v, want %#v", got, want)
	}
}

func TestCalculateVirtualRangeBottom(t *testing.T) {
	got := calculateVirtualRange(100, 100, 20, 2, 5000)
	want := VirtualRange{Start: 97, End: 100, TopSpacer: 1940, BottomSpacer: 0, TotalHeight: 2000}
	if got != want {
		t.Fatalf("range = %#v, want %#v", got, want)
	}
}

func TestCalculateVirtualRangeOverscanClamps(t *testing.T) {
	got := calculateVirtualRange(10, 90, 30, 20, 120)
	want := VirtualRange{Start: 0, End: 10, TopSpacer: 0, BottomSpacer: 0, TotalHeight: 300}
	if got != want {
		t.Fatalf("range = %#v, want %#v", got, want)
	}
}

func TestCalculateVirtualRangeNegativeOverscan(t *testing.T) {
	got := calculateVirtualRange(10, 90, 30, -2, 120)
	want := VirtualRange{Start: 4, End: 7, TopSpacer: 120, BottomSpacer: 90, TotalHeight: 300}
	if got != want {
		t.Fatalf("range = %#v, want %#v", got, want)
	}
}

func TestCalculateVirtualRangeNegativeScroll(t *testing.T) {
	got := calculateVirtualRange(10, 90, 30, 1, -100)
	want := VirtualRange{Start: 0, End: 4, TopSpacer: 0, BottomSpacer: 180, TotalHeight: 300}
	if got != want {
		t.Fatalf("range = %#v, want %#v", got, want)
	}
}

func TestCalculateVirtualRangeRequiresPositiveDimensions(t *testing.T) {
	assertPanics(t, func() {
		calculateVirtualRange(10, 0, 30, 1, 0)
	})
	assertPanics(t, func() {
		calculateVirtualRange(10, 90, 0, 1, 0)
	})
}

func TestVirtualVisibleStartChangesOnlyOnRowBoundary(t *testing.T) {
	if got := virtualVisibleStart(100, 30, 29); got != 0 {
		t.Fatalf("visible start before row boundary = %d, want 0", got)
	}
	if got := virtualVisibleStart(100, 30, 30); got != 1 {
		t.Fatalf("visible start at row boundary = %d, want 1", got)
	}
}

func TestVirtualItemKey(t *testing.T) {
	if got := virtualItemKey[int](nil, 42, 7); got != "index-7" {
		t.Fatalf("fallback key = %q, want index-7", got)
	}
	got := virtualItemKey(func(item int, index int) string {
		return "id-" + ToString(item) + "-" + ToString(index)
	}, 42, 7)
	if got != "id-42-7" {
		t.Fatalf("stable key = %q", got)
	}
}

func TestVirtualListCreatesComponentBoundary(t *testing.T) {
	node, ok := VirtualList(VirtualListProps[int]{
		Items:      []int{1, 2, 3},
		Height:     100,
		ItemHeight: 20,
		RenderItem: func(item VirtualItem[int]) Node {
			return Text(ToString(item.Item))
		},
	}).(ComponentNode)
	if !ok {
		t.Fatal("VirtualList did not create component boundary")
	}
	if node.Name != "VirtualList" {
		t.Fatalf("component name = %q, want VirtualList", node.Name)
	}
}

func TestVirtualTableCreatesComponentBoundary(t *testing.T) {
	node, ok := VirtualTable(VirtualTableProps[int]{
		Items:     []int{1, 2, 3},
		Height:    100,
		RowHeight: 20,
		RenderRow: func(row VirtualRow[int]) Node {
			return El("tr", Props{}, El("td", Props{}, Text(ToString(row.Item))))
		},
	}).(ComponentNode)
	if !ok {
		t.Fatal("VirtualTable did not create component boundary")
	}
	if node.Name != "VirtualTable" {
		t.Fatalf("component name = %q, want VirtualTable", node.Name)
	}
}

func assertPanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}
