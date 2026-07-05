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

	ColumnCount int

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

const (
	virtualTableTopSpacerKey    = "\x00vt"
	virtualTableBottomSpacerKey = "\x00vb"
	virtualTableEmptyKey        = "\x00ve"
	virtualTableRowKeyPrefix    = "\x00vr:"
)

func renderVirtualList[T any](props VirtualListProps[T]) Node {
	validateVirtualListDimensions(props.Height, props.ItemHeight)
	if props.RenderItem == nil {
		panic("goframe: VirtualList requires RenderItem")
	}
	rangeStart, setRangeStart := UseState(0)
	rangeInfo := calculateVirtualRangeFromStart(len(props.Items), props.Height, props.ItemHeight, props.Overscan, rangeStart)

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
		}, renderVirtualListItem(props.RenderItem, item))))
	}

	outerProps := Props{
		"class": joinVirtualClass("gf-virtual-list", props.Class),
		"style": virtualViewportStyle(props.Height),
		"OnScroll": func(event ScrollEvent) {
			next := virtualRangeStartAfterScroll(rangeInfo, rangeStart, len(props.Items), props.Height, props.ItemHeight, props.Overscan, event.ScrollTop())
			if next != rangeStart {
				setRangeStart(next)
			}
		},
	}
	if props.TestID != "" {
		outerProps["data-testid"] = props.TestID
	}

	return El("div", outerProps,
		El("div", Props{
			"class": "gf-virtual-list-spacer",
			"style": "height:" + ToString(rangeInfo.TotalHeight) + "px;position:relative;overflow-anchor:none;",
		}, children...),
	)
}

