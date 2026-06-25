# GoFrame Pre-Preview Deep Audit I

## Audit Snapshot

| Field | Value |
|---|---|
| Audited commit | `8c5744560dfad9ef4cb1bd3510c1fc6fb4ffaaa2` |
| Tag at commit | `v0.1.0-mvp30` |
| Audit branch | `chore/pre-preview-deep-audit-i` |
| Module path | `github.com/graybuton/goframe` |
| Audit type | Independent report-only audit |
| Report date | 2026-06-25 |
| Local `main` | `8c5744560dfad9ef4cb1bd3510c1fc6fb4ffaaa2` |
| Local `origin/main` | `8c5744560dfad9ef4cb1bd3510c1fc6fb4ffaaa2` |
| Ahead/behind from `origin/main` before report commit | `0 0` |
| Remote freshness | Not verified. `git fetch origin main` failed with a TLS transport error in this environment. Local `main` and local `origin/main` matched the audited commit. |

This audit intentionally did not implement runtime, compiler, toolchain, example,
script, or CI changes. The only repository change is this report.

## Executive Summary

GoFrame at `8c5744560dfad9ef4cb1bd3510c1fc6fb4ffaaa2` is a coherent
experimental Go-first browser/WASM framework and toolchain with strong local
evidence for the current runtime, GOX compiler, hidden workspace model, package
pipeline, examples, browser smoke, size budgets, and filesystem/package safety
work from MVP 30.

The baseline is green in this audit environment:

- Go, race, vet, debug-tag, and GOX golden tests pass.
- Browser smoke passes across the focused examples, reference app, runtime error
  fixtures, Error Boundary fixture, resource example, and production bundle
  restore pass.
- Dashboard DOM pressure passes with stable live DOM and net listener counts.
- Size budgets pass for every current example.
- Focused filesystem/package safety tests pass, including physical path alias
  overlap, fail-closed ownership, partial publication, symlink rejection, and
  serve symlink entries.

The preliminary public-preview verdict is still conditional. The repository's
own readiness document says `Status: Needs hardening`, primarily because
platform evidence and some public contract decisions remain incomplete. The audit
found no confirmed Critical blocker in the exact audited commit, but it does
confirm High public-preview risks around platform support breadth and reusable
package/component identity policy.

## Preliminary Verdict

| Decision target | Verdict | Rationale |
|---|---|---|
| Continued internal feature work | Go | The checked baseline is green and the main subsystems have focused tests. |
| Narrow evaluator preview on Linux/Chrome | Conditional Go | Viable if release notes clearly bound platform/API expectations and known limits. |
| Broad public preview | No-Go until High findings are resolved or explicitly scoped out | Platform evidence and reusable package identity policy are not preview-complete. |
| Production use | No-Go | The project remains experimental; no production server, SSR/hydration, history router, route loaders, global cache, LSP, or 1.0 API promise. |

## Scope

Audited:

- `pkg/goframe` runtime behavior and API surface.
- `pkg/gox` GOX compiler, diagnostics, parser/codegen tests, and exported
  compiler-facing helpers.
- `cmd/goxc` manifest, workspace, generate, build, package, export, clean, size,
  serve, and doctor paths.
- Examples and browser smoke coverage.
- CI scripts and GitHub Actions workflows.
- Documentation, compatibility, release, platform, and security policy docs.
- Repository hygiene, generated artifacts, optional scanner availability, size
  budgets, and DOM pressure signals.

Not audited:

- Remote GitHub branch freshness beyond the local `origin/main` ref because
  network fetch failed.
- Browser engines other than Chrome/Chromium.
- Windows/macOS filesystem behavior.
- Real deployment infrastructure, CDN behavior, TLS, cache headers, or MIME
  configuration outside the local dev/package checks.
- Hostile concurrent filesystem mutation between validation and operation.
- Third-party vulnerability database state because network-backed audit tools
  were unavailable or blocked.

## Methodology

1. Freeze exact commit identity and compare local `main`, local `origin/main`,
   and the requested SHA.
2. Inventory tracked source, docs, examples, workflows, generated ignored state,
   and public API surface.
3. Run baseline validation and repeatability checks.
4. Run focused filesystem/package safety checks.
5. Run browser smoke and dashboard DOM pressure audits.
6. Run size and benchmark reports.
7. Inspect runtime, compiler, toolchain, scripts, CI, and docs for contract
   alignment.
8. Separate confirmed findings from hypotheses and document reproduction paths.

## Environment

| Tool | Observed value |
|---|---|
| OS | `Linux JIN-WU 6.17.0-35-generic #35-Ubuntu SMP PREEMPT_DYNAMIC Tue May 26 13:10:28 UTC 2026 x86_64 GNU/Linux` |
| Go | `go version go1.24.4 linux/amd64` |
| TinyGo | `tinygo version 0.41.1 linux/amd64 (using go version go1.24.4 and LLVM version 20.1.1)` |
| Node | `v20.19.4` |
| npm | `9.2.0` |
| Git | `git version 2.51.0` |
| Chrome | `Google Chrome 149.0.7827.196` |
| Chromium | Not installed as `chromium` |
| `goxc version` | `goxc version 0.1.0` with Go 1.24.4 and TinyGo 0.41.1 |
| `goxc doctor` | Status `ok` |

Network-backed checks were restricted. `git fetch origin main` failed with a TLS
transport error. `npm audit --json` failed with `connect EPERM 127.0.0.1:10809`.

## Repository Inventory

| Inventory item | Count / result |
|---|---|
| Tracked files | 401 |
| Go files | 156 |
| Go test files | 30 |
| Production Go files under `pkg`/`cmd` | 69 |
| Example apps | 11 |
| Docs markdown files under `docs/` | 31 |
| Total markdown files considered | 50 |
| JS/MJS files | 16 |
| GitHub workflow files | 4 |
| Go modules | Main module only |

