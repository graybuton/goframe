# Final Preview Readiness Audit

## Audit Metadata

| Field | Value |
|---|---|
| Repository | `github.com/graybuton/goframe` |
| Branch | `docs/final-preview-readiness-audit` |
| Base SHA | `a0f3aa16fd5aee644a6c4050ccaf33d2c2c1b07e` |
| Base commit | `Merge branch 'docs/v0.1.0-preview-release-story'` |
| Audit date | 2026-06-27 |
| Audited target | `v0.1.0-preview.1` |
| Mode | Audit-only |
| Fetch status | Failed: `fatal: could not read Username for 'https://github.com': No such device or address` |
| `main == origin/main` at start | Yes, both local refs pointed at `a0f3aa16fd5aee644a6c4050ccaf33d2c2c1b07e` |
| Commit range audited | Local `main`/`origin/main` at `a0f3aa16fd5aee644a6c4050ccaf33d2c2c1b07e`; no remote freshness beyond the local matching refs was verified |

Environment observed during the audit:

| Tool | Observed value |
|---|---|
| Go | `go version go1.24.4 linux/amd64` |
| TinyGo | `tinygo version 0.41.1 linux/amd64 (using go version go1.24.4 and LLVM version 20.1.1)` |
| Node.js | `v20.19.4` |
| Git | `git version 2.51.0` |
| Chrome | `/usr/bin/google-chrome` |
| Compression tools | `gzip`, `brotli`, and `zstd` available |
| Go modules | Main module only: `github.com/graybuton/goframe` |

The earlier `docs/pre-preview-deep-audit-i.md` report is not present on
`main`; it was inspected from `chore/pre-preview-deep-audit-i` with
`git show chore/pre-preview-deep-audit-i:docs/pre-preview-deep-audit-i.md`.

## Executive Verdict

Verdict: **Conditional Go** for `v0.1.0-preview.1` evaluator preview.

No Blocker or High finding was confirmed against the audited `main`. The local
validation suite passed, release notes and evaluator guidance exist, and the
remaining material limitations are stated in the preview docs instead of being
hidden.

The verdict is conditional because the actual release still requires ordinary
mechanical release work outside this audit PR:

- merge this audit report;
- verify the audit PR's GitHub Actions run;
- create the signed `v0.1.0-preview.1` tag from the intended merge commit;
- keep the documented platform/browser and API maturity caveats in the release
  notes.

This is not a production-readiness verdict and not a 1.0 compatibility promise.

## Scope Boundary

The factual preview boundary is the current browser/WASM application layer:

- `pkg/goframe` runtime primitives;
- `pkg/gox` GOX compiler and diagnostics;
- `cmd/goxc` generate/build/package/export/serve/size/clean/doctor toolchain;
- static package/export workflow;
- package metadata and manifest compatibility model;
- focused examples plus the router-dashboard reference app;
- docs, CI, browser smoke, size budget, and release evidence.

Non-goals for `v0.1.0-preview.1`:

- production readiness;
- stable 1.0 API compatibility;
- SSR or hydration;
- history-mode routing or server fallback automation;
- route loaders, middleware, Suspense, global resource cache, or server
  resources;
- production deployment server;
- broad reusable multi-module package/component ecosystem;
- Player/Engine or `.gfapp` packaging.

Scope preview != scope project. GoFrame remains an experimental Go-first
application platform. The preview validates the evidenced browser/WASM layer
without deleting or hiding working experimental surfaces such as router,
resources, runtime error reporting, and Error Boundaries.

## Findings

Summary:

| Severity | Count |
|---|---:|
| Blocker | 0 |
| High | 0 |
| Medium | 0 |
| Low | 1 |
| Non-blocking Observation | 5 |

### GF-AUDIT-001

Severity: Low

Surface: Documentation wording hygiene.

Files/commands inspected:

- `docs/compatibility.md`
- `docs/architecture.md`
- `rg -n '\b(will|planned|future follow-up|may be added later|may become|in the future|roadmap)\b' README.md CHANGELOG.md docs`

Fact: Product-facing preview docs are mostly fact-first. A small amount of
bounded policy/vision wording remains, including `planned release stage` in
`docs/compatibility.md` and future-vision wording in architecture context.
These statements do not contradict the release notes or evaluator guide, and
they do not promise a shipped preview feature.

