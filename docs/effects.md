# Lifecycle and Effects

MVP 9 adds a minimal lifecycle model for component-owned side effects. It is
designed for Go-first applications, browser/WASM size constraints, and clear
cleanup semantics. It is not React Fiber, Suspense, or a general lifecycle
framework.

Effects are collected while a component renders and are flushed only after the
runtime has patched the DOM. A state update from an effect schedules a later
browser update instead of mutating the tree during render.

## API

```go
type Cleanup func()

func UseMount(effect func() Cleanup)
func UseUnmount(cleanup Cleanup)
func UseEffect(effect func() Cleanup, deps Deps)
```

Dependencies are explicit comparable values:

```go
func NoDeps() Deps
func AlwaysDeps() Deps
func DepsOf(values ...Dep) Deps

func DepString(value string) Dep
func DepBool(value bool) Dep
func DepInt(value int) Dep
func DepInt64(value int64) Dep
func DepUint(value uint) Dep
func DepUint64(value uint64) Dep
func DepFloat64(value float64) Dep
```

The runtime intentionally does not accept `[]any` dependencies and does not use
reflection or deep equality. Complex values should be reduced to explicit
strings, counters, IDs, or other primitive dependency values by the
application.

## UseMount

`UseMount` runs once after the component instance is first mounted:

```go
gf.UseMount(func() gf.Cleanup {
    println("mounted")
    return func() {
        println("unmounted")
    }
})
```

The returned cleanup runs when the component instance unmounts. A key or
component-name change creates a new instance, so the mount effect runs again.

## UseUnmount

`UseUnmount` registers cleanup without a mount body:

```go
gf.UseUnmount(func() {
    println("released")
})
```

The latest cleanup registered at that hook position runs on unmount.

## UseEffect

`UseEffect` runs after mount and after explicit dependency changes:

```go
value := text.Get()

gf.UseEffect(func() gf.Cleanup {
    documentTitleSet("Todo: " + value)
    return nil
}, gf.DepsOf(gf.DepString(value)))
```

When dependencies change, the previous cleanup runs before the next effect
body. The latest cleanup also runs on unmount.

`NoDeps()` means run once after mount. `AlwaysDeps()` means run after every
component render.

## Cleanup Timing

Cleanup runs when a component instance is removed through normal reconciliation
paths:

- a conditional component disappears;
- a keyed component is removed from a list;
- a component key changes;
- a component name changes;
- a fragment subtree containing a component is removed;
- a mounted application is replaced.

Unmount cleanups run while the DOM range still exists, but applications should
not depend on this detail. Treat cleanup as the place to release timers,
external event listeners, subscriptions, and retained browser resources.

## Hook Slots

Effects use component-scoped positional hook slots, just like `UseState`.
Calls must stay in a stable order between renders.

Calling lifecycle hooks outside component render panics with a focused message.
Changing a lifecycle hook kind at the same effect slot also panics.

## Debug Safety

Production builds keep lifecycle diagnostics as no-op stubs.

`goframe_debug` browser builds warn when:

- `State.Set` is called after component unmount;
- `State.Set` is called during component render;
- an effect-triggered update loop exceeds the MVP guard threshold.

The loop guard is intentionally small. It prevents obvious effect-to-state
runaway loops from continuing forever in debug builds, but it is not a
priority scheduler.

## Todo Persistence

The Todo example uses effects to persist tasks:

- `UseMount` loads compact localStorage state after first mount;
- `UseEffect` writes localStorage only when the encoded todo list changes;
- encoding lives in the example, not in `pkg/goframe`, so the runtime does not
  import `encoding/json`.

## Limitations

- No `UseEffect` dependency inference.
- No cleanup ordering guarantees beyond each component instance cleaning its
  own registered hooks.
- No lifecycle hooks for before-render or before-patch.
- No component memoization.
- No context, router integration, SSR, hydration, or async component model.
- Hook order changes are unsupported and may panic only when the slot type
  changes.
