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

GOX capitalized tags generate `gf.Component` nodes:

```go
gf.Component("Header", HeaderProps{
	Title: "Demo",
}, Header)
```

A `ComponentNode` stores the component name, props, and deferred render
function. The function is not called until the runtime mounts or rerenders the
component. Lowercase tags still generate `gf.El`.

Direct Go calls such as `Header(HeaderProps{...})` remain valid, but they
execute immediately and do not create component identity or a separate state
scope.

## Component instances and identity

Each mounted component boundary owns a component instance:

```text
component instance
  -> component name and optional key
  -> latest props
  -> component-scoped state slots
  -> dirty flag
  -> mounted child subtree
```

An instance is reused when the component name matches and its sibling identity
matches. Unkeyed components use position. `gf.Key` or `gf.WithKey` gives a
component stable keyed identity across sibling reorder and removal. Changing a
component name or key creates a new instance and unmounts the old subtree.

Component names should be unique for distinct component implementations. MVP 8
does not compare Go function identities.

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
- incompatible node kinds, tags, component names, or keys are replaced.

Stable input DOM identity means controlled typing normally preserves focus and
cursor position. Stable-ID focus restoration remains a replacement fallback.

`patchChildren` checks whether a mounted DOM range is already directly before
its required reference. It calls `insertBefore` only when an actual move is
needed.

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
tinygo build -target=wasm -no-debug -panic=trap -tags=goframe_debug \
  -o ./examples/todo/dist/assets/bundle.wasm ./examples/todo
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
- no context or error boundaries;
- component identity uses the declared component name, not Go function identity;
- no automatic props memoization;
- dirty component scheduling has no priorities, interruption, or concurrency;
- duplicate-key diagnostics are debug-only and do not include source locations;
- no SSR or hydration;
- patching assumes the managed DOM subtree is not mutated externally.
