# Runtime model

This document describes the current MVP runtime. It is a small component, DOM
reconciliation, and lifecycle foundation, not React Fiber or a production
effect scheduler.

## From DOM identity to component identity

MVP 7 removed full-root DOM replacement. It retained DOM nodes and patched
text, props, events, positional children, and keyed children. However, every
state update still rebuilt the root virtual tree. Direct component function
calls meant that unchanged components such as `Header` were called again and
their virtual subtrees were visited by the patch traversal.

These are different costs:

- DOM node replacement discards browser node identity;
- DOM mutation changes an existing browser node;
- virtual rerender calls Go render functions and builds nodes;
- patch traversal compares old and new virtual subtrees;
- component render calls one component function.

MVP 8 adds explicit component boundaries so a local state update can avoid all
five costs outside the dirty component subtree.

## ComponentNode

GOX capitalized tags generate typed `gf.ComponentT` nodes:

```go
var _goxComponent_app_Header = gf.NewComponentType("main.Header", "Header")

gf.ComponentT(_goxComponent_app_Header, HeaderProps{
	Title: "Demo",
}, Header)
```

`gf.ComponentT` receives a `ComponentType` identity token plus props and a
deferred render function. The debug name remains human-readable for probes,
but the runtime reuse decision uses the typed identity token. The function is
not called until the runtime mounts or rerenders the component. Lowercase tags
still generate `gf.El`.

The legacy `gf.Component(name, props, render)` helper remains supported for
handwritten Go and older generated code. Its identity is namespaced separately
from typed identities, so `gf.Component("Header", ...)` does not accidentally
reuse `gf.ComponentT(gf.NewComponentType("Header", "Header"), ...)`.

Direct Go calls such as `Header(HeaderProps{...})` remain valid, but they
execute immediately and do not create component identity or a separate state
scope.

## Component instances and identity

Each mounted component boundary owns a component instance:

```text
component instance
  -> component identity, debug name, and optional key
  -> latest props
  -> component-scoped state slots
  -> context provider values and selector subscriptions
  -> dirty flag
  -> mounted child subtree
```

An instance is reused when the component identity matches and its sibling
identity matches. Unkeyed components use position. `gf.Key` or `gf.WithKey`
gives a component stable keyed identity across sibling reorder and removal.
Changing a component identity or key creates a new instance and unmounts the
old subtree.

GOX-generated identity uses a typed component token. When `goxc` can determine
the package import path, generated ids use import-path-aware identity such as
`github.com/graybuton/goframe/examples/multipackage/internal/ui.Header`.
Direct `GenerateNamed` calls can still fall back to package-name identity such
as `main.Header`. The runtime does not compare Go function identities.

Component boundaries use stable start and end comment anchors. Their mounted
range therefore remains valid even if a component changes its rendered root
from an element to a fragment or another component.

## Component-scoped state

The root `App` is mounted as a component instance, and every `gf.Component`
boundary creates another state scope.

During component render, the runtime tracks the current component and resets
its state-slot index. `gf.UseState(initial)` reads or creates the next slot in
that component and returns the current value plus a setter:

```go
count, setCount := gf.UseState(0)
setCount(count + 1)
```

Slots survive later renders of the same instance.

```text
current component
  -> state slot 0
  -> state slot 1
  -> ...
```

State calls remain positional within each component. They must run in a stable
order. Calling `UseState` outside a component render panics with a focused
message. Setting the same supported primitive value is a no-op; slices and
other composite values conservatively schedule an update.

`gf.UseReducer(initial, reducer)` uses the same component-scoped state slots,
but returns a dispatch function instead of a setter:

```go
type Reducer[S any, A any] func(state S, action A) S

issues, dispatchIssues := gf.UseReducer(resetDemoIssues(), reduceIssues)
dispatchIssues(IssueAction{Kind: IssueActionToggle, ID: id})
```