Risk: A strict release-note reader could interpret the deprecation-policy word
`planned` as roadmap-like rather than as release-stage policy.

Recommended follow-up: Optional docs-only wording polish that replaces the
remaining policy phrase with current-contract language.

Blocks `v0.1.0-preview.1`: No.

### GF-AUDIT-002

Severity: Non-blocking Observation

Surface: Platform/browser evidence.

Files/commands inspected:

- `docs/platform-support.md`
- `docs/release-notes-v0.1.0-preview.1.md`
- `.github/workflows/ci-core.yml`
- `.github/workflows/ci-browser-smoke.yml`
- `scripts/browser-smoke.sh`

Fact: Linux/Chrome is the strongest browser evidence. macOS and Windows have
minimal Go/toolchain CI evidence. Firefox and Safari are explicitly unverified.

Risk: Browser engine differences remain possible outside Chrome/Chromium.

Recommended follow-up: Add a separate minimal non-Chrome validation branch only
when the project wants to widen preview evidence. Do not imply current
equivalent browser support.

Blocks `v0.1.0-preview.1`: No, because the limitation is accurately scoped in
the release notes and platform docs.

### GF-AUDIT-003

Severity: Non-blocking Observation

Surface: Component identity.

Files/commands inspected:

- `docs/component-identity.md`
- `docs/api-stability.md`
- `pkg/goframe/component_identity_test.go`
- `pkg/gox/generate_test.go`
- `cmd/goxc/workspace_test.go`

Fact: Generated typed identity and package-qualified GOX behavior are
documented and tested for one app/module tree. Broad reusable multi-module
component package identity remains outside the preview promise.

Risk: Users evaluating reusable packages across independent modules can see
remounts or identity changes that are not preview-stable.

Recommended follow-up: Keep broad reusable package identity outside the
preview promise until a dedicated design/test pass extends evidence.

Blocks `v0.1.0-preview.1`: No, because the release notes and identity docs
state the limit directly.

### GF-AUDIT-004

Severity: Non-blocking Observation

Surface: Package publication.

Files/commands inspected:

- `cmd/goxc/package.go`
- `cmd/goxc/filesystem_safety_test.go`
- `docs/deployment.md`
- `docs/security-symlink-policy.md`
- `docs/manifest-compatibility.md`

Fact: Package publication is staged, metadata-last, fail-closed, and verified
after publication. `goframe-package.json` is the authoritative completion
marker. The docs correctly say this is not a full transactional rollback
installer.

Risk: A failed copy after old managed artifacts are cleaned may require another
successful package/export run to restore the directory.

Recommended follow-up: No preview fix required. Keep the current limitation in
release notes and security docs.

Blocks `v0.1.0-preview.1`: No.

### GF-AUDIT-005

Severity: Non-blocking Observation

Surface: Release mechanics.

Files/commands inspected:

- `docs/release.md`
- `docs/release-notes-v0.1.0-preview.1.md`
- `docs/evaluator-guide.md`
- `README.md`

Fact: The repository has draft release notes and an evaluator guide. No
`v0.1.0-preview.1` tag is created by these docs. The install command using
`@latest` becomes release-accurate only after the preview tag exists on the
remote module path.

Risk: Before the tag is cut, `@latest` may resolve to the latest existing
milestone tag rather than the preview release content.

Recommended follow-up: During release execution, create the signed preview tag
from the intended merge commit before telling external evaluators to use
`@latest`.

Blocks `v0.1.0-preview.1`: No. This is a normal release-order requirement.

### GF-AUDIT-006

Severity: Non-blocking Observation

Surface: Remote evidence.

Files/commands inspected:

- `git fetch origin main`
- local `main` and `origin/main`
- local validation suite

Fact: Network authentication for `git fetch origin main` was unavailable in
this environment. Local `main` and local `origin/main` matched at
`a0f3aa16fd5aee644a6c4050ccaf33d2c2c1b07e`, and local validation passed.
Remote GitHub Actions status for this audit branch was not checked here.

Risk: A remote-only CI condition could still fail after pushing the audit PR.

Recommended follow-up: Check the audit PR's GitHub Actions run before creating
the preview tag.

Blocks `v0.1.0-preview.1`: No, assuming remote CI is verified before tagging.

