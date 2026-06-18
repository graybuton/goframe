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
	want := VirtualRange{Start: 0, End: 9, TopSpacer: 0, BottomSpacer: 1820, TotalHeight: 2000}
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
	want := VirtualRange{Start: 91, End: 100, TopSpacer: 1820, BottomSpacer: 0, TotalHeight: 2000}
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
	want := VirtualRange{Start: 0, End: 5, TopSpacer: 0, BottomSpacer: 150, TotalHeight: 300}
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

func TestVirtualVisibleCountUsesCeilDivision(t *testing.T) {
	if got := virtualVisibleCount(100, 20); got != 5 {
		t.Fatalf("exact visible count = %d, want 5", got)
	}
	if got := virtualVisibleCount(101, 20); got != 6 {
		t.Fatalf("ceil visible count = %d, want 6", got)
	}
}

func TestVirtualRangeCoversVisibleInsideBuffer(t *testing.T) {
	rangeInfo := VirtualRange{Start: 10, End: 19}
	if !virtualRangeCoversVisible(rangeInfo, 12, 5) {
		t.Fatalf("range %#v should cover visible start 12 count 5", rangeInfo)
	}
}

func TestVirtualRangeCoversVisibleOutsideBuffer(t *testing.T) {
	rangeInfo := VirtualRange{Start: 10, End: 19}
	if virtualRangeCoversVisible(rangeInfo, 15, 5) {
		t.Fatalf("range %#v should not cover visible start 15 count 5", rangeInfo)
	}
}

func TestVirtualRangeStartForVisibleStartRecentersWithOverscan(t *testing.T) {
	if got := virtualRangeStartForVisibleStart(100, 100, 20, 2, 22); got != 20 {
		t.Fatalf("middle range start = %d, want 20", got)
	}
	if got := virtualRangeStartForVisibleStart(100, 100, 20, 2, 1); got != 0 {
		t.Fatalf("top range start = %d, want 0", got)
	}
	if got := virtualRangeStartForVisibleStart(100, 100, 20, 2, 99); got != 91 {
		t.Fatalf("bottom range start = %d, want 91", got)
	}
}

func TestVirtualRangeStartAfterScrollInsideBufferKeepsRangeStart(t *testing.T) {
	rangeInfo := calculateVirtualRangeFromStart(100, 100, 20, 2, 0)
	got := virtualRangeStartAfterScroll(rangeInfo, 0, 100, 100, 20, 2, 80)
	if got != 0 {
		t.Fatalf("range start after covered scroll = %d, want 0", got)
	}
}

