# CI and Regression Gates

This repository keeps local scripts and GitHub Actions aligned. The goal is to
make runtime, GOX, WASM size, browser smoke, and VS Code extension regressions
visible before merge.

All workflows request read-only repository contents permissions by default.

## Workflows

### Core

`.github/workflows/ci-core.yml` runs on pull requests and pushes to `main`.

It uses a small OS matrix:

- `ubuntu-latest`: full `artifact/module path/docs` + existing core gates.
- `macos-15-intel`, `windows-latest`: minimal Go/toolchain evidence (core
  formatting/tests/vet/debug-tag/selected GOX tests).

It checks:

- tracked artifact gate;
- canonical module path gate;
- docs/example consistency check;
- `go fmt ./...` plus a clean diff check;
- `go test ./...`;
- `go test -race ./pkg/... ./cmd/...`;
- `go vet ./...`;
- `go test -tags=goframe_debug ./...`;
- GOX golden tests, including source-oriented error diagnostics.
- GOX fuzz seed targets through the normal `go test ./...` seed pass.

The `cmd/goxc` test suite includes manifest/path/package/export/workspace
regression tests, including root-aware symlink checks for app roots, entry
directories, source files, manifest assets, package/export roots, generated
outputs, workspace cleanup behavior, package ownership markers, asset
namespace collisions, lexical and physical source/output overlap, partial
publication metadata, custom/generated standalone `index.html` integrity,
fail-closed legacy ownership, and dev-server symlink entries.

The same Core `go test` gate covers the watched `goxc dev` workflow. Pure tests
exercise option parsing, content snapshots, debounce, serialized coordination,
failure recovery, and shutdown with injected dependencies. A focused integration
test uses the real standard Go WASM package and static-server path in a temporary
external application and workspace. Core CI does not claim a TinyGo dev-loop
automation pass.

### WASM Size

`.github/workflows/ci-wasm-size.yml` runs on pull requests, pushes to `main`,
and manually through `workflow_dispatch`.

It installs Go, TinyGo `0.41.1`, brotli, and zstd. Then it packages the
counter, components, todo, dashboard, context, virtualized, multipackage,
cmdapp, router, router-dashboard, and resource examples with TinyGo. It also
runs a release-style package pass with `--asset-hash --preload
--compress=gzip,br` before checking:

```bash
scripts/size-budget.sh
```

The workflow uploads the printed size report as an artifact.

The size gate measures the packaged WASM entrypoint under
`.goframe/package/standalone/assets/bundle*.wasm`, with a legacy fallback for
older `main.wasm` packages.

### Browser Smoke

`.github/workflows/ci-browser-smoke.yml` runs on pull requests, pushes to
`main`, and manually through `workflow_dispatch` on `ubuntu-latest` only.

It installs Go, TinyGo `0.41.1`, Node.js 20, Chrome, and compression tools,
then runs:

```bash
scripts/browser-smoke.sh
```

The current smoke harness is Chrome/Chromium-specific. The Node scripts launch
Chrome with a remote debugging port and use the Chrome DevTools Protocol over
WebSocket for DOM probes, event simulation, runtime error collection, and DOM
pressure metrics. Firefox, Safari/WebKit, and other non-Chrome engines do not
have automated browser-smoke evidence in the current preview CI contract.

