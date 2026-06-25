# Performance Baseline

## Purpose

This document records the measurable baseline through MVP 29. It is a
decision aid for future runtime and tooling work, not a claim that GoFrame is
production-ready.

The goal is to separate hard regression gates from noisy informational metrics
so future changes can be judged by stable invariants instead of browser-feel
alone.

## Scope

The baseline covers:

- Go, race, vet, debug-tag, and GOX golden tests;
- TinyGo raw, gzip, brotli, and zstd size budgets;
- browser smoke behavior for Todo, duplicate keys, runtime errors, Error
  Boundaries, dashboard, context, virtualized collections, the multi-package
  example, the child-entry example, the hash-router example, and the
  router-dashboard reference example, and the resource example;
- dashboard DOM pressure and listener stability;
- pure runtime benchmark reports;
- package, export, artifact, and module path safety gates.

It does not define production SLOs, browser frame budgets, or public API
compatibility guarantees.

## Hard Invariants

These failures should block CI or a PR:

- `go test ./...`, race tests, `go vet`, debug-tag tests, or GOX golden tests
  fail.
- Production runtime source gates fail.
- `scripts/artifact-check.sh` or `scripts/module-path-check.sh` fails.
- `scripts/size-budget.sh` exceeds a raw or compressed WASM budget.
- Browser smoke reports a harness failure or app failure.
- Error Boundary smoke fails to report a render failure once, switch to
  fallback UI, reset cleanly, preserve shell identity, or keep nested-boundary
  bubbling semantics.
- Resource smoke fails explicit loading/ready/failed transitions, reload,
  stale-completion guards, cleanup-after-unmount behavior, or non-boundary
  failed-state semantics.
- A smoke app loads from the wrong origin or cannot validate WASM MIME type.
- Dashboard mounted `.issue-row` count becomes unbounded.
- Dashboard DOM pressure shows live DOM growth across Open -> All cycles.
- Dashboard DOM pressure shows net listener growth across cycles.
- Virtual table top or bottom spacer identity becomes unstable.
- Inside-buffer virtual scroll causes render, patch, DOM operation, or listener
  churn.
- Package/export ownership checks allow deleting non-GoFrame user output.

## Informational Metrics

These metrics should be watched and reported, but should not be hard gates
without a separate stabilization pass:

- absolute browser interaction timing;
- exact `Layout`, `Paint`, and scripting durations;
- exact Chrome DevTools Protocol `Nodes` count drift;
- exact dashboard Open -> All duration;
- exact Go benchmark `ns/op`;
- exact compressed size changes that remain below budget.

Browser timing is affected by CPU scheduling, headless Chrome behavior, CDP
collection overhead, font/layout differences, and host load. For this project,
DOM liveness, listener liveness, mounted row bounds, and structural operation
invariants are more reliable than single-run paint timing.

CDP `Nodes` can drift upward in long debug sessions even when
`document.querySelectorAll("*").length` and listener counts are stable. Treat
CDP node drift as an investigation signal, not a failure by itself.

## Current Baseline

Updated during MVP 29 from local `main` after the MVP 28 resource baseline.

Tool versions:

- Go: `go1.24.4 linux/amd64`
- TinyGo: `0.41.1`
- Node.js: `v20.19.4`
- Git: `2.51.0`

`git pull origin main` could not authenticate in this environment. Local
`main` and `origin/main` pointed at the same commit when this baseline was
taken.

Current verification includes:

- `go fmt ./...`
- `git diff --check`
- `go test ./...`
- `go test -race ./pkg/... ./cmd/...`
- `go vet ./...`
- `go test -tags=goframe_debug ./...`
- `go test ./pkg/gox -run 'TestGolden|TestErrorGolden'`
- `go test ./examples/dashboard`
- `go test ./examples/context`
- `go test ./examples/virtualized`
- `go test ./examples/multipackage`
- `go test ./examples/cmdapp`
- `go test ./examples/router`
- `go test ./examples/router-dashboard`
- `go test ./examples/resource`
- `scripts/check.sh`
- `scripts/size-budget.sh`
- `scripts/perf-report.sh`
- `scripts/browser-smoke.sh`
- `node --experimental-websocket scripts/dashboard-dom-pressure.mjs`
- `scripts/artifact-check.sh`
- `scripts/module-path-check.sh`
- `node scripts/docs-check.mjs`

## Size Baseline

TinyGo packaged WASM sizes:

| Example | Raw | gzip | br | zstd |
|---|---:|---:|---:|---:|
| counter | 83550 | 33578 | 28000 | 30158 |
| components | 89198 | 35217 | 29238 | 31563 |
| todo | 117409 | 44989 | 37487 | 40506 |
| dashboard | 168628 | 62883 | 50808 | 55093 |
| context | 115354 | 43252 | 35569 | 38445 |
| virtualized | 123144 | 47417 | 38780 | 42098 |
| multipackage | 94354 | 36850 | 30728 | 33175 |
| cmdapp | 94380 | 36839 | 30720 | 33124 |
| router | 114716 | 43602 | 36062 | 39026 |
| router-dashboard | 225649 | 90888 | 74402 | 79524 |
| resource | 147562 | 64576 | 54640 | 58090 |

The authoritative gate remains `scripts/size-budget.sh`. This table is a
snapshot, not a second budget file.