Examples:

- `examples/counter`
- `examples/components`
- `examples/todo`
- `examples/dashboard`
- `examples/context`
- `examples/virtualized`
- `examples/multipackage`
- `examples/cmdapp`
- `examples/router`
- `examples/router-dashboard`
- `examples/resource`

Ignored generated state exists under example `.goframe/` directories and the VS
Code extension's `node_modules`, `out`, and `.vsix` outputs. No generated
`.gox.go`, `.wasm`, `dist/`, `build/`, `node_modules`, or `.vsix` files are
tracked.

## Baseline Validation

| Check | Result |
|---|---|
| `git diff --check` | Pass |
| Go formatting check | Pass; no files reported by `gofmt -l`; `scripts/check.sh` also ran `go fmt ./...` |
| `go mod verify` | Pass, `all modules verified` |
| `go mod tidy -diff` | Pass, no diff |
| `go test ./...` | Pass |
| `go test -race ./pkg/... ./cmd/...` | Pass |
| `go vet ./...` | Pass |
| `go test -tags=goframe_debug ./...` | Pass |
| `go test ./pkg/gox -run 'TestGolden|TestErrorGolden'` | Pass |
| Explicit example package tests | Pass for dashboard, context, virtualized, multipackage, cmdapp, router, router-dashboard, resource |
| `scripts/check.sh` | Pass, ended `check: ok` |
| `scripts/size-budget.sh` | Pass |
| `scripts/perf-report.sh` | Pass |
| `scripts/browser-smoke.sh` | Pass, ended `browser smoke: ok` |
| `node --experimental-websocket scripts/dashboard-dom-pressure.mjs` | Pass, ended `Dashboard DOM pressure audit: ok` |
| `scripts/artifact-check.sh` | Pass |
| `scripts/module-path-check.sh` | Pass |
| `node scripts/docs-check.mjs` | Pass |
| JS syntax check for tracked `.js`/`.mjs` files | Pass |
| VS Code extension `npm ci` and `npm run compile` | Pass |
| `npm audit --json` | Not completed; network/proxy restriction |

Repeatability and focused checks:

- `go test -shuffle=on -count=5 ./pkg/goframe ./pkg/gox ./cmd/goxc`: Pass.
- `go test -count=10 ./pkg/goframe`: Pass.
- `go test -count=5 ./pkg/gox`: Pass.
- `go test -count=5 ./cmd/goxc`: Pass.
- `go test ./pkg/gox -run 'TestGolden|TestErrorGolden' -count=3`: Pass.
- `go test ./cmd/goxc -run 'Physical|Canonical|Alias|Overlap|Build|Generate|Ownership|Completion|Marker|Legacy|Partial|Publish|Cleanup|Manifest|Symlink|Path|Serve'`: Pass.
- Same focused `cmd/goxc` test selection with `-race`: Pass.

Package matrix:

- TinyGo release-style package for all 11 examples passed after setting
  `XDG_CACHE_HOME=/tmp/goframe-audit-tinygo-cache`. A first attempt without that
  cache override failed for several examples because `~/.cache/tinygo` was
  read-only in the audit environment.
- Go compiler package path passed for `multipackage`, `cmdapp`, `router`,
  `router-dashboard`, and `resource`.

## Verified Strengths

1. **Runtime containment is meaningfully tested.** Event, effect, cleanup, render,
   memo comparator, Error Boundary, virtualized callback, and resource loader
   panic behavior have pure/runtime and browser evidence.
2. **Resource semantics are crisp for MVP scope.** Component-scoped resources have
   stale generation protection, manual reload, cleanup, no automatic retry after
   loader panic, and browser smoke for focused lifecycle flows.
3. **The reference app is integration-grade.** `router-dashboard` covers router,
   query state, forms, validation, a single resource owner, explicit failed UI,
   safe route Error Boundary recovery, and stable shell identity.
4. **The `goxc` safety model is substantially improved.** The tool rejects
   root-level and intermediate symlinks at controlled boundaries, physical alias
   overlap, false ownership markers, authored-source output, and asset namespace
   collisions.
5. **Generated-file cleanliness is preserved.** Normal workflows keep generated
   `.gox.go` files under `.goframe`; no generated source files are tracked.
6. **Docs are unusually honest for a pre-preview project.** API tiers, platform
   limits, TinyGo panic limitations, package ownership, symlink policy, and
   public-preview readiness are documented with limitations.
7. **The browser harness checks real app behavior, not just compile success.**
   Smoke verifies route navigation, DOM identity, resource attempt counts,
   form validation, runtime errors, Error Boundaries, virtualization, and
   production bundle restore.
8. **The repo has no external Go module dependency surface today.** `go list -m
   all` reported only the main module.

## Findings Summary

| Severity | Count | IDs |
|---|---:|---|
| Blocker | 0 | None confirmed |
| High | 2 | `ARCH-001`, `CI-001` |
| Medium | 5 | `GOXC-001`, `GOX-001`, `API-001`, `SEC-001`, `TEST-001` |
| Low | 2 | `PERF-001`, `DOCS-001` |
| Hygiene | 1 | `HYGIENE-001` |
| Open hypotheses | 3 | `DOM-HYP-001`, `GOXC-HYP-001`, `CI-HYP-001` |

## Blocker Findings

No confirmed Critical/Blocker defect was found at the exact audited commit.

This is not the same as public-preview approval. The preliminary No-Go for broad
preview comes from High readiness risks, not a reproduced correctness failure in
the checked baseline.

## High Findings

### ARCH-001: Reusable and multi-module component identity policy is not preview-complete

**Severity:** High
**Status:** Confirmed readiness blocker
**Location:** `docs/public-preview-readiness.md:94-97`, `docs/api-stability.md:172-184`, `cmd/goxc/workspace.go`, `cmd/goxc/workspace_test.go`

