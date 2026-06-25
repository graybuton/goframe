# API Stability

## Current Status

GoFrame is experimental. APIs, GOX syntax, manifests, and toolchain behavior
may still change between MVPs.

This document classifies current surfaces so future changes can be discussed
with clear expectations. It does not create a stable 1.0 compatibility promise.

## Stability Tiers

### Experimental

Can change between MVPs. Changes should be documented, but migration support is
best-effort.

### Public-Candidate

Intended direction for user-facing APIs. Changes should include migration
notes, tests, and a compatibility reason.

### Internal

Runtime, compiler, toolchain, or smoke-test implementation details. These are
not user APIs and may change without compatibility guarantees.

### Legacy / Deprecated

Kept temporarily for migration or compatibility. New code should avoid these
surfaces.

## Compatibility Policy

Before public preview:

- compatibility is important but not absolute;
- breaking changes are allowed when they remove unsafe behavior, simplify a
  wrong API, or unblock the architecture;
- breaking changes should be documented in `CHANGELOG.md` and relevant docs;
- generated output and hidden workspace internals are not stable.

After public preview, this policy should be revisited and tightened.

## Current API Classification

### User-Facing Public-Candidate

Runtime:

- `gf.Node`
- `gf.Component`
- `gf.C`
- `gf.NewComponentType`
- `gf.ComponentT`
- `gf.UseState`
- `gf.UseReducer`
- `gf.UseEffect`
- `gf.UseUnmount`
- `gf.Deps`
- `gf.Once`
- `gf.EveryRender`
- `gf.CreateContext`
- `gf.ProvideContext`
- `gf.UseContext`
- `gf.UseContextSelector`
- props-level `MemoEqual` convention
- `gf.VirtualList`
- `gf.VirtualTable`
- `gf.RoutePath`
- `gf.NotFoundRoute`
- `gf.NewHashRouter`
- `gf.RouterView`
- `gf.RouterLink`
- `gf.Navigate`
- `gf.HashHref`
- `gf.QueryValues`
- `gf.ParseQuery`
- `gf.WithQuery`
- `gf.ResourceStatus`
- `gf.Resource`
- `gf.ResourceLoader`
- `gf.UseResource`
- `gf.ErrorBoundary`
- `gf.ErrorBoundaryProps`
- `gf.ErrorBoundaryContext`
- basic event facades such as `gf.Event`, `gf.InputEvent`, and
  `gf.ScrollEvent`

Tooling:

- `goxc generate`
- `goxc build`
- `goxc package`
- `goxc export`
- `goxc serve`
- `goxc size`
- `goxc doctor`
- `goxc clean`
- `goxc version`

These are public-candidate because examples and docs rely on them, but their
exact shapes can still change before public preview.

### Exported Compiler-Facing / Low-Level

Runtime helpers:

- `gf.El`
- `gf.Text`
- `gf.Fragment`
- `gf.Empty`
- `gf.Child`
- `gf.Key`
- `gf.WithKey`
- `gf.Props`
- `gf.If`
- `gf.IfElse`
- `gf.Map`
- `gf.MapIndexed`
- `gf.ToString`
- node structs such as `gf.VNode`, `gf.TextNode`, `gf.FragmentNode`,
  `gf.EmptyNode`, and `gf.KeyedNode`

These remain exported because GOX-generated code and handwritten low-level Go
need them. Most application authors should prefer GOX markup for structure.

Compiler package:

- `gox.Generate`
- `gox.GenerateNamed`
- `gox.GenerateWithOptions`
- `gox.GenerateOptions`
- `gox.GenerateFile`
- `gox.GenerateFileTo`
- `gox.GenerateFileToWithOptions`
- `gox.FindFiles`
- `gox.Codegen`
- `gox.ParseElement`
- `gox.Diagnostic`
- `gox.DiagnosticError`

These are exported for the toolchain and tests. `Generate`, `GenerateNamed`,
`GenerateWithOptions`, diagnostics, and file generation are tooling contracts.
The AST, lexer, and parser structs are exported today but should be treated as
compiler-facing and experimental rather than stable user APIs.

Tooling contracts:

- `goframe.json` user-authored manifest input;
- `asset-manifest.json` generated package metadata and companion entrypoint
  manifest;
- `goframe-package.json` generated package metadata and authoritative current
  package completion/ownership marker;
- `GOFRAME_WORKSPACE` / `--workspace` external workspace override;
- default hidden `.goframe` workspace behavior.

VS Code extension:

- syntax highlighting, snippets, and command wrappers over `goxc` are
  experimental tooling contracts, not language-server stability promises.

### Experimental

- GOX syntax surface.
- GOX expression ergonomics.
- GOX package-qualified component tags (`packageAlias.Component`).
- GOX diagnostic wording beyond the filename/line/column/source-line contract.
- Context topology behavior and selector limitations.
- Virtualization details such as fixed-height range buffering and table spacer
  structure.
