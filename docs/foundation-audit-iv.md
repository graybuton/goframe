# Foundation Audit IV

## Summary

Foundation Audit IV reviewed the repository after MVP 17 virtualized
collections. The audit found no blocking runtime, compiler, packaging, or smoke
regression on the local baseline. The only code/config change made during the
audit is GitHub Actions permission hardening: all workflows now request
read-only repository contents access.

This pass intentionally did not add runtime features. Router, SSR, hydration,
Player/Engine, bundle splitting, dynamic virtualization measurement, infinite
loading, formatter/LSP, global store, and new GOX syntax remain out of scope.

## Scope

Reviewed areas:

- `pkg/goframe`: runtime, hooks, effects, context selectors, memoization,
  reconciliation, event handling, debug probes, and virtualization.
- `pkg/gox`: GOX lexer, parser, codegen, diagnostics, and golden tests.
- `cmd/goxc`: generate, build, package, export, size, serve, clean, doctor, and
  workspace layout.
- `examples`: counter, components, todo, dashboard, context, and virtualized.
- `scripts`: browser smoke, size, perf, artifact, module path, and DOM pressure
  gates.
- `.github/workflows`: CI coverage and workflow permissions.
- `docs`, `README.md`, `CHANGELOG.md`, and example READMEs.

## Baseline Status

Baseline was taken on `chore/foundation-audit-iv` from local `main` at
`v0.1.0-mvp17`. `git pull origin main` could not authenticate in this
environment, but local `main` matched `origin/main`.

Tool versions:

- Go: `go1.24.4 linux/amd64`
- TinyGo: `0.41.1`
- Node.js: `v20.19.4`
- Git: `2.51.0`

Baseline checks passed:

- `go fmt ./...` and `git diff --check`
- `go test ./...`
- `go test -race ./pkg/... ./cmd/...`
- `go vet ./...`
- `go test -tags=goframe_debug ./...`
- `go test ./pkg/gox -run 'TestGolden|TestErrorGolden'`
- `scripts/check.sh`
- `scripts/size-budget.sh`
- `scripts/perf-report.sh`
- `scripts/browser-smoke.sh`
- `node --experimental-websocket scripts/dashboard-dom-pressure.mjs`
- `scripts/artifact-check.sh`
- `scripts/module-path-check.sh`

`shellcheck` is not installed in this environment. All `scripts/*.mjs` files
passed `node --check`.

## Architecture Review

The repository boundaries remain clear:

- runtime/framework code is in `pkg/goframe`;
- GOX parsing and codegen are in `pkg/gox`;
- application toolchain behavior is in `cmd/goxc`;
- examples are isolated under `examples`;
- regression harnesses live under `scripts`;
- documentation and CI are separated from runtime code.

No example-specific code was found in production runtime. Virtualization is now
framework-level (`gf.VirtualList`, `gf.VirtualTable`) rather than a dashboard
workaround. Packaging and build responsibilities remain separated by the clean
workspace model:

- generation to `.goframe/gen`;
- build output to `.goframe/build`;
- standalone packages to `.goframe/package/standalone`;
- explicit deploy output through `goxc export`.

Remaining architectural risk: component identity is still string/name based.
This is acceptable for the current examples and documented in
`docs/component-identity.md`, but future compiler-generated identity tokens may
be needed before larger apps.

## Runtime Review

Reviewed runtime areas:

- component identity and dirty queue pruning;
- dirty descendant accounting for memoization and context selectors;
- `UseState`, `UseReducer`, hook-kind checks, and latest-state dispatch;
- effects and unmount cleanup;
- context selector subscriptions and provider topology refresh;
- keyed reconciliation and fragment/component anchors;
- event listener update/release behavior;
- debug render/patch/memo probes;
- fixed-height virtualized collections.

Existing tests cover the critical correctness edges introduced by recent MVPs:

- memoized components do not skip when dirty descendants exist;
- context provider appearance/removal and nested provider topology changes
  dirty affected consumers through memoized ancestors;
- reducer dispatch applies actions to latest state;
- virtual table spacer rows use stable internal keys and remain mounted;
- production runtime avoids heavy imports and debug globals.

No runtime bug was fixed in this audit.

## Virtualization Review

`pkg/goframe/virtual.go` and `pkg/goframe/virtual_test.go` cover:

- empty, short, top, middle, and bottom ranges;
- negative overscan;
- scroll positions beyond the total height;
- range buffering so scroll inside the current window does not schedule work;
- stable fallback and custom keys;
- virtual table `ColumnCount`;
- stable top/bottom spacer keys;
- row key namespacing to avoid collisions with internal spacer keys;
- keyed empty state rows.

Dashboard smoke and DOM pressure gates currently verify:

- table layout has seven readable columns;
- mounted `.issue-row` count stays bounded at about 28;
- spacer DOM identity stays stable;
- inside-buffer scroll has zero runtime/DOM/listener churn;
- beyond-buffer scroll uses proportional bounded churn gates;
- Open -> All has stable live DOM and net listener counts.