**Contract:** Component identity is central to state preservation, memoization,
debugging, and generated GOX components. A public preview should clearly define
what identities mean across reusable packages, module path changes, and
workspace layouts.

**Evidence:**

- `docs/public-preview-readiness.md` explicitly lists "full multi-module
  workspace/reusable package identity is not promised yet" as a blocker.
- The implementation and tests cover import-aware identity for current
  app-root/multi-package/child-entry layouts, but not a general reusable
  multi-module package story.
- API docs classify generated component id format and router/resource details as
  still experimental.

**Reproduction:**

1. Inspect `docs/public-preview-readiness.md:75-97`.
2. Inspect identity tests in `pkg/goframe/component_identity_test.go`,
   `pkg/gox/generate_test.go`, and `cmd/goxc/workspace_test.go`.
3. Observe absence of an accepted multi-module/reusable-package fixture or
   compatibility contract.

**Impact:** Early users may build assumptions about module path changes,
vendored/reusable packages, or future multi-module layouts that later require
state remounts or migration notes.

**Why Existing Tests Miss It:** Existing tests focus on app-root,
multi-package-within-app, and child-entry package identities. They intentionally
do not cover a full multi-module/reusable package matrix.

**Recommended Remediation:** Before broad preview, either define and test the
reusable-package identity contract or explicitly scope the preview to
single-module app-root workspaces.

**Required Regression Test:** Add fixtures with at least two modules/packages
that import shared GOX components and assert stable component ids, remount
expectations, and diagnostics.

**Dependencies:** Manifest/workspace policy and release compatibility notes.

### CI-001: Platform and browser support evidence is Linux/Chrome-only

**Severity:** High
**Status:** Confirmed readiness blocker
**Location:** `docs/platform-support.md:16-44`, `docs/public-preview-readiness.md:167-180`, `.github/workflows/ci-core.yml:13-59`, `.github/workflows/ci-browser-smoke.yml:17-57`, `.github/workflows/ci-wasm-size.yml:17-89`

**Contract:** A public preview should be clear about supported hosts and browser
targets. If the project appears cross-platform, CI evidence should match that
claim or the preview scope should be narrow.

**Evidence:**

- Platform docs classify Linux amd64 as CI-tested, macOS as expected, Windows as
  unverified, Firefox/Safari as unverified.
- Core, browser smoke, and WASM size workflows all use `ubuntu-latest`.
- Browser smoke uses Chrome through `browser-actions/setup-chrome@v2`.

**Reproduction:**

1. Inspect `docs/platform-support.md:16-44`.
2. Inspect `.github/workflows/*`.
3. Observe no macOS, Windows, Firefox, or Safari workflow matrix.

**Impact:** Windows path behavior, symlink restrictions, shell script behavior,
browser event timing, and Safari/Firefox DOM/WASM differences may remain
unknown for public users.

**Why Existing Tests Miss It:** The current test fleet is strong but
single-platform. Symlink tests skip when the platform cannot create symlinks.

**Recommended Remediation:** Add a small host matrix for pure Go/toolchain tests
and a browser matrix or documented staged plan. If not feasible, make the first
preview explicitly Linux/Chrome evaluator-only.

**Required Regression Test:** CI jobs for macOS and Windows pure Go/toolchain
tests; at least one non-Chrome smoke lane or documented external validation.

**Dependencies:** CI capacity and browser automation availability.

## Medium Findings

### GOXC-001: Package publication is safe-marker oriented but not a full transaction

**Severity:** Medium
**Status:** Confirmed limitation
**Location:** `cmd/goxc/package.go:464-502`, `cmd/goxc/package.go:729-784`, `docs/security-symlink-policy.md:226-242`

**Contract:** A failed package/export publication must not leave a directory
that falsely appears complete or GoFrame-owned.

**Evidence:**

- `cleanPackageArtifacts` removes `goframe-package.json` first, then managed
  artifacts and the assets directory.
- `publishPackageArtifacts` validates staged entries and copies
  `goframe-package.json` last.
- Docs explicitly state this is not a full transactional installer and a failed
  copy may require another successful run.
- Focused tests confirm partial publication does not grant ownership.

**Reproduction:**

1. Inspect `cmd/goxc/package.go:464-502` and `cmd/goxc/package.go:729-784`.
2. Inspect `docs/security-symlink-policy.md:226-242`.
3. Run `go test ./cmd/goxc -run 'Partial|Publish|Cleanup'`.

**Impact:** A failed publish should not be dangerous, but it may leave an output
directory requiring a rerun or manual cleanup. This is acceptable for pre-preview
only because completion markers fail closed.

**Why Existing Tests Miss It:** Existing tests cover marker invalidation and
partial ownership, not full rollback or preservation of a previously complete
package.

**Recommended Remediation:** Consider a transactional publish model: stage a
complete destination tree, verify it, then swap directories or maintain rollback
state where the platform supports it.

**Required Regression Test:** Existing valid package remains usable after
injected mid-publish failure, or the documented rollback guarantee is asserted.

**Dependencies:** Cross-platform directory replacement behavior.

### GOX-001: GOX parser/codegen has no fuzz target

**Severity:** Medium
**Status:** Confirmed test gap
**Location:** `pkg/gox`, `pkg/gox/generate.go`, parser/lexer/codegen tests

**Contract:** GOX is a language surface. Parser and codegen error handling should
be hardened against malformed input beyond curated golden fixtures.

**Evidence:**

- `rg '^func Fuzz' .` returned no fuzz targets.
- Golden and error golden tests pass, but they are example-based.
- Coverage shows strong but not exhaustive parser/codegen exercise; file helper
  branches and some lexer/expression helpers remain low or uncovered.

**Reproduction:**