func TestVirtualRangeStartAfterScrollBeyondBufferUpdatesRangeStart(t *testing.T) {
	rangeInfo := calculateVirtualRangeFromStart(100, 100, 20, 2, 0)
	got := virtualRangeStartAfterScroll(rangeInfo, 0, 100, 100, 20, 2, 100)
	if got != 3 {
		t.Fatalf("range start after uncovered scroll = %d, want 3", got)
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

func TestVirtualTableColumnCount(t *testing.T) {
	tests := []struct {
		value int
		want  string
	}{
		{value: 0, want: "1"},
		{value: -1, want: "1"},
		{value: 7, want: "7"},
	}
	for _, test := range tests {
		if got := virtualTableColumnCount(test.value); got != test.want {
			t.Fatalf("column count %d = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestVirtualTableSpacerRowUsesColumnCount(t *testing.T) {
	row := virtualTableSpacerRow("top", 48, 7).(VNode)
	cell := row.Children[0].(VNode)
	if got := cell.Props["colspan"]; got != "7" {
		t.Fatalf("spacer colspan = %#v, want 7", got)
	}
}

func TestVirtualTableContentRowUsesColumnCount(t *testing.T) {
	row := virtualTableContentRow(Text("empty"), 7).(VNode)
	cell := row.Children[0].(VNode)
	if got := cell.Props["colspan"]; got != "7" {
		t.Fatalf("content colspan = %#v, want 7", got)
	}
}

func TestVirtualTableUsesStableSpacerAndNamespacedRowKeys(t *testing.T) {
	userKeys := []string{}
	children := renderVirtualTableBodyChildrenForTest(VirtualTableProps[int]{
		Items:       []int{1, 2, 3},
		Height:      40,
		RowHeight:   20,
		Overscan:    0,
		ColumnCount: 7,
		Key: func(item int, index int) string {
			if item == 1 {
				return virtualTableTopSpacerKey
			}
			return "item-" + ToString(item)
		},
		RenderRow: func(row VirtualRow[int]) Node {
			userKeys = append(userKeys, row.Key)
			return Key(row.Key, El("tr", Props{"style": row.RowStyle}, El("td", Props{}, Text(ToString(row.Item)))))
		},
	})

	if len(children) != 4 {
		t.Fatalf("tbody child count = %d, want 4", len(children))
	}
	top := requireKeyedNode(t, children[0])
	firstRow := requireKeyedNode(t, children[1])
	secondRow := requireKeyedNode(t, children[2])
	bottom := requireKeyedNode(t, children[3])

	if top.Key != virtualTableTopSpacerKey {
		t.Fatalf("top spacer key = %q, want %q", top.Key, virtualTableTopSpacerKey)
	}
	if bottom.Key != virtualTableBottomSpacerKey {
		t.Fatalf("bottom spacer key = %q, want %q", bottom.Key, virtualTableBottomSpacerKey)
	}
	if firstRow.Key != virtualTableRowKeyPrefix+virtualTableTopSpacerKey {
		t.Fatalf("first row internal key = %q, want namespaced user key", firstRow.Key)
	}
	if secondRow.Key != virtualTableRowKeyPrefix+"item-2" {
		t.Fatalf("second row internal key = %q, want namespaced item key", secondRow.Key)
	}
	if firstRow.Key == top.Key || firstRow.Key == bottom.Key {
		t.Fatalf("row internal key %q collided with spacer keys", firstRow.Key)
	}
	if len(userKeys) != 2 || userKeys[0] != virtualTableTopSpacerKey || userKeys[1] != "item-2" {
		t.Fatalf("user-facing row keys = %#v, want original keys", userKeys)
	}
}

func TestVirtualTableKeepsZeroHeightSpacersMounted(t *testing.T) {
	children := renderVirtualTableBodyChildrenForTest(VirtualTableProps[int]{
		Items:       []int{1, 2},
		Height:      100,
		RowHeight:   20,
		Overscan:    0,
		ColumnCount: 7,
		RenderRow: func(row VirtualRow[int]) Node {
			return El("tr", Props{"style": row.RowStyle}, El("td", Props{}, Text(ToString(row.Item))))
		},
	})

	if len(children) != 4 {
		t.Fatalf("tbody child count = %d, want top spacer, 2 rows, bottom spacer", len(children))
	}
	top := requireVNode(t, requireKeyedNode(t, children[0]).Node)
	bottom := requireVNode(t, requireKeyedNode(t, children[3]).Node)
	if got := top.Props["style"]; got != "height:0px;overflow-anchor:none;" {
		t.Fatalf("top spacer style = %#v, want height:0px;overflow-anchor:none;", got)
	}
	if got := bottom.Props["style"]; got != "height:0px;overflow-anchor:none;" {
		t.Fatalf("bottom spacer style = %#v, want height:0px;overflow-anchor:none;", got)
	}
	topCell := requireVNode(t, top.Children[0])
	if got := topCell.Props["style"]; got != "height:0px;padding:0;border:0;line-height:0;font-size:0;overflow-anchor:none;" {
		t.Fatalf("top spacer cell style = %#v, want zero-height style", got)
	}
}

func TestVirtualTableKeysEmptyState(t *testing.T) {
	children := renderVirtualTableBodyChildrenForTest(VirtualTableProps[int]{
		Items:       nil,
		Height:      100,
		RowHeight:   20,
		ColumnCount: 7,
		RenderRow: func(row VirtualRow[int]) Node {
			return El("tr", Props{}, El("td", Props{}, Text(ToString(row.Item))))
		},
		Empty: func() Node {
			return Text("empty")
		},
	})

	if len(children) != 1 {
		t.Fatalf("empty tbody child count = %d, want 1", len(children))
	}
	empty := requireKeyedNode(t, children[0])
	if empty.Key != virtualTableEmptyKey {
		t.Fatalf("empty key = %q, want %q", empty.Key, virtualTableEmptyKey)
	}
	row := requireVNode(t, empty.Node)
	cell := requireVNode(t, row.Children[0])
	if got := cell.Props["colspan"]; got != "7" {
		t.Fatalf("empty colspan = %#v, want 7", got)
	}
}

func renderVirtualTableBodyChildrenForTest[T any](props VirtualTableProps[T]) []Node {
	node := VirtualTable(props).(ComponentNode)
	instance := newComponentInstance(node, "", nil, nil)
	outer := renderComponentInstance(instance).(VNode)
	table := outer.Children[0].(VNode)
	tbody := table.Children[len(table.Children)-1].(VNode)
	return tbody.Children
}

func requireKeyedNode(t *testing.T, node Node) KeyedNode {
	t.Helper()
	keyed, ok := node.(KeyedNode)
	if !ok {
		t.Fatalf("node = %#v, want KeyedNode", node)
	}
	return keyed
}

func requireVNode(t *testing.T, node Node) VNode {
	t.Helper()
	vnode, ok := node.(VNode)
	if !ok {
		t.Fatalf("node = %#v, want VNode", node)
	}
	return vnode
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