Dispatch reads the latest slot state and latest reducer function at dispatch
time. This means an old event handler retained by a memoized child can still
apply an action to current state rather than to a value captured during an
older render. Reducers should be pure state transitions; they should not depend
on mutable render-local captures unless that coupling is intentional.

## Scoped context selectors

Context values are scoped by component parent links:

```go
ctx := gf.CreateContext(defaultValue)

gf.ProvideContext(ctx, value)
selected := gf.UseContextSelector(ctx, func(value T) S {
	return value.Field
})
```

`ProvideContext` is a render-time hook on a normal component. It stores a typed
value on that provider component instance for descendants. The nearest provider
wins, and nested providers isolate their subscribers from outer provider
updates.

`UseContextSelector` stores a selector subscription in positional context slots.
The selected result type must be `comparable`; the runtime uses `==` to decide
whether the consumer should be marked dirty. There is no reflection, deep
equality, or generated selector equality.

`UseContext` returns the full nearest value and subscribes broadly. Since the
runtime cannot compare arbitrary context values without reflection, broad
consumers rerender on provider updates. Performance-sensitive code should use
selectors.

When a provider updates, selector consumers are checked immediately. Consumers
whose selected value changed are marked dirty, and dirty-descendant accounting
prevents memoized ancestors from skipping over those updates. Unmounted
consumers unsubscribe, and unmounted providers detach their subscriber set.

See [context selectors](context.md) for API guidance and limitations.

## Dirty subtree updates

`State.Set` updates its slot immediately, marks the owning component dirty, and
queues that instance. Multiple updates before the next
`requestAnimationFrame` are coalesced.

At flush time, the runtime rerenders each dirty component and patches only its
mounted child subtree:

```text
State.Set
  -> mark owner component dirty
  -> schedule animation-frame flush
  -> call owner render function
  -> patch owner mounted subtree
```

For example, Todo input state belongs to `TodoForm`. Typing rerenders and
patches `TodoForm`; it does not call or traverse `Header` or the root `App`.

Before a flush, the dirty queue is pruned. If a dirty component has a dirty
ancestor in the same queue, the child update is removed because the ancestor
patch will already cover that subtree:

```text
[Parent, Child, GrandChild, Sibling] -> [Parent, Sibling]
```

Unmounted dirty instances and duplicate queue entries are skipped safely.

If a parent rerenders because its props or state changed, component descendants
encountered in that subtree rerender so new props are always applied. Unrelated
ancestors and sibling component subtrees are not traversed.

## Props equality

MVP 8.1 deliberately removes automatic props equality from the runtime hot
path. `reflect.DeepEqual` increased Go and TinyGo bundles, treated function
props poorly, and made skip behavior harder to reason about.

Correctness uses a simpler rule:

- a component rerenders when its own state marks it dirty;
- a component rerenders when its parent subtree is rerendered and reaches it;
- unrelated ancestors and siblings are untouched because dirty updates begin
  directly at the owner component.

Applications should introduce component boundaries around independently
changing state. The Todo example keeps Header outside `TodoApp`, keeps todo
state in `TodoApp`, and keeps input text state in `TodoForm`.

## DOM patching

Component subtree updates reuse the MVP 7 mounted DOM patcher:

- changed text updates an existing text node's `nodeValue`;
- same-tag elements retain their DOM node and patch props;
- one stable event listener is retained per event name while its current Go
  callback is updated;
- unkeyed children patch by position;
- keyed children reuse and move mounted DOM ranges;
- fragments and component boundaries use start and end comment anchors;
- incompatible node kinds, tags, component identities, or keys are replaced.

Stable input DOM identity means controlled typing normally preserves focus and
cursor position. Stable-ID focus restoration remains a replacement fallback.

`patchChildren` checks whether a mounted DOM range is already directly before
its required reference. It calls `insertBefore` only when an actual move is
needed.

## Virtualized collections

`gf.VirtualList` and `gf.VirtualTable` are framework-level fixed-height
collection primitives. They keep the mounted DOM bounded to the visible window
plus overscan while preserving a much larger logical item count.