1. Run `rg '^func Fuzz' .`.
2. Run `go test -cover ./pkg/gox`.
3. Inspect coverage for lexer/expression/file-generation helpers.

**Impact:** Parser panics, slow paths, or confusing diagnostics could survive
until a user writes unusual GOX. This is more likely as package-qualified tags
and expression parsing grow.

**Why Existing Tests Miss It:** Golden tests encode known good/bad cases; they do
not generate malformed nesting, expressions, comments, strings, UTF-8, or very
large inputs.

**Recommended Remediation:** Add fuzz targets for `GenerateWithOptions`,
`ParseElement`, and error diagnostic formatting. Seed with current golden and
error golden files.

**Required Regression Test:** Fuzz corpus plus fixed regression seeds for any
found panics or diagnostic crashes.

**Dependencies:** CI budget for short fuzz smoke or nightly fuzzing.

### API-001: Exported `pkg/gox` file helpers do not inherit `goxc` filesystem hardening

**Severity:** Medium
**Status:** Confirmed API/contract risk
**Location:** `pkg/gox/generate.go:101-132`, `pkg/gox/generate.go:135-158`, `docs/api-stability.md:128-146`

**Contract:** The public compiler package is exported. Even if primarily
compiler-facing, external tools can call `GenerateFileToWithOptions` and
`FindFiles` directly.

**Evidence:**

- `GenerateFileToWithOptions` reads with `os.ReadFile`, creates parent
  directories with `os.MkdirAll`, and writes with `os.WriteFile`.
- `FindFiles` starts with `os.Stat` and `filepath.WalkDir`.
- The hardened no-follow, physical-overlap, and atomic destination contracts live
  in `cmd/goxc`, not in these exported helpers.
- API docs classify these functions as exported compiler-facing/tooling
  contracts, not internal-only.

**Reproduction:**

1. Inspect `pkg/gox/generate.go:101-158`.
2. Compare with `cmd/goxc/helpers.go:40-101` and `cmd/goxc/generate.go`.
3. Observe that direct library users can bypass CLI safety by calling
   `pkg/gox` file helpers.

**Impact:** This is not a `goxc` CLI vulnerability at the audited commit, but it
is a public-package footgun for third-party tooling that assumes the compiler
package has the same no-follow semantics as the CLI.

**Why Existing Tests Miss It:** Filesystem safety tests target `cmd/goxc`.
`pkg/gox` tests focus on parsing, generation, and golden diagnostics.

**Recommended Remediation:** Either harden `pkg/gox` file helpers or document
them as trusted-filesystem convenience helpers and recommend byte-oriented
`Generate*` plus caller-owned safe writes for untrusted trees.

**Required Regression Test:** Direct `pkg/gox.GenerateFileToWithOptions` tests
with symlink source/destination if the helper is hardened, or docs-check rule if
it remains trusted-only.

**Dependencies:** API stability policy for `pkg/gox` exported helpers.

### SEC-001: Supply-chain scanner evidence is incomplete

**Severity:** Medium
**Status:** Confirmed audit limitation
**Location:** `.github/workflows/*.yml`, `extensions/vscode-gox/package.json`, local tool environment

**Contract:** Public-preview readiness should include a repeatable dependency
and workflow supply-chain story.

**Evidence:**

- `govulncheck`, `staticcheck`, `shellcheck`, and `actionlint` were not installed
  in the audit environment.
- `npm audit --json` failed with a network/proxy restriction.
- GitHub Actions are version-tag pinned, not immutable SHA pinned:
  `actions/checkout@v7`, `actions/setup-go@v6`, `actions/setup-node@v6`,
  `browser-actions/setup-chrome@v2`, `actions/upload-artifact@v7`.
- TinyGo is downloaded by workflow via `curl` from GitHub release assets and then
  installed without a checked-in checksum.

**Reproduction:**

1. Run `command -v govulncheck staticcheck shellcheck actionlint`.
2. Run `npm audit --json` under the current restricted environment.
3. Inspect `.github/workflows/ci-browser-smoke.yml:36-49` and
   `.github/workflows/ci-wasm-size.yml:31-40`.

**Impact:** No vulnerability was found by this audit, but vulnerability absence
was not proven. Action or binary supply-chain changes could affect CI before a
human review notices.

**Why Existing Tests Miss It:** Existing CI focuses on correctness, size, smoke,
and extension compile checks. It does not run vulnerability/static/action lint
gates.

**Recommended Remediation:** Add `govulncheck` and action/shell workflow linting
to CI. Consider checksums for downloaded TinyGo archives and a policy for
immutable GitHub Action SHAs before broad preview.

**Required Regression Test:** CI job that runs the selected scanners and fails on
new actionable findings, with documented exemptions.

**Dependencies:** Network availability and scanner version policy.

### TEST-001: Some public command and helper paths have low branch/function coverage

**Severity:** Medium
**Status:** Confirmed test-quality gap
**Location:** `cmd/goxc`, `pkg/gox`, coverage reports from this audit

**Contract:** Public-preview tools should have direct tests for user-facing CLI
surfaces and exported helper behavior, not only lower-level helper coverage.

**Evidence:**

- Coverage summary: `pkg/goframe` 83.9%, `pkg/gox` 81.0%, `cmd/goxc` 62.7%.
- `cmd/goxc` command wrapper functions such as `buildCommand`,
  `generateCommand`, `cleanCommand`, `exportCommand`, `packageCommand`,
  `serveCommand`, `versionCommand`, `main`, and `usage` showed 0% in coverage
  output, even though their internals are tested through helper functions.
- `pkg/gox` file helpers `GenerateFile`, `GenerateFileTo`,
  `GenerateFileToWithOptions`, and `FindFiles` showed 0% in coverage output.
- Runtime pure coverage has intentional gaps for browser-only renderer,
  non-JS stubs, debug stubs, and deprecated dependency aliases.

