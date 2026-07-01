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
- Component identity preview contract clarifies stable API shape vs experimental
  edge-case semantics for cross-package identity, remount expectations, and
  multi-module frontier behavior in docs.
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
- Component-scoped resource prototype with `gf.UseResource`, explicit
  loading/ready/failed state, stale completion guards, cleanup/cancellation
  semantics, and a focused resource example plus browser smoke coverage.
- Preview-facing manifest asset directory contract for `goframe.json`, including
  `"assets": "./assets"`, legacy explicit asset list compatibility, automatic
  default `index.html` generation, and updated examples/docs.
- Package preview closeout coverage for manifest/assets error paths, generated
  and custom package `index.html` output, versioned metadata presence,
  package ownership verification, and fact-checked package/manifest docs.
- Preview tooling closeout coverage for `pkg/gox` trusted-filesystem file
  helper classification, export destination false-marker rejection, package
  publication limitation wording, and lightweight supply-chain/tooling evidence.
- Runtime experimental-surface hardening coverage for resource reload/stale
  callback edges, ErrorBoundary reset/new-incident behavior, router route-key
  remount policy, and preview contract docs alignment.
- `v0.1.0-preview.1` release-story docs, including draft release notes,
  evaluator guidance, compatibility/readiness alignment, and fact-first preview
  documentation cleanup.
- `v0.1.0-preview.2` maintenance release notes documenting post-preview.1
  community health files, `goxc serve` path hardening, browser smoke CodeQL
  eval-pattern cleanup, and no runtime/API/GOX/package semantics expansion.
- Post-preview v0.2 focus documentation selecting reusable component/package
  identity as the next design/test focus without behavior changes or preview
  promise expansion.
- `goxc` build workspace materialization now preserves app module
  `require`/`replace` directives for external Go package resolution, rewrites
  relative local replace targets to the original module locations, and keeps
  external dependency `.gox` generation outside the current workspace model.
- MVP 29 reference app consolidation: `examples/router-dashboard` now serves as
  the flagship integrated tutorial app with packaged data, one app-local
  resource owner, explicit loading/failed UI, manual reload, query filters,
  controlled form validation, scoped render Error Boundary composition, and
  strengthened browser smoke coverage.
- `docs/tutorial.md` with a recommended learning path through focused,
  reference, pressure, and toolchain examples.
- MVP 30 public-preview readiness contracts, including API surface
  classification, component identity policy, manifest/package compatibility,
  platform support, compatibility/deprecation policy, migration note template,
  and public-preview release checklist.
- GitHub Actions workflows for core Go/GOX checks, TinyGo WASM size budgets,
  browser smoke, and VS Code extension compile checks.
- Core CI matrix now includes minimal macOS and Windows Go/toolchain evidence (`go fmt`, `go test`, `go vet`, debug-tag, and selective GOX tests), while TinyGo/browser smoke evidence remains Linux-first.
- Initial bounded GOX fuzz targets for whole-file generation and element
  parser/codegen, seeded from existing golden/error fixtures and small inline
  snippets.
- Artifact and module path regression gates.
- CI and release hygiene documentation.
- Pre-preview action plan and vision-preserving preview contract wording that
  separates first-preview scope from long-term GoFrame platform direction.

### Fixed

- The router-dashboard route Error Boundary fallback now distinguishes retrying
  the current crashing route from safely navigating back to the issues list, and
  browser smoke covers recovery from `?panic=render` without reloading the
  resource owner.
- `goxc` now rejects symlinked entry directories, source files, manifest
  assets, package output roots, and export output roots at safety-sensitive
  boundaries instead of following them.
- `goxc` now rejects intermediate symlink components under declared
  app/workspace/output roots, symlinked package sources, destination symlink
  writes, source/output overlap, false package ownership markers, and package
  asset namespace collisions with generated WASM/runtime/sidecar files.
- `goxc` now detects physical symlink-alias overlaps for external workspaces
  and explicit build/generate/package/export outputs, preventing output paths
  from pointing back into authored source through alternate spelling.
- Manifest `wasm` values now must be relative `.wasm` child paths, preventing
  build/package output from targeting authored files such as `main.go`,
  `go.mod`, or `goframe.json`.
- Package/export ownership detection now requires complete, regular,
  structured GoFrame metadata instead of trusting placeholder filenames,
  standalone `asset-manifest.json`, or generic web/Go-WASM `manifest.json`.
- Legacy package ownership is fail-closed and recognized only for the
  historical GoFrame `manifest.json` shape found in repository history.
- Package publication now validates staged source entries before copying and
  publishes `goframe-package.json` last. Package cleanup removes that
  authoritative completion marker before destructive cleanup, reducing the
  chance of partial package trees being treated as complete.
- Standalone packages now always publish package root `index.html`: it is
  rewritten from a selected custom template or generated by `goxc package` when
  no custom template exists. Package/export replacement removes stale managed
  `index.html`, and successful package/export commands verify current package
  ownership before printing success.
- Resource loader panics no longer leave the internal effect slot pending, so a
  same-key rerender after failed state does not automatically restart the same
  panicking loader. Explicit retry remains available through `reload` or key
  change, and first completion still wins if a loader resolves or rejects before
  panicking.
- The resource example browser loader now releases cancelled promise/timer
  callbacks on inactive response paths.

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
