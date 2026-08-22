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

func TestVirtualRangeTransitionCarriesNormalizedStart(t *testing.T) {
	for _, transition := range []struct {
		name   string
		large  int
		shrink int
	}{
		{name: "100 to 2 to 100", large: 100, shrink: 2},
		{name: "100 to 0 to 100", large: 100, shrink: 0},
		{name: "1000 to 10 to 1000", large: 1000, shrink: 10},
	} {
		t.Run(transition.name, func(t *testing.T) {
			const height = 100
			const itemHeight = 20
			const overscan = 2
			distant := calculateVirtualRange(transition.large, height, itemHeight, overscan, transition.large*itemHeight)
			shrunk := calculateVirtualRangeFromStart(transition.shrink, height, itemHeight, overscan, distant.Start)
			restored := calculateVirtualRangeFromStart(transition.large, height, itemHeight, overscan, shrunk.Start)
			if restored.Start != shrunk.Start {
				t.Fatalf("restored start = %d, want committed shrink start %d", restored.Start, shrunk.Start)
			}
		})
	}
}

func TestVirtualComponentsCommitNormalizedRangeAcrossCollectionShrinkGrow(t *testing.T) {
	for _, primitive := range []string{"list", "table"} {
		for _, transition := range []struct {
			name   string
			large  int
			shrink int
		}{
			{name: "100 to 2 to 100", large: 100, shrink: 2},
			{name: "100 to 0 to 100", large: 100, shrink: 0},
			{name: "1000 to 10 to 1000", large: 1000, shrink: 10},
		} {
			t.Run(primitive+"/"+transition.name, func(t *testing.T) {
				harness := newVirtualRangeHarness(primitive, transition.large)
				initial := harness.render()
				harness.scroll(initial, transition.large*harness.itemHeight)
				distant := harness.committedStart(t)
				if distant == 0 {
					t.Fatal("distant scroll did not commit a non-zero range")
				}

				harness.length = transition.shrink
				harness.render()
				want := calculateVirtualRangeFromStart(
					transition.shrink,
					harness.height,
					harness.itemHeight,
					harness.overscan,
					distant,
				).Start
				if transition.shrink > 0 && harness.firstRendered(t) != want {
					t.Fatalf("shrink rendered start = %d, want %d", harness.firstRendered(t), want)
				}
				if transition.shrink == 0 && len(harness.rendered) != 0 {
					t.Fatalf("empty shrink rendered indices %v, want none", harness.rendered)
				}

				harness.length = transition.large
				harness.render()
				if got := harness.firstRendered(t); got != want {
					t.Fatalf("grow restored start = %d, want committed shrink start %d", got, want)
				}
				if got := harness.committedStart(t); got != want {
					t.Fatalf("committed start after grow = %d, want %d", got, want)
				}
			})
		}
	}
}

func TestVirtualComponentsCommitNormalizedRangeAcrossWindowChanges(t *testing.T) {
	for _, primitive := range []string{"list", "table"} {
		for _, change := range []struct {
			name       string
			height     int
			itemHeight int
			overscan   int
		}{
			{name: "height", height: 1000, itemHeight: 20, overscan: 0},
			{name: "item height", height: 100, itemHeight: 5, overscan: 0},
			{name: "overscan", height: 100, itemHeight: 20, overscan: 20},
		} {
			t.Run(primitive+"/"+change.name, func(t *testing.T) {
				harness := newVirtualRangeHarness(primitive, 100)
				harness.overscan = 0
				initial := harness.render()
				harness.scroll(initial, 100*harness.itemHeight)
				distant := harness.committedStart(t)

				originalHeight := harness.height
				originalItemHeight := harness.itemHeight
				originalOverscan := harness.overscan
				harness.height = change.height
				harness.itemHeight = change.itemHeight
				harness.overscan = change.overscan
				harness.render()
				want := calculateVirtualRangeFromStart(
					harness.length,
					harness.height,
					harness.itemHeight,
					harness.overscan,
					distant,
				).Start

				harness.height = originalHeight
				harness.itemHeight = originalItemHeight
				harness.overscan = originalOverscan
				harness.render()
				if got := harness.firstRendered(t); got != want {
					t.Fatalf("restored window start = %d, want committed expanded-window start %d", got, want)
				}
			})
		}
	}
}