**Reproduction:**

1. Run `go test -cover ./pkg/goframe ./pkg/gox ./cmd/goxc`.
2. Run `go test -coverprofile=/tmp/goframe-goxc.cover ./cmd/goxc`.
3. Run `go tool cover -func=/tmp/goframe-goxc.cover`.

**Impact:** Important behavior is still exercised through scripts and smoke, but
coverage does not make all user-facing command branches easy to track. Future
CLI regression risk is higher than the green baseline alone suggests.

**Why Existing Tests Miss It:** Many tests call internal helper functions and
scripts invoke real commands. Coverage attribution still leaves wrappers and
some file helpers unmeasured.

**Recommended Remediation:** Add focused command-level tests for parse/error
paths and direct `pkg/gox` file helper tests. Do not chase 100% coverage; target
public branch risk.

**Required Regression Test:** A coverage-aware test plan for command wrappers,
serve error paths, compression helpers, and `pkg/gox` file helpers.

**Dependencies:** None.

## Low Findings

### PERF-001: Several WASM budgets have little headroom

**Severity:** Low
**Status:** Confirmed performance-maintenance risk
**Location:** `scripts/size-budget.sh:130-141`, size report from this audit

**Contract:** Size budgets should catch regressions without becoming so tight
that small intentional changes constantly require budget churn.

**Evidence:**

- Current budgets pass.
- Some margins are small, for example dashboard raw `168628 B / 168960 B` and
  context raw `115354 B / 116736 B`.
- Router-dashboard is larger by design but still within budget:
  `226274 B / 230400 B` raw.

**Reproduction:** Run `scripts/size-budget.sh`.

**Impact:** Small future runtime/example changes may break budgets. This is good
as an early warning but can create release friction if budgets are not tied to a
review policy.

**Why Existing Tests Miss It:** Budgets detect current overages, not future
maintenance pressure.

**Recommended Remediation:** Keep budgets strict, but record expected headroom
and require investigation before changing them.

**Required Regression Test:** Existing `scripts/size-budget.sh` gate is adequate.

**Dependencies:** TinyGo version stability.

### DOCS-001: Public readiness status is intentionally mixed and could confuse readers

**Severity:** Low
**Status:** Confirmed documentation clarity risk
**Location:** `docs/public-preview-readiness.md:1-35`, `docs/public-preview-readiness.md:132-180`

**Contract:** A newcomer should understand the difference between subsystem
readiness and full public-preview readiness.

**Evidence:**

- The document starts with `Status: Needs hardening`.
- Later subsystem tables mark runtime, GOX, toolchain, docs, and filesystem as
  `Ready with limitations`.
- This is accurate, but easy to skim as contradictory.

**Reproduction:** Read `docs/public-preview-readiness.md` top-to-bottom.

**Impact:** A release reviewer may mistake subsystem "Ready with limitations"
for preview approval, or vice versa.

**Why Existing Tests Miss It:** Docs checks verify links and inventory, not
reader interpretation.

**Recommended Remediation:** Add a one-paragraph "how to read this status" note
at the top of the readiness doc.

**Required Regression Test:** Not necessary; docs review item.

**Dependencies:** Release owner decision.

## Hygiene Findings

### HYGIENE-001: Local validation creates large ignored artifacts

**Severity:** Hygiene
**Status:** Confirmed operational note
**Location:** `.gitignore`, example `.goframe/` directories,
`extensions/vscode-gox/node_modules`, `extensions/vscode-gox/out`,
`extensions/vscode-gox/*.vsix`

**Contract:** The working tree should stay clean after validation, but ignored
generated state can still accumulate.

**Evidence:**

- `git status --ignored --short` reports ignored `.goframe/` directories under
  examples and ignored VS Code extension build/dependency outputs.
- `git status --short` remains clean.

**Reproduction:** Run `git status --ignored --short`.

**Impact:** Disk usage and local confusion only. No tracked artifact issue was
found.

**Why Existing Tests Miss It:** Artifact checks intentionally focus on tracked
files.

**Recommended Remediation:** Keep current `.gitignore`; optionally document a
local cleanup command for contributors.

**Required Regression Test:** Existing `scripts/artifact-check.sh` is enough for
tracked artifact hygiene.

**Dependencies:** None.

## Runtime Audit

Reviewed files:

- `pkg/goframe/component.go`
- `pkg/goframe/state.go`
- `pkg/goframe/effects.go`
- `pkg/goframe/context.go`
- `pkg/goframe/errors.go`
- `pkg/goframe/resource.go`
- `pkg/goframe/router.go`
- `pkg/goframe/router_js.go`
- `pkg/goframe/render_js.go`
- `pkg/goframe/mount_js.go`
- `pkg/goframe/event.go`
- `pkg/goframe/virtual.go`

Verified runtime behaviors:

- Render panics are reported as `ErrorPhaseRender` and can be captured by the
  nearest scoped Error Boundary.
- Event handler panics are recovered and reported as `ErrorPhaseEvent`.
- Effect setup, effect cleanup, and unmount cleanup panics are reported with the
  documented phase behavior.
- Memo comparator panics report and fall back to "do not skip render".
- Context selector panic during render reports and re-panics where no safe
  fallback exists.
- Virtualized item/row callbacks report render errors and fall back to empty
  item/row content where implemented.
- Resource loader panics are contained inside resource-specific logic so
  same-key rerenders do not automatically retry a panicking loader.
- Error Boundary fallback self-capture is guarded so fallback panics bubble.

No confirmed runtime correctness defect was found.

## Browser And DOM Audit

Browser smoke passed.

Covered smoke scenarios:

