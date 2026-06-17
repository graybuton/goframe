# GoFrame Dashboard Example

This example is a dashboard-sized pressure test for the current GoFrame stack.
It is intentionally larger than counter/todo, but still small enough to inspect.

The app models an operations dashboard with deterministic issue data. It tests:

- 300 generated rows;
- controlled search and select filters;
- derived filtered/sorted views;
- metric cards;
- keyed table rows and reorder/filter behavior;
- row selection and detail panel updates;
- a small document-title effect;
- multi-file GOX components in one Go package.

## Structure

- `app.gox` wires top-level state and derived views.
- `components_layout.gox` contains layout primitives.
- `components_metrics.gox` contains metric cards.
- `components_filters.gox` contains controlled inputs/selects.
- `components_table.gox` contains keyed table rows.
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

- raw <= 150 KiB;
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

Known remaining cost: without table virtualization, full-table actions such as
search, status filtering, sorting, reset, and simulated dataset updates still
visit or rerender many visible rows. That is an intentional baseline for future
runtime optimization rather than a hidden runtime feature.

For DOM pressure audits, run:

```bash
node --experimental-websocket scripts/dashboard-dom-pressure.mjs
```

The pressure audit repeatedly switches the status filter from Open to All. On
the current 300-row fixture, Open shows 72 rows and All restores 300 rows, so
the All transition recreates 228 rows. In a debug TinyGo build that is roughly
6,156 created DOM nodes and 456 event listeners reattached per All transition.
The audit fails if the live DOM node count or net listener count grows across
cycles. A stable live DOM count with growing DevTools/Performance `Nodes`
should be treated as a detached-node/GC accounting signal to investigate, not
as proof of a live DOM leak by itself.

The expected next product-level answer to this pressure is row virtualization,
not a hidden runtime heuristic. Virtualization is intentionally outside this
example and outside the current runtime MVPs.

## Known Limitations

- There is no router or URL state.
- There is no context API, so state is passed through typed props.
- There is no virtualization; all 300 rows are real DOM rows.
- GOX has no spread props, style objects, namespaces, or template loops.
- Timing numbers printed by smoke are informational, not CI performance budgets.
- There is no row virtualization, so full-table state changes still visit all
  rendered rows.