The smoke script chooses dynamic ports, verifies the expected app origin before
storage cleanup, checks WASM MIME type, and separates harness failures from app
failures. It currently covers Todo reconciliation, duplicate-key debug
diagnostics, runtime error containment for event/effect/cleanup panics, scoped
render Error Boundary fallback/reset behavior, dashboard-sized
filtering/sorting/selection behavior, context selector rerender isolation,
virtualized collection scroll/selection/toggle behavior, a multi-package GOX
workspace smoke, a child-entry package smoke, hash-router navigation smoke, and
the router-dashboard reference app smoke for query filters, form validation,
single-owner resource loading, no duplicate reloads across navigation/query
changes, manual reload, explicit resource failure UI, and stable shell
identity. It also covers route Error Boundary fallback and safe navigation
recovery in the reference app. It also covers the resource example for explicit
loading/ready/failed state, reload, stale completion guards, and
cleanup-after-unmount behavior.
The suite also runs a focused `goxc dev` reload lifecycle against a temporary
standard-Go browser/WASM application. It verifies the initial non-reloading
connection, GOX, Go, and asset rebuild reloads, burst coalescing, failure
preservation and recovery, two connected pages, current and stale generation
reconnection, completed-generation serving during a later package attempt, and
shutdown cleanup. This is standard-Go browser evidence; it does not claim a
TinyGo development-reload pass.
The persistent `examples/server-backed` smoke records a narrow Go `net/http`
backend integration boundary. A packaged GoFrame browser/WASM app is served by
a same-origin Go backend and renders data from `/api/greeting` before and after
a small form-driven request. The same smoke covers a controlled backend failure
state, delayed stale request no-overwrite behavior, and recovery after a later
valid form submission. This does not add a GoFrame server API.

The runtime error containment fixture, Error Boundary fixture, and
router-dashboard reference-app smoke are compiled with the Go WASM compiler
where recover-based render containment is being asserted. The size-oriented
TinyGo package path uses trap-style panic behavior, which is documented as a
runtime error containment limitation.

GOX diagnostic golden tests intentionally assert filenames, line/column
prefixes, specific unsupported-syntax messages, and source snippets. They are
part of the compiler/toolchain contract even though the broader GOX syntax
surface remains experimental.

### VS Code Extension

`.github/workflows/ci-vscode.yml` runs on pull requests and pushes to `main`.

It validates extension JSON files, installs dependencies with `npm ci`,
compiles the TypeScript extension, and runs pure Node tests for the diagnostics
transport helpers through:

```bash
npm test
```

## Dependabot

Dependabot version updates are configured in `.github/dependabot.yml`.

The project tracks:

- GitHub Actions updates;
- Go module updates;
- VS Code extension npm dependency updates.

Dependabot runs weekly on Monday in the Europe/Helsinki timezone. Dependabot
PRs are not auto-merged; they must pass CI, including size budgets and browser
smoke when relevant.

Security alerts and security updates should be enabled from the GitHub
repository security settings. Recommended labels are `dependencies`,
`github-actions`, `go`, `vscode`, and `npm`.

Current supply-chain evidence is lightweight:

- GitHub Actions workflows use read-only repository contents permissions by
  default;
- Dependabot checks GitHub Actions, Go modules, and VS Code extension npm
  dependencies;
- the VS Code extension workflow installs from `package-lock.json` with
  `npm ci`;
- the root Go module currently has no third-party module requirements beyond
  the standard library.

No SBOM, package signing, or heavyweight dependency scanner is part of the
current preview CI contract.

## Local Checks

Core local verification:

```bash
node scripts/docs-check.mjs
go fmt ./...
go test ./...
go test -race ./pkg/... ./cmd/...
go vet ./...
go test -tags=goframe_debug ./...
go test ./pkg/gox -run 'TestGolden|TestErrorGolden'
```

Public-preview readiness spot checks:

```bash
go test ./pkg/gox -run 'Identity|Package|Golden|ErrorGolden|Qualified'
go test ./cmd/goxc -run 'Physical|Canonical|Alias|Overlap|Build|Generate|Ownership|Completion|Marker|Legacy|Partial|Publish|Cleanup|Manifest|Symlink|Path'
```

GOX parser/codegen fuzz smoke, for manual use during compiler work:

```bash
go test ./pkg/gox -run=Fuzz -fuzz=FuzzGenerate -fuzztime=30s
go test ./pkg/gox -run=Fuzz -fuzz=FuzzParseElement -fuzztime=30s
```

These targets reuse the existing golden/error fixtures and small inline seeds.
They are bounded fuzz entry points, not exhaustive GOX language verification.

