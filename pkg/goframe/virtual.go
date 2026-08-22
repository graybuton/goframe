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

type virtualViewportState struct {
	RangeStart int
	ScrollTop  int
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
		panicRuntimeInvariant("goframe: VirtualList requires RenderItem")
	}
	rangeInfo, onScroll := useVirtualRange(
		len(props.Items),
		props.Height,
		props.ItemHeight,
		props.Overscan,
	)

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
		"class":    joinVirtualClass("gf-virtual-list", props.Class),
		"style":    virtualViewportStyle(props.Height),
		"OnScroll": onScroll,
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
		panicRuntimeInvariant("goframe: VirtualTable requires RenderRow")
	}
	rangeInfo, onScroll := useVirtualRange(
		len(props.Items),
		props.Height,
		props.RowHeight,
		props.Overscan,
	)

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
		rowStyle := "height:" + ToString(props.RowHeight) + "px;"
		for index := rangeInfo.Start; index < rangeInfo.End; index++ {
			key := virtualItemKey(props.Key, props.Items[index], index)
			row := VirtualRow[T]{
				Item:     props.Items[index],
				Index:    index,
				Key:      key,
				RowStyle: rowStyle,
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
		"class":    "gf-virtual-table-viewport",
		"style":    virtualViewportStyle(props.Height),
		"OnScroll": onScroll,
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
		panicRuntimeInvariant("goframe: VirtualList requires positive Height and ItemHeight")
	}
}

func validateVirtualTableDimensions(height int, rowHeight int) {
	if height <= 0 || rowHeight <= 0 {
		panicRuntimeInvariant("goframe: VirtualTable requires positive Height and RowHeight")
	}
}

func validateVirtualRangeDimensions(height int, itemHeight int) {
	if height <= 0 || itemHeight <= 0 {
		panicRuntimeInvariant("goframe: virtual range requires positive Height and ItemHeight")
	}
}

func calculateVirtualRange(length int, height int, itemHeight int, overscan int, scrollTop int) VirtualRange {
	validateVirtualRangeDimensions(height, itemHeight)
	visibleStart, visibleEnd := virtualVisibleRange(length, height, itemHeight, scrollTop)
	return calculateVirtualRangeForVisible(length, itemHeight, overscan, visibleStart, visibleEnd)
}

func calculateVirtualRangeForVisible(length int, itemHeight int, overscan int, visibleStart int, visibleEnd int) VirtualRange {
	if length <= 0 {
		return VirtualRange{}
	}
	if overscan < 0 {
		overscan = 0
	}
	visibleStart = clampInt(visibleStart, 0, length-1)
	visibleEnd = clampInt(visibleEnd, visibleStart+1, length)
	windowSize := visibleEnd - visibleStart + 2*overscan
	if windowSize > length {
		windowSize = length
	}
	start := clampInt(visibleStart-overscan, 0, length-windowSize)
	end := start + windowSize
	return VirtualRange{
		Start:        start,
		End:          end,
		TopSpacer:    start * itemHeight,
		BottomSpacer: (length - end) * itemHeight,
		TotalHeight:  length * itemHeight,
	}
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

func useVirtualRange(length int, height int, itemHeight int, overscan int) (VirtualRange, func(ScrollEvent)) {
	viewport := useStateSlot(virtualViewportState{}, "UseState")
	rangeInfo, viewportState := normalizeVirtualViewport(
		viewport.get(),
		length,
		height,
		itemHeight,
		overscan,
	)
	viewportState = stageStateValueForRender(viewport, viewportState)
	return rangeInfo, func(event ScrollEvent) {
		scrollTop := virtualScrollTop(length, height, itemHeight, event.ScrollTop())
		next := virtualViewportState{
			RangeStart: viewportState.RangeStart,
			ScrollTop:  scrollTop,
		}
		if virtualRangeCoversViewport(rangeInfo, length, height, itemHeight, scrollTop) {
			if next == viewportState {
				return
			}
			if recordStateValueWithoutRender(viewport, next) {
				viewportState = next
			}
			return
		}

		next.RangeStart = calculateVirtualRange(length, height, itemHeight, overscan, scrollTop).Start
		viewportState = next
		viewport.set(next)
	}
}

func normalizeVirtualViewport(state virtualViewportState, length int, height int, itemHeight int, overscan int) (VirtualRange, virtualViewportState) {
	state.ScrollTop = virtualScrollTop(length, height, itemHeight, state.ScrollTop)
	rangeInfo := calculateVirtualRangeFromStart(length, height, itemHeight, overscan, state.RangeStart)
	if !virtualRangeCoversViewport(rangeInfo, length, height, itemHeight, state.ScrollTop) {
		rangeInfo = calculateVirtualRange(length, height, itemHeight, overscan, state.ScrollTop)
	}
	state.RangeStart = rangeInfo.Start
	return rangeInfo, state
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
	if virtualRangeCoversViewport(rangeInfo, length, height, itemHeight, scrollTop) {
		return currentStart
	}
	return calculateVirtualRange(length, height, itemHeight, overscan, scrollTop).Start
}

func virtualRangeCoversViewport(rangeInfo VirtualRange, length int, height int, itemHeight int, scrollTop int) bool {
	visibleStart, visibleEnd := virtualVisibleRange(length, height, itemHeight, scrollTop)
	return visibleStart >= rangeInfo.Start && visibleEnd <= rangeInfo.End
}

func virtualVisibleRange(length int, height int, itemHeight int, scrollTop int) (int, int) {
	if length <= 0 || height <= 0 || itemHeight <= 0 {
		return 0, 0
	}
	scrollTop = virtualScrollTop(length, height, itemHeight, scrollTop)
	visibleStart := clampInt(scrollTop/itemHeight, 0, length-1)
	visibleEnd := clampInt(ceilDiv(scrollTop+height, itemHeight), visibleStart+1, length)
	return visibleStart, visibleEnd
}

func virtualScrollTop(length int, height int, itemHeight int, scrollTop int) int {
	if length <= 0 || height <= 0 || itemHeight <= 0 {
		return 0
	}
	return clampInt(scrollTop, 0, maxInt(0, length*itemHeight-height))
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

func maxInt(first int, second int) int {
	if first > second {
		return first
	}
	return second
}