DOM pressure baseline:

- Open logical rows: 72
- All logical rows: 300
- Mounted row max: 28
- Average All duration: about `52.77 ms`
- Average All created nodes: `621`
- Average All removed nodes: `69`
- Live DOM All start/end: `486 -> 486`
- Net listeners All start/end: `0 -> 0`
- Continuous scroll: `321` steps, `8` `VirtualTable` renders, `72`
  `IssueRow` renders, listener net delta `0`

Remaining risk: virtualization is fixed-height only. Dynamic row measurement,
advanced table accessibility, keyboard navigation, and infinite loading should
be designed separately.

## Context Review

Context selectors are scoped through component parent links. The tests cover:

- default values without a provider;
- nearest provider wins;
- nested provider isolation;
- selected value dirtying;
- unchanged selected values staying clean;
- broad `UseContext` consumers rerendering on provider updates;
- consumer and provider unmount cleanup;
- provider appearance/removal;
- inner provider appearance/removal;
- topology changes through memoized ancestors;
- context hook kind mismatch diagnostics.

Remaining risk: provider topology refresh uses a global per-context
subscription registry and can be O(number of consumers for that context) on
provider appearance/removal. This is acceptable for current examples but should
be profiled before very large provider trees.

## State / Reducer / Effects Review

`UseState` and `UseReducer` share positional state slots with hook-kind checks.
Primitive same-value updates are no-ops; composite values conservatively
schedule. `UseReducer` dispatch reads the latest slot value and latest reducer,
which prevents stale row handlers from reverting dashboard data.

Effects remain post-patch and component-scoped. Debug builds warn for
Set-after-unmount, Set-during-render, and guarded effect update loops.

Remaining risk: render-time updates are still allowed with debug warnings. A
future pass may want a stricter policy or more visible diagnostics.

## Memoization Review

Memoization remains explicit through props-level `MemoEqual`. There is no
reflection, deep comparison, or automatic function-prop equality.

Correctness guards verified by tests and smoke:

- components without `MemoEqual` do not skip;
- dirty components do not skip;
- dirty descendants block memo skip;
- key/name mismatch blocks skip;
- dashboard row selection renders only the changed rows while memo-skipping the
  rest of the mounted window.

Remaining risk: comparator authors can still ignore function props incorrectly.
The documented safe pattern is reducer dispatch or future stable callback APIs.

## DOM Reconciliation Review

The reconciler keeps stable nodes for same-kind text/element/component
subtrees, patches props in place, updates event callbacks without listener
churn, and uses keyed child matching for moves/removals.

Recent virtual table bugs were guarded by smoke and unit tests:

- spacer rows use explicit keys;
- user row keys are namespaced internally;
- table layout uses explicit `ColumnCount` instead of a large magic colspan.

Remaining risk: `insertBefore` counts can be high for whole-window replacement
even when mounted rows stay bounded. Smoke thresholds now scale with mounted
window size rather than using a fixed magic number.

## GOX Compiler Review

GOX parser/codegen tests and golden tests cover:

- elements, components, fragments, props, children, and event props;
- key pseudo-props;
- conditional and ternary render expressions;
- nested GOX markup in callback returns;
- unsupported spread props and namespace tags with clear errors;
- mismatched/unclosed tags with line/column diagnostics.

No compiler bug was fixed in this audit. Remaining risk: GOX still has no
formatter, LSP, spread props, namespaces, or generated props equality. These
are future features, not cleanup work.

## goxc CLI Review

Audited commands:

- `generate`
- `build`
- `package`
- `export`
- `size`
- `serve`
- `doctor`
- `clean`
- `version`

Safety findings:

- manifest paths are validated as relative child paths;
- unknown manifest fields fail;
- hidden workspace build supported `entry: "."` apps with child packages under
  the app root at the time of the audit; MVP 22 later added child entry package
  support inside the app root;
- package output uses staging before publishing;
- explicit package destinations must be empty or GoFrame-owned;
- export refuses non-empty non-GoFrame directories unless `--force` is used;
- clean removes legacy `dist` only when it looks GoFrame-owned;
- serve is development-only and binds to `127.0.0.1`;
- compression commands use `exec.Command` with explicit arguments.

No CLI bug was fixed in this audit.

Remaining risk: symlink behavior is not deeply specified. Current path checks
protect manifest strings, but a future security pass should define policy for
symlinked app assets and workspace directories.

## Examples Review

Examples currently align with present APIs:

- no tracked `.gox.go` files;
- generated/build/package outputs remain under ignored `.goframe`;
- examples package with TinyGo;
- browser smoke covers Todo, duplicate keys, dashboard, context, and
  virtualized collections.

The dashboard remains the main pressure test. It uses reducer dispatch,
explicit row memoization, context-free state ownership, and `gf.VirtualTable`.

Remaining risk: examples include browser debug probes in `index.html` for
smoke coverage. That is intentional but should be kept out of future polished
production templates.

## Scripts and CI Review