- Todo reconciliation and persistence.
- Duplicate key debug diagnostics.
- Runtime error containment.
- Scoped Error Boundary behavior.
- Dashboard interaction.
- Context selector rerender isolation.
- Virtualized collection scroll/selection/toggle behavior.
- Multi-package GOX workspace.
- Child-entry package app.
- Hash router navigation.
- Router-dashboard reference app, including route-error safe recovery.
- Resource lifecycle, stale completion, failure, retry, and cleanup.
- Production bundle restore.

Dashboard DOM pressure passed:

- `liveDOMAllStart`: 486
- `liveDOMAllEnd`: 486
- `netListenersAllStart`: 0
- `netListenersAllEnd`: 0
- `postIdleJSEventListeners`: 63
- `mountedRowsMax`: 28
- `continuousScrollSteps`: 321
- `continuousVirtualTableRenders`: 8
- `continuousIssueRowRenders`: 72
- `continuousRenderScrollRatio`: 0.02
- `continuousListenerNetDelta`: 0
- `spacerTopStable`: true
- `spacerBottomStable`: true

CDP `Nodes` drifted by 25346 and was reported as informational by the harness.
The live DOM and listener invariants passed.

## GOX Audit

Verified:

- Golden and error golden tests pass repeatedly.
- Source-oriented diagnostics are covered.
- Package-qualified component tags are documented and tested.
- Import-aware component identity works for current app-root, multi-package, and
  child-entry layouts.
- Generated files stay under `.goframe` in normal workflows.

Risks:

- `GOX-001`: no fuzz target yet.
- `API-001`: exported `pkg/gox` file helpers do not carry CLI filesystem safety.

## goxc Audit

Audited commands:

- `generate`
- `build`
- `package`
- `export`
- `serve`
- `size`
- `doctor`
- `clean`
- `version`

Verified:

- App-root symlink rejection.
- Entry, `.gox`, `.go`, asset, package source, output, and serve symlink
  rejection.
- Root-aware component checks with `os.Lstat`.
- Physical/canonical overlap detection for independently supplied roots.
- External workspace overlap rejection.
- Explicit build/generate/package/export output overlap rejection.
- Manifest `wasm` `.wasm` contract.
- Package asset namespace collision rejection.
- Safe destination writes through temporary sibling files and `os.Rename`.
- `goframe-package.json` as the current authoritative completion marker.
- `asset-manifest.json` as companion metadata only.
- Legacy package ownership fail-closed to historical GoFrame manifest shape.
- Partial publication does not grant current package ownership.
- Clean removes final symlink targets as links and rejects intermediate symlink
  traversal.
- Dev serve rejects symlink root and entries.

Limitations:

- `GOXC-001`: package publication is not a full rollback transaction.
- Hostile concurrent filesystem mutation remains out of scope by design.
- Windows filesystem behavior is not CI-verified.

## Security Audit

Confirmed security posture:

- No `unsafe` matches in `./pkg ./cmd`.
- `reflect` matches are tests or metadata assertions, not production runtime
  behavior.
- `pkg/goframe` production runtime avoids `fmt`, `net/http`, `encoding/json`,
  `regexp`, and `runtime/debug`.
- `cmd/goxc` uses standard library filesystem, JSON, HTTP, regexp, and debug
  helpers where expected for tooling.
- `goxc` has a documented fail-closed file safety model and matching tests.

Limitations:

- No `govulncheck` evidence due missing tool.
- No `npm audit` evidence due network/proxy restriction.
- GitHub Actions are tag-pinned rather than SHA-pinned.
- TinyGo workflow download has no checked-in checksum.
- Full TOCTOU resistance against a hostile local process is out of scope.

## Dependency And Supply-Chain Audit

Go dependency surface:

- `go list -m all` reported only the main module.
- `go mod verify` passed.
- `go mod tidy -diff` was clean.

VS Code extension:

- `npm ci` passed.
- `npm run compile` passed.
- `npm audit --json` could not complete due network/proxy restriction.

GitHub Actions:

- Workflows use read-only repository contents permissions.
- Actions are version-tag pinned, not immutable SHA pinned.
- TinyGo installation uses a remote release asset download.

Confirmed finding: `SEC-001`.

## CI And Scripts Audit

Workflows:

- Core Go/GOX checks.
- TinyGo WASM size/package checks.
- Browser smoke.
- VS Code extension compile.

Scripts:

- `scripts/check.sh`
- `scripts/size-budget.sh`
- `scripts/perf-report.sh`
- `scripts/browser-smoke.sh`
- `scripts/dashboard-dom-pressure.mjs`
- `scripts/artifact-check.sh`
- `scripts/module-path-check.sh`
- `scripts/docs-check.mjs`

Verified:

- Local script suite is aligned with docs.
- Browser smoke is broad and high value.
- Size budgets include all current examples.
- Docs check passes.

Gaps:

- No scanner/linter gates for `govulncheck`, `staticcheck`, `shellcheck`, or
  `actionlint`.
- No cross-platform CI matrix yet.

## Test-Quality Audit

Coverage summary:

- `pkg/goframe`: 83.9%
- `pkg/gox`: 81.0%
- `cmd/goxc`: 62.7%

Positive:

- High-value behavior tests exist for resources, Error Boundaries, router,
  context selectors, virtualization, runtime error handling, GOX diagnostics,
  package identity, filesystem safety, package/export ownership, and browser
  smoke.
- Race tests pass for `pkg/...` and `cmd/...`.
- Repeated and shuffled tests passed for `pkg/goframe`, `pkg/gox`, and
  `cmd/goxc`.

Gaps:

- No fuzz targets.
- Some command wrappers and file helper functions have low direct coverage.
- Browser-only code is necessarily covered mostly by smoke rather than pure Go
  coverage.

Confirmed findings: `GOX-001`, `TEST-001`.

## Performance And Size Audit

Size report:

