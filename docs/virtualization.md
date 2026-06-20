# Virtualized Collections

`goframe` provides fixed-height collection virtualization for large lists and
tables. Virtualization keeps only the visible window plus overscan mounted in
the DOM. It is not the same as memoization: memoization skips render work for
mounted components, while virtualization avoids mounting offscreen components
at all.

## VirtualList

```go
gf.VirtualList[Item](gf.VirtualListProps[Item]{
	Items:      items,
	Height:     360,
	ItemHeight: 44,
	Overscan:   8,
	Key: func(item Item, index int) string {
		return gf.ToString(item.ID)
	},
	RenderItem: func(item gf.VirtualItem[Item]) gf.Node {
		return <ItemCard Key={item.Key} Item={item.Item} />
	},
	Class:  "items",
	TestID: "items-list",
})
```

`Height` is the scroll viewport height in pixels. `ItemHeight` is the fixed
logical item height in pixels. `Overscan` renders extra rows above and below
the visible window to avoid edge flicker during normal scrolling.

`VirtualItem.Style` contains the absolute-position style for the item. The
built-in `VirtualList` wrapper applies the positioning to a `gf-virtual-item`
element before calling `RenderItem`.

## VirtualTable

```go
gf.VirtualTable[Issue](gf.VirtualTableProps[Issue]{
	Items:     issues,
	Height:    560,
	RowHeight: 48,
	Overscan:  8,
	ColumnCount: 7,
	Key: func(issue Issue, index int) string {
		return gf.ToString(issue.ID)
	},
	Header: func() gf.Node {
		return <IssueTableHeader />
	},
	RenderRow: func(row gf.VirtualRow[Issue]) gf.Node {
		return <IssueRow
			Key={row.Key}
			Issue={row.Item}
			Style={row.RowStyle}
		/>
	},
	Empty: func() gf.Node {
		return <EmptyState />
	},
	TestID: "issue-table",
})
```

`VirtualTable` renders a scrollable viewport containing a normal table. It uses
spacer rows above and below the mounted window to preserve the logical scroll
height. `VirtualRow.RowStyle` should be applied to the rendered `<tr>` so row
height stays consistent.

Set `ColumnCount` to the number of rendered columns in the table. GoFrame uses
that value as the `colspan` for spacer and empty rows. If `ColumnCount <= 0`,
the runtime falls back to `1`; examples should not rely on that fallback unless
the table really has one column.

The top and bottom spacer rows are internal keyed rows and remain mounted even
when their height is `0px`. User row keys are namespaced internally before
reconciliation, while `VirtualRow.Key` remains the user-facing item key passed
to `RenderRow`. This prevents spacer rows and user rows from matching each
other during scroll, filter, and sort updates.

If `Header`, `RenderRow`, or `Empty` panics, the runtime reports a render-phase
error with a virtual-table operation label. `RenderRow` falls back to an empty
content row with the configured `ColumnCount`, so the table tree remains valid.

## Range Calculation

The current range model is intentionally fixed-height:

```text
visibleStart = scrollTop / itemHeight
visibleCount = ceil(height / itemHeight)
windowSize = min(len(items), visibleCount + 2 * overscan)
rangeStart = clamp(visibleStart - overscan, 0, len(items) - windowSize)
start = rangeStart
end = start + windowSize
topSpacer = start * itemHeight
bottomSpacer = (len(items) - end) * itemHeight
```

Negative overscan is treated as zero. Invalid `Height` or `ItemHeight` values
panic with a focused runtime message.

Scroll handling stores the rendered range start, not every raw pixel offset or
every first visible row. At the top and bottom edges, the range keeps a full
`visible + 2*overscan` window whenever enough items exist, so missing overscan
on one side is compensated on the other side.

During scroll, GoFrame first checks whether the current visible rows are still
inside the already-rendered range. If so, it leaves component state alone and
the DOM window stays mounted. A new range is scheduled only after the visible
viewport leaves that buffer. Virtualized viewports and spacer rows disable
browser scroll anchoring with `overflow-anchor:none` so spacer height changes do
not fight the user's scroll position.

## Keys

Always prefer stable item IDs:

```go
Key: func(item Item, index int) string {
	return gf.ToString(item.ID)
}
```

If `Key` is nil, `goframe` falls back to index-based keys. That is acceptable
for static append-only lists, but it is not recommended for filtered, sorted,
or mutable data. The dashboard uses issue IDs as virtual row keys.

If `VirtualList.RenderItem` panics, the runtime reports a render-phase error
and falls back to an empty item subtree for that slot.

## Dashboard Pressure

Before virtualization, the dashboard `Open -> All` status transition mounted
all 300 issue rows and created roughly 6k DOM nodes in the debug pressure audit.
The live DOM and listener counts stabilized, so this was DOM pressure rather
than a classic leak.

After moving the issue table to `gf.VirtualTable`, the dashboard still has 300
logical issues, but the mounted `.issue-row` count stays bounded. A typical
debug pressure run reports about 28 mounted rows, bounded created nodes for
`Open -> All`, stable live DOM count, and stable net listener count.

The pressure script gates the important invariants:

- mounted rows remain bounded;
- top and bottom spacer rows keep stable DOM identity;
- scroll inside the overscan buffer does not rerender or churn table rows;
- continuous scroll renders `VirtualTable` far less often than scroll events;
- live DOM count stabilizes across cycles;
- net listener count stabilizes across cycles;
- `Open -> All` no longer creates thousands of row nodes.

## Limitations

- Fixed item and row heights only.
- No dynamic measurement or variable-height layout engine yet.
- No infinite loading or data-window fetching.
- Table accessibility is basic; keyboard navigation is future work.
- No SSR or hydration integration.
- Hidden rows are not virtualization. Offscreen rows should not remain mounted
  merely with `display:none` or `visibility:hidden`.

Future work can add dynamic row-height measurement, richer table helpers,
keyboard navigation, and infinite loading once the fixed-height model has more
usage mileage.
