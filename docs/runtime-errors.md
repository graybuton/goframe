# Runtime Error Semantics

## Purpose

GoFrame is still experimental, but applications are now large enough that user
panics need clear runtime semantics. MVP 23 defines how the runtime reports and
contains common user-code failures without adding a full Error Boundary,
Suspense, route-level error boundary, or async resource model.

## Current Problem

User code can run during render, event dispatch, effects, cleanups, context
selectors, memo comparators, and virtualized item render callbacks. Before MVP
23, a panic in those paths generally escaped through the browser/WASM call stack
or interrupted runtime cleanup.

That made failures hard to reason about:

- event handler panics could leave the app effectively crashed;
- cleanup panics could stop later cleanups;
- memo comparator panics could prevent safe fallback rendering;
- render and selector failures had no documented containment model.

## Error Phases

The runtime classifies reported failures with `gf.ErrorPhase`:

- `gf.ErrorPhaseRender`
- `gf.ErrorPhaseEvent`
- `gf.ErrorPhaseEffect`
- `gf.ErrorPhaseEffectCleanup`
- `gf.ErrorPhaseUnmountCleanup`
- `gf.ErrorPhaseMemo`
- `gf.ErrorPhaseContext`

Virtualized item and row render callbacks are ordinary render work. They report
as render failures with an operation label that identifies the virtualized
callback.

## Reporting Model

Applications can install a process-wide runtime error handler:

```go
restore := gf.SetErrorHandler(func(info gf.ErrorInfo) {
    // Send to app logging, tests, or a browser-visible probe.
})
defer restore()
```

`ErrorInfo` contains:

- `Phase`: where the panic happened;
- `Component`: the current component debug name when known;
- `Operation`: a small runtime operation label;
- `Panic`: the recovered panic value.

If no handler is installed, recoverable runtime paths still use their documented
fallback behavior. The default handler is intentionally silent.

If an installed handler panics, the runtime lets that panic escape. Handlers
should be small and should avoid calling back into rendering.

## Containment Model

Containment is intentionally phase-specific. The runtime only swallows a panic
when it has a deterministic fallback that keeps mounted state, DOM resources,
and hook bookkeeping coherent.

When no safe fallback exists, the runtime reports and re-panics.

## Phase Semantics

### Component Render

A component render panic is reported as `ErrorPhaseRender`. The MVP 23 fallback
is `gf.Empty()` for that render. This keeps the mounted component instance and
anchor range alive while replacing its rendered child subtree with an empty
comment node.

Future updates may retry the component render. This is containment, not a
component-level error UI. A full Error Boundary API remains future work.

### Event Handlers

An event handler panic is reported as `ErrorPhaseEvent` and contained. The app
stays mounted, the DOM listener remains installed, and future events can still
run.

Unsupported event handler signatures are also reported in this phase.

### Effect Setup

An effect body panic is reported as `ErrorPhaseEffect` and contained. The failed
effect does not register a cleanup. The component remains mounted. A later
render may queue and retry the effect according to the normal dependency rules.

### Effect Cleanup

A cleanup returned by `UseEffect` can run before a rerun or during unmount. If
that cleanup panics, the runtime reports `ErrorPhaseEffectCleanup`, clears that
cleanup slot, and continues with remaining cleanup work.

### Unmount Cleanup

`UseUnmount` cleanup panics are reported as `ErrorPhaseUnmountCleanup`. The
runtime continues releasing effect slots, context subscriptions, event
listeners, and other component resources where possible.

### Memo Comparators

If a props `MemoEqual` implementation panics, the runtime reports
`ErrorPhaseMemo` and falls back to "do not skip render." A broken comparator
therefore cannot freeze stale UI behind a memo bailout.

### Context Selectors

Selector failures have two cases.

During component render, there may be no previous selected value for that hook
slot. The runtime reports `ErrorPhaseContext` and re-panics, because pretending a
new selection exists would corrupt hook state.

During provider notifications or provider topology refresh, an existing
subscription already has a previous selected value. The runtime reports
`ErrorPhaseContext`, keeps the previous selected value, and does not mark the
consumer dirty from that failed selector evaluation.

### Virtualized Item/Row Render

`VirtualList.RenderItem`, `VirtualTable.Header`, `VirtualTable.RenderRow`, and
`VirtualTable.Empty` are user render callbacks. Panics are reported as render
errors with operation labels such as `VirtualTable.RenderRow`. The fallback is
an empty item or row subtree, matching component render containment.

## Default Behavior

Without a custom handler, recoverable errors are contained silently. This keeps
production bundles small and avoids forcing a logging policy into the runtime.

Programmer errors outside a containable user callback can still panic normally.
Examples include invalid hook order, invalid virtual dimensions, unsupported
effect dependency types, and calling hooks outside component render.

## Debug Behavior

MVP 23 does not add a browser debug global by default. Tests and applications
can install `SetErrorHandler` when they need observable reports.

Existing `goframe_debug` probes for render, patch, memo, duplicate keys, and
lifecycle warnings remain separate.

## Browser Smoke Behavior

Browser smoke verifies that recoverable event and cleanup failures do not
unmount the app or leak listeners. It uses a Go-compiled WASM fixture because
the current TinyGo package path uses trap-style panic lowering, where panics do
not return to Go `recover`.

It should not use timing gates for runtime error behavior.

## TinyGo Panic Mode Note

GoFrame's normal TinyGo packaging path is size-oriented and currently builds
with trap-style panic behavior. In that mode, a panic may abort before GoFrame's
`recover`-based containment can run. Runtime error containment is therefore
defined for recover-capable builds, including ordinary Go/WASM builds and Go
unit tests.

Future toolchain work may add an explicit recover-capable TinyGo build mode if
the size and behavior tradeoff is acceptable.

## Limitations

- No full Error Boundary component API.
- No error UI rendering policy.
- No route-level error handling.
- No async resource or Suspense-style model.
- No production crash reporting integration.
- Context selector containment during initial render is report + re-panic.
- TinyGo trap-style panic builds cannot provide recover-based containment.

## Future Error Boundaries

A future Error Boundary design should decide:

- how boundaries are declared in Go/GOX;
- whether boundaries catch only render errors or also event/effect errors;
- how boundary state resets after key changes or route changes;
- how browser diagnostics and app logging integrate.

MVP 23 deliberately stops before that API. It creates the smaller foundation:
phase classification, reporting, and deterministic containment where the
runtime can safely continue.