No separate public-preview script is required yet; the readiness checks are
small enough to stay inside the existing Go test and docs-check gates.

Full local gate, including TinyGo packaging and benchmarks:

```bash
scripts/check.sh
scripts/perf-report.sh
```

Size budget only:

```bash
scripts/size-budget.sh
```

Package delivery details, including content-hashed assets and preload hints,
are documented in `docs/deployment.md`.

Browser smoke:

```bash
scripts/browser-smoke.sh
```

Dashboard DOM pressure:

```bash
node --experimental-websocket scripts/dashboard-dom-pressure.mjs
```

VS Code extension:

```bash
cd extensions/vscode-gox
npm ci
npm test
```

Use `npm run compile` separately during extension development when the pure
test run is not needed.

## Required Tools

Local checks use:

- Go 1.22 or newer;
- TinyGo 0.41.1 for WASM size and browser smoke gates;
- gzip;
- brotli;
- zstd, optional locally but installed in CI;
- Node.js 20 or newer for browser smoke and extension checks;
- Chrome or Chromium for browser smoke;
- curl for smoke server readiness checks.

Set `CHROME=/path/to/chrome` when Chrome is not available as `google-chrome`.
There is no local non-Chrome smoke command in the current preview contract.

## Size Budget Failures

Do not loosen budgets by default. First identify whether the growth comes from:

- production runtime imports;
- debug code accidentally compiled into production;
- example-level code;
- TinyGo version changes;
- compression tool changes.

Run:

```bash
scripts/size-budget.sh
scripts/perf-report.sh
```

Update budgets only when the increase is intentional and documented.

## Performance Baseline

The current runtime and browser baseline is documented in
`docs/performance-baseline.md`.

Not every metric is a hard CI budget. Timing metrics are used for trend
analysis unless explicitly listed as hard gates. Hard gates focus on stable
invariants such as:

- Go, race, vet, debug-tag, and GOX golden tests;
- source import, artifact, and module path gates;
- raw and compressed TinyGo size budgets;
- browser smoke correctness failures;
- dashboard mounted row bounds;
- live DOM and net listener stability;
- virtual table spacer stability;
- no-op scroll scenarios that must not render or mutate DOM.

Informational metrics include exact interaction milliseconds, exact
Layout/Paint duration, exact CDP `Nodes` drift, and exact benchmark `ns/op`.
These should trigger investigation when they move sharply, but they are not
hard CI budgets yet.

## Browser Smoke Failures

Treat smoke failures as either harness failures or app failures.

Harness failures include server bind errors, wrong CDP target, wrong origin,
missing Chrome, unavailable storage, or stale server state. App failures include
DOM identity loss, render counter regressions, localStorage persistence issues,
duplicate key diagnostics failures, dashboard filter/sort regressions,
virtualized collection window regressions, or listener churn regressions.
Router smoke failures include broken hash navigation, missing route params,
not-found fallback regressions, browser back handling regressions, query helper
regressions, form validation regressions in the reference app, duplicate
reference-app data loads on navigation/query changes, resource failure escaping
to Error Boundary fallback UI, or unstable shell layout identity.
Resource smoke failures include broken loading/ready/failed transitions, stale
completion updates, cleanup-after-unmount regressions, or resource failures
escaping into Error Boundary fallback UI.
Error Boundary smoke failures include missing render-failure reports, fallback
or reset regressions, fallback component self-capture/report-loop regressions,
nested-boundary bubbling regressions, protected-subtree cleanup regressions, or
shell identity loss.

The smoke script must not continue against an unknown server or `about:blank`.

## Artifact and Module Gates

`scripts/artifact-check.sh` fails if tracked files include generated bundles,
WASM files, `node_modules`, VSIX packages, or test binaries.

`scripts/module-path-check.sh` enforces:

- `module github.com/graybuton/goframe`;
- no legacy repository path references;
- README documents `go install github.com/graybuton/goframe/cmd/goxc@latest`.
