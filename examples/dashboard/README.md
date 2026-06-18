# GoFrame Dashboard Example

This example is a dashboard-sized pressure test for the current GoFrame stack.
It is intentionally larger than counter/todo, but still small enough to inspect.

The app models an operations dashboard with deterministic issue data. It tests:

- 300 generated rows;
- controlled search and select filters;
- derived filtered/sorted views;
- metric cards;
- keyed table rows and reorder/filter behavior;
- fixed-height table virtualization through `gf.VirtualTable`;
- row selection and detail panel updates;
- a small document-title effect;
- multi-file GOX components in one Go package.

## Structure

- `app.gox` wires top-level state and derived views.
- `components_layout.gox` contains layout primitives.
- `components_metrics.gox` contains metric cards.
- `components_filters.gox` contains controlled inputs/selects.
- `components_table.gox` contains the virtualized keyed issue table.
- `components_detail.gox` contains the selected issue panel.
- `model.go`, `data.go`, and `filters.go` contain pure app logic.

The example is deliberately not split into nested Go packages because GOX does
not support component namespaces or dotted component tags yet.

## Run

```bash
goxc generate ./examples/dashboard
goxc package ./examples/dashboard --compiler=tinygo
goxc serve ./examples/dashboard --port=8080
```

Generated, build, and package artifacts stay under `examples/dashboard/.goframe`
by default. The authored app directory should not gain adjacent `.gox.go`,
`build/`, or `dist/` entries.

Release-style package check:

```bash
goxc package ./examples/dashboard --compiler=tinygo --asset-hash --preload --compress=gzip,br
```

Export only when you want a visible deploy directory:

```bash
goxc export ./examples/dashboard --out ./dist
```

Fallback through the standard Go WASM compiler:

```bash
goxc package ./examples/dashboard --compiler=go
```

## Smoke And Size

Dashboard smoke runs as part of:

```bash
scripts/browser-smoke.sh
```

Dashboard size budgets run as part of:

```bash
scripts/size-budget.sh
```

Expected TinyGo size is dashboard-sized but still below the MVP budget:

- raw <= 165 KiB;
- gzip <= 70 KiB;
- brotli <= 52 KiB;
- zstd <= 60 KiB, when zstd is available.

## Performance Notes

`scripts/dashboard-browser-smoke.mjs` prints a non-gating performance report
for each interaction. It separates:

- component render deltas;
- component patch deltas;
- structural DOM operations;
- MutationObserver records;
- approximate action timing.

Focus-only interaction is expected to produce zero runtime work. If the browser
visually paints a focus ring while the report shows zero renders, patches, DOM
ops, and mutations, that is browser paint rather than a GoFrame render.

State ownership is intentionally visible in this example. `DashboardApp` owns
the issue data and filters because metrics and the visible table derive from
them. `IssueWorkspace` owns only row selection so selecting a row does not
rerender `DashboardApp`, `MetricsGrid`, or `FilterBar`.

`IssueRow` implements `MemoEqual`, and dashboard smoke expects row selection to
rerender only the rows whose selected state changed. Unchanged rows should
record memo skips. Dataset-changing actions use `gf.UseReducer`, so old row
event handlers dispatch actions against the latest issue state instead of a
slice captured by an older render. That removes the earlier `DataVersion`
workaround and keeps row memoization useful for single-row data changes.

The table uses `gf.VirtualTable` with fixed-height rows. The app still has 300
logical issues and summaries still report the logical filtered count, but the
mounted `.issue-row` elements stay bounded to the visible window plus overscan.
The table passes `ColumnCount: 7` so spacer rows preserve the real dashboard
columns: Issue, Status, Priority, Owner, Service, Events, and Action. This is
real virtualization: offscreen rows are not hidden DOM nodes. The framework
keeps top and bottom spacer rows keyed and mounted, including at `0px` height,
so scroll/filter updates do not remount spacer `tr` nodes or match them against
user rows.

For DOM pressure audits, run:

```bash
node --experimental-websocket scripts/dashboard-dom-pressure.mjs
```

The pressure audit repeatedly switches the status filter from Open to All. The
pre-virtualization baseline created roughly 6,156 DOM nodes and reattached
about 456 event listeners for the All transition, while live DOM and net
listener counts still stabilized. That proved DOM pressure rather than a
classic leak.

With `gf.VirtualTable`, a typical debug run reports:

- logical All count: 300 issues;
- mounted issue rows: about 20, with a smoke bound of 70;
- Open -> All average duration: about 47 ms in the local debug audit;
- Open -> All created nodes: about 432;
- virtual table spacer rows stable across filter and scroll cycles;
- live DOM count stable across cycles;
- net listener count stable across cycles.

Exact timing remains informational and machine-dependent. The hard invariant is
bounded mounted rows plus stable live DOM/listener counts.

## Known Limitations

- There is no router or URL state.
- State is still mostly passed through typed props; the dashboard intentionally
  remains a reducer/memoization/virtualization pressure test rather than a
  context example.
- Virtualization requires fixed row height. There is no dynamic measurement,
  infinite loading, or advanced keyboard navigation yet.
- GOX has no spread props, style objects, namespaces, or template loops.
- Timing numbers printed by smoke are informational, not CI performance budgets.
