# Runtime Error Semantics

## Purpose

GoFrame is still experimental, but applications are now large enough that user
panics need clear runtime semantics. MVP 23 defined how the runtime reports and
contains common user-code failures. MVP 27 adds a narrow scoped Error Boundary
for descendant render failures.

This is still not Suspense, a route loader framework, or a route-level error
framework. MVP 28 resources use explicit component state rather than
panic/throw-style render blocking.

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

A component render panic is reported as `ErrorPhaseRender`.

If the failing component has an active ancestor `gf.ErrorBoundary`, the nearest
boundary captures the first incident, cancels pending effects under its
protected subtree, and renders fallback UI on the next patch. The failing render
still returns `gf.Empty()` for that immediate pass so reconciliation can finish
deterministically.

If no boundary exists, the fallback remains the MVP 23 behavior: `gf.Empty()`
for that render. Future state or parent updates may retry the component.

### Event Handlers

An event handler panic is reported as `ErrorPhaseEvent` and contained. The app
stays mounted, the DOM listener remains installed, and future events can still
run.

Unsupported event handler signatures are also reported in this phase.

### Effect Setup

An effect body panic is reported as `ErrorPhaseEffect` and contained. The failed
effect does not register a cleanup. The component remains mounted. A later
render may queue and retry the effect according to the normal dependency rules.

`UseResource` starts loaders through this same after-patch effect path. A
loader setup panic reports `ErrorPhaseEffect` and leaves the resource in failed
state where recover is available. Ordinary `reject(err)` is not a runtime
panic; it is represented as `ResourceFailed`.

### Effect Cleanup

A cleanup returned by `UseEffect` can run before a rerun or during unmount. If
that cleanup panics, the runtime reports `ErrorPhaseEffectCleanup`, clears that
cleanup slot, and continues with remaining cleanup work.

### Unmount Cleanup

`UseUnmount` cleanup panics are reported as `ErrorPhaseUnmountCleanup`. The
runtime continues releasing effect slots, context subscriptions, event
listeners, and other component resources where possible.

Resource loader cleanup panics are reported as `ErrorPhaseEffectCleanup`.
Cleanup still invalidates that resource generation first, so later callbacks
from the same run are ignored.

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
errors with operation labels such as `VirtualTable.RenderRow`. These callbacks
keep their local empty item/row fallback behavior in MVP 27 rather than
switching surrounding boundaries, because they are invoked by virtualization
primitives rather than a distinct user component boundary.

## Error Boundaries

`gf.ErrorBoundary` is a scoped render fallback primitive:

```go
gf.ErrorBoundary(gf.ErrorBoundaryProps{
    ResetKey: routeKey,
    Fallback: func(ctx gf.ErrorBoundaryContext) gf.Node {
        return retryPanel(ctx.Info, ctx.Reset)
    },
    Children: []gf.Node{content},
})
```

In GOX:

```gox
<gf.ErrorBoundary ResetKey={routeKey} Fallback={fallback}>
    <pages.Content />
</gf.ErrorBoundary>
```

Boundary rules:

- nearest active boundary wins;
- nested inner boundaries catch before outer boundaries;
- a boundary does not catch failures from the fallback subtree it is currently
  displaying;
- fallback subtree failures can be captured by an outer boundary;
- first error wins until manual reset or `ResetKey` reset;
- `ctx.Reset()` remounts the protected subtree fresh;
- changing `ResetKey` while failed clears the incident and remounts children;
- runtime invariant panics whose value starts with `goframe:` bypass boundary
  containment.

Boundaries do not catch event, effect, cleanup, memo comparator, or context
selector update failures. Those phases keep the phase-specific containment
listed above and continue to report through `SetErrorHandler`.
Ordinary resource failed state is also not a boundary incident; applications
render loading, ready, and failed UI explicitly from the returned
`gf.Resource[T]`.

See [Error Boundaries](error-boundaries.md) for lifecycle details and the
TinyGo panic-mode matrix.

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
unmount the app or leak listeners. A separate Error Boundary fixture verifies
render fallback, retry, `ResetKey`, nested boundaries, and cleanup behavior.

Both fixtures use Go-compiled WASM because the current TinyGo package path uses
trap-style panic lowering, where panics do not return to Go `recover`.

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

- Error Boundaries catch render-path failures only.
- No automatic route-level Error Boundary installation.
- No route-level error handling.
- No Suspense-style resource throwing, render blocking, or route loader model.
- No production crash reporting integration.
- Context selector containment during initial render is report + re-panic.
- TinyGo trap-style panic builds cannot provide recover-based containment.