func TestVirtualComponentsTreatNegativeOverscanAsZero(t *testing.T) {
	for _, primitive := range []string{"list", "table"} {
		t.Run(primitive, func(t *testing.T) {
			harness := newVirtualRangeHarness(primitive, 100)
			harness.overscan = 0
			initial := harness.render()
			harness.scroll(initial, 40*harness.itemHeight)
			wantStart := harness.committedStart(t)
			wantRendered := append([]int(nil), harness.rendered...)

			harness.overscan = -7
			harness.render()
			if got := harness.committedStart(t); got != wantStart {
				t.Fatalf("negative overscan committed start = %d, want zero-equivalent %d", got, wantStart)
			}
			if len(harness.rendered) != len(wantRendered) {
				t.Fatalf("negative overscan rendered %d entries, want %d", len(harness.rendered), len(wantRendered))
			}
			for index := range wantRendered {
				if harness.rendered[index] != wantRendered[index] {
					t.Fatalf("negative overscan rendered indices = %v, want %v", harness.rendered, wantRendered)
				}
			}
		})
	}
}

func TestVirtualComponentsCoveredScrollRetainsNormalizedStart(t *testing.T) {
	for _, primitive := range []string{"list", "table"} {
		t.Run(primitive, func(t *testing.T) {
			harness := newVirtualRangeHarness(primitive, 100)
			initial := harness.render()
			harness.scroll(initial, 100*harness.itemHeight)
			harness.length = 2
			shrunk := harness.render()
			beforeSchedules := harness.schedules

			harness.scrollWithoutRender(shrunk, 0)
			if harness.instance.dirty || harness.schedules != beforeSchedules {
				t.Fatalf("covered scroll dirty=%v schedules=%d, want clean/%d",
					harness.instance.dirty, harness.schedules, beforeSchedules)
			}
			if got := harness.committedStart(t); got != 0 {
				t.Fatalf("covered scroll committed start = %d, want 0", got)
			}
		})
	}
}

func TestVirtualComponentsRollbackRangeNormalizationAfterFailedRender(t *testing.T) {
	for _, primitive := range []string{"list", "table"} {
		t.Run(primitive, func(t *testing.T) {
			errorsSeen := captureRuntimeErrors(t)
			harness := newVirtualRangeHarness(primitive, 100)
			initial := harness.render()
			harness.scroll(initial, 100*harness.itemHeight)
			distant := harness.committedStart(t)

			harness.length = 2
			harness.failKey = true
			if rendered := harness.render(); rendered != (EmptyNode{}) {
				t.Fatalf("failed render result = %#v, want EmptyNode", rendered)
			}
			if got := harness.committedStart(t); got != distant {
				t.Fatalf("failed render committed start = %d, want previous %d", got, distant)
			}

			harness.failKey = false
			harness.render()
			if got := harness.committedStart(t); got != 0 {
				t.Fatalf("successful retry committed start = %d, want 0", got)
			}
			requireRuntimeError(t, errorsSeen(), ErrorPhaseRender, "Virtual"+titleCaseVirtualPrimitive(primitive), "component render", "virtual range render failure")
		})
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
		want  int
	}{
		{value: 0, want: 1},
		{value: -1, want: 1},
		{value: 7, want: 7},
	}
	for _, test := range tests {
		if got := virtualTableColumnCount(test.value); got != test.want {
			t.Fatalf("column count %d = %d, want %d", test.value, got, test.want)
		}
	}
}

