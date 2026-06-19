# Performance Baseline

## Purpose

This document records the measurable baseline after MVP 17 and Foundation
Audit IV. It is a decision aid for future runtime and tooling work, not a
claim that GoFrame is production-ready.

The goal is to separate hard regression gates from noisy informational metrics
so future changes can be judged by stable invariants instead of browser-feel
alone.

## Scope

The baseline covers:

- Go, race, vet, debug-tag, and GOX golden tests;
- TinyGo raw, gzip, brotli, and zstd size budgets;
- browser smoke behavior for Todo, duplicate keys, dashboard, context, and
  virtualized collections;
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

Recorded on `chore/runtime-benchmark-baseline` from local `main` at
`f2192e9` after Foundation Audit IV.

Tool versions:

- Go: `go1.24.4 linux/amd64`
- TinyGo: `0.41.1`
- Node.js: `v20.19.4`
- Git: `2.51.0`

`git pull origin main` could not authenticate in this environment. Local
`main` and `origin/main` pointed at the same commit when this baseline was
taken.

Baseline checks passed before documentation changes:

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
- `scripts/check.sh`
- `scripts/size-budget.sh`
- `scripts/perf-report.sh`
- `scripts/browser-smoke.sh`
- `node --experimental-websocket scripts/dashboard-dom-pressure.mjs`
- `scripts/artifact-check.sh`
- `scripts/module-path-check.sh`

## Size Baseline

TinyGo packaged WASM sizes:

| Example | Raw | gzip | br | zstd |
|---|---:|---:|---:|---:|
| counter | 81350 | 32477 | 27098 | 29282 |
| components | 86725 | 34131 | 28324 | 30555 |
| todo | 113935 | 43622 | 36285 | 39263 |
| dashboard | 163897 | 61217 | 49482 | 53454 |
| context | 111440 | 42093 | 34413 | 37111 |
| virtualized | 119729 | 45575 | 37524 | 40759 |

The authoritative gate remains `scripts/size-budget.sh`. This table is a
snapshot, not a second budget file.

## Browser / DOM Baseline

`scripts/browser-smoke.sh` currently covers:

- Todo DOM identity, controlled input behavior, localStorage, keyed reorder,
  and event listener stability.
- Duplicate key debug diagnostics.
- Dashboard focus, search, selection, sorting, filters, reducer-safe row
  toggles, virtual table layout, spacer stability, and bounded scroll churn.
- Context selector rerender isolation.
- Virtualized list/table mounted-window behavior, scroll correctness,
  selection/toggle after scroll, and listener net stability.

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
| Average All duration | 49.9 ms |
| Max All duration | 61.7 ms |
| Average All created nodes | 621 |
| Average All removed nodes | 69 |
| Average All runtime render | 11.91 ms |
| Average All script | 13.64 ms |
| Average All layout | 3.82 ms |
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
| DirtyQueuePruning | 193.1 ns/op |
| MatchChildIndicesKeyed | 18085 ns/op |
| MatchChildIndicesUnkeyed | 3546 ns/op |
| SplitProps | 3626 ns/op |
| EventNameNormalization | 316.5 ns/op |
| StateSlotAccess | 469.2 ns/op |
| UnwrapKeyedNode | 8.358 ns/op |

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
