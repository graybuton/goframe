# Changelog

## Unreleased

### Added

- VS Code inline GOX source diagnostics backed by the schema-v1 `goxc check`
  transport, with manual and saved-file workspace checks, stale-run protection,
  and multi-root isolation.
- Read-only `goxc check` validation for authored GOX files and directory trees,
  with text output, a versioned JSON diagnostics contract, nonzero diagnostic
  exit status, and no generated output or workspace materialization.
- `v0.2.0-preview.5` release notes documenting retained input selection
  restoration, GOX embedded-expression source diagnostics, nested diagnostic
  source-location preservation, exact O(n log n) LIS-aware keyed placement, and
  release-process documentation cleanup.
- `v0.2.0-preview.4` release notes documenting runtime hot-path work, WASM
  size headroom recovery, keyed placement behavior, scheduler reset
  correctness, GOX diagnostics, `goxc doctor` exit behavior, community docs,
  branding assets, audit outcomes, compatibility notes, and remaining preview
  limits.
- Newcomer-first README structure with Quick Start near the top, a branded hero
  header, and links to the current preview notes.
- Lightweight GoFrame brand assets under `assets/brand/`, including the project
  mark, 128px PNG export, brand README, and no-affiliation disclaimer.
- Real repository CODEOWNERS coverage, clarified NOTICE ownership, and polished
  CONTRIBUTING, SECURITY, and code-of-conduct wording for the current
  browser/WASM preview.
- Runtime, GOX, and `cmd/goxc` audit passes completed with no blocker or
  high-severity findings after focused follow-up fixes.
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
- Strategy documentation now positions GoFrame as a web-first Go application
  framework and toolchain, marks Player/Engine and `.gfapp` as inactive
  directions, and keeps preview promises limited to current evidence.
- `v0.2.0-preview.1` release notes document the web-first strategy closeout,
  external Go component package identity/workspace/build evidence, inactive
  Player/Engine status, and explicit limits around raw external `.gox`, remote
  modules, fullstack/server APIs, production readiness, and broad reusable
  package ecosystem stability.
- Server-backed reference fixture showing a packaged browser/WASM app served by
  a plain Go `net/http` backend with a same-origin `/api/greeting` resource/form
  flow and browser smoke coverage, without adding a GoFrame server framework or
  production/fullstack claim.
- Server-backed reference fixture now covers a controlled backend API failure
  and recovery through existing resource/form state in browser smoke.
- Server-backed browser smoke now covers delayed stale backend response
  no-overwrite behavior through existing resource/form state and the
  browser text fetch path.
- Experimental `gf.FetchText` browser/WASM text loader for `gf.UseResource`,
  with the server-backed reference fixture migrated to the low-level helper
  without adding JSON, cache, route-loader, server-framework, or production
  server behavior.
- Resource example issue loading now composes `gf.FetchText` for packaged text
  transport while keeping issue parsing, slow-delay, stale completion, and
  cleanup-after-unmount evidence local to the example.
- `v0.2.0-preview.2` release notes summarize the server-backed browser/WASM
  evidence, experimental `gf.FetchText`, resource example adoption, and
  `goxc` workspace VCS-stamping baseline fix while keeping fullstack/server,
  route-loader, JSON/data framework, and production-server claims out of scope.
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

- `splitProps` allocation behavior was characterized and reduced for common
  runtime prop paths while preserving nil and empty zero-allocation behavior.
- `VirtualTable` runtime code was reduced to recover dashboard WASM size
  headroom.
- Keyed child placement now avoids additional redundant moves by keeping the
  longest increasing keyed subsequence stable during placement, preserving the
  characterized rotate-right one-move behavior and reducing a middle-backward
  keyed reorder from two existing-node moves to one.
- Scheduler reset now invalidates stale queued update callbacks so pre-reset
  work cannot run after a newer request is queued.
- Retained focused inputs now restore their selection range after dirty
  component updates.
- GOX now rejects Go keyword component prop names before codegen with
  source-level diagnostics, while preserving DOM attributes such as `type`.
- GOX now reports malformed embedded expressions as source-level diagnostics
  before generated Go parsing.
- `goxc doctor` now returns nonzero when required checks fail.
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
- `goxc version` and generated `goframe-package.json` metadata now report the
  tagged module version from Go build information for module installs and
  `devel` for local checkout builds, fixing the stale `0.1.0` self-report in
  `v0.2.0-preview.2`. No runtime, GOX, package workflow/layout,
  build/export/serve, example, or script behavior changed outside toolchain
  version self-reporting.

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