| Example | Raw | gzip | br | zstd |
|---|---:|---:|---:|---:|
| counter | 83550 | 33569 | 28000 | 30158 |
| components | 89198 | 35208 | 29238 | 31563 |
| todo | 117409 | 44980 | 37487 | 40506 |
| dashboard | 168628 | 62874 | 50808 | 55093 |
| context | 115354 | 43243 | 35569 | 38445 |
| virtualized | 123144 | 47408 | 38780 | 42098 |
| multipackage | 94354 | 36850 | 30728 | 33175 |
| cmdapp | 94380 | 36839 | 30720 | 33124 |
| router | 114716 | 43602 | 36062 | 39026 |
| router-dashboard | 226274 | 91135 | 74530 | 79742 |
| resource | 147673 | 64582 | 54635 | 58106 |

Runtime benchmark snapshot:

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| DirtyQueuePruning | 391.2 | 32 | 1 |
| MatchChildIndicesKeyed | 12148 | 8744 | 6 |
| MatchChildIndicesUnkeyed | 2295 | 8744 | 6 |
| SplitProps | 2559 | 848 | 15 |
| EventNameNormalization | 265.4 | 24 | 3 |
| StateSlotAccess | 300.8 | 96 | 4 |
| UnwrapKeyedNode | 8.348 | 0 | 0 |

Confirmed finding: `PERF-001`.

## Public API Audit

Verified:

- `docs/api-stability.md` classifies public-candidate, experimental,
  compiler-facing, internal, and deprecated surfaces.
- `go doc ./pkg/goframe` exposes the expected runtime APIs:
  components, hooks, contexts, router, query helpers, resources, Error
  Boundaries, virtualization, events, and low-level node helpers.
- Runtime error, resource, router, and Error Boundary APIs are documented as
  experimental/public-candidate rather than stable 1.0.
- `pkg/gox` exported APIs are documented as compiler-facing/experimental.

Risk:

- `API-001`: exported file helpers in `pkg/gox` have a weaker filesystem safety
  contract than `goxc`.

## Documentation Audit

Verified:

- README, tutorial, API stability, runtime model, router, resources, Error
  Boundaries, forms, deployment, performance, CI, manifest compatibility,
  symlink policy, platform support, release, and public preview readiness docs
  are present and link-check clean.
- Docs correctly distinguish TinyGo size-oriented builds from Go/WASM
  recover-capable panic demos.
- Docs correctly state that `goxc serve` is development-only.
- Docs correctly state no router loaders, global cache, SSR/hydration, production
  server, LSP, formatter, Player/Engine, or schema validation framework.

Risk:

- `DOCS-001`: readiness status is accurate but easy to misread without context.

## Repository Hygiene

Verified:

- `scripts/artifact-check.sh` passes.
- `scripts/module-path-check.sh` passes.
- No tracked generated `.gox.go`, `.wasm`, `.goframe`, `dist`, `build`,
  `node_modules`, or `.vsix` artifacts were found.
- Working tree remained clean before creating this report.

Hygiene note:

- Validation creates ignored artifacts. This is expected and does not affect
  tracked state.

## Dead And Legacy Inventory

Intentional legacy/deprecated surfaces:

- `UseMount`
- `NoDeps`
- `AlwaysDeps`
- `DepsOf` and `Dep*` helpers
- `For` and `ForIndexed`
- `goxc build --release`
- Legacy `"wasm": "main.wasm"`
- Legacy `manifest.json` package migration support, fail-closed to historical
  GoFrame shape
- `goxc generate --in-place`

No untracked-but-unignored generated artifact problem was found.

## Test-Gap Map

| Area | Gap | Suggested next test |
|---|---|---|
| GOX parser/codegen | No fuzz target | Fuzz `GenerateWithOptions` and `ParseElement` with golden seeds |
| `pkg/gox` file helpers | No direct filesystem safety coverage | Symlink source/destination tests or trusted-helper docs |
| CLI wrappers | Low direct coverage | Command-level option/error path tests |
| Cross-platform path behavior | Linux-only evidence | Windows/macOS pure toolchain CI lanes |
| Browser compatibility | Chrome-only evidence | Firefox/WebKit exploratory smoke or documented external validation |
| Supply chain | No scanner gates | Add `govulncheck`, action lint, shell lint, npm audit where network permits |
| Package publication | No full rollback guarantee | Injected failure preserving previous package, if full transaction is added |

## Risk Register

| ID | Risk | Likelihood | Impact | Owner area |
|---|---|---:|---:|---|
| ARCH-001 | Component identity semantics change for reusable packages | Medium | High | Architecture/toolchain |
| CI-001 | Platform-specific path/browser failure appears after preview | Medium | High | CI/toolchain |
| GOXC-001 | Failed publish requires manual repair/rerun | Low | Medium | Toolchain |
| GOX-001 | Parser edge case discovered by user input | Medium | Medium | GOX |
| API-001 | Third-party tool assumes `pkg/gox` file helpers are no-follow safe | Medium | Medium | API/tooling |
| SEC-001 | Undetected dependency/workflow vulnerability | Low/Medium | Medium | Release/CI |
| TEST-001 | CLI wrapper regression escapes helper tests | Medium | Medium | Toolchain tests |
| PERF-001 | Small intended change breaks tight size budgets | Medium | Low | Release/perf |

## Open Hypotheses

### DOM-HYP-001: Browser scheduler callbacks assume eventual invocation

`mount_js.go` releases scheduled JS callbacks from inside the callback path.
This is normal for `requestAnimationFrame`/`queueMicrotask` use, and browser
smoke did not show listener or DOM leaks. I did not reproduce a leak. A future
stress fixture could simulate scheduler starvation if this becomes important.

### GOXC-HYP-001: Windows filesystem behavior may differ in edge cases

The code uses `filepath`, `os.Lstat`, `os.SameFile`, and standard-library
operations, but Windows symlink permissions, rename behavior, and path casing are
not CI-verified. This remains a platform evidence gap, not a reproduced failure.

