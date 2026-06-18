package goframe

// VirtualItem is passed to VirtualList item renderers.
type VirtualItem[T any] struct {
	Item  T
	Index int
	Key   string
	Style string
}

// VirtualListProps configures a fixed-height virtualized list.
type VirtualListProps[T any] struct {
	Items      []T
	Height     int
	ItemHeight int
	Overscan   int

	Key        func(item T, index int) string
	RenderItem func(item VirtualItem[T]) Node

	Class  string
	TestID string
}

// VirtualList renders only the visible window plus overscan for a fixed-height
// list. Dynamic item measurement is intentionally out of scope.
func VirtualList[T any](props VirtualListProps[T]) Node {
	return Component("VirtualList", props, renderVirtualList[T])
}

// VirtualRow is passed to VirtualTable row renderers.
type VirtualRow[T any] struct {
	Item     T
	Index    int
	Key      string
	RowStyle string
}

// VirtualTableProps configures a fixed-row-height virtualized table.
type VirtualTableProps[T any] struct {
	Items     []T
	Height    int
	RowHeight int
	Overscan  int

	Key       func(item T, index int) string
	Header    func() Node
	RenderRow func(row VirtualRow[T]) Node
	Empty     func() Node

	Class  string
	TestID string
}

// VirtualTable renders a scrollable fixed-row-height table while keeping the
// mounted row count bounded by the visible window plus overscan.
func VirtualTable[T any](props VirtualTableProps[T]) Node {
	return Component("VirtualTable", props, renderVirtualTable[T])
}

type VirtualRange struct {
	Start        int
	End          int
	TopSpacer    int
	BottomSpacer int
	TotalHeight  int
}

func renderVirtualList[T any](props VirtualListProps[T]) Node {
	validateVirtualDimensions("VirtualList", props.Height, props.ItemHeight, "ItemHeight")
	if props.RenderItem == nil {
		panic("goframe: VirtualList requires RenderItem")
	}
	visibleStart, setVisibleStart := UseState(0)
	rangeInfo := calculateVirtualRangeFromStart(len(props.Items), props.Height, props.ItemHeight, props.Overscan, visibleStart)

	children := make([]Node, 0, rangeInfo.End-rangeInfo.Start)
	for index := rangeInfo.Start; index < rangeInfo.End; index++ {
		key := virtualItemKey(props.Key, props.Items[index], index)
		top := index * props.ItemHeight
		style := "position:absolute;top:" + ToString(top) + "px;height:" + ToString(props.ItemHeight) + "px;width:100%;"
		item := VirtualItem[T]{
			Item:  props.Items[index],
			Index: index,
			Key:   key,
			Style: style,
		}
		children = append(children, Key(key, El("div", Props{
			"class": "gf-virtual-item",
			"style": style,
		}, props.RenderItem(item))))
	}

	outerProps := Props{
		"class": joinVirtualClass("gf-virtual-list", props.Class),
		"style": virtualViewportStyle(props.Height),
		"OnScroll": func(event ScrollEvent) {
			next := virtualVisibleStart(len(props.Items), props.ItemHeight, event.ScrollTop())
			if next != visibleStart {
				setVisibleStart(next)
			}
		},
	}
	if props.TestID != "" {
		outerProps["data-testid"] = props.TestID
	}

	return El("div", outerProps,
		El("div", Props{
			"class": "gf-virtual-list-spacer",
			"style": "height:" + ToString(rangeInfo.TotalHeight) + "px;position:relative;",
		}, children...),
	)
}