The virtual range stores the first visible row rather than raw scroll pixels.
Scroll events that remain inside the same row boundary do not schedule another
state update. When the row boundary changes, the virtualized component updates
its range and keyed reconciliation mounts, unmounts, or moves only the window
that should be present.

Virtualization and memoization solve different costs. Memoization skips render
and patch work for clean mounted components; virtualization avoids mounting
offscreen components at all. The dashboard uses both: `IssueRow` remains
memoized for row selection/toggle work, while `gf.VirtualTable` prevents
filter transitions from creating hundreds of offscreen rows.

The current virtualized model assumes fixed item or row height. Dynamic
measurement, infinite loading, keyboard navigation, and richer table
accessibility are future work. See [virtualized collections](virtualization.md).

## Client routing

MVP 24 adds a small hash-based client router for browser/WASM apps. Routes are
declared in Go and matched by normalized hash path:

```go
var router = gf.NewHashRouter([]gf.Route{
	gf.RoutePath("/", homeRoute),
	gf.RoutePath("/issues/:id", issueRoute),
	gf.NotFoundRoute(notFoundRoute),
})
```

`gf.RouterView(router)` is a component boundary. It reads the current hash,
subscribes to `hashchange`, and renders the matched route handler. The route
subtree is keyed by route pattern. Navigating between different patterns
remounts the route subtree; navigating between the same pattern with different
params updates the route context and may preserve route-local state.

The recommended layout model is a stable shell component with `RouterView` as
an outlet. There is no nested route DSL, file-based routing, path/history-mode
server fallback, route loader system, or route-level Error Boundary API in this
MVP. See [client router](router.md).

## DOM stability regression

DOM node replacement, component render, patch traversal, DOM mutation, and
browser repaint are different events. MVP 8.1's Todo typing regression test
uses node references, MutationObserver, and wrapped DOM APIs.

For one input character after TodoList is mounted, the measured result is:

| Measurement | Result |
|---|---:|
| root childList mutations | 0 |
| Header mutations | 0 |
| TodoList mutations | 0 |
| TodoForm childList mutations | 0 |
| createElement/createTextNode | 0 |
| appendChild/removeChild/replaceChild/insertBefore | 0 |
| addEventListener/removeEventListener | 0 |

The dirty update starts at `TodoForm`. Its unchanged DOM remains in place.
Paint Flashing is not directly automated; the absence of structural mutations
and DOM operations is the regression proxy.

## Lifecycle and effects

MVP 9 adds component-scoped lifecycle slots next to state slots. MVP 10 makes
the public API terser:

```go
gf.UseEffect(func() gf.Cleanup { ... })
gf.UseEffect(func() gf.Cleanup { ... }, gf.Deps(value, count))
gf.UseEffect(func() gf.Cleanup { ... }, gf.EveryRender())
gf.UseUnmount(func() { ... })
```

Lifecycle hooks are collected during component render and flushed only after
the mounted DOM subtree has been patched. Effects therefore do not run while
the runtime is building the virtual tree.

`UseEffect(fn)` runs once after the component instance is first mounted. Its
cleanup runs on unmount. `UseUnmount` registers cleanup-only work.
`UseEffect(fn, gf.Deps(...))` runs after mount and after explicit dependency
changes; previous cleanup runs before the next effect body and again on
unmount. `UseEffect(fn, gf.EveryRender())` runs after every component render.

Dependencies are intentionally explicit and lightweight:

```go
gf.Deps(value, count)
gf.EveryRender()
```

The runtime accepts primitive dependency values through a type switch and does
not use `reflect.DeepEqual`. Unsupported complex values panic during render;
applications reduce them to primitive dependency tokens.

Unmount cleanup is tied to the existing reconciliation paths: conditional
removal, keyed list removal, key/name replacement, fragment subtree removal,
and application replacement. State updates after unmount are safe no-ops in
production and debug warnings in `goframe_debug` builds.