### CI-HYP-001: Action tag pinning may be acceptable for current phase

Workflows use version tags rather than immutable SHAs. This is a supply-chain
policy question. I did not find a compromised action or current exploit path.

## Audit Limitations

- Remote freshness could not be verified because `git fetch origin main` failed
  with a TLS transport error.
- Network-backed vulnerability checks could not complete.
- Optional scanners were not installed locally.
- No Firefox, Safari, macOS, or Windows validation was performed.
- Browser validation used local Chrome/CDP.
- No hostile concurrent filesystem mutation testing was performed.
- The audit did not attempt to prove all possible TinyGo panic/recover behavior;
  docs already state TinyGo trap-mode limitations.

## Remediation Map

| Priority | Item | Proposed owner area |
|---|---|---|
| P0 before broad preview | Resolve or explicitly scope `ARCH-001` | Architecture/toolchain |
| P0 before broad preview | Resolve or explicitly scope `CI-001` | CI/release |
| P1 | Decide transactional package publication policy | Toolchain |
| P1 | Add GOX fuzz targets | Compiler |
| P1 | Clarify or harden `pkg/gox` file helper safety contract | API/tooling |
| P1 | Add supply-chain scanner gates | Release/CI |
| P2 | Improve command wrapper coverage | Toolchain tests |
| P2 | Add readiness status reading note | Docs |
| P2 | Track size budget headroom intentionally | Performance/release |

## Recommended Next Step

Before broad public preview, run a targeted "Preview Readiness Closeout" pass:

1. Decide whether the first preview explicitly excludes reusable/multi-module
   component identity guarantees, or add the missing policy/tests.
2. Add at least minimal macOS/Windows pure toolchain CI or make Linux-only scope
   explicit in release notes.
3. Add `govulncheck` plus workflow/script linting where practical.
4. Add GOX fuzz seeds from current golden/error fixtures.
5. Decide whether `pkg/gox` file helpers should be hardened or documented as
   trusted convenience helpers.

## Appendix A: Command Log Summary

Snapshot and inventory:

```bash
git branch --show-current
git status --short
git rev-parse HEAD
git rev-parse main
git rev-parse origin/main
git rev-list --left-right --count origin/main...HEAD
git tag --points-at 8c5744560dfad9ef4cb1bd3510c1fc6fb4ffaaa2
git ls-files | wc -l
git ls-files '*.go' | wc -l
git ls-files '*_test.go' | wc -l
go doc ./pkg/goframe
```

Validation:

```bash
git diff --check
go mod verify
go mod tidy -diff
go test ./...
go test -race ./pkg/... ./cmd/...
go vet ./...
go test -tags=goframe_debug ./...
go test ./pkg/gox -run 'TestGolden|TestErrorGolden'
scripts/check.sh
scripts/size-budget.sh
scripts/perf-report.sh
scripts/browser-smoke.sh
node --experimental-websocket scripts/dashboard-dom-pressure.mjs
scripts/artifact-check.sh
scripts/module-path-check.sh
node scripts/docs-check.mjs
```

Repeatability:

```bash
go test -shuffle=on -count=5 ./pkg/goframe ./pkg/gox ./cmd/goxc
go test -count=10 ./pkg/goframe
go test -count=5 ./pkg/gox
go test -count=5 ./cmd/goxc
go test ./pkg/gox -run 'TestGolden|TestErrorGolden' -count=3
```

Focused safety:

```bash
go test ./cmd/goxc -run 'Physical|Canonical|Alias|Overlap|Build|Generate|Ownership|Completion|Marker|Legacy|Partial|Publish|Cleanup|Manifest|Symlink|Path|Serve'
go test -race ./cmd/goxc -run 'Physical|Canonical|Alias|Overlap|Build|Generate|Ownership|Completion|Marker|Legacy|Partial|Publish|Cleanup|Manifest|Symlink|Path|Serve'
```

Package matrix:

```bash
XDG_CACHE_HOME=/tmp/goframe-audit-tinygo-cache \
  goxc package ./examples/<all current examples> --compiler=tinygo --asset-hash --preload --compress=gzip,br

goxc package ./examples/multipackage --compiler=go --asset-hash --preload --compress=gzip,br
goxc package ./examples/cmdapp --compiler=go --asset-hash --preload --compress=gzip,br
goxc package ./examples/router --compiler=go --asset-hash --preload --compress=gzip,br
goxc package ./examples/router-dashboard --compiler=go --asset-hash --preload --compress=gzip,br
goxc package ./examples/resource --compiler=go --asset-hash --preload --compress=gzip,br
```

Scanner availability:

```bash
command -v govulncheck
command -v staticcheck
command -v shellcheck
command -v actionlint
npm audit --json
```

## Appendix B: Source Safety Grep

`rg 'reflect|unsafe' ./pkg ./cmd` found no `unsafe`. `reflect` matches were in
tests or import-list assertions, not production runtime logic.

Heavy production runtime import expectations held: `pkg/goframe` production
files did not add `fmt`, `net/http`, `encoding/json`, `regexp`, or
`runtime/debug`.

## Appendix C: Historical Legacy Package Evidence

Repository history at commit `5cc776e` shows the historical package
`manifest.json` structure included:

- `Name`
- `Compiler`
- `WASM`
- `Assets`
- `ToolchainVersion`

This supports the current fail-closed legacy recognition policy. Older
`isGoframeOwnedExport` behavior at commit `40b9f6a` trusted marker filenames more
broadly; the audited commit no longer does that for current ownership.

## Appendix D: Report-Only Diff Contract

The intended diff for this audit branch is:

```text
docs/pre-preview-deep-audit-i.md
```

No runtime, compiler, toolchain, example, script, workflow, size budget, or CI
metadata changes are part of this audit report.