func renderVirtualTable[T any](props VirtualTableProps[T]) Node {
	validateVirtualDimensions("VirtualTable", props.Height, props.RowHeight, "RowHeight")
	if props.RenderRow == nil {
		panic("goframe: VirtualTable requires RenderRow")
	}
	visibleStart, setVisibleStart := UseState(0)
	rangeInfo := calculateVirtualRangeFromStart(len(props.Items), props.Height, props.RowHeight, props.Overscan, visibleStart)

	bodyChildren := make([]Node, 0, rangeInfo.End-rangeInfo.Start+2)
	if len(props.Items) == 0 {
		if props.Empty != nil {
			bodyChildren = append(bodyChildren, virtualTableContentRow(props.Empty()))
		}
	} else {
		if rangeInfo.TopSpacer > 0 {
			bodyChildren = append(bodyChildren, virtualTableSpacerRow("top", rangeInfo.TopSpacer))
		}
		for index := rangeInfo.Start; index < rangeInfo.End; index++ {
			key := virtualItemKey(props.Key, props.Items[index], index)
			row := VirtualRow[T]{
				Item:     props.Items[index],
				Index:    index,
				Key:      key,
				RowStyle: "height:" + ToString(props.RowHeight) + "px;",
			}
			bodyChildren = append(bodyChildren, Key(key, props.RenderRow(row)))
		}
		if rangeInfo.BottomSpacer > 0 {
			bodyChildren = append(bodyChildren, virtualTableSpacerRow("bottom", rangeInfo.BottomSpacer))
		}
	}

	tableChildren := make([]Node, 0, 2)
	if props.Header != nil {
		tableChildren = append(tableChildren, props.Header())
	}
	tableChildren = append(tableChildren, El("tbody", Props{}, bodyChildren...))

	outerProps := Props{
		"class": "gf-virtual-table-viewport",
		"style": virtualViewportStyle(props.Height),
		"OnScroll": func(event ScrollEvent) {
			next := virtualVisibleStart(len(props.Items), props.RowHeight, event.ScrollTop())
			if next != visibleStart {
				setVisibleStart(next)
			}
		},
	}
	if props.TestID != "" {
		outerProps["data-testid"] = props.TestID
	}

	return El("div", outerProps,
		El("table", Props{
			"class": joinVirtualClass("gf-virtual-table", props.Class),
		}, tableChildren...),
	)
}

func validateVirtualDimensions(name string, height int, itemHeight int, itemHeightName string) {
	if height <= 0 || itemHeight <= 0 {
		panic("goframe: " + name + " requires positive Height and " + itemHeightName)
	}
}

func calculateVirtualRange(length int, height int, itemHeight int, overscan int, scrollTop int) VirtualRange {
	return calculateVirtualRangeFromStart(length, height, itemHeight, overscan, virtualVisibleStart(length, itemHeight, scrollTop))
}

func calculateVirtualRangeFromStart(length int, height int, itemHeight int, overscan int, visibleStart int) VirtualRange {
	validateVirtualDimensions("virtual range", height, itemHeight, "ItemHeight")
	if length <= 0 {
		return VirtualRange{}
	}
	if overscan < 0 {
		overscan = 0
	}
	visibleStart = clampInt(visibleStart, 0, length-1)
	visibleCount := ceilDiv(height, itemHeight)
	if visibleCount < 1 {
		visibleCount = 1
	}

	start := visibleStart - overscan
	if start < 0 {
		start = 0
	}
	end := visibleStart + visibleCount + overscan
	if end > length {
		end = length
	}
	if end < start {
		end = start
	}

	return VirtualRange{
		Start:        start,
		End:          end,
		TopSpacer:    start * itemHeight,
		BottomSpacer: (length - end) * itemHeight,
		TotalHeight:  length * itemHeight,
	}
}

func virtualVisibleStart(length int, itemHeight int, scrollTop int) int {
	if length <= 0 || itemHeight <= 0 {
		return 0
	}
	if scrollTop < 0 {
		scrollTop = 0
	}
	return clampInt(scrollTop/itemHeight, 0, length-1)
}

func virtualItemKey[T any](key func(T, int) string, item T, index int) string {
	if key != nil {
		return key(item, index)
	}
	return "index-" + ToString(index)
}

func virtualViewportStyle(height int) string {
	return "height:" + ToString(height) + "px;overflow-y:auto;position:relative;"
}

func virtualTableSpacerRow(name string, height int) Node {
	return El("tr", Props{
		"class":       "gf-virtual-table-spacer gf-virtual-table-spacer-" + name,
		"aria-hidden": "true",
		"style":       "height:" + ToString(height) + "px;",
	}, El("td", Props{
		"colspan": "999",
		"style":   "height:" + ToString(height) + "px;padding:0;border:0;",
	}))
}

func virtualTableContentRow(content Node) Node {
	return El("tr", Props{
		"class": "gf-virtual-table-content",
	}, El("td", Props{
		"colspan": "999",
	}, content))
}

func joinVirtualClass(base string, extra string) string {
	if extra == "" {
		return base
	}
	return base + " " + extra
}

func ceilDiv(value int, divisor int) int {
	return (value + divisor - 1) / divisor
}

func clampInt(value int, low int, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
