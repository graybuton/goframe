# Context Selectors

MVP 16 adds scoped context with selector-based consumption. It is not a global
store, router context, Redux clone, or deep comparison system.

## API

```go
type Context[T any]

func CreateContext[T any](defaultValue T) *Context[T]
func ProvideContext[T any](ctx *Context[T], value T)
func UseContext[T any](ctx *Context[T]) T
func UseContextSelector[T any, S comparable](
    ctx *Context[T],
    selector func(T) S,
) S
```

## Provider Hook

Providers are ordinary components that call `ProvideContext` during render:

```go
var PreferencesContext = gf.CreateContext(defaultPreferences())

func PreferencesProvider(props PreferencesProviderProps) gf.Node {
    preferences, dispatch := gf.UseReducer(defaultPreferences(), reducePreferences)

    gf.ProvideContext(PreferencesContext, preferences)

    return (
        <section>
            <PreferencesControls Dispatch={dispatch} />
            {props.Children}
        </section>
    )
}
```

This avoids a new GOX provider syntax. Scope follows the mounted component tree
through parent links. Descendant consumers read the nearest provider. Nested
providers override outer providers for their own descendants.

`ProvideContext` must be called during component render. Calling it outside
render panics with a focused message.

## Selectors

Use selectors when a consumer only needs one comparable part of the context:

```go
density := gf.UseContextSelector(PreferencesContext, func(value Preferences) string {
    return value.Density
})
```

The selector result type `S` must be `comparable`. The runtime compares the
previous selected value and the next selected value with `==`. It does not use
reflection, `unsafe`, or deep equality.

Selectors should be deterministic and side-effect-free. They may close over
stable values, but they should not mutate state or depend on changing external
state.

If a selector panics during provider notification or provider topology refresh,
GoFrame reports `gf.ErrorPhaseContext`, keeps the previous selected value, and
does not mark the consumer dirty from that failed selector evaluation. If a
selector panics during the consumer's own render before a stable previous value
exists, the panic is reported as a context failure and then flows through normal
component render error handling. A nearest scoped `gf.ErrorBoundary` can catch
that initial render-path failure, but it does not catch later provider
notification failures.

## Broad Consumers

`UseContext(ctx)` returns the full nearest value. Because the runtime cannot
compare arbitrary `T` without reflection or a user-provided equality function,
`UseContext` subscribes broadly. Provider updates rerender broad consumers.

Use it for low-frequency or simple consumers. Prefer `UseContextSelector` for
components on hot update paths.

## Update Mechanics

Each provider component stores its current context values. Each consumer stores
its context subscriptions in positional context slots. When a provider updates:

1. The provider value is updated during provider render.
2. Selector subscribers under that provider recompute their selected value.
3. Only subscribers whose selected value changed are marked dirty.
4. Dirty descendant accounting prevents memoized ancestors from hiding those
   consumer updates.

Consumers under nested providers subscribe to the inner provider, not the outer
provider. Unmounted consumers unsubscribe, and unmounted providers detach their
subscribers.

Provider topology changes are observable. If a provider appears above an
existing consumer, disappears, or a nearer nested provider appears between an
outer provider and a consumer, affected consumers are marked dirty and resubscribe
to the new nearest provider on their next render. This happens even when a
selector's comparable result is equal, because the provider scope itself changed.

## Memoization Interaction

Context selectors are designed to work with explicit `MemoEqual` bailouts.

If a parent/provider rerenders, clean memoized descendants may skip. A context
consumer whose selected value changed is marked dirty, so memoized ancestors
above it cannot skip over the update.

Memoized ancestors also cannot hide context topology changes. When a provider
appears, is removed, or a nearer nested provider takes over, the runtime marks
affected consumers dirty and updates dirty descendant accounting through any
memoized wrappers above them.

This means selector consumers should usually be component boundaries, and clean
structural wrappers can use explicit `MemoEqual` where useful.

## Example

`examples/context` demonstrates:

- scoped provider hook;
- density, accent, and counter selector consumers;
- a broad `UseContext` consumer;
- nested provider override;
- memoized static leaves;
- browser smoke assertions that only the selected consumer rerenders.

Run it locally:

```bash
goxc package ./examples/context --compiler=tinygo
goxc serve ./examples/context --port=8080
```

## Limitations

- Selector results must be comparable.
- There is no custom equality function for non-comparable selections yet.
- Context is synchronous and render-time only.
- No server context, async context, or context devtools.
- Context is not a global store; provider scope is the component tree.
- Context selectors do not replace reducer dispatch, explicit memoization,
  virtualization, or the hash router's route state.
