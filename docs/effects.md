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

func UseUnmount(cleanup Cleanup)
func UseEffect(effect func() Cleanup, deps ...EffectDeps)

type EffectDeps struct {
    // internal lightweight representation
}
```

`UseEffect(fn)` runs once after mount. Dependencies are explicit primitive
values:

```go
func Deps(values ...any) EffectDeps
func Once() EffectDeps
func EveryRender() EffectDeps
```

`Deps` accepts strings, booleans, signed and unsigned integer types, floats,
and nil. Unsupported dependency types panic during render with a focused
message. The runtime intentionally does not use reflection or deep equality.
Complex values should be reduced to strings, counters, IDs, versions, or other
primitive dependency values by the application.

Deprecated compatibility helpers (`UseMount`, `NoDeps`, `AlwaysDeps`,
`DepsOf`, and `Dep*`) may remain during the experimental cleanup period, but
new code should use the API above.

## Run Once After Mount

Call `UseEffect` without dependencies:

```go
gf.UseEffect(func() gf.Cleanup {
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

## Dependency Effects

`UseEffect` runs after mount and after explicit dependency changes:

```go
value := text

gf.UseEffect(func() gf.Cleanup {
    documentTitleSet("Todo: " + value)
    return nil
}, gf.Deps(value))
```

When dependencies change, the previous cleanup runs before the next effect
body. The latest cleanup also runs on unmount.

`gf.EveryRender()` means run after every component render:

```go
gf.UseEffect(func() gf.Cleanup {
    println("rendered")
    return nil
}, gf.EveryRender())
```

## Cleanup Timing

Cleanup runs when a component instance is removed through normal reconciliation
paths:

- a conditional component disappears;
- a keyed component is removed from a list;
- a component key changes;
- a component identity changes;
- a fragment subtree containing a component is removed;
- a mounted application is replaced.

Unmount cleanups run while the DOM range still exists, but applications should
not depend on this detail. Treat cleanup as the place to release timers,
external event listeners, subscriptions, and retained browser resources.

If an effect body panics, the runtime reports `gf.ErrorPhaseEffect` through the
installed runtime error handler and does not register a cleanup for that failed
effect run. If an effect cleanup panics, the runtime reports
`gf.ErrorPhaseEffectCleanup`, clears that cleanup slot, and continues later
cleanup work where possible. `UseUnmount` cleanup panics report
`gf.ErrorPhaseUnmountCleanup` and do not stop other cleanup slots from running.

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

- `UseEffect(fn)` loads compact localStorage state after first mount;
- `UseEffect` writes localStorage only when the encoded todo list changes;
- encoding lives in the example, not in `pkg/goframe`, so the runtime does not
  import `encoding/json`.

## Limitations

- No `UseEffect` dependency inference.
- No cleanup ordering guarantees beyond each component instance cleaning its
  own registered hooks.
- No lifecycle hooks for before-render or before-patch.
- No automatic component memoization in the GOX compiler or runtime. Memoization is
  explicit via `MemoEqual` on component props and is intentionally opt-in.
- No route-aware effect lifecycle, SSR, hydration, or async component model.
- Hook order changes are unsupported and may panic only when the slot type
  changes.
