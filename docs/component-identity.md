# Component Identity

This document records the current component identity strategy and the options
considered during Foundation Hardening II.

## Current Model

`goframe` currently identifies component instances by:

```text
component name + key/position
```

GOX capitalized tags generate runtime-visible component boundaries:

```go
gf.Component("Header", HeaderProps{
	Title: "Demo",
}, Header)
```

The mounted runtime reuses an existing component instance when:

- the component name is the same;
- the key is the same when a key is present;
- otherwise the sibling position is the same.

Direct Go calls such as `Header(HeaderProps{...})` remain ordinary function
calls. They do not create a component boundary, state scope, or independent
dirty update target.

## Risks

Name-based identity is compact and easy to debug, but it has limits:

- two different component functions can use the same declared name;
- package path is not part of identity;
- the runtime does not know the Go function implementation behind a name;
- renaming a component resets its identity;
- duplicate sibling keys are still a user error, although debug builds now
  warn about them.

These risks are acceptable for the current MVP examples but should be revisited
before larger applications or reusable component packages.

## Alternative A: Keep Name/Key/Position

Pros:

- smallest runtime shape;
- works with current GOX codegen;
- readable debug output;
- no migration cost;
- TinyGo-friendly.

Cons:

- possible collisions across packages or generated code;
- function implementation is not part of identity;
- name changes are identity changes.

This remains the current strategy.

## Alternative B: Generated Stable Type Token

GOX codegen could generate package-aware component tokens:

```go
var _goxComponentHeader = gf.ComponentType("main.Header")

gf.ComponentT(_goxComponentHeader, HeaderProps{
	Title: "Demo",
}, Header)
```

Pros:

- can include package/import context;
- avoids accidental same-name collisions;
- does not require comparing Go function values;
- can preserve readable names for debug output.

Cons:

- changes generated code;
- introduces another public or semi-public runtime API;
- needs a migration story from `gf.Component`;
- may increase bundle size if token metadata grows.

Recommendation: this is the most promising next identity improvement, but it
should be introduced as an optional, non-breaking foundation after current
regression gates are stable.

## Alternative C: Function Identity

Using Go function identity directly is not recommended now.

Go function values are not comparable except to `nil`. Reflection or unsafe
runtime tricks would be needed to derive implementation identity, and both are
bad fits for this project:

- `reflect` is intentionally kept out of the production runtime;
- unsafe/runtime internals are risky across Go and TinyGo;
- TinyGo compatibility is uncertain;
- the likely size cost is not worth it for the current MVP.

## Recommendation

Stay with name/key/position for now, with these hardening rules:

- keep component names stable and unique within an application;
- use keys for reordered lists;
- treat duplicate keys as a bug;
- keep duplicate-key diagnostics in `goframe_debug` builds only;
- revisit generated stable type tokens before adding reusable component package
  workflows.

## Migration Plan For Tokens

If generated type tokens become necessary:

1. Add a small optional token API while keeping `gf.Component`:

   ```go
   type ComponentType struct {
   	name string
   }
   ```

2. Add `gf.ComponentT` or a compatible overload-style helper.
3. Update GOX codegen to emit tokens behind a feature gate or versioned mode.
4. Keep string-based `gf.Component` as a compatibility API.
5. Measure TinyGo sizes before making token codegen the default.

## Size Considerations

Any identity strategy must preserve the current size posture:

- no `reflect`;
- no `unsafe` runtime identity tricks;
- no large metadata tables;
- debug diagnostics behind `goframe_debug`;
- measured TinyGo budget checks before and after the change.
