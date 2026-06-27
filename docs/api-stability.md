# API Stability

## Current Status

GoFrame is experimental. APIs, GOX syntax, manifests, and toolchain behavior
may still change between MVPs.

This document classifies current surfaces so future changes can be discussed
with clear expectations. It does not create a stable 1.0 compatibility promise.

## Stability Tiers

### Public-Candidate

Intended direction for user-facing APIs. Changes should include migration
notes, tests, and a compatibility reason.

### Experimental Frontier

Can change between MVPs. Changes should be documented, but migration support is
best-effort. Experimental frontier surfaces are real working surfaces, not
hidden or deprecated features; they need more contract hardening before broad
preview promises.

### Compiler-Facing / Low-Level

Exported for GOX, `goxc`, generated-code-like use, or third-party tooling. These
surfaces are not the preferred high-level app authoring path unless explicitly
documented.

### Internal

Runtime, compiler, toolchain, or smoke-test implementation details. These are
not user APIs and may change without compatibility guarantees.

### Future Vision

Strategic direction that should remain visible but is not a current preview
promise.

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
- generated typed component identity contracts

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

These are exported for the toolchain, editor integrations, and tests.
`Generate`, `GenerateNamed`, and `GenerateWithOptions` are the primary
in-memory generation boundary for callers that choose their own source bytes.
`GenerateFile`, `GenerateFileTo`, `GenerateFileToWithOptions`, and `FindFiles`
are trusted-filesystem convenience helpers for tooling/editor/test workflows.
They use ordinary standard-library file operations and do not inherit `goxc`'s
root-aware symlink rejection, physical output-overlap checks, package ownership
checks, or package publication guarantees. Use `goxc` or add a caller-side
filesystem policy when processing untrusted repository trees.

Diagnostics and file generation are tooling contracts. The AST, lexer, and
parser structs are exported today but should be treated as compiler-facing and
experimental rather than stable user APIs.

Tooling contracts:

- `goframe.json` user-authored manifest input, including preview-facing
  `"assets": "./assets"` directory mode and legacy explicit asset lists;
- `asset-manifest.json` generated package metadata and companion entrypoint
  manifest;
- `goframe-package.json` generated package metadata and authoritative current
  package completion/ownership marker;
- `GOFRAME_WORKSPACE` / `--workspace` external workspace override;
- default hidden `.goframe` workspace behavior.

VS Code extension:

- syntax highlighting, snippets, and command wrappers over `goxc` are
  experimental tooling contracts, not language-server stability promises.

### Experimental Frontier

- GOX syntax surface.
- GOX expression ergonomics.
- GOX package-qualified component tags (`packageAlias.Component`).
- Component boundary API shape that is already public-facing (for example
  `gf.ComponentT` and typed identity tokens) can still have experimental
  semantics in specific lifecycle, remount, and edge-case compatibility areas.
  The exported surface is documented for preview usage, while deep behavior
  under module/version/package-path edge cases remains experimental until
  broader identity guarantees are proven.
- GOX diagnostic wording beyond the filename/line/column/source-line contract.
- Context topology behavior and selector limitations.
- Virtualization details such as fixed-height range buffering and table spacer
  structure.
- Component identity id format for generated GOX component tokens.
- Runtime error reporting API and exact phase containment behavior:
  `gf.SetErrorHandler`, `gf.ErrorInfo`, `gf.ErrorHandler`, and
  `gf.ErrorPhase`. Current tests cover event, effect, cleanup, memo, context,
  virtual callback, render, and boundary-related reporting paths, but this is
  still not a production error framework.
- Scoped render Error Boundary reset/fallback semantics beyond the current
  public-candidate API shape. Internal boundary phases such as protected,
  captured, and fallback are not public API. Current tests cover containment,
  nested fallback bubbling, reset, `ResetKey`, pending effect cancellation, and
  cleanup release for failed subtrees.
- Component-scoped resource API and exact lifecycle semantics:
  `gf.ResourceStatus`, `gf.Resource`, `gf.ResourceLoader`, and
  `gf.UseResource`. Current tests cover loading/ready/failed state, reload,
  key changes, stale completions, cleanup, loader panic containment, and
  ErrorBoundary interaction. Global caching, deduplication, retry policy,
  route loaders, and Suspense-style semantics are outside the preview
  contract.
- Hash router details such as route remount policy, declaration-order matching
  edge cases, link props, query helper edge cases, and browser listener
  internals. Current tests cover route matching, params, query helpers,
  not-found fallback, hash hrefs, and route subtree keys by route pattern;
  browser back/forward and listener behavior are covered by smoke tests rather
  than a stable low-level listener API.
- Form and validation patterns. MVP 25 documents controlled-input patterns but
  intentionally does not add a runtime form framework.
- Package manifest field stability.
- Browser smoke scripts and debug probe output.
- VS Code extension commands and snippets.
- `pkg/gox` AST/lexer/parser structures.

These surfaces should be hardened, tested, and documented before wider
promises. They should not be removed from project positioning merely because
their contracts are still maturing.

### Future Vision

- Player/Engine host and bundle model.
- `.gfapp` package format.
- Broader host/runtime story beyond the browser DOM target.
- Stronger editor tooling such as LSP/formatter behavior.
- Future package ecosystem and reusable component distribution story.
- Production deployment/server integration, if the project chooses that path in
  a later phase.

These are not part of the first public preview promise.

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
- Explicit asset list manifests such as `"assets": ["index.html"]`: still
  supported, but examples and docs use `"assets": "./assets"`.
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
| `pkg/gox` file helpers | Trusted-filesystem convenience. | Useful for editor/test tooling; hardened untrusted filesystem handling belongs to `goxc` or caller-side policy. |

## Deprecation Policy

For now, deprecations should:

- keep a clear warning or documentation note;
- have a replacement path;
- remain covered by tests if behavior is still accepted;
- be removed only in a documented cleanup stage.

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