## Contract Checks

| Contract area | Result | Notes |
|---|---|---|
| API stability tiers and public-candidate vs experimental semantics | Pass | `docs/api-stability.md` separates Public-Candidate, Experimental Frontier, Compiler-Facing, Internal, Legacy, and Future Vision. It also documents API shape vs deeper semantics. |
| Component identity preview contract | Pass | `docs/component-identity.md` distinguishes direct Go calls from runtime component boundaries and scopes reusable multi-module identity out of preview promises. |
| Runtime resources/router/ErrorBoundary/runtime error semantics | Pass | `pkg/goframe` tests and runtime docs cover resource reload/stale callbacks, loader panic behavior, route remount policy, ErrorBoundary reset/new incidents, and runtime error phases. |
| GOX language/generator diagnostics and fuzz seed evidence | Pass | Golden/error golden tests, package-qualified component tests, and bounded fuzz seeds are present in `pkg/gox`. |
| `goxc` command boundaries and package/export behavior | Pass | Manifest, workspace, package, export, clean, serve, symlink, ownership, asset collision, and publication tests are present under `cmd/goxc`. |
| Manifest/assets compatibility | Pass | Versionless `goframe.json`, `"assets": "./assets"`, legacy explicit asset lists, omitted/null/empty assets, generated/default root `index.html`, and `.wasm` validation are documented and tested. |
| Package metadata and ownership model | Pass | `goframe-package.json` is documented as the authoritative current completion marker. `asset-manifest.json` alone does not grant ownership. |
| Filesystem/symlink safety model | Pass | Root-aware and physical-overlap checks are documented; tests cover symlink roots, intermediate symlinks, source/output overlap, false ownership, and serve symlink entries. |
| Platform/browser evidence | Pass with limitations | Linux/Chrome is strongest. macOS/Windows minimal Go/toolchain evidence exists. Firefox/Safari remain unverified and scoped. |
| Release notes/evaluator guide accuracy | Pass | Dedicated release notes and evaluator guide exist and match current examples, manifest behavior, TinyGo/Go panic containment caveats, and validation commands. |
| Migration/compatibility notes | Pass | `docs/migrations.md` records the `v0.1.0-preview.1` compatibility baseline. |
| Changelog hygiene | Pass | `CHANGELOG.md` has factual Unreleased entries and no `### Planned` section. |
| README docs map | Pass | README links evaluator guide, release notes, tutorial, API stability, platform, deployment, and related docs. |
| CI evidence and workflow naming | Pass | Four workflows exist: Core, Browser Smoke, WASM Size, and VS Code Extension. Docs match workflow scope. |
| Examples matching docs | Pass | All example manifests use the preview asset directory contract and docs-check verifies README mentions example directories with `goframe.json`. |

## Validation Results

Required audit commands:

| Command | Result | Notes |
|---|---|---|
| `git diff --check` | Pass | No whitespace errors. |
| `node scripts/docs-check.mjs` | Pass | `docs check: ok`. |
| `go test ./...` | Pass | All packages passed or had no test files. |
| `go test ./pkg/gox -run 'TestGolden|TestErrorGolden'` | Pass | GOX golden/error golden tests passed. |
| `go test -race ./pkg/... ./cmd/...` | Pass | `pkg/goframe`, `pkg/gox`, and `cmd/goxc` passed under race detector. |
| `go vet ./...` | Pass | No vet output. |
| `go test -tags=goframe_debug ./...` | Pass | Debug-tag tests passed. |
| `scripts/size-budget.sh` | Pass | All raw/gzip/brotli/zstd budgets passed for all 11 examples. |
| `scripts/browser-smoke.sh` | Pass | Ended `browser smoke: ok`. |

Additional release-evidence commands:

| Command | Result | Notes |
|---|---|---|
| `scripts/check.sh` | Pass | Release hygiene script passed, including artifact/module/docs gates, Go tests, debug tests, GOX golden tests, race tests, vet, `goxc doctor`, TinyGo packaging, size budgets, and pure benchmarks. |
| `node --experimental-websocket scripts/dashboard-dom-pressure.mjs` | Pass | Ended `Dashboard DOM pressure audit: ok`; mounted rows max `28`, live DOM stable at `486`, net listener delta `0`, spacer stability true. CDP node drift remained warning-only while live invariants passed. |

