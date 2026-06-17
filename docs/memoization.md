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

## Why explicit only

Automatic memoization is hard to predict and can create subtle behavior changes.
MVP 14 uses explicit `MemoEqual` so authors decide the comparator semantics.

## Function props

`MemoEqual` is user-defined. If you intentionally ignore function props, event
handlers may remain the previous callback until the component is rendered again.
Design your comparators accordingly.

## Limits today

- No automatic memoization for all components.
- No reflection-based comparison.
- No `unsafe`, no generated equality wrappers, no deep auto-compare.
- No context/selectors as part of memoization strategy.
- No virtualization or router/Player integration in this stage.

## Interaction with keys and identity

Memoized skip applies within the same component identity boundary:
component name + key + same instance. Key or component identity changes still
remount/recreate as normal.

## What memoization does not change

- Dirty subtree scheduling and patch mechanics are unchanged.
- Effects and unmount cleanups remain governed by component lifecycle.
- Memoization does not override `UseState`-driven rerenders when a component
  is marked dirty.