## Browser / DOM Baseline

`scripts/browser-smoke.sh` currently covers:

- Todo DOM identity, controlled input behavior, localStorage, keyed reorder,
  and event listener stability.
- Duplicate key debug diagnostics.
- Runtime error containment for event/effect/cleanup panic reporting.
- Scoped render Error Boundary fallback, reset, nested bubbling, and failed
  protected-subtree cleanup behavior.
- Dashboard focus, search, selection, sorting, filters, reducer-safe row
  toggles, virtual table layout, spacer stability, and bounded scroll churn.
- Context selector rerender isolation.
- Virtualized list/table mounted-window behavior, scroll correctness,
  selection/toggle after scroll, and listener net stability.
- Multi-package GOX workspace loading, internal package component rendering,
  readable debug names, and a simple state interaction.
- Child-entry package loading from `cmd/app`, internal package component
  rendering, readable debug names, and a simple state interaction.
- Hash-router initial route, hash links, route params, programmatic
  navigation, browser back handling, not-found rendering, and stable shell
  layout.
- Router-dashboard query filters, browser back query restoration, route params,
  controlled form validation, reset behavior, not-found rendering, and stable
  shell layout.
- Resource example explicit loading/ready/failed state, reload behavior,
  stale completion guards, cleanup after unmount, and failed-state behavior
  that does not activate Error Boundary fallback.

Hard browser gates focus on correctness and structural invariants. Timing
printed by smoke scripts is useful for trend spotting, but it is not a stable
CI budget yet.

## Dashboard DOM Pressure Baseline

`node --experimental-websocket scripts/dashboard-dom-pressure.mjs` baseline:

| Metric | Value |
|---|---:|
| cycles | 20 |
| Open logical rows | 72 |
| All logical rows | 300 |
| All mounted row max | 28 |
| Open mounted row max | 28 |
| Average All duration | 51.08 ms |
| Max All duration | 68.9 ms |
| Average All created nodes | 621 |
| Average All removed nodes | 69 |
| Average All runtime render | 14.43 ms |
| Average All script | 17.48 ms |
| Average All layout | 4.39 ms |
| Live DOM All start/end | 486 -> 486 |
| Net listeners All start/end | 0 -> 0 |
| Post-idle live DOM nodes | 486 |
| Post-idle JS event listeners | 63 |
| Top spacer stable | true |
| Bottom spacer stable | true |

Continuous scroll baseline:

| Metric | Value |
|---|---:|
| Scroll steps | 321 |
| VirtualTable renders | 8 |
| IssueRow renders | 72 |
| Render/scroll ratio | 0.02 |
| Mounted rows max | 28 |
| Listener net delta | 0 |
| Spacers stable | true |

The same run reported CDP `Nodes` drift after GC. That did not fail because
live DOM and listener invariants stayed stable.

## Virtualized Collections Baseline

Virtualization hard gates:

- mounted list/table items stay bounded;
- scroll inside the buffered range performs no runtime or DOM work;
- scroll beyond the buffer performs bounded window replacement;
- spacer rows remain mounted and keyed;
- user row keys are namespaced internally;
- listener add/remove counts balance on window replacement;
- selection/toggle after scroll affects the intended logical item.

The current model is fixed-height only. Dynamic measurement, infinite loading,
advanced keyboard behavior, and richer table accessibility are future work.

## Runtime Benchmarks

`scripts/perf-report.sh` runs pure runtime benchmarks and then size budgets.
Current benchmark output is informational:

| Benchmark | Current sample |
|---|---:|
| DirtyQueuePruning | 387.8 ns/op |
| MatchChildIndicesKeyed | 11468 ns/op |
| MatchChildIndicesUnkeyed | 1935 ns/op |
| SplitProps | 1324 ns/op |
| EventNameNormalization | 162.0 ns/op |
| StateSlotAccess | 295.0 ns/op |
| UnwrapKeyedNode | 8.403 ns/op |

Use these numbers to spot large regressions, but do not gate on exact values
until the benchmark environment is controlled.

## CI Gates

CI should keep running:

- core Go/GOX checks;
- module path and artifact gates;
- TinyGo size budgets;
- browser smoke;
- dashboard DOM pressure when available;
- VS Code extension compile.

Timing metrics may be uploaded or printed for review, but only explicitly
listed structural invariants should fail CI.

## How To Update This Document

Update this document when:

- an MVP intentionally changes runtime/package behavior;
- size budgets change;
- smoke scripts add or remove hard invariants;
- dashboard or virtualized examples change their data shape;
- TinyGo or Go versions change enough to affect sizes materially.

Use fresh output from:

```bash
scripts/size-budget.sh
scripts/perf-report.sh
scripts/browser-smoke.sh
node --experimental-websocket scripts/dashboard-dom-pressure.mjs
```

Do not update only the numbers while leaving the hard/informational split
ambiguous.

## What Not To Gate Yet

Do not hard-gate yet on:

- exact interaction milliseconds;
- exact Layout/Paint durations;
- exact CDP `Nodes` counts;
- exact benchmark `ns/op`;
- exact compressed byte deltas below budget;
- manual DevTools paint flashing impressions.

These are still valuable signals. They should trigger investigation, not
automatic failure, unless paired with a stable structural regression.