Size-budget snapshot from the direct `scripts/size-budget.sh` run:

| Example | Raw | gzip | br | zstd |
|---|---:|---:|---:|---:|
| counter | 83,550 | 33,578 | 28,000 | 30,158 |
| components | 89,198 | 35,217 | 29,238 | 31,563 |
| todo | 117,409 | 44,980 | 37,487 | 40,506 |
| dashboard | 168,628 | 62,874 | 50,808 | 55,093 |
| context | 115,354 | 43,243 | 35,569 | 38,445 |
| virtualized | 123,144 | 47,408 | 38,780 | 42,098 |
| multipackage | 94,354 | 36,850 | 30,728 | 33,175 |
| cmdapp | 94,380 | 36,839 | 30,720 | 33,124 |
| router | 114,716 | 43,602 | 36,062 | 39,026 |
| router-dashboard | 226,274 | 91,135 | 74,530 | 79,742 |
| resource | 147,673 | 64,582 | 54,635 | 58,106 |

Browser smoke covered:

- Todo reconciliation and persistence;
- duplicate-key debug diagnostics;
- runtime event/effect/cleanup error containment;
- ErrorBoundary fallback/reset/nesting behavior;
- dashboard app behavior;
- context selectors;
- virtualized collection behavior;
- multipackage and cmdapp layouts;
- hash router navigation;
- router-dashboard integrated reference flow, route ErrorBoundary safe
  recovery, resource failed/retry UI, forms, validation, and not-found route;
- resource loading/reload/failure/stale completion/unmount cleanup;
- production bundle restore for TinyGo examples.

## Release Readiness Checklist

| Item | Status |
|---|---|
| Docs current | Pass |
| Release notes current | Pass |
| Evaluator guide current | Pass |
| Migrations current | Pass |
| API tiers current | Pass |
| Platform caveats current | Pass |
| Package/manifest contract current | Pass |
| No generated artifacts tracked | Pass |
| No stale future/promise wording in product docs | Pass with Low wording note `GF-AUDIT-001` |
| Validation complete | Pass locally |
| Tag not created in this PR | Pass |
| Remote CI for this audit branch | Not checked locally; verify after push |

## Recommended Follow-up Branches

These follow-ups are not required before the narrow evaluator preview unless
the project wants to widen the promise being made.

### `docs/fact-first-wording-polish`

PR title: `docs(preview): polish residual fact-first wording`

Scope: Replace the remaining small policy/vision phrases that contain
`planned` or similar wording where a present-tense contract phrase is clearer.

Expected files:

- `docs/compatibility.md`
- optionally `docs/architecture.md` if the review wants stricter product-doc
  wording.

Validation:

```bash
git diff --check
node scripts/docs-check.mjs
go test ./...
```

### `ci/non-chrome-browser-evidence`

PR title: `ci(preview): add minimal non-chrome browser evidence`

Scope: Add a small non-Chrome browser evidence lane only if the harness can do
so without destabilizing the preview gates.

Expected files:

- `.github/workflows/*`
- `docs/platform-support.md`
- `docs/ci.md`

Validation:

```bash
node scripts/docs-check.mjs
go test ./...
scripts/browser-smoke.sh
```

### `design/reusable-component-identity-evidence`

PR title: `docs(identity): scope reusable component package evidence`

Scope: Extend the component identity contract only when the project wants to
claim behavior beyond a single app/module tree.

Expected files:

- `docs/component-identity.md`
- `docs/api-stability.md`
- targeted `pkg/gox` or `cmd/goxc` fixtures if the promise widens.

Validation:

```bash
go test ./pkg/gox -run 'Identity|Package|Qualified|Golden|ErrorGolden'
go test ./cmd/goxc -run 'Workspace|PackageIdentity'
node scripts/docs-check.mjs
```

## Final Recommendation

Ready to tag after merge, provided that:

- this audit PR's remote CI is green;
- the signed `v0.1.0-preview.1` tag is created from the intended merge commit;
- release notes keep the documented limitations visible.

There are no confirmed Blocker or High findings in the audited `main`.

The preview remains intentionally narrow: browser/WASM evaluator preview, not
production platform, not 1.0 API stability, not full browser/platform support,
and not broad reusable multi-module component ecosystem support.
