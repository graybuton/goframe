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

## Known Limitations

- There is no router or URL state.
- There is no context API, so state is passed through typed props.
- There is no virtualization; all 300 rows are real DOM rows.
- GOX has no spread props, style objects, namespaces, or template loops.
- Timing numbers printed by smoke are informational, not CI performance budgets.
