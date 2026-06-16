# Changelog

## Unreleased

### Added

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