- Component identity id format for generated GOX component tokens.
- Runtime error reporting API and exact phase containment behavior:
  `gf.SetErrorHandler`, `gf.ErrorInfo`, `gf.ErrorHandler`, and
  `gf.ErrorPhase`.
- Scoped render Error Boundary reset/fallback semantics beyond the current
  public-candidate API shape. Internal boundary phases such as protected,
  captured, and fallback are not public API.
- Component-scoped resource API and exact lifecycle semantics:
  `gf.ResourceStatus`, `gf.Resource`, `gf.ResourceLoader`, and
  `gf.UseResource`.
- Hash router details such as route remount policy, declaration-order matching
  edge cases, link props, query helper edge cases, and browser listener
  internals.
- Form and validation patterns. MVP 25 documents controlled-input patterns but
  intentionally does not add a runtime form framework.
- Package manifest field stability.
- Browser smoke scripts and debug probe output.
- VS Code extension commands and snippets.
- `pkg/gox` AST/lexer/parser structures.

### Internal

- dirty queue internals;
- mounted tree structures;
- component instance fields;
- state/effect/context slot storage;
- virtual range helper functions;
- `.goframe/work`, `.goframe/build`, `.goframe/package`, and `.goframe/cache`
  internal layout;
- `.goframe/gen` generated file layout;
- debug globals and browser probe object shapes;
- package staging directories;
- smoke harness implementation details.
- generated component variable names;
- exact generated `.goframe/gen` filenames;
- browser debug global object shapes;
- package/export staging temporary directories.

### Legacy / Deprecated

- `goxc build --release`: accepted temporarily, but package/compress behavior
  belongs to `goxc package`.
- Explicit `"wasm": "main.wasm"` manifests: still supported, but examples and
  docs use `bundle.wasm`.
- Legacy `manifest.json` package marker: fail-closed migration support only
  for the historical GoFrame package manifest shape; `goframe-package.json` is
  current.
- `goxc generate --in-place`: debug/legacy only. Generated `.gox.go` files
  should live under `.goframe/gen` or an explicit output directory.
- `UseMount`: deprecated alias for once-after-mount effect behavior.
- `NoDeps`: deprecated alias for `Once`.
- `AlwaysDeps`: deprecated alias for `EveryRender`.
- `DepsOf` and `Dep*` explicit helpers: retained for compatibility; prefer
  `Deps`.
- `For` and `ForIndexed`: deprecated aliases for `Map` and `MapIndexed`.

## Questionable APIs And Decisions

| Surface | Decision | Rationale |
|---|---|---|
| `Component` vs `ComponentT` | Keep both. | `Component` preserves handwritten compatibility; generated GOX uses typed identity. |
| `El`/`Text`/`Fragment`/`Props` | Compiler-facing but available. | Needed by generated code and low-level Go; GOX remains the recommended authoring path. |
| `UseMount`/deps aliases | Deprecated, not removed. | Existing code may use them; replacement APIs are already present. |
| `ErrorHandler` and ErrorBoundary | Experimental/Public-Candidate split. | Useful and tested, but full route-level/error-boundary policy is not final. |
| Resources | Experimental. | Component-scoped lifecycle is tested, but no global cache, Suspense, or route loader contract exists. |
| Router query helpers | Public-Candidate with limitations. | Good for simple URL state; not a typed query-state manager. |
| Virtualization | Public-Candidate fixed-height contract. | Dynamic measurement and advanced accessibility remain future work. |
| `pkg/gox` parser/AST exports | Compiler-facing experimental. | Exported today for tooling/tests, but not a stable language-service API. |

## Deprecation Policy

For now, deprecations should:

- keep a clear warning or documentation note;
- have a replacement path;
- remain covered by tests if behavior is still accepted;
- be removed only in a planned cleanup MVP.

## What Is Not Stable Yet

Not stable:

- path/history-mode routing and server fallback behavior;
- file-based routing, route loaders, route middleware, and route-level error
  boundary policy;
- schema validation or a form framework;
- global resource cache, transport helpers, route loaders, and Suspense-style
  resource story;
- SSR/hydration;
- Player/Engine or `.gfapp` format;
- multi-module app support;
- final public component package identity policy;
- dynamic virtualization measurement;
- infinite loading;
- advanced accessibility/keyboard model for tables;
- LSP/formatter behavior;
- stable callback hook;
- full Error Boundary model beyond scoped render fallback and reset;
- automatic route-level Error Boundaries;
- production deployment server behavior.

## Road To Public Preview

Before public preview, GoFrame should have:

- a component identity decision for multi-module workspaces and reusable
  component packages;
- a symlink/file safety test matrix;
- clearer package manifest compatibility policy;
- stable migration notes for GOX syntax changes;
- documented browser support expectations;
- repeatable performance baseline updates;
- explicit public/deprecated/internal API review.
