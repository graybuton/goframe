# Component Identity

This document records the component identity strategy and the options
considered during Foundation Hardening II through MVP 19.

## Current Model

`goframe` has two compatible component identity paths.

Legacy handwritten components identify instances by:

```text
component name + key/position
```

GOX capitalized tags generate runtime-visible component boundaries:

```go
gf.Component("Header", HeaderProps{
	Title: "Demo",
}, Header)
```

Generated GOX now uses explicit component identity tokens:

```go
var _goxComponent_app_Header = gf.NewComponentType("main.Header", "Header")

gf.ComponentT(_goxComponent_app_Header, HeaderProps{
	Title: "Demo",
}, Header)
```

The mounted runtime reuses an existing component instance when:

- the component identity is the same;
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

GOX codegen now emits prototype component tokens:

```go
var _goxComponentHeader = gf.NewComponentType("main.Header", "Header")

gf.ComponentT(_goxComponentHeader, HeaderProps{
	Title: "Demo",
}, Header)
```

Pros:

- can include package/import context;
- avoids accidental same-name collisions;
- does not require comparing Go function values;
- can preserve readable names for debug output.

Remaining cons:

- current ids use package name plus component name, not full import path;
- token variable names are generated-code details;
- full multi-package support still needs a package-aware identity decision;
- may increase bundle size if token metadata grows.

Recommendation: keep the MVP 19 token path as the generated default while
retaining `gf.Component` as compatibility API. Do not claim multi-package
component identity until package-aware ids are designed.

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

Use generated typed identity for GOX, with these hardening rules:

- keep generated identity ids stable within an application;
- use keys for reordered lists;
- treat duplicate keys as a bug;
- keep duplicate-key diagnostics in `goframe_debug` builds only;
- revisit package-aware identity before adding reusable component package
  workflows.

## Migration Plan For Tokens

The current migration path:

1. Keep the optional token API while keeping `gf.Component`:

   ```go
   gf.NewComponentType("main.Header", "Header")
   ```

2. Use `gf.ComponentT` in generated GOX output.
3. Keep string-based `gf.Component` as a compatibility API.
4. Measure TinyGo sizes after token codegen changes.
5. Design import-path or compiler package tokens before multi-package apps.

## Size Considerations

Any identity strategy must preserve the current size posture:

- no `reflect`;
- no `unsafe` runtime identity tricks;
- no large metadata tables;
- debug diagnostics behind `goframe_debug`;
- measured TinyGo budget checks before and after the change.
