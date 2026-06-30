# Component Identity

This document defines the first-preview component identity contract.
It distinguishes stable usage guidance from experimental edge-case behavior.

## What is stable enough for preview users

For the first public preview, the following behavior is considered stable enough to rely on:

- Handwritten components can still use function-call style:
  `Header(HeaderProps{...})`.
- Direct Go calls are ordinary function composition and are not runtime component
  boundaries.
- They do not create independent component state/effect/context scope, and they do
  not create an independent dirty-update target.
- Generated GOX boundaries use explicit typed identity:
  `gf.ComponentT(gf.NewComponentType("importpath.Symbol", "Debug"), props, Header)`.
- Identity defaults are derived from canonical component import/package path plus symbol name.
- Import aliases are diagnostic labels and `gf.ComponentT` debug names, not identity keys.
- Package-qualified GOX tags (`<layout.Shell />`, `<ui.Header />`) are stable
  inside the current app/workspace model when imports resolve to a package
  identity.
- Typed identity can be combined with keys (`gf.Key` / `Key`) for explicit list locality control.

GoFrame examples and docs rely on these assumptions when composing
multi-package trees inside one app/root. The `multipackage` and `cmdapp`
examples exercise app/workspace composition; they do not claim a stable
reusable component package ecosystem across independently versioned modules.

## Preview contract by API kind

### `gf.Component`

- API shape remains public for compatibility and continues to exist.
- Its identity is the legacy string-based namespace.
- It is accepted for existing examples and manual wiring.
- It is a compatibility path, not the recommended basis for stable cross-package identity assertions.

### `gf.ComponentT`, `gf.NewComponentType`, generated GOX code

- API shape is public-candidate for the preview phase.
- Generated token identity is the intended way to reason about component identity.
- Two tags that resolve to the same import path and symbol share identity.
- Debug names can differ from import alias without changing runtime identity.
- Variable naming in generated code is not part of runtime identity.

### Package-qualified tags and `PackageIdentity`

- Generated tags use package import resolution from GOX selectors and the supplied
  `PackageIdentity` in `gox` generation options when explicit.
- For cross-package composition in the same app, this gives predictable reuse and avoids local alias collisions.
- `PackageIdentity` exists to make emitted identity deterministic where default resolution
  cannot infer module-relative canonical package import path.

## Stable preview expectations

For users running one module/application tree:

- Same canonical component identity + same key preserves component instance.
- Different package-qualified identity, even with same symbol name, does not.
- Duplicate symbol names in different packages do not collide.
- Same package path with alias changes (`ui.Header` vs `widgets.Header`) should not alter identity.
- Direct `v2` path suffixes are part of the identity key.

## Experimental frontier under preview

These areas are experimental or only partially evidenced. Current preview
evidence covers single-module app/workspace composition; broad multi-module or
reusable package identity is outside the current preview contract.

- Multi-module or module-reused component package identity.
- Identity under `replace`, workspace aliasing, and non-trivial module path churn.
- Remount behavior for module/version and package relocation edges.
- Legacy `gf.Component` behavior as a full cross-package identity strategy.
- Reusing generated component tokens across independently built package sets.

### Practical implication

The API shape is usable and documented. The deeper contract under module/path
edges remains experimental and requires dedicated design/test evidence before
being claimed.

## Remount and state expectations

- Preserve state when identity and key are stable across renders.
- Remount is expected when identity changes or key changes.
- A module or import path change is treated as an identity change and may remount.
- Render-time component renaming changes identity.
- Stateful components without keys can still remount when sibling position changes.

## Public preview recommendation

Use package-qualified GOX tags and typed components for non-trivial, multi-file
apps.
Use `gf.ComponentT` for runtime-visible typed boundaries, and keep
`gf.Component` as a compatibility path for direct composition.
Use direct function calls only for ordinary helper composition.
Do not assume stable behavior across module path edits, module replacement,
or cross-module ecosystem sharing in the current preview contract.

## Risks and non-goals

- No new migration layer is introduced in preview.
- No broader reusable package ecosystem promise.
- No hidden identity migration protocol.
- No production/1.0 compatibility commitment.

## Evidence

- `pkg/goframe/component.go`
- `pkg/goframe/component_identity_test.go`
- `pkg/gox/generate_test.go`
- `pkg/goxc/workspace_test.go`