func TestVirtualTableSpacerRowUsesColumnCount(t *testing.T) {
	row := virtualTableSpacerRow("top", 48, 7).(VNode)
	cell := row.Children[0].(VNode)
	if got := cell.Props["colspan"]; got != 7 {
		t.Fatalf("spacer colspan = %#v, want 7", got)
	}
}

func TestVirtualTableContentRowUsesColumnCount(t *testing.T) {
	row := virtualTableContentRow(Text("empty"), 7).(VNode)
	cell := row.Children[0].(VNode)
	if got := cell.Props["colspan"]; got != 7 {
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
	if got := cell.Props["colspan"]; got != 7 {
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

type virtualRangeHarness struct {
	primitive  string
	length     int
	height     int
	itemHeight int
	overscan   int
	failKey    bool
	rendered   []int
	schedules  int
	instance   *componentInstance
}

func newVirtualRangeHarness(primitive string, length int) *virtualRangeHarness {
	return &virtualRangeHarness{
		primitive:  primitive,
		length:     length,
		height:     100,
		itemHeight: 20,
		overscan:   2,
	}
}

func (harness *virtualRangeHarness) render() Node {
	harness.rendered = harness.rendered[:0]
	node := harness.componentNode()
	if harness.instance == nil {
		harness.instance = newComponentInstance(node, "", nil, func(*componentInstance) {
			harness.schedules++
		})
	} else {
		harness.instance.node = node
	}
	return renderComponentInstance(harness.instance)
}

func (harness *virtualRangeHarness) componentNode() ComponentNode {
	items := make([]int, harness.length)
	for index := range items {
		items[index] = index
	}
	key := func(item int, _ int) string {
		if harness.failKey {
			panic("virtual range render failure")
		}
		return ToString(item)
	}
	if harness.primitive == "list" {
		return VirtualList(VirtualListProps[int]{
			Items:      items,
			Height:     harness.height,
			ItemHeight: harness.itemHeight,
			Overscan:   harness.overscan,
			Key:        key,
			RenderItem: func(item VirtualItem[int]) Node {
				harness.rendered = append(harness.rendered, item.Index)
				return Text(ToString(item.Item))
			},
		}).(ComponentNode)
	}
	return VirtualTable(VirtualTableProps[int]{
		Items:       items,
		Height:      harness.height,
		RowHeight:   harness.itemHeight,
		Overscan:    harness.overscan,
		ColumnCount: 1,
		Key:         key,
		RenderRow: func(row VirtualRow[int]) Node {
			harness.rendered = append(harness.rendered, row.Index)
			return El("tr", Props{"style": row.RowStyle}, El("td", Props{}, Text(ToString(row.Item))))
		},
	}).(ComponentNode)
}

func (harness *virtualRangeHarness) scroll(rendered Node, scrollTop int) Node {
	harness.scrollWithoutRender(rendered, scrollTop)
	return harness.render()
}

func (harness *virtualRangeHarness) scrollWithoutRender(rendered Node, scrollTop int) {
	outer := rendered.(VNode)
	handler := outer.Props["OnScroll"].(func(ScrollEvent))
	handler(ScrollEvent{scrollTop: func() int { return scrollTop }})
}

func (harness *virtualRangeHarness) committedStart(t *testing.T) int {
	t.Helper()
	if harness.instance == nil || len(harness.instance.stateSlots) != 1 {
		t.Fatalf("state slots = %#v, want one committed range slot", harness.instance)
	}
	start, ok := harness.instance.stateSlots[0].value.(int)
	if !ok {
		t.Fatalf("range slot value = %#v, want int", harness.instance.stateSlots[0].value)
	}
	return start
}

func (harness *virtualRangeHarness) firstRendered(t *testing.T) int {
	t.Helper()
	if len(harness.rendered) == 0 {
		t.Fatal("virtual component rendered no items")
	}
	return harness.rendered[0]
}

func titleCaseVirtualPrimitive(primitive string) string {
	if primitive == "list" {
		return "List"
	}
	return "Table"
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
