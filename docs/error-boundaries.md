# Error Boundaries

## Purpose

Error Boundaries add a small scoped fallback UI for render failures. They build
on the runtime error reporting model from MVP 23 without turning GoFrame into a
React-style boundary, Suspense, or async resource framework.

The boundary scope is intentionally narrow: it catches render-path failures in
descendant component subtrees, reports them through the existing global runtime
error handler, and renders deterministic fallback UI until the boundary is
reset.

## Relationship To Runtime Error Reporting

`gf.SetErrorHandler` remains the reporting API. Error Boundaries do not replace
it and do not install a global error store.

When a boundary captures a render failure:

- the runtime reports one `gf.ErrorInfo` for that failing render;
- the nearest active boundary stores the first incident;
- the boundary switches to fallback UI;
- later fallback rerenders do not report the original incident again.

If no boundary exists, component render panics keep the MVP 23 behavior:
report the failure and render `gf.Empty()` for that pass.

## API

```go
type ErrorBoundaryContext struct {
    Info  gf.ErrorInfo
    Reset func()
}

type ErrorBoundaryProps struct {
    ResetKey string
    Fallback func(gf.ErrorBoundaryContext) gf.Node
    Children []gf.Node
}

func ErrorBoundary(props gf.ErrorBoundaryProps) gf.Node
```

In GOX, use ordinary package-qualified component tags:

```gox
<gf.ErrorBoundary
    ResetKey={routeKey}
    Fallback={func(ctx gf.ErrorBoundaryContext) gf.Node {
        return <pages.ErrorPanel Info={ctx.Info} OnRetry={ctx.Reset} />
    }}
>
    <pages.Content />
</gf.ErrorBoundary>
```

There is no special GOX lowering for boundaries. The tag compiles through the
same `ComponentT` path as other `gf.*` package-qualified components.

`Fallback` is required. A nil fallback is a runtime invariant panic with a
`goframe:` prefix, so it is not swallowed by the boundary mechanism.

## Render-Only Scope

Error Boundaries catch only render-path failures from descendant components.
They do not catch:

- event handler panics;
- effect setup panics;
- effect cleanup panics;
- `UseUnmount` cleanup panics;
- memo comparator panics;
- context selector update panics;
- runtime invariant panics with a `goframe:` prefix.

Those phases keep the MVP 23 semantics and continue to report through
`SetErrorHandler` without switching the boundary to fallback UI.

Initial context selector panics happen during component render. They still
report `ErrorPhaseContext`, then flow through render containment and can be
captured by a nearest boundary as a render failure.

## Nearest Boundary Semantics

The runtime walks the component parent chain from the failing component toward
the root. The first active Error Boundary wins.

For nested boundaries:

```text
OuterBoundary
  InnerBoundary
    FailingChild
```

`InnerBoundary` captures `FailingChild`. `OuterBoundary` remains healthy.

## Fallback Rendering

After capture, the failing render returns a deterministic empty node for that
pass. The boundary is marked dirty, pending effects under the protected subtree
are cancelled, and the next patch replaces the protected subtree with fallback
UI.

The fallback receives the captured `ErrorInfo` and a reset callback:

```go
Fallback: func(ctx gf.ErrorBoundaryContext) gf.Node {
    return gf.El("section", nil,
        gf.Text(ctx.Info.Component),
        gf.El("button", gf.Props{"OnClick": ctx.Reset}, gf.Text("Retry")),
    )
}
```

The first incident wins while the boundary is failed. Further descendant
render failures before reset do not replace the saved incident, although each
actual failing render is still reported through the global handler.

## Manual Reset

`ctx.Reset()`:

- is idempotent;
- is a no-op after the boundary unmounts;
- clears the captured error;
- advances an internal generation key;
- remounts the protected subtree fresh;
- does not report an error by itself.

If the retried subtree panics again, that is a new incident and is reported as
a new render failure.

## ResetKey

`ResetKey` is an optional string. When it changes while the boundary is failed,
the boundary automatically clears the incident and remounts the protected
subtree.

Changing `ResetKey` while the boundary is healthy updates the stored key but
does not remount the subtree by itself. Apps that want route-aware reset
behavior can pass a route path, route id, or version string explicitly.

`ResetKey` is intentionally string-only. There is no `[]any`, deep equality,
reflection, or automatic router subscription.

## Nested Boundaries

Nested boundaries are ordinary components. The nearest active boundary catches
descendant render failures first.

If an inner boundary fallback panics, that panic happens while rendering the
inner boundary itself. The inner boundary does not catch its own fallback. The
runtime reports the new render failure and lets the nearest outer boundary
capture it. If there is no outer boundary, the default render fallback applies.

## Cleanup And Lifecycle Guarantees

Boundary fallback must not leave partially committed lifecycle work behind.

When a descendant render failure is captured:

- pending effects under the protected subtree are cancelled before effect
  flushing;
- previous successful effect cleanups run once when the protected subtree is
  replaced by fallback;
- failed subtree state, effect, unmount, and context slots are released when
  reconciliation unmounts that subtree;
- DOM event listeners and `js.Func` handlers in the failed subtree are removed
  by the normal mounted tree release path;
- queued dirty updates for inactive failed descendants are ignored;
- dirty descendant accounting is cleared during component deactivation.

This is why the boundary changes the subtree through normal reconciliation
rather than keeping hidden failed DOM or a global boundary registry.

## Router Integration Pattern

The router does not install boundaries automatically. Route handlers can wrap
their page content manually:

```go
func issueRoute(ctx gf.RouteContext) gf.Node {
    return gf.ErrorBoundary(gf.ErrorBoundaryProps{
        ResetKey: ctx.Path,
        Fallback: routeFallback,
        Children: []gf.Node{
            pages.Issue(pages.IssueProps{
                ID: ctx.Param("id"),
            }),
        },
    })
}
```

Apps that want a stable shell can still keep the shell outside `RouterView` and
put the boundary inside selected route handlers. This keeps routing and error
UI policy separate. Automatic route-level error pages, loaders, async
resources, and Suspense remain future work.

## TinyGo Panic-Mode Matrix

Recover-based containment requires a recover-capable build:

| Build path | Boundary containment |
|---|---|
| Go unit tests | Covered |
| Standard Go/WASM | Covered by browser smoke |
| Default TinyGo package path with `panic=trap` | API compiles, recover behavior is not guaranteed |

GoFrame's size-oriented TinyGo smoke builds use trap-style panic lowering.
They should not be treated as proof that runtime recover handlers execute.

## Limitations

- Render-path only.
- No full Error Boundary component API beyond fallback and reset.
- No route-level automatic boundary.
- No event/effect/cleanup containment through boundaries.
- No async resources or Suspense-like model.
- No stack trace parsing or source-map integration.
- No production crash-reporting integration.
- No special TinyGo recover-capable packaging profile yet.

## Future Work

Future stages may consider:

- route-level boundary composition helpers after router needs are clearer;
- recover-capable TinyGo package mode if the size tradeoff is acceptable;
- richer app diagnostics or debug probes;
- optional crash reporting integration points;
- async resource/error semantics, if GoFrame later adds a resource model.
