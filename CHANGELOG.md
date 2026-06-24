# Changelog

## Unreleased

### Added

- Hashed asset packaging for `goxc package`, including `assets/`,
  `asset-manifest.json`, `goframe-package.json`, preload hints, and
  `bundle.wasm` naming.
- Clean app workspace layout under `.goframe/`, plus `goxc export` for explicit
  deployment directories.
- Packaging safety checks for export ownership, legacy clean migration, and
  clearer hidden-workspace module errors.
- Foundation Audit III safety coverage for explicit package output ownership,
  package asset path validation, legacy `manifest.json` cleanup markers, and
  clean workspace CLI edge cases.
- Dependabot configuration for GitHub Actions, Go modules, and VS Code
  extension npm dependencies.
- Dashboard-sized pressure-test example with 300 deterministic issue rows,
  filters, keyed table sorting, detail panel updates, and smoke coverage.
- Explicit `MemoEqual`-based component memoization in runtime, with dashboard row
  memo skip coverage and browser smoke assertions.
- Memoization safety coverage for dirty descendants and dashboard callback
  freshness.
- `UseReducer` state dispatch that applies actions to the latest component
  state slot, used by the dashboard to remove the `DataVersion` memoization
  workaround.
- Scoped context selectors with nearest-provider lookup, comparable selector
  dirtying, and a focused context example plus browser smoke coverage.
- Fixed-height `VirtualList` and `VirtualTable` primitives, dashboard table
  virtualization, and a focused virtualized collections example plus smoke
  coverage.
- Foundation Audit IV documentation.
- Read-only GitHub Actions workflow permissions.
- MVP 18 runtime and benchmark baseline documentation, including performance
  hard/informational gate policy, API stability tiers, component identity next
  steps, and symlink/file safety policy.
- Component identity v2 prototype with `gf.NewComponentType`, `gf.ComponentT`,
  generated GOX component tokens, and legacy `gf.Component` compatibility.
- Multi-package GOX workspace support for `entry: "."` apps, including hidden
  workspace materialization for child packages, import-path-aware generated
  component identities, and a focused multipackage example plus browser smoke.
- GOX diagnostic hardening with structured source diagnostics, clearer
  unsupported namespace/spread errors, stricter parser-side validation for
  empty expressions and `Key`, and multi-package source-path error coverage.
- Child entry package support for `goxc`, including `"entry": "./cmd/app"` and
  other relative child package directories, app-root-wide GOX generation,
  import-aware component identities, and a focused `cmdapp` example plus
  browser smoke.
- Foundation Audit V public surface polish, including a refreshed README,
  updated example/docs wording, local docs/example consistency checks, and
  `docs/public-surface-audit-v.md`.
- Runtime error semantics for recoverable user-code panics, including
  `gf.SetErrorHandler`, phase-specific error reports, event/effect/cleanup
  containment, memo comparator fallback, render fallback, and browser smoke
  coverage for recoverable runtime errors.
- Hash-based client router primitives, including route params, not-found
  handling, `RouterView`, `RouterLink`, programmatic navigation, a Go-first
  router example, and browser smoke coverage.
- MVP 25 public preview hardening, including router query helpers, documented
  forms/validation patterns, a router-dashboard reference example, browser
  smoke coverage, and public API surface clarification.
- GOX package-qualified component tags such as `<layout.Shell />`,
  `<gf.RouterLink />`, and `<filters.FilterControls />`, including
  import-path-aware generated component identity, clearer unsupported selector
  diagnostics, example refactors away from wrapper-only cross-package
  components, and VS Code syntax highlighting updates.
- Scoped render-only Error Boundaries with `gf.ErrorBoundary`,
  `gf.ErrorBoundaryProps`, `gf.ErrorBoundaryContext`, manual reset,
  `ResetKey`, nested-boundary behavior, Go/WASM browser smoke coverage, and
  lifecycle safety tests for failed render subtrees.
- Error Boundary fallback correctness: a boundary no longer self-captures
  failures from the fallback subtree it is currently displaying; those failures
  bubble to an outer boundary or the default render fallback.
- GitHub Actions workflows for core Go/GOX checks, TinyGo WASM size budgets,
  browser smoke, and VS Code extension compile checks.
- Artifact and module path regression gates.
- CI and release hygiene documentation.

### Planned

- CI pipeline tuning after the first public pull requests.
- Release packaging decisions for `goxc`.
- Dashboard-sized benchmark exploration.

## v0.1.0-mvp10 - 2026-06-17

### Added

- GOX expression-oriented conditional rendering with
  `{condition && <Node />}`.
- GOX ternary rendering with `{condition ? <A /> : <B />}`.
- Nested GOX markup in callback return expressions.
- `Key={...}` pseudo-prop lowering.
- Simplified `UseState` API returning `(value, setValue)`.
- Simplified `UseEffect` API with optional `gf.Deps(...)` and
  `gf.EveryRender()`.
- `gf.Map` and `gf.MapIndexed` list helpers.
- Compressed WASM size budgets for raw, gzip, brotli, and optional zstd.
- Hardened browser smoke harness with dynamic ports and origin preflight.

### Notes

This is an internal MVP milestone tag, not a stable public release.