func renderVirtualTable[T any](props VirtualTableProps[T]) Node {
	validateVirtualTableDimensions(props.Height, props.RowHeight)
	if props.RenderRow == nil {
		panic("goframe: VirtualTable requires RenderRow")
	}
	rangeStart, setRangeStart := UseState(0)
	rangeInfo := calculateVirtualRangeFromStart(len(props.Items), props.Height, props.RowHeight, props.Overscan, rangeStart)

	bodyChildren := make([]Node, 0, rangeInfo.End-rangeInfo.Start+2)
	if len(props.Items) == 0 {
		if props.Empty != nil {
			bodyChildren = append(bodyChildren, Key(virtualTableEmptyKey, virtualTableContentRow(renderVirtualTableEmpty(props.Empty), props.ColumnCount)))
		}
	} else {
		bodyChildren = append(bodyChildren, Key(
			virtualTableTopSpacerKey,
			virtualTableSpacerRow("top", rangeInfo.TopSpacer, props.ColumnCount),
		))
		for index := rangeInfo.Start; index < rangeInfo.End; index++ {
			key := virtualItemKey(props.Key, props.Items[index], index)
			row := VirtualRow[T]{
				Item:     props.Items[index],
				Index:    index,
				Key:      key,
				RowStyle: "height:" + ToString(props.RowHeight) + "px;",
			}
			bodyChildren = append(bodyChildren, Key(virtualTableRowKeyPrefix+key, renderVirtualTableRow(props.RenderRow, row, props.ColumnCount)))
		}
		bodyChildren = append(bodyChildren, Key(
			virtualTableBottomSpacerKey,
			virtualTableSpacerRow("bottom", rangeInfo.BottomSpacer, props.ColumnCount),
		))
	}

	tableChildren := make([]Node, 0, 2)
	if props.Header != nil {
		tableChildren = append(tableChildren, renderVirtualTableHeader(props.Header))
	}
	tableChildren = append(tableChildren, El("tbody", Props{}, bodyChildren...))

	outerProps := Props{
		"class": "gf-virtual-table-viewport",
		"style": virtualViewportStyle(props.Height),
		"OnScroll": func(event ScrollEvent) {
			next := virtualRangeStartAfterScroll(rangeInfo, rangeStart, len(props.Items), props.Height, props.RowHeight, props.Overscan, event.ScrollTop())
			if next != rangeStart {
				setRangeStart(next)
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

func renderVirtualListItem[T any](render func(VirtualItem[T]) Node, item VirtualItem[T]) (node Node) {
	defer func() {
		if recovered := recover(); recovered != nil {
			reportRecoveredRuntimeError(ErrorInfo{
				Phase:     ErrorPhaseRender,
				Component: "VirtualList",
				Operation: "VirtualList.RenderItem",
			}, recovered)
			node = Empty()
		}
	}()
	return render(item)
}

func renderVirtualTableHeader(render func() Node) (node Node) {
	defer func() {
		if recovered := recover(); recovered != nil {
			reportRecoveredRuntimeError(ErrorInfo{
				Phase:     ErrorPhaseRender,
				Component: "VirtualTable",
				Operation: "VirtualTable.Header",
			}, recovered)
			node = Empty()
		}
	}()
	return render()
}

func renderVirtualTableRow[T any](render func(VirtualRow[T]) Node, row VirtualRow[T], columnCount int) (node Node) {
	defer func() {
		if recovered := recover(); recovered != nil {
			reportRecoveredRuntimeError(ErrorInfo{
				Phase:     ErrorPhaseRender,
				Component: "VirtualTable",
				Operation: "VirtualTable.RenderRow",
			}, recovered)
			node = virtualTableContentRow(Empty(), columnCount)
		}
	}()
	return render(row)
}

func renderVirtualTableEmpty(render func() Node) (node Node) {
	defer func() {
		if recovered := recover(); recovered != nil {
			reportRecoveredRuntimeError(ErrorInfo{
				Phase:     ErrorPhaseRender,
				Component: "VirtualTable",
				Operation: "VirtualTable.Empty",
			}, recovered)
			node = Empty()
		}
	}()
	return render()
}

func validateVirtualListDimensions(height int, itemHeight int) {
	if height <= 0 || itemHeight <= 0 {
		panic("goframe: VirtualList requires positive Height and ItemHeight")
	}
}

func validateVirtualTableDimensions(height int, rowHeight int) {
	if height <= 0 || rowHeight <= 0 {
		panic("goframe: VirtualTable requires positive Height and RowHeight")
	}
}

func validateVirtualRangeDimensions(height int, itemHeight int) {
	if height <= 0 || itemHeight <= 0 {
		panic("goframe: virtual range requires positive Height and ItemHeight")
	}
}

func calculateVirtualRange(length int, height int, itemHeight int, overscan int, scrollTop int) VirtualRange {
	validateVirtualRangeDimensions(height, itemHeight)
	visibleStart := virtualVisibleStart(length, itemHeight, scrollTop)
	rangeStart := virtualRangeStartForVisibleStart(length, height, itemHeight, overscan, visibleStart)
	return calculateVirtualRangeFromStart(length, height, itemHeight, overscan, rangeStart)
}

func calculateVirtualRangeFromStart(length int, height int, itemHeight int, overscan int, rangeStart int) VirtualRange {
	if length <= 0 {
		return VirtualRange{}
	}
	if overscan < 0 {
		overscan = 0
	}
	windowSize := virtualWindowSize(length, height, itemHeight, overscan)
	start := clampInt(rangeStart, 0, length-windowSize)
	end := start + windowSize

	return VirtualRange{
		Start:        start,
		End:          end,
		TopSpacer:    start * itemHeight,
		BottomSpacer: (length - end) * itemHeight,
		TotalHeight:  length * itemHeight,
	}
}

func virtualVisibleCount(height int, itemHeight int) int {
	if height <= 0 || itemHeight <= 0 {
		return 0
	}
	count := ceilDiv(height, itemHeight)
	if count < 1 {
		return 1
	}
	return count
}

func virtualWindowSize(length int, height int, itemHeight int, overscan int) int {
	if length <= 0 {
		return 0
	}
	if overscan < 0 {
		overscan = 0
	}
	windowSize := virtualVisibleCount(height, itemHeight) + 2*overscan
	if windowSize < 1 {
		windowSize = 1
	}
	if windowSize > length {
		return length
	}
	return windowSize
}

func virtualRangeCoversVisible(rangeInfo VirtualRange, visibleStart int, visibleCount int) bool {
	if visibleCount < 0 {
		visibleCount = 0
	}
	visibleEnd := visibleStart + visibleCount
	return visibleStart >= rangeInfo.Start && visibleEnd <= rangeInfo.End
}

func virtualRangeStartForVisibleStart(length int, height int, itemHeight int, overscan int, visibleStart int) int {
	if length <= 0 {
		return 0
	}
	if overscan < 0 {
		overscan = 0
	}
	windowSize := virtualWindowSize(length, height, itemHeight, overscan)
	visibleStart = clampInt(visibleStart, 0, length-1)
	return clampInt(visibleStart-overscan, 0, length-windowSize)
}

func virtualRangeStartAfterScroll(rangeInfo VirtualRange, currentStart int, length int, height int, itemHeight int, overscan int, scrollTop int) int {
	visibleStart := virtualVisibleStart(length, itemHeight, scrollTop)
	visibleCount := virtualVisibleCount(height, itemHeight)
	if virtualRangeCoversVisible(rangeInfo, visibleStart, visibleCount) {
		return currentStart
	}
	return virtualRangeStartForVisibleStart(length, height, itemHeight, overscan, visibleStart)
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
	return "height:" + ToString(height) + "px;overflow-y:auto;position:relative;overflow-anchor:none;"
}

func virtualTableSpacerRow(name string, height int, columnCount int) Node {
	heightValue := ToString(height)
	return El("tr", Props{
		"class":       "gf-virtual-table-spacer gf-virtual-table-spacer-" + name,
		"aria-hidden": "true",
		"style":       "height:" + heightValue + "px;overflow-anchor:none;",
	}, El("td", Props{
		"colspan": virtualTableColumnCount(columnCount),
		"style":   "height:" + heightValue + "px;padding:0;border:0;line-height:0;font-size:0;overflow-anchor:none;",
	}))
}

func virtualTableContentRow(content Node, columnCount int) Node {
	return El("tr", Props{}, El("td", Props{
		"colspan": virtualTableColumnCount(columnCount),
	}, content))
}

func virtualTableColumnCount(value int) int {
	if value <= 0 {
		return 1
	}
	return value
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