State updates during render are still scheduled safely, but debug builds warn
because repeated render-time updates can create loops. Effect-triggered update
loops also have a small debug guard that reports and stops pending updates
after the guard threshold.

See [lifecycle and effects](effects.md) for API details.

## Runtime error semantics

MVP 23 adds a small runtime error reporting model for recoverable user-code
panics:

```go
restore := gf.SetErrorHandler(func(info gf.ErrorInfo) {
    // app logging, tests, or browser diagnostics
})
defer restore()
```

The handler receives a phase, component debug name, operation label, and the
recovered panic value. Event handler panics are reported and contained so the
listener remains installed and the app stays interactive. Effect setup,
effect cleanup, and `UseUnmount` cleanup panics are also reported and
contained, with later cleanup work continuing where possible.

Memo comparator panics report `ErrorPhaseMemo` and fall back to "do not skip
render." Component render panics report `ErrorPhaseRender` and render
`gf.Empty()` for that pass. Future state or parent updates may retry the
component. Context selector panics during provider notification keep the
previous selected value; selector panics during initial render report and flow
through render containment.

Runtime invariant panics whose message starts with `goframe:` remain hard
programmer errors. Examples include invalid hook order, calling hooks outside
render, unsupported effect dependency types, invalid component types, and
invalid virtualization dimensions.

This is not a full Error Boundary API. There is no component-level fallback UI,
route-level error page, async resource model, or production crash reporting
integration yet. The current TinyGo package path uses trap-style panic lowering,
so recover-based containment is only available in recover-capable builds such
as Go/WASM and Go tests. See [runtime error semantics](runtime-errors.md).

## Debug probes

Render, patch, and performance probes compile only with the `goframe_debug`
build tag:

```js
window.goframeComponentRenderProbe = (name) => {
	console.log("render", name);
};
```

Production builds use no-op stubs and do not include the browser probe code.

Duplicate sibling key diagnostics also compile only with `goframe_debug`.
Warnings are written to `console.warn` and recorded in
`globalThis.goframeDuplicateKeyWarnings` for browser smoke tests.

## Browser smoke test

Build the instrumented Todo WASM and serve it on port `18080`:

```bash
(cd ./examples/todo/.goframe/work/dev/examples/todo && \
  tinygo build -target=wasm -no-debug -panic=trap -tags=goframe_debug \
    -o ../../../../package/standalone/assets/bundle.wasm .)
goxc serve ./examples/todo --port=18080
node --experimental-websocket scripts/todo-browser-smoke.mjs
```

The dependency-free headless Chrome probe verifies:

- root, Header, input, and keyed Todo DOM identity;
- root, Header, TodoList, and TodoForm MutationObserver results;
- structural DOM operation and event-listener churn counters;
- focus and controlled input behavior;
- `Header` render count remains unchanged during Todo interactions;
- a single event callback after repeated updates;
- keyed reorder, removal, text, and props patching.

## Current limitations

- one mounted application and one browser thread;
- positional state and lifecycle slots require stable hook call order;
- lifecycle/effects are minimal and have no priorities or async scheduler;
- context is scoped and selector-based, but has no async/server bridge or
  custom non-comparable selector equality;
- virtualized collections are fixed-height only and do not include dynamic
  measurement, infinite loading, or keyboard navigation;
- hash routing only; no path/history-mode server fallback, file-based routing,
  route loaders, or nested route layout DSL;
- no error boundaries;
- GOX-generated component identity uses a typed token derived from package
  import path when `goxc` knows it, with package-name fallback for lower-level
  generation helpers. Legacy `gf.Component` still uses string identity.
- explicit opt-in props memoization via `MemoEqual`; components still rerender
  by default when props are not memoized.
- dirty component scheduling has no priorities, interruption, or concurrency;
- duplicate-key diagnostics are debug-only and do not include source locations;
- no SSR or hydration;
- patching assumes the managed DOM subtree is not mutated externally.
