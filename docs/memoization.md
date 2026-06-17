# Component Memoization

`goframe` supports **explicit, opt-in** component memoization.
It is intentionally narrow for MVP 14: there is no automatic comparison,
no reflection, and no deep equality by default.

## API

Implement this method on your props type:

```go
func (props MyRowProps) MemoEqual(next MyRowProps) bool {
    return props.ID == next.ID &&
        props.Selected == next.Selected &&
        props.Status == next.Status
}
```

Then `goframe` may skip rerendering that component when all of the following are true:

- component name/key identity matches
- the previous and next props satisfy `MemoEqual`
- the component instance is not dirty from its own local state/effects
- no dirty descendant is waiting inside the component subtree

## Why explicit only

Automatic memoization is hard to predict and can create subtle behavior changes.
MVP 14 uses explicit `MemoEqual` so authors decide the comparator semantics.

## Function props

`MemoEqual` is user-defined. If you intentionally ignore function props, event
handlers may remain the previous callback until the component is rendered again.
This is correct only when the ignored callback is stable or when another prop
forces a rerender whenever the callback's captured data changes.

Safe patterns:

- use `gf.UseReducer` dispatch for state-changing callbacks so old DOM
  handlers apply actions to the latest state slot;
- include a callback/data version token in props and compare it in `MemoEqual`
  when callbacks cannot be expressed as reducer actions;
- compare a stable command ID or owner version instead of comparing functions;
- include data fields that the callback closes over.

The dashboard example uses `gf.UseReducer` for issue dataset changes. `IssueRow`
callbacks dispatch typed actions, and dispatch reads the latest issue slice from
the component state slot at event time. This lets `IssueRowProps.MemoEqual`
ignore function props without keeping stale issue data in old handlers.

Do not ignore function props when the callback closes over changing data and no
other compared prop tracks that data.

## Limits today

- No automatic memoization for all components.
- No reflection-based comparison.
- No `unsafe`, no generated equality wrappers, no deep auto-compare.
- No context/selectors as part of memoization strategy.
- No stable event callback hook yet; reducer dispatch covers state transitions
  that can be represented as pure actions.
- No virtualization or router/Player integration in this stage.

## Interaction with keys and identity

Memoized skip applies within the same component identity boundary:
component name + key + same instance. Key or component identity changes still
remount/recreate as normal.

## Dirty descendants

A memoized component is not allowed to skip when a descendant component is dirty.
This matters with dirty queue ancestor pruning:

```txt
Parent dirty
  MemoChild props equal
    GrandChild dirty from local state
```

The flush queue may prune `GrandChild` because `Parent` will patch its subtree.
`MemoChild` must still render/patch through to the dirty grandchild, even though
its own props compare equal. The runtime tracks dirty descendants on component
ancestors and blocks the memo skip until those descendant updates are rendered
or the subtree is unmounted.

## What memoization does not change

- Dirty subtree scheduling and patch mechanics are unchanged.
- Effects and unmount cleanups remain governed by component lifecycle.
- Memoization does not override `UseState`-driven rerenders when a component
  is marked dirty.