Scripts are aligned with the `.goframe/package/standalone` layout. The size
budget gate measures packaged `assets/bundle*.wasm` with legacy fallback.
Browser smoke uses dynamic ports and distinguishes harness failures from app
failures.

Fixed during this audit:

- GitHub Actions workflows now declare `permissions: contents: read`.

Remaining risk: `shellcheck` is not available in the local environment and is
not yet a CI gate. Adding it later would catch shell portability issues.

## Security / Safety Review

Security-oriented checks:

- production runtime source gate rejects `reflect`, `fmt`, `encoding/json`,
  `regexp`, `net/http`, `runtime/debug`, `log`, and `unsafe` imports;
- text rendering uses text nodes rather than `innerHTML`;
- no `eval`/`new Function` usage was found;
- toolchain deletion is limited to known package/workspace artifacts;
- export/package destination guards protect user-owned `assets/` directories;
- static dev server is intentionally local-only.

No security bug was fixed in this audit. The workflow permissions hardening
reduces default GitHub token privileges.

## Size Review

Current size budgets pass:

| Example | Raw | gzip | br | zstd |
|---|---:|---:|---:|---:|
| counter | 81350 B | 32477 B | 27098 B | 29282 B |
| components | 86725 B | 34131 B | 28324 B | 30555 B |
| todo | 113935 B | 43622 B | 36285 B | 39263 B |
| dashboard | 163897 B | 61217 B | 49482 B | 53454 B |
| context | 111440 B | 42093 B | 34413 B | 37111 B |
| virtualized | 119729 B | 45575 B | 37524 B | 40759 B |

No size budget was loosened.

## Performance Review

Pure runtime benchmarks from the baseline:

- dirty queue pruning: about `482 ns/op`
- keyed child matching: about `20523 ns/op`
- unkeyed child matching: about `3209 ns/op`
- split props: about `3877 ns/op`
- event name normalization: about `387 ns/op`
- state slot access: about `511 ns/op`
- unwrap keyed node: about `8.39 ns/op`

Dashboard browser baseline:

- focus search: render/patch/DOM/listener deltas all `0`;
- row selection: `IssueRow` renders `2`, memo skips `26`, DOM structural ops
  `0`;
- row toggle/simulate/post-simulate toggle: one mounted row rerenders, unrelated
  mounted rows memo-skip;
- virtualized table mounted rows stay bounded.

## Documentation Review

Current docs describe:

- experimental status;
- clean `.goframe` workspace;
- `goxc export`;
- cache-safe package manifests;
- explicit memoization;
- reducer latest-state dispatch;
- context selector limitations;
- fixed-height virtualization limitations.

This audit adds a dedicated Foundation Audit IV report. No stale documentation
requiring immediate correction was found beyond recording current audit results.

## Dead Code / Cleanup

No tracked generated `.gox.go`, build, package, WASM, node_modules, VSIX, or
test artifacts are present. Ignored local working outputs exist under
`.goframe`, `extensions/vscode-gox/node_modules`, `extensions/vscode-gox/out`,
and a local VSIX file; they are not tracked.

No dead runtime/helper code was removed in this audit because the reviewed
helpers still have tests or active call sites.

## Fixed Issues

- Hardened GitHub Actions workflows with explicit read-only repository
  permissions:
  - `.github/workflows/ci-core.yml`
  - `.github/workflows/ci-wasm-size.yml`
  - `.github/workflows/ci-browser-smoke.yml`
  - `.github/workflows/ci-vscode.yml`

## Remaining Risks

- String/name-based component identity may need compiler-generated identity in
  larger applications.
- Context provider topology refresh is intentionally simple and may need
  profiling for very large trees.
- Virtualization supports fixed heights only; dynamic measurement and richer
  accessibility remain future work.
- Comparator authors can still create stale callback bugs if they ignore
  function props without reducer dispatch or another stable callback pattern.
- `goxc serve` is development-only and does not implement production content
  negotiation.
- Symlink policy for app assets/workspaces should be specified before public
  hardening claims.
- `shellcheck` is not currently enforced in CI.

## Regression Gates

Important gates after this audit:

- runtime source import gate;
- artifact and module path gates;
- GOX golden/error golden tests;
- clean workspace and packaging tests;
- export ownership tests;
- context topology tests;
- memoization dirty descendant tests;
- virtual range and spacer identity tests;
- Todo DOM identity smoke;
- dashboard layout/performance/DOM pressure smoke;
- context selector smoke;
- virtualized collection smoke;
- TinyGo raw/gzip/br/zstd budgets.

## Recommended Next Steps

Before the next runtime feature:

1. Keep Foundation Audit IV green through CI after PR.
2. Add CI `shellcheck` if the project wants shell portability enforcement.
3. Decide whether symlink policy needs explicit documentation and tests.
4. Continue the MVP 19 component identity prototype into a package-aware
   identity design before larger component systems.
5. Keep dynamic virtualization measurement, accessibility hardening, and stable
   callback hooks as separate design tasks rather than audit cleanup.
