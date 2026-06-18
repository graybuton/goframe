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
