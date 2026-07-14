# GoFrame Post-Preview.6 Deep Audit

## Audit Metadata

| Field | Value |
|---|---|
| Repository | `graybuton/goframe` |
| Canonical module | `github.com/graybuton/goframe` |
| Audit date | 2026-07-14 |
| Audit base | `3997797c40f764601df9bf6bbec6a070eaaa0ffb` |
| Base subject | `Merge pull request #100 from graybuton/docs/release-mark-v0.2.0-preview.6-published` |
| Published preview | `v0.2.0-preview.6` |
| Tag object | `fe38f0b6c964355931f213d8141ef287ad680a1c` |
| Tagged commit | `9548345776e6398cd70e8fc58435dd5dab687c7d` |
| History range | Root `df3234111f5e8a055ad54d2d12bcfec4e8d4b7f5` through audit base |
| Scope | Report only; no implementation, roadmap, release, or workflow change |

The audit evaluates the post-publication `main` snapshot, not only the tagged
release commit. PR #100 was independently confirmed merged, and the public
release was confirmed published, non-draft, and marked as a pre-release. The
tagged commit is an ancestor of the audit base.

## Executive Verdict

- Overall Progress Verdict: **REAL PROGRESS**
- Process Verdict: **OVERWEIGHT**
- Roadmap Verdict: **REFRAME**
- v0.3.0-preview.1 Verdict: **REFRAME**

GoFrame has expanded the class of programs it can execute. It is no longer
only a renderer experiment: the repository contains a retained browser/WASM
runtime, a bounded GOX compiler, a packaging toolchain, multi-package and
child-entry layouts, routing, component-scoped asynchronous resources,
runtime containment, static delivery, structured diagnostics, and saved-source
editor diagnostics. Those claims are backed by implementation, unit tests,
browser runs, and integrated examples.

The progress is nevertheless surrounded by too much closeout work. Across 95
first-parent merges, 19 were capability-bearing runtime or tooling changes,
while 40 were explicit review/release follow-ups. In the latest PR #94-#100
window, two PRs added user-facing tooling, two added executable evidence, and
three were roadmap/release documentation. Most of that work was defensible,
but the sequence should not become the normal cost of every preview.

Today GoFrame can support small and medium interactive browser/WASM
applications, including multi-package authored GOX, hash-routed dashboards,
resource-driven loading and retry states, static packaged delivery, and a
same-origin Go HTTP backend. It does not yet demonstrate navigation-owned
asynchronous transitions that preserve the old screen, cancel obsolete work,
and commit route plus data atomically. Production support, SSR, hydration,
broad browser support, and external package ecosystem maturity are also not
established.

The current roadmap remains useful as a problem inventory, but its ordered
`v0.3` through `v0.9` train projects more certainty than the evidence supports.
The immediate next step should be an executable async navigation flow built
with the existing router, component ownership, and `UseResource`. It should
measure real boilerplate and missing semantics before selecting a public
transition or loader API. The project should not implement a generic route
loader, a second async lifecycle, a commit buffer, SSR, or another planning
document next.

### Hypothesis Results

| Hypothesis | Result | Evidence |
|---|---|---|
| H1: real technical progress | Confirmed | Integrated runtime/toolchain capabilities and passing browser evidence open concrete application classes. |
| H2: process overweight | Confirmed | 40 of 95 merges are explicit follow-ups; 3 of the latest 7 PRs are release/process docs. |
| H3: application ceiling remains | Qualified | Meaningful browser apps work, but navigation-owned async state and broad production boundaries remain unproved. |
| H4: first `v0.3` API may be premature | Confirmed | Existing primitives can express the basic flow; no executable example proves that a public loader is required. |
| H5: long version train is too precise | Confirmed | Slots after the nearest evidence stage lack demonstrated application demand and validated prerequisites. |

## Method And Limitations

The audit inspected all 404 commits, all 95 first-parent merges, all 30 tags,
the changed paths and shortstats for every merge, the current implementation
under `pkg/goframe`, `pkg/gox`, and `cmd/goxc`, all VS Code extension source and
metadata, all 12 examples, all active workflows, relevant scripts, and 18
current audit/planning documents. The removed historical
`pre-preview-deep-audit-i.md` was recovered from its source commit so its
findings could be reconciled rather than repeated from memory.

Every merge has one primary category in Appendix A. `New` means that the merge
introduced a capability-bearing runtime or tooling boundary. `Support` means
it validated or hardened an existing boundary. `Follow-up` means explicit
audit, review, polish, closeout, correction, release preparation, or
post-publication work. `Maintenance` is dependency-only work. These definitions
make the process ratios reproducible; they are not quality scores.

Validation ran in detached temporary worktrees. One aggregate-check attempt in
`/tmp` failed because the host exposes a read-only `/tmp/.git` pseudo-directory
and Go VCS stamping walked to it. The same frozen commit passed from `/var/tmp`;
the first result is an environment artifact, not a repository failure. Local
Go was 1.24.4 rather than CI's 1.22.x, so local WASM bytes are informative.
TinyGo matched CI at 0.41.1, and the frozen GitHub head's WASM Size check was
successful.

The audit has no user interviews, downstream application corpus, package
telemetry, or credible adoption data. Repository quality and local usefulness
can be assessed; external adoption cannot. Benchmark timings are one-machine
observations, not release thresholds.

## Repository And Release Snapshot

| Measure | Snapshot |
|---|---:|
| Tracked files | 454 |
| Commits | 404 |
| First-parent merges | 95 |
| Tags | 30 |
| Production source lines | 22,937 |
| Test source lines | 11,574 |
| Documentation lines | 13,439 |
| `pkg/goframe` | 50 files / 9,912 lines |
| `pkg/gox` | 109 files / 3,963 lines, including testdata and goldens |
| `cmd/goxc` | 26 files / 8,591 lines |
| Examples | 149 files / 7,581 lines across 12 directories |
| Scripts | 32 files / 9,146 lines |
| VS Code extension | 20 files / 2,413 lines |
| Docs | 46 files / 11,371 lines |
| Active workflows | 4 |
| Approximate exported declarations in `pkg/goframe` | 96 |
| Approximate exported declarations in `pkg/gox` | 67 |

PR #100 merged at `3997797c4`. The `v0.2.0-preview.6` release is published and
the tag points to `954834577`. Public API inspection found no open issues and
two open dependency-update PRs (#81 and #82). All checks observed on the frozen
post-release head were successful: core Go/GOX jobs on Linux, macOS, and
Windows; Browser Smoke; WASM Size; VS Code Extension; and CodeQL jobs.

The source-to-test ratio shows a substantial evidence surface, not maturity by
itself. The script and documentation surfaces together are almost as large as
production source. That is a maintenance fact relevant to the process verdict.

## Historical Timeline

| Phase | Boundary | Material change |
|---|---|---|
| Bootstrap and language/runtime MVP | `e646e43` to `1c7ec3f`, 2026-06-16 to 2026-06-17 | Initial runtime, GOX, CLI, effects, expression ergonomics, memoization, reducers, packaging, and dashboard evidence. |
| Framework expansion | `4cd0b75` to `135b715`, 2026-06-18 to 2026-06-24 | Context selectors, virtualization, typed component identity, multi-package and child-entry apps, hash routing, Error Boundaries, and resources. `78fef67` is the clearest identity boundary after which reusable application structure became central. |
| Preview hardening | `c61360e` to `9b10c13`, 2026-06-25 to 2026-06-29 | Integrated reference flow, contracts, platform evidence, fuzzing, artifact ownership, API qualification, and first preview readiness. |
| Application and delivery evidence | PR #47 to PR #68, 2026-06-30 to 2026-07-03 | Security fixes, external-component tests, same-origin backend evidence, `FetchText`, resource recovery, and version propagation. |
| Runtime economics and correctness | PR #72 to PR #93, 2026-07-04 to 2026-07-10 | Allocation work, exact LIS placement, focus/selection correctness, GOX diagnostics, and preview.4/preview.5 release loops. |
| Diagnostics and Editor DX checkpoint | PR #94 to PR #100, 2026-07-10 to 2026-07-14 | DOM bridge attribution, `goxc check`, VS Code diagnostics, versioned roadmap, preview.6 publication, and post-publish alignment. |

The project moved beyond its initial MVP when typed component identity,
multi-package generation, child-entry packaging, and routing appeared as one
connected application boundary. The later resource and reference-app work made
that boundary executable rather than merely architectural.

## Capability Delta Matrix

| Capability | First introduced | Current evidence | Integrated app | Application class unlocked | Current limits |
|---|---|---|---|---|---|
| Components, state, reducers, effects | Bootstrap through `1c7ec3f` | Unit tests, Todo and dashboard smoke | Todo, dashboard, router-dashboard | Retained interactive browser UI with lifecycle-owned state and cleanup | Browser/WASM only; hooks require component ownership |
| Context selectors | `4cd0b75` | Selector/topology tests and context smoke | Context, router-dashboard | Shared application state with selective consumer updates | No global store or persistence contract |
| Explicit memoization | `3c530b8` | Dirty-descendant and comparator tests | Dashboard | Expensive component subtrees can skip clean renders | User-supplied equality; conceptual cost with dirty propagation |
| Virtualization | `5f34392` | Range tests and virtualized/dashboard smoke | Dashboard, virtualized | Large fixed-row datasets can render bounded visible windows | Fixed height; no dynamic measurement, infinite loader, or advanced a11y |
| Multi-package and child-entry apps | `85e2cbc`, `2ba0c6e` | Generation/build tests and browser smoke | Multipackage, cmdapp | Internal packages and nested command entrypoints can contain authored GOX | Broad external multi-module ecosystem contract remains limited |
| Packaging and export | `10334a3` onward | Artifact, symlink, manifest, package, compression, and metadata tests | All packaged examples | Static browser/WASM artifacts can be deployed with hashes, preloads, and sidecars | No bundle splitting, dynamic linking, or production server |
| Router and query state | `d98b7fb`, `2258e28` | Matcher/query tests and router smokes | Router-dashboard | Hash-routed multi-screen apps with params, query filters, back navigation, and not-found states | No history contract, loaders, blockers, or transition state |
| Forms and validation pattern | `2258e28` | Router-dashboard submit/error/reset smoke | Router-dashboard | Controlled forms with validation and route-local edit flows | Pattern, not a form framework or typed action contract |
| Error Boundaries | `5c9eb0a` | Extensive render-panic tests and boundary smoke | Router-dashboard | Render failures can be isolated to a subtree with recovery | Render phase only; effects and async failures use other paths |
| Component-scoped resources | `135b715` | 29 focused tests plus resource and reference-app smoke | Resource, router-dashboard, server-backed | Async data with loading, failure, retry, stale suppression, and cleanup | No cache, Suspense, mutation system, or navigation ownership |
| Same-origin server boundary | PR #60-#63 | Backend integration and server-backed browser smoke | Server-backed | A packaged WASM app can communicate with a plain Go `net/http` API and recover from stale/failing requests | Evidence fixture, not a GoFrame server/fullstack framework |
| Structured diagnostics | PR #96 | `goxc` tests, schema-v1 check, GOX source diagnostics | CLI and editor flow | Tools can validate saved GOX without writing generated output | Syntax/generation diagnostics only; no Go/TinyGo semantic check |
| VS Code diagnostics | PR #97 | 43 pure Node tests and VS Code CI | Extension workflow | Saved GOX gets inline diagnostics with multi-root isolation and Workspace Trust | Process-based, save/rename driven; no LSP or unsaved-text analysis |

## PR And Work Classification

### Full-History Counts

| Primary category | Merges | Share | Introduced branch commits |
|---|---:|---:|---:|
| `CAPABILITY` | 13 | 13.7% | 89 |
| `CORRECTNESS` | 15 | 15.8% | 44 |
| `PERFORMANCE` | 5 | 5.3% | 9 |
| `TOOLING_DX` | 6 | 6.3% | 33 |
| `EVIDENCE` | 24 | 25.3% | 54 |
| `RELEASE_DOCS` | 13 | 13.7% | 26 |
| `PROCESS_GOVERNANCE` | 10 | 10.5% | 44 |
| `DEPENDENCY_MAINTENANCE` | 9 | 9.5% | 9 |
| Total | 95 | 100.0% | 308 |

The capability-bearing ratio is defined as `CAPABILITY + TOOLING_DX`: 19 of
95 merges, or 20.0%. Support/closeout is defined as correctness, performance,
evidence, release docs, and governance: 67 of 95, or 70.5%. Dependency-only
maintenance is the remaining 9.5%. A second path-based role classification
finds 19 new, 27 support, 40 explicit follow-up, and 9 maintenance merges.

`RELEASE_DOCS + PROCESS_GOVERNANCE` account for 23 of 95 merges, or 24.2%.
This does not mean one quarter of the work was waste. It means release and
coordination cost is large enough to constrain how often the current process
should be repeated.

“Introduced branch commits” is the number of non-first-parent commits brought
in by each merge, summed by its primary category. It covers 308 commits. The
remaining repository commits include merge commits and direct first-parent
history, so assigning all 404 commits a second inferred category would create
false precision.

### Recent PR #94-#100 Cycle

| Work | PRs | Count | Assessment |
|---|---|---:|---|
| User-facing tooling capability | #96 `goxc check`, #97 VS Code diagnostics | 2 | Observable value: read-only diagnostics and editor feedback |
| Executable evidence | #94 Todo bridge, #95 dashboard attribution | 2 | Necessary to decide #70 without speculative runtime work; #95 was a follow-up to make attribution exact |
| Release/process closeout | #98 roadmap, #99 prep, #100 publish closeout | 3 | A roadmap and durable release record were useful; three separate PRs for a two-capability checkpoint is too much as a default |

The diagnostics checkpoint required five PRs from first observable capability
to completed release state (#96-#100): two capability PRs and three
planning/release PRs. The mutation evidence was a separate two-PR decision
chain. Marginal value dropped after the durable release scope was accurate:
repeated wording synchronization and fixed commit-count bookkeeping did not
open another workflow class. Future previews should combine bounded
release-prep corrections and use one post-publish status update only when
repository state truly depends on publication.

In ratio form, the recent window is 28.6% user-facing tooling, 28.6% evidence,
and 42.9% release/process. The full-history comparison is 20.0%
capability-bearing, 25.3% evidence, and 24.2% release/process. The recent cycle
therefore delivered a higher capability share than the full history, but also
nearly doubled the release/process share.

## Progress Versus Theater

### Full-History View

Capability growth is real. Component ownership grew into typed identity,
multi-package authored source, routing, resources, error containment, static
delivery, structured diagnostics, and editor feedback. Correctness and safety
also grew materially: path sanitization, symlink policy, fail-closed package
ownership, stale async suppression, focus/selection restoration, exact keyed
placement, and diagnostic position conversion are behavioral results, not
presentation.

Product/tooling growth is narrower but real. `goxc` can check, generate, build,
package, export, clean, serve for development, report size, diagnose the local
toolchain, and propagate versions. The extension turns the schema into saved
source editor diagnostics. Process growth, however, now competes with product
work: evidence, scripts, audits, release documents, and review closeouts occupy
most merge events.

### Recent-Cycle View

The user value in PR #96 and #97 is direct. The evidence in PR #94 and #95
prevented an unsupported commit-buffer project and should be retained as an
example of measurement changing a decision. The preview.6 release docs were
necessary to publish a credible experimental contract. The avoidable part was
fragmentation: review wording, roadmap sequencing, preparation state, and
published state each became a separate synchronization surface with strict
commit-shape requirements.

This cycle should not be the normal model. The preferred chain remains
capability, focused evidence, bounded closeout, release. “Bounded” must mean one
closeout, not an expanding series of reviews about the closeout itself.

### Counterfactual

Without recent release/docs work, diagnostics would still function, but users
would lack a durable schema contract, published install target, trust boundary,
and evaluator guidance. Without the bridge evidence, #70 might have triggered
a broad runtime design without a measured redundant operation class. Those
parts were required.

The version-pointer cleanup, wording corrections, and publication-state
alignment could have been combined more aggressively. Fixed commit-count
contracts, duplicated PR-body checklists, repeated full release narratives,
and audits that re-prove closed findings should not recur for each preview.

## Current Product Capability

### What Is Proven

- **Repository quality:** broad unit, race, vet, debug-tag, golden, browser,
  size, artifact, and platform gates pass on the frozen snapshot. Contracts are
  unusually explicit for an experimental project.
- **Framework usefulness:** the examples prove retained interactive UI,
  dashboard-scale fixed-row rendering, nested package layouts, hash routing,
  controlled forms, resource recovery, and same-origin backend integration.
- **External adoption:** not proven. Tags, stars, examples, and green CI do not
  establish downstream use, upgrade cost, or API fit.

### Current Application Envelope

The credible envelope is a static-delivered, Chrome-validated browser/WASM
application written in Go and GOX, optionally served by an ordinary Go HTTP
server. It may have multiple packages, nested command entrypoints, local
routes, query state, controlled forms, fixed-height virtualized data, and
component-scoped async requests with retry and stale-result protection.

The next blocked class is an application where navigation itself owns async
work: keep the current screen while a new route prepares, cancel or supersede
obsolete work, coordinate redirects/errors, and commit route plus data as one
observable transition. A developer can assemble loading states inside route
components today, but the stronger navigation contract is not demonstrated.

The smallest external feedback loop is not another version train. Publish one
bounded evaluator task around an existing reference app, ask a small set of
external Go developers to implement a route-driven data flow, and record setup
failures, API confusion, generated diff friction, binary size, and missing
semantics. No telemetry or adoption claim is needed.

## Architecture Health

Component ownership remains the coherent organizing principle. State slots,
reducers, effects, context subscriptions, resource generations, boundary
failure state, cleanup, dirty propagation, and unmount all attach to a
component instance. [`pkg/goframe/component.go`](../pkg/goframe/component.go),
[`state.go`](../pkg/goframe/state.go),
[`effects.go`](../pkg/goframe/effects.go), and
[`resource.go`](../pkg/goframe/resource.go) reinforce the same model.

The router is adjacent rather than fully integrated: hash observation is a
browser concern and `RouterView` produces a pattern-keyed component boundary.
That is acceptable for the current synchronous contract. It becomes risky only
if route transitions introduce another owner for generation, pending, stale,
cancel, cleanup, and error state.

Browser-specific state is largely isolated to JS build files: DOM mounting and
patching, focus/selection, hash events, and fetch. General node, state,
resource, router matching, and reconciliation decisions remain testable on the
host. Reconciliation complexity is material but bounded by focused tests,
exact O(n log n) LIS placement, and browser attribution. The scheduler already
coalesces dirty work into one rAF flush in characterized paths.

The highest combined conceptual and size costs are resource lifecycle, Error
Boundaries, router/query handling, virtualization, and retained reconciliation.
Each is used by more than one meaningful flow except the most advanced error
boundary combinations, which are nevertheless justified by containment tests.
No additional general lifecycle abstraction is warranted merely because these
features share words such as “generation” or “cleanup.”

## Runtime And Lifecycle Audit

| Concern | Current model | Evidence | Assessment |
|---|---|---|---|
| Dirty updates | Per-component dirty queue with `updateBatch`; rAF and microtask fallback | State tests and Todo bridge output | Coalesced and bounded in measured scenarios |
| Focus/selection | Captured before dirty flush and restored after patch | Retained input and bridge smoke | Correct for characterized controlled-input and reorder flows |
| Effects | Component slot ownership; setup and cleanup after render/unmount | Effect and error tests | Coherent; effect failures intentionally bypass Error Boundaries |
| Context | Provider topology and selector subscriptions owned by components | Context tests and smoke | Coherent; topology logic is complex but covered |
| Resources | Key/generation, first-completion wins, cleanup, stale suppression | 29 resource tests and three browser flows | Strong component-local async contract |
| Error containment | Global reporter plus render-only Error Boundaries | Unit and browser tests | Explicit phase boundary; not a general async exception system |
| Reconciliation | Identity match plus exact LIS-aware stable placement | Unit benchmarks and bridge evidence | Complexity justified by retained-node behavior |
| Virtualization | Fixed range and stable spacer/row keys | Unit and dashboard/virtualized smoke | Useful but intentionally narrow |

Lifecycle logic is not materially duplicated inside the runtime today.
Resources own async generation and cleanup; the router owns synchronous URL
matching and one listener. The VS Code extension also uses generations, but it
is a separate process-coordination layer, not runtime duplication. A route
transition implementation would be the first likely duplicate and should not
be added before evidence identifies the shared boundary.

## Router And Resource Boundary

| Concern | Router | Resource | Shared need | Duplication risk |
|---|---|---|---|---|
| Identity | Route pattern, current path, params, and raw query; view keyed by pattern | String key plus owning component slot | Stable identity when inputs change | Medium if transitions invent a second key model |
| Generation | None | Incremented generation invalidates old callbacks | Only needed for async navigation work | High if copied independently |
| Pending state | No transition state | `ResourceLoading` snapshot | UI may need pending navigation plus data | High if represented twice |
| Stale completion | Not applicable to synchronous matching | Old resolve/reject callbacks ignored | Required for async route work | High if separately implemented |
| Cancellation | No async cancellation | Loader cleanup on reload, key change, and unmount | Obsolete route work may need cleanup | High if route loaders bypass resources |
| Cleanup | Hash listener and component unmount | Loader cleanup and slot cleanup | Owner must release work exactly once | Medium |
| Error result | Not-found/route render behavior; no async result | `ResourceFailed` with explicit error | Route data failures need a rendering policy | Medium |
| Unmount | Pattern-keyed route component unmounts when pattern changes | Completion after owner unmount is ignored | Route lifetime can naturally own a resource | Low with composition; high with parallel ownership |
| Query/param change | Same pattern retains route component; context values change | Key change reloads | Resource key can include params/query | Low for current composition |

Router plus ordinary components plus `UseResource` can already express a route
that derives a resource key from params/query, shows loading/failed/ready UI,
reloads, and cancels on unmount or key change. It cannot directly express a
navigation transaction that keeps the previous route committed while the next
route prepares, coordinates redirects/blockers, or atomically commits URL,
screen, and data.

The existing examples do not demonstrate that stronger pain. Router-dashboard
places resource ownership above `RouterView`, so it proves persistence and
retry across routes, not route-owned cancellation. The missing semantics are
plausible, but the amount of boilerplate and the right owner are unmeasured.

A public loader now would likely create a second asynchronous lifecycle. An
internal generation/cancellation helper could eventually serve both models,
but extraction before two real implementations would be speculative. A public
route loader is therefore not necessary for the first `v0.3` slice.

## GOX Compiler Audit

GOX remains an intentionally bounded, handwritten source transformer rather
than a second general-purpose language. Its lexer, parser, AST, expression
handling, package identity, and code generation are compact enough to reason
about together. Package-qualified component tags and nested expression markup
increase parser/codegen coupling, but the grammar has not sprawled into type
checking, formatting, or semantic indexing.

Source diagnostics now preserve authored paths and nested locations. Schema-v1
columns are one-based UTF-8 byte columns when known; editor consumers convert
them using saved bytes. Golden/error-golden tests, two fuzz targets
(`FuzzGenerate` and `FuzzParseElement`), malformed-expression coverage, and
external package build tests provide credible parser evidence.

No current compiler correctness defect surfaced in this audit. Long-term
obligations are different from defects: source maps need generated-to-authored
position ownership; a formatter needs syntax governance; an LSP needs semantic
state and Go integration. Those features would materially expand the product
and should not be implied by the current CLI diagnostic transport.

Generated component identity depends on Go package identity where available,
with tested internal/external package flows. Broad reusable multi-module
distribution is still a stated limitation, but older claims that identity is
fundamentally file-local are invalidated by current evidence.

## goxc Toolchain Audit

Verdict: **growing but manageable**.

`goxc` remains cohesive around one authored application lifecycle: inspect and
validate GOX, materialize an isolated workspace, generate, build, package,
export, serve locally, clean owned output, report size, diagnose prerequisites,
and propagate version metadata. Schema-v1 diagnostics provide an explicit CLI
boundary rather than coupling the extension to compiler internals.

The 16 production files and roughly 8,600 total lines are no longer a small
wrapper. Manifest rules, workspace/module replacement, package ownership,
symlink safety, asset hashing, compression, export publication, and development
serving create substantial maintenance surface. The fail-closed ownership and
path tests justify that surface. Full transactional package rollback and broad
multi-module publishing remain limited.

The tool is not yet a kitchen-sink build system because it does not own a
production server, dependency resolver, JavaScript bundler, semantic language
service, deployment provider, or hidden RPC framework. `serve` remains a local
static development boundary. New responsibilities should be admitted only when
they are inseparable from the authored GOX-to-browser artifact path.

## Examples And Application Evidence

| Example | Classification | What it proves | Residual limitation |
|---|---|---|---|
| `counter` | Quickstart | Minimal install/generate/package/serve path | Too small for lifecycle conclusions |
| `components` | Focused capability demo | Typed components and composition | Overlaps counter but covers component boundary |
| `todo` | Reference/regression fixture | Controlled input, state, keyed reorder, focus and selection | Local-only data flow |
| `dashboard` | Pressure test | Dense component tree, memo/context/virtual table, DOM bridge attribution | Synthetic dataset and fixed-row virtualization |
| `context` | Focused capability demo | Provider/selector update isolation | Not an application shell |
| `virtualized` | Pressure test | Bounded visible range and scrolling | Fixed height only |
| `multipackage` | Toolchain/layout fixture | Authored GOX in internal packages | Not a rich user flow |
| `cmdapp` | Toolchain/layout fixture | Child command entrypoint and internal packages | Intentionally overlaps multipackage layout evidence |
| `router` | Focused capability demo | Params, query, links, back, navigate, not-found | No async data |
| `router-dashboard` | Reference application | Integrated routes, forms, errors, query, resources, retry | Resource owner is above router; no navigation-owned async transition |
| `resource` | Focused/regression fixture | Loading, reload, failure, stale result, unmount cleanup | Component-local contract only |
| `server-backed` | Reference boundary fixture | Same-origin Go API, stale suppression, failure recovery | No GoFrame server or deployment contract |

No example is clearly disposable today. Multipackage/cmdapp and
dashboard/router-dashboard overlap visually but test different ownership and
layout contracts. The next evidence should extend router-dashboard or
server-backed rather than create another showcase. Router-dashboard is
reference-grade for current synchronous routing and component resources; it is
not evidence for route transitions. Server-backed proves a useful client/server
boundary without proving fullstack framework behavior.

## Public API Utilization

| API family | Classification | Evidence and decision |
|---|---|---|
| `Node`, `VNode`, `Props`, element/text/fragment/conditional/list helpers | Widely exercised | Compiler output, every example, unit tests; core authored/render boundary |
| `Component`, `ComponentT`, `ComponentType`, `C` | Compiler-facing and narrow-but-justified | `ComponentT` carries generated identity; legacy helpers overlap but remain compatibility surface |
| `UseState`, `UseReducer` | Widely exercised | Todo, dashboard, router-dashboard, unit/browser tests |
| Effect/dependency APIs | Widely exercised | Runtime features and cleanup behavior; explicit dependency model is verbose but coherent |
| Context and selectors | Narrow but justified | Context and integrated dashboard evidence; selective updates are measured |
| Event wrappers | Compiler-facing | Generated handlers and form/input/scroll examples; not orphaned despite thin direct use |
| Error reporting and `ErrorBoundary` | Narrow but justified | Strong focused coverage and router-dashboard recovery |
| Router/query APIs | Widely exercised | Router and router-dashboard; still experimental and hash-specific |
| Resource and `FetchText` | Widely exercised | Resource, router-dashboard, server-backed; async lifecycle is justified |
| Virtual list/table types | Narrow but justified | Two pressure examples and dashboard; fixed-height scope prevents overreach |
| `Mount` | Compiler/application boundary | Required by every browser entrypoint; JS/host stubs keep builds testable |

No family is demonstrably orphaned. `Component`/`C` versus typed
`ComponentT` is the clearest overlap, but generated identity and preview
compatibility make removal premature. Low textual use is not evidence against
compiler-facing constructors or event wrappers.

## Size And Performance Economics

The following local release-like TinyGo packages were regenerated after
browser smoke. TinyGo matched CI (0.41.1); Go did not (local 1.24.4 versus CI
1.22.x), so bytes are informative and the successful frozen-head CI WASM Size
job remains authoritative.

| App | Raw / budget / headroom | Gzip (ratio) | Brotli (ratio) | Zstd (ratio) |
|---|---|---|---|---|
| counter | 83,950 / 97,280 / 13,330 B | 33,570 (39.99%) | 27,979 (33.33%) | 30,236 (36.02%) |
| components | 89,590 / 107,520 / 17,930 B | 35,436 (39.55%) | 29,363 (32.77%) | 31,659 (35.34%) |
| todo | 117,867 / 122,880 / 5,013 B | 45,504 (38.61%) | 37,631 (31.93%) | 40,555 (34.41%) |
| dashboard | 168,231 / 168,960 / 729 B | 62,990 (37.44%) | 50,681 (30.13%) | 54,725 (32.53%) |
| context | 115,746 / 116,736 / 990 B | 43,534 (37.61%) | 35,690 (30.83%) | 38,428 (33.20%) |
| virtualized | 122,708 / 124,928 / 2,220 B | 47,339 (38.58%) | 38,679 (31.52%) | 41,905 (34.15%) |
| multipackage | 94,754 / 110,592 / 15,838 B | 37,171 (39.23%) | 30,783 (32.49%) | 33,190 (35.03%) |
| cmdapp | 94,772 / 110,592 / 15,820 B | 37,179 (39.23%) | 30,814 (32.51%) | 33,186 (35.02%) |
| router | 115,119 / 116,736 / 1,617 B | 43,969 (38.19%) | 36,120 (31.38%) | 38,936 (33.82%) |
| router-dashboard | 226,752 / 230,400 / 3,648 B | 91,066 (40.16%) | 74,590 (32.89%) | 79,700 (35.15%) |
| resource | 149,516 / 153,600 / 4,084 B | 64,909 (43.41%) | 55,026 (36.80%) | 58,387 (39.05%) |

Dashboard, context, and router have dangerous raw headroom below 2 KiB.
Previous split-prop, virtual-table, and reconciliation work recovered or
protected headroom, but budgets now actively constrain architecture. That is
useful when it forces evidence and feature-local cost accounting; it becomes a
blocker when every shared capability must fit under an unrelated smallest
fixture without a stated budget policy.

Current benchmarks are baselines, not targets: dirty-queue pruning measured
229 ns/op; keyed reorder matching about 8.8-18.0 microseconds across cases;
long backward stable placement about 101 microseconds; empty prop splitting
2.6 ns/op with zero allocation; dashboard-row prop splitting 1.6 microseconds
and five allocations on this host. Browser evidence is more decision-relevant:
controlled and burst Todo updates each used one rAF flush with no structural or
listener churn; keyed reorder used three `insertBefore` calls; dashboard search
retained four of 28 rows and exactly attributed 24 new/24 removed rows and
48/48 listener calls.

No measured redundant operation class currently justifies a broad commit
buffer. A route-transition API could fit only after a prototype reports its
incremental bytes against dashboard, context, router, and integrated-app
budgets. Raising budgets before that measurement would erase the constraint
rather than answer it.

## CI, Documentation And Process Cost

The four workflows separate core, browser, size, and VS Code concerns. Core
duplicates useful checks across OSes, while browser and size correctly pin
Chrome/TinyGo-sensitive evidence. `scripts/check.sh` is a credible aggregate
local gate, although CI repeats parts of it. The duplication is acceptable
when one script is the local contract and workflows select platform-specific
subsets; it is wasteful when PR instructions independently enumerate the same
commands and literal wording checks.

On the frozen head, the longest observed check was Browser Smoke at 245
seconds; TinyGo size took 127 seconds, core OS jobs 63-94 seconds, VS Code 17
seconds, and CodeQL analyses 41-85 seconds. They run independently, so the
parallel wall-clock cost is reasonable. The larger cost is human repetition in
prompts, review closeouts, and release-state synchronization.

Documentation consistency has real value for an experimental contract:
published versus prepared tags, diagnostic schema semantics, Workspace Trust,
and deployment limits must be accurate. Release preparation and one
post-publish state transition are necessary overhead. PR-body duplication,
fixed commit-count acceptance, exact prose checks for non-contractual wording,
and repeated review of already validated history are avoidable duplication.

The recent process contained both rigor and theater:

- Valuable rigor: frozen refs, clean-tree checks, signed tags/commits, exact
  module verification, source/byte position tests, stale-run tests, browser
  attribution, size budgets, and durable release contracts.
- Necessary release overhead: release notes, one preparation state, tag/release
  verification, and one repository post-publish alignment.
- Avoidable duplication: repeating full release narratives in prompts and PR
  bodies, separate wording-only follow-ups that could be caught before merge,
  and manually restating workflow matrices already encoded in CI.
- Process theater: treating a predetermined commit count or literal prose
  shape as evidence when neither changes behavior nor a durable contract.

## Historical Finding Reconciliation

Closed findings must not become recurring audit folklore. Reopen them only on
new regression evidence.

| Historical finding/recommendation | Source snapshot | Current status | Current evidence | Reopen? |
|---|---|---|---|---|
| Component identity was name/file fragile | Foundation and pre-preview audits | Closed | Typed package identity, keyed matching, generated `ComponentT`, identity tests | only if regression appears |
| Multi-package and child-entry apps were unsupported | Foundation Audit IV | Closed | Multipackage/cmdapp generation, build, package, and smoke | only if regression appears |
| External component package boundary was unproved | Pre-preview deep audit | Partially closed | External component identity/build tests exist; broad ecosystem distribution does not | yes — residual issue |
| Filesystem and symlink safety was incomplete | Public preview readiness | Closed | Central path policy, serve sanitization, artifact/module checks, symlink tests | only if regression appears |
| Package ownership/publication could overwrite output | Public preview readiness | Partially closed | Fail-closed ownership and metadata are tested; full transactional rollback remains limited | yes — residual issue |
| Runtime errors lacked containment | Foundation audits | Partially closed | Reporter and render Error Boundaries are strong; effect/async phases intentionally use other paths | no |
| Router/query support was absent | Public surface audits | Closed | Hash router, params/query helpers, two browser flows | only if regression appears |
| Async resources were absent | Post-preview focus | Closed | `UseResource`, `FetchText`, stale/cancel/retry tests and examples | only if regression appears |
| Virtualization was absent | Foundation Audit III/IV | Closed | Fixed-height list/table, tests, pressure evidence | only if regression appears |
| GOX source diagnostics were weak | Preview.4/preview.5 notes | Closed | Nested/malformed source locations, schema-v1 transport, goldens | only if regression appears |
| Editor diagnostics were missing | Post-preview focus | Closed | Saved-source VS Code diagnostics, byte-to-UTF-16 mapping, trust and stale isolation | only if regression appears |
| Fuzz evidence was absent | Pre-preview audit | Invalidated by later evidence | `FuzzGenerate`, `FuzzParseElement`, seeded fuzz corpus | no |
| Platform evidence was Linux-only | Pre-preview deep audit | Partially closed | macOS/Windows core CI exists; automated browser evidence remains Chrome/Linux-centered | yes — residual issue |
| Release discipline was missing | Readiness audits | Closed | Signed preview tags, durable notes, exact module checks, release process | no |
| WASM headroom was unmeasured | Size headroom audit | Partially closed | Per-app raw/compressed budgets and CI gate; several fixtures now have under 2 KiB raw headroom | yes — residual issue |
| Documentation was fragmented/stale | Foundation/readiness audits | Partially closed | Strong contract docs; architecture/API-stability wording still contains some pre-preview-era framing | yes — residual issue |
| Web-first direction was unresolved | Historical strategy docs | Superseded | Current browser/WASM identity and roadmap explicitly replaced that debate | no |

## Risk Register

| ID | Severity | Layer | Evidence | Consequence | Recommended action | Blocks next stage? |
|---|---|---|---|---|---|---|
| AUD-01 | Medium | Architecture | Router has no async lifecycle; resources already own generation/cancel/stale rules | A public loader could duplicate lifecycle and freeze the wrong owner | Build existing-primitive route/data evidence first | No; it defines the next stage |
| AUD-02 | Medium | Size | Dashboard 729 B, context 990 B, router 1,617 B local raw headroom; CI gate green | Shared runtime additions may fail smallest relevant budgets | Measure incremental bytes in prototype; do not raise budgets preemptively | No for evidence-only fixture |
| AUD-03 | Process | Delivery | 40/95 explicit follow-up merges; latest cycle has 3 release/process PRs for 2 tooling capabilities | Capability throughput and reviewer attention decline | Collapse closeout, eliminate commit-count/prose theater | No |
| AUD-04 | Medium | Product | No external usage or task-based evaluator evidence | Roadmap may optimize repository concerns rather than user pain | Run a small task-based external evaluation before multiple version lines | No |
| AUD-05 | Medium | Platform | Browser automation is strongest on Chrome/Chromium | Firefox/WebKit regressions or DOM differences may be missed | Add another browser only when a concrete compatibility target is selected | No |
| AUD-06 | Low | Documentation | Architecture and API-stability docs retain some older framing | Maintainers may reopen closed findings or misread current phase | Correct only when touching those contracts; avoid another cleanup campaign | No |

No Blocker or High finding is supported by current evidence. Inflating the
route/API uncertainty into a blocker would confuse a decision gap with a
runtime defect.

## Roadmap Audit

| Version line | Problem evidence | Prerequisites | Application class unlocked | Confidence | Recommended status |
|---|---|---|---|---|---|
| `v0.3` Application Model II | Plausible navigation/data gap, but no measured fixture | Existing router/resource evidence; async navigation flow; size accounting | Navigation-owned async browser apps | Medium for problem, low for API | selected for evidence stage; public API remains candidate |
| `v0.4` Modular Delivery | Large integrated bundle and static-only packaging, but no user demand for split graph | Artifact ownership, multi-entry lifecycle, cache/load evidence | Independently delivered features/routes | Low | remove from ordered train |
| `v0.5` SSR/prerender | No executable server-render requirement or DOM-independent renderer | Pure render subset, escaping, server lifecycle, product demand | Search/indexable or server-first HTML apps | Low | research |
| `v0.6` Hydration/islands | No server output exists | Deterministic SSR output and identity handoff | Incrementally activated server-rendered apps | Very low | defer |
| `v0.7` Dev loop/language services | Current save/build loop and process diagnostics expose real DX limits | Source mapping ownership and incremental build measurements | Faster authoring/debugging workflow | Medium | candidate; evaluate earlier than SSR if user evidence supports it |
| `v0.8` Fullstack contracts | Same-origin fixture proves boundary, not demand for framework RPC/contracts | Stable client app model, HTTP error/validation evidence, users | Typed Go client/server workflows | Low | remove from ordered train |
| `v0.9` Ecosystem/1.0 readiness | External package and browser breadth remain limited | Downstream users, compatibility policy, accessibility/platform evidence | Reusable ecosystem and supported upgrades | Medium as goal, low as scheduled line | candidate, not ordered implementation |

`v0.3` is correctly prioritized only as a learning objective. Route
transitions are a reasonable first question, not a selected API. History
routing is too early until deployment fallback and navigation semantics are
needed by an app. Modular delivery does not have to precede SSR in theory;
neither should hold an ordered slot without evidence. Dev-loop/source-map work
has more immediate observed friction and may move earlier. SSR, hydration, and
fullstack should remain a capability reservoir rather than numbered promises.

Roadmap Verdict: **REFRAME**. Keep the current baseline and nearest problem,
replace post-`v0.3` ordering with status-qualified candidates/research, and
promote a line only after executable demand and prerequisites exist.

## v0.3.0-preview.1 Decision

| Criterion | A: public API now | B: evidence fixture | C: internal primitive | D: different capability |
|---|---|---|---|---|
| Observable user value | Potentially high, but API value is assumed | Immediate route/data flow and measurable authoring result | None directly | Depends on replacement; dev-loop value is plausible |
| Evidence already present | Router/resources exist separately; missing transition flow | Strong enough to build the experiment | Only one current generation implementation needs sharing | No candidate has stronger integrated evidence yet |
| Public API commitment | High | None | None | Variable |
| Lifecycle duplication risk | High | Low | Medium; premature abstraction risk | Variable |
| Implementation cost | Medium/high | Low/medium | Medium | Variable |
| WASM-size risk | Shared runtime growth | Mostly example/smoke cost | Shared internal runtime growth | Variable |
| Migration risk | High if contract is wrong | None | Low while private | Variable |
| Learning value | Medium after commitment | High before commitment | Medium, biased toward a presumed solution | Medium |
| Fit with current examples | Requires redesign | Extends router-dashboard or server-backed | No second runtime consumer yet | Dev-loop work fits tooling, not Application Model II |

v0.3.0-preview.1 Verdict: **REFRAME**.

Choose Candidate B. The result is not “route loaders are unnecessary.” It is
“the repository has not measured the missing semantics or compared them with
existing composition.” Candidate A commits too early. Candidate C extracts a
shared primitive before a second implementation exists. Candidate D remains a
fallback if the evidence flow finds no material application-model gap.

Success means a deterministic route/data flow exposes one or more concrete
problems such as preserving old content during navigation, cancellation
ownership, redirect/error coordination, or repeated boilerplate. Failure to
find such a problem is useful evidence to redirect v0.3, not a reason to invent
an API.

## Recommended Next Three Stages

| Order | Stage | User-visible outcome | Evidence required | Kill/reframe condition |
|---|---|---|---|---|
| 1 | Async navigation evidence | One existing reference app demonstrates route-param/query-driven data, stale cancellation, pending/failure/recovery behavior using current primitives | Browser assertions, ownership diagram, boilerplate inventory, bridge/flush counts, incremental raw/compressed size | Reframe if current composition is clear and sufficient, retained work is recreated, or size cost cannot fit without budget increases |
| 2 | Private lifecycle experiment, only if Stage 1 finds duplication | Less duplicated generation/cancel/commit logic behind no new public API | Before/after code, behavior parity, cancellation ordering tests, size delta | Kill if helper serves only one owner or obscures component cleanup |
| 3 | Smallest public application-model contract, or redirect | A public transition state/cancel boundary only if stages 1-2 prove it; otherwise choose the highest-evidence DX/product gap | External evaluator task, API alternatives, migration and size evidence | Redirect if no repeated user-facing pain or if API requires a parallel resource system |

### First Stage Detail

- Proposed branch: `feat/v03-async-navigation-evidence`
- PR theme: demonstrate route-driven async state with existing primitives
- Goal: extend `router-dashboard` or `server-backed` with one deterministic
  navigation-triggered load and measure ownership, stale cancellation, pending
  UI, error recovery, boilerplate, DOM work, and size.
- Allowed conceptual scope: example-local composition, focused browser
  instrumentation, and tests needed to make the flow deterministic.
- Non-goals: public router/loader/action API, scheduler or reconciler changes,
  global cache, mutation framework, history routing, SSR, new example, release
  preparation, or generic lifecycle extraction.
- Expected areas: one existing example, its browser smoke, and only the
  minimum test helper required for evidence. Production runtime should remain
  unchanged unless the evidence stage is explicitly reframed later.
- Gates: current aggregate checks, two deterministic browser runs, size before
  browser debug packaging on the CI toolchain, focus/identity/stale assertions,
  and a concise measured comparison of existing composition versus the missing
  contract.
- Why this is not process-only: it creates an executable user flow and produces
  behavioral and size evidence that can approve, narrow, or reject a public
  v0.3 API.

## Practices To Keep, Simplify And Stop

### Keep

- Frozen refs, clean-tree discipline, signed release objects, and exact tagged
  module verification.
- Focused unit/golden/fuzz/browser evidence tied to behavior.
- Per-example size budgets with clean release-like packaging.
- Explicit experimental scope, platform limits, schema contracts, Workspace
  Trust, and fail-closed artifact ownership.
- Evidence that can reject a proposed architecture, as PR #94/#95 did for an
  immediate broad commit buffer.

### Simplify

- Make `scripts/check.sh` and CI the source of truth instead of repeating every
  command in each PR narrative.
- Combine review-closeout wording before merge and keep one release-prep plus,
  when needed, one post-publish status change.
- Summarize stable contracts by link rather than restating them in README,
  release notes, evaluator docs, PR bodies, and prompts.
- Replace ordered distant version slots with a candidate/research reservoir.
- Use historical audit status tables so closed findings are not re-litigated.

### Stop

- Stop treating exact commit counts as a correctness property.
- Stop using literal wording checks when the wording is not a durable machine
  or compatibility contract.
- Stop creating a new audit merely because a release completed; require a real
  decision that existing evidence cannot answer.
- Stop giving every capability a new example when an existing reference app can
  carry the evidence.
- Stop selecting public abstractions before an executable flow demonstrates the
  duplicated work or missing semantics.

## Final Conclusion

GoFrame's progress is real. The current repository can build and validate
application shapes that the initial project could not: retained component
systems, multi-package GOX apps, routed/resource-driven dashboards, static
packages, same-origin backend clients, and editor-integrated diagnostics. The
runtime model remains coherent and the strongest historical blockers have been
closed by implementation and tests.

The project is not stalled by architecture; it is at risk of being slowed by
its own proof and release machinery. Rigor should stay, but fewer artifacts
must carry it. The roadmap should stop pretending that SSR, hydration,
fullstack contracts, and ecosystem readiness already have a validated order.

The next move is one executable async navigation flow with existing primitives.
Measure what is awkward, what is impossible, what it costs, and who owns
cancellation. Only then choose a public transition contract. That is a
capability-seeking step with a built-in way to say no, which is the right first
move for `v0.3`.

## Appendix A — Merged PR Classification

The table covers every first-parent merge. “Class” names the concrete workflow
opened; a dash means no new application/workflow class. “Role” records whether
the merge introduced a new capability, supported one, followed up review or
release work, or only maintained dependencies.

| # | Merge | Primary / secondary | User/API delta | Evidence | Burden / class | Role |
|---:|---|---|---|---|---|---|
| 1 | `e646e43` bootstrap history | `CAPABILITY` / `EVIDENCE` | Initial runtime, GOX, CLI, examples, and extension surface | Unit and browser harness baseline | Large initial maintenance surface; opens authored browser UI | New |
| 2 | `a0b52ac` effects cleanup | `CAPABILITY` / `CORRECTNESS` | Component effects and cleanup lifecycle | Todo and lifecycle tests | Adds hook slots; opens cleanup-safe interactive components | New |
| 3 | `c21e537` GOX expression ergonomics | `CAPABILITY` / `TOOLING_DX` | Expression-oriented GOX and nested rendering | Goldens and example updates | Parser/codegen coupling; opens richer authored templates | New |
| 4 | `10f29dc` CI/release hygiene | `PROCESS_GOVERNANCE` / `TOOLING_DX` | No runtime API; repeatable checks and release policy | CI, artifact, and module checks | Ongoing gate cost; supports bootstrap | Support |
| 5 | `3b18741` dashboard benchmark | `EVIDENCE` / `PERFORMANCE` | No API; dense dashboard fixture | Dashboard smoke and size budget | Large fixture; validates non-trivial UI | Support |
| 6 | `1049eb5` Dependabot setup | `DEPENDENCY_MAINTENANCE` | Automated update configuration | Configuration only | Update churn; no class | Maintenance |
| 7 | `3e48016` setup-go update | `DEPENDENCY_MAINTENANCE` | Action version only | CI run | No class | Maintenance |
| 8 | `418c2b3` upload-artifact update | `DEPENDENCY_MAINTENANCE` | Action version only | CI run | No class | Maintenance |
| 9 | `dff9cfe` checkout update | `DEPENDENCY_MAINTENANCE` | Action version only | CI run | No class | Maintenance |
| 10 | `ef1646c` setup-node update | `DEPENDENCY_MAINTENANCE` | Action version only | CI run | No class | Maintenance |
| 11 | `d9625d0` TypeScript update | `DEPENDENCY_MAINTENANCE` | Compiler dependency only | Extension compile/tests | No class | Maintenance |
| 12 | `a27c5fc` Node types update | `DEPENDENCY_MAINTENANCE` | Type dependency only | Extension compile/tests | No class | Maintenance |
| 13 | `10334a3` hashed assets | `TOOLING_DX` / `CAPABILITY` | Hashed package assets and delivery metadata | Package/artifact tests and examples | Packaging complexity; opens cache-safe static delivery | New |
| 14 | `46faea8` Foundation Audit III | `EVIDENCE` / `PROCESS_GOVERNANCE` | No behavior change; gap inventory | Repository audit | Audit upkeep; no class | Follow-up |
| 15 | `3c530b8` explicit memoization | `CAPABILITY` / `PERFORMANCE` | Component memo equality and skip behavior | Dirty-descendant tests and dashboard | More render semantics; opens bounded expensive subtrees | New |
| 16 | `1c7ec3f` reducer dispatch | `CAPABILITY` | `UseReducer` state transitions | Reducer tests and dashboard | Additional hook kind; opens action-driven local state | New |
| 17 | `e061735` dashboard DOM pressure | `EVIDENCE` / `PERFORMANCE` | No public API | DOM pressure script | Harness cost; no new class | Follow-up |
| 18 | `4cd0b75` context selectors | `CAPABILITY` | Context providers and selected subscriptions | Unit and browser tests | Topology/subscription complexity; opens selective shared state | New |
| 19 | `5f34392` virtualized collections | `CAPABILITY` / `PERFORMANCE` | Fixed-height `VirtualList` and `VirtualTable` | Range tests, example, smoke, budgets | Runtime and size cost; opens large fixed-row datasets | New |
| 20 | `f2192e9` Foundation Audit IV | `EVIDENCE` / `PROCESS_GOVERNANCE` | No behavior change | Readiness audit | Audit upkeep; no class | Follow-up |
| 21 | `bdc27a9` runtime benchmark baseline | `EVIDENCE` / `PERFORMANCE` | No API | Benchmarks and performance contract | Benchmark maintenance; supports runtime | Support |
| 22 | `78fef67` component identity v2 | `CAPABILITY` / `CORRECTNESS` | Typed package-aware component identity | Identity/reconciliation tests | Dual legacy/typed APIs; opens reusable stable boundaries | New |
| 23 | `85e2cbc` multipackage workspace | `TOOLING_DX` / `CAPABILITY` | GOX generation across internal packages | Multipackage build/smoke | Workspace materialization cost; opens multi-package apps | New |
| 24 | `7e67742` GOX diagnostics hardening | `CORRECTNESS` / `TOOLING_DX` | Better authored-source diagnostics | Error tests | Diagnostic plumbing; supports GOX authoring | Follow-up |
| 25 | `2ba0c6e` child-entry package | `TOOLING_DX` / `CAPABILITY` | Nested command entrypoint discovery/build | Cmdapp smoke and package checks | More layout rules; opens command-subdirectory apps | New |
| 26 | `23e92ab` public surface polish | `PROCESS_GOVERNANCE` / `RELEASE_DOCS` | API/docs qualification, no new behavior | Docs checks | Contract maintenance; no class | Follow-up |
| 27 | `86f19e0` runtime error semantics | `CORRECTNESS` / `CAPABILITY` | Error phases and global reporting | Panic/cleanup browser and unit tests | Error contract cost; safer interactive apps | Support |
| 28 | `d98b7fb` hash router | `CAPABILITY` | Hash routes, params, links, navigation | Matcher/query and browser tests | Browser listener/query concepts; opens multi-screen SPAs | New |
| 29 | `2258e28` public preview hardening | `CORRECTNESS` / `EVIDENCE` | Forms/reference integration and contract tightening | Router-dashboard and checks | Broad closeout surface; supports preview apps | Follow-up |
| 30 | `c433c0a` package-qualified components | `CAPABILITY` / `TOOLING_DX` | Qualified GOX component tags/import resolution | Goldens and multi-package examples | Grammar/import complexity; opens cross-package component use | New |
| 31 | `4b47f13` checkout update | `DEPENDENCY_MAINTENANCE` | Action version only | CI run | No class | Maintenance |
| 32 | `373ee64` Chrome action update | `DEPENDENCY_MAINTENANCE` | Browser action version only | Browser CI | No class | Maintenance |
| 33 | `5c9eb0a` Error Boundaries | `CAPABILITY` / `CORRECTNESS` | Render-failure subtree containment/reset | Extensive unit and browser tests | Boundary state/lifecycle cost; opens recoverable app shells | New |
| 34 | `135b715` resources | `CAPABILITY` / `CORRECTNESS` | Component-scoped async loading/retry/stale cleanup | Resource tests, example, smoke | Async lifecycle cost; opens data-driven components | New |
| 35 | `c61360e` reference app/tutorial | `EVIDENCE` / `CAPABILITY` | Integrated flow, no new core API | Router-dashboard/tutorial smoke | Reference maintenance; proves combined app | Support |
| 36 | `8c57445` public preview readiness | `PROCESS_GOVERNANCE` / `EVIDENCE` | Compatibility/platform/release contracts | Deep readiness audit | Large documentation surface; no class | Follow-up |
| 37 | `03e7096` identity preview contract | `PROCESS_GOVERNANCE` / `CORRECTNESS` | Clarified component identity scope | Contract comparison | Documentation upkeep; no class | Follow-up |
| 38 | `2cc753a` minimal platform evidence | `EVIDENCE` / `PROCESS_GOVERNANCE` | Added non-Linux core evidence | CI matrix | Platform CI cost; supports portability | Support |
| 39 | `08aca4d` GOX fuzz seeds | `EVIDENCE` / `CORRECTNESS` | No API | Fuzz targets/corpus | Fuzz maintenance; supports parser | Support |
| 40 | `38545a3` asset manifest contract | `TOOLING_DX` / `CORRECTNESS` | Explicit package/asset metadata across examples | Package tests and docs | Manifest compatibility burden; opens inspectable delivery | New |
| 41 | `2926256` package preview closeout | `CORRECTNESS` / `TOOLING_DX` | Tightened package behavior | Focused tests/contracts | More ownership rules; no new class | Follow-up |
| 42 | `b96d2e1` tooling preview closeout | `CORRECTNESS` / `TOOLING_DX` | Tooling safety and diagnostics corrections | Focused CLI/compiler tests | Closeout cost; no new class | Follow-up |
| 43 | `a08da0a` experimental surface hardening | `CORRECTNESS` / `PROCESS_GOVERNANCE` | Runtime API qualification/guardrails | Runtime tests/docs | Compatibility burden; no new class | Follow-up |
| 44 | `a0f3aa1` preview.1 release story | `RELEASE_DOCS` / `PROCESS_GOVERNANCE` | Published-scope documentation | Docs check | Release upkeep; no class | Follow-up |
| 45 | `9b10c13` final preview readiness audit | `EVIDENCE` / `PROCESS_GOVERNANCE` | No behavior change | Audit report | Audit overhead; no class | Follow-up |
| 46 | `d4d8877` community health files | `PROCESS_GOVERNANCE` | Contribution/conduct templates | File review | Maintainer process surface; contributor workflow | Support |
| 47 | `1c09af8` serve path sanitization | `CORRECTNESS` / `TOOLING_DX` | Hardened local server path handling | Security tests/docs | Safety code; no new class | Follow-up |
| 48 | `648ccc5` browser eval sanitization | `CORRECTNESS` / `EVIDENCE` | Hardened smoke expression handling | Browser scripts | Harness complexity; no class | Follow-up |
| 49 | `0441fe2` release-note status polish | `RELEASE_DOCS` | Wording only | Docs review | Release follow-up; no class | Follow-up |
| 50 | `e9f6217` preview.2 release notes | `RELEASE_DOCS` | Published scope record | Docs check | Release upkeep; no class | Follow-up |
| 51 | `4dfcf00` fact-first wording polish | `RELEASE_DOCS` | Corrected historical/current wording | Docs checks | Wording synchronization; no class | Follow-up |
| 52 | `214466f` non-Chrome evidence docs | `EVIDENCE` | Clarified platform evidence | CI/docs comparison | Documentation upkeep; no class | Support |
| 53 | `7f553c2` reusable identity evidence | `EVIDENCE` / `CORRECTNESS` | Clarified identity contract | Docs and existing tests | Review closeout; no class | Follow-up |
| 54 | `caa301d` post-preview focus | `PROCESS_GOVERNANCE` | Selected next problem line | Planning doc | Planning surface; no class | Follow-up |
| 55 | `01723c6` external identity boundaries | `EVIDENCE` / `CORRECTNESS` | No API; external package identity proof | `goxc` tests | Fixture burden; supports reusable packages | Support |
| 56 | `3b471bc` app module dependencies | `CORRECTNESS` / `TOOLING_DX` | Fixed workspace module dependency handling | CLI tests | Module logic; no new class | Follow-up |
| 57 | `43a0f13` external component build flow | `EVIDENCE` / `TOOLING_DX` | No API | End-to-end build test | Fixture burden; supports external package flow | Support |
| 58 | `e863e32` web-first strategy closeout | `PROCESS_GOVERNANCE` | Retired superseded strategy | Docs consistency | Process/doc cost; no class | Follow-up |
| 59 | `0cfc08e` preview.1 notes | `RELEASE_DOCS` | Published scope record | Docs check | Release upkeep; no class | Follow-up |
| 60 | `6c057ee` backend integration boundary | `EVIDENCE` | No framework API; same-origin boundary test | Backend/browser smoke | Harness/server fixture; validates client/API workflow | Support |
| 61 | `21f1231` server-backed fixture | `EVIDENCE` / `CAPABILITY` | Reference Go HTTP + WASM application | Browser smoke | Example maintenance; proves same-origin app class | Support |
| 62 | `8131d62` server-backed recovery | `EVIDENCE` / `CORRECTNESS` | Failure/recovery flow | Browser smoke | More scenario upkeep; supports backend boundary | Support |
| 63 | `729e3b4` server-backed stale cleanup | `EVIDENCE` / `CORRECTNESS` | Stale request behavior proof | Browser smoke | More scenario upkeep; supports resources | Support |
| 64 | `2db7f08` disable build VCS test | `EVIDENCE` / `CORRECTNESS` | No API | Workspace regression test | Test-only cost; supports external builds | Support |
| 65 | `de59d73` `FetchText` loader | `CAPABILITY` | Browser fetch helper with cleanup contract | Tests and server-backed use | Browser API surface; opens common text/JSON-adjacent loading | New |
| 66 | `fb156aa` resource fetch adoption | `EVIDENCE` | No new API; example uses `FetchText` | Resource example | Example upkeep; supports loader | Support |
| 67 | `53b2a15` preview.2 notes | `RELEASE_DOCS` | Published scope record | Docs check | Release upkeep; no class | Follow-up |
| 68 | `b8f92e1` build-info version | `CORRECTNESS` / `RELEASE_DOCS` | Tagged CLI/package version propagation | CLI tests and notes | Version plumbing; no class | Follow-up |
| 69 | `b270f82` deployment version example | `RELEASE_DOCS` | Correct metadata documentation | Docs check | Version churn; no class | Follow-up |
| 70 | `24a8007` split-props allocation evidence | `EVIDENCE` / `PERFORMANCE` | No API | Allocation benchmark | Benchmark cost; supports renderer economics | Support |
| 71 | `46cdb13` lazy prop map allocation | `PERFORMANCE` | Same behavior, fewer allocations | Benchmark/tests | Optimization complexity; supports all DOM apps | Support |
| 72 | `ffc3b6c` split-props slice output | `PERFORMANCE` / `CORRECTNESS` | Internal representation change | Tests/benchmarks | Renderer complexity; supports size/perf | Support |
| 73 | `587c06d` keyed reorder characterization | `EVIDENCE` / `PERFORMANCE` | No API | Exact reorder tests/benchmarks | Test complexity; supports reconciliation | Support |
| 74 | `31d0ea4` WASM headroom audit | `EVIDENCE` / `PERFORMANCE` | No behavior change | Size audit | Audit upkeep; no class | Follow-up |
| 75 | `74dbf6c` virtual-table size reduction | `PERFORMANCE` | Same API with smaller runtime | Size/tests | Specialized code; supports virtualized apps | Support |
| 76 | `437082a` LIS reconciliation | `PERFORMANCE` / `CORRECTNESS` | Exact stable keyed placement | Tests/benchmarks | Algorithm complexity; supports retained lists | Support |
| 77 | `44eb46d` scheduler focus correctness | `CORRECTNESS` / `PERFORMANCE` | Batched dirty scheduling with focus restoration | Unit/browser tests | Scheduler/focus coupling; supports controlled inputs | Follow-up |
| 78 | `f3d0937` keyword component props | `CORRECTNESS` | Fixed GOX keyword property generation | Golden/error tests | Compiler edge-case handling; no class | Follow-up |
| 79 | `4a90e8a` doctor exit code | `CORRECTNESS` / `TOOLING_DX` | Correct CLI failure status | CLI tests | Small contract burden; no class | Follow-up |
| 80 | `9be25e0` community surface polish | `PROCESS_GOVERNANCE` | Simplified contributor/public docs | Docs review | Governance upkeep; no class | Follow-up |
| 81 | `8ddb369` project branding | `PROCESS_GOVERNANCE` / `RELEASE_DOCS` | Logo/landing presentation | Asset/docs review | Asset maintenance; no workflow class | Support |
| 82 | `67dd2f4` preview.4 prep | `RELEASE_DOCS` | Prepared release contract | Docs checks | Release upkeep; no class | Follow-up |
| 83 | `c3be38b` preview.4 publish closeout | `RELEASE_DOCS` | Aligned published state | Docs checks | Post-publish churn; no class | Follow-up |
| 84 | `2478033` retained input selection | `CORRECTNESS` | Preserved focused selection after dirty update | Unit/browser smoke | Focus restoration logic; no new class | Follow-up |
| 85 | `f93eef2` GOX source expression diagnostics | `CORRECTNESS` | Accurate malformed/nested source locations | Compiler tests/goldens | Diagnostic mapping cost; no class | Follow-up |
| 86 | `ce9ef9b` exact keyed LIS placement | `PERFORMANCE` / `CORRECTNESS` | O(n log n) placement under matching semantics | Unit/benchmark evidence | Algorithm maintenance; no new class | Support |
| 87 | `2d5cab1` preview.5 prep | `RELEASE_DOCS` | Prepared release contract | Docs checks | Release upkeep; no class | Follow-up |
| 88 | `91e7e51` preview.5 publish closeout | `RELEASE_DOCS` | Aligned published state | Docs checks | Post-publish churn; no class | Follow-up |
| 89 | `926c96f` Todo mutation characterization | `EVIDENCE` / `PERFORMANCE` | No runtime API | DOM bridge counters and stable assertions | Smoke instrumentation; informs #70 | Support |
| 90 | `304dc2c` dashboard bridge attribution | `EVIDENCE` / `PERFORMANCE` | No runtime API | Exact row/listener/placement attribution | Large harness follow-up; informs #70 | Follow-up |
| 91 | `22353ce` `goxc check` diagnostics | `TOOLING_DX` / `CAPABILITY` | Read-only text/schema-v1 GOX validation | CLI tests and docs | CLI/schema maintenance; opens machine-readable diagnostics | New |
| 92 | `352bf6a` VS Code diagnostics | `TOOLING_DX` / `CORRECTNESS` | Saved-source inline diagnostics, trust, multi-root isolation | 43 Node tests and extension CI | Process/editor integration; opens editor feedback workflow | New |
| 93 | `cc04464` versioned roadmap | `PROCESS_GOVERNANCE` | Replaced focus doc with version train | Docs checks | Planning commitment surface; no class | Support |
| 94 | `9548345` preview.6 prep | `RELEASE_DOCS` | Durable diagnostics release contract | Docs and release gates | Release upkeep; no class | Follow-up |
| 95 | `3997797` preview.6 publish closeout | `RELEASE_DOCS` | Published-state evaluator/deployment/roadmap alignment | Docs and release verification | Post-publish churn; no class | Follow-up |

## Appendix B — Validation Evidence

### Environment

| Tool | Version |
|---|---|
| Git | 2.51.0 |
| Go | 1.24.4, linux/amd64 |
| TinyGo | 0.41.1, LLVM 20.1.1, using Go 1.24.4 |
| Node | 20.19.4 |
| npm | 9.2.0 |
| Chrome | 149.0.7827.196 |
| gzip | 1.13 |
| Brotli | 1.1.0 |
| Zstandard | 1.5.7 |

### Gate Results

| Gate | Exit | Result | Evidence |
|---|---:|---|---|
| `git diff --check` | 0 | Validated | `diff-check.log` |
| `node scripts/docs-check.mjs` | 0 | Validated | `docs.log` |
| `scripts/artifact-check.sh` | 0 | Validated | `artifact.log` |
| `scripts/module-path-check.sh` | 0 | Validated | `module.log` |
| `go test ./...` | 0 | Validated | `go-test-all.log` |
| `go test -race ./pkg/... ./cmd/...` | 0 | Validated | `race.log` |
| `go vet ./...` | 0 | Validated | `vet.log` |
| `go test -tags=goframe_debug ./...` | 0 | Validated | `debug-tags.log` |
| GOX golden/error-golden tests | 0 | Validated | `gox-goldens.log` |
| `go test ./cmd/goxc -count=1` | 0 | Validated | `goxc-tests.log` |
| Coverage | 0 | 84.9% goframe, 83.8% gox, 68.8% goxc | `coverage.log` |
| Runtime benchmarks | 0 | Validated; observational timings only | `benchmarks.log` |
| `npm ci --prefix extensions/vscode-gox` | 0 | Validated | `npm-ci.log` |
| `npm test --prefix extensions/vscode-gox` | 0 | Validated; 43 pure test cases in source | `npm-test.log` |
| `scripts/check.sh` | 0 | Validated from `/var/tmp` | `check-var-tmp.log` |
| `scripts/browser-smoke.sh` | 0 | Validated every configured example and error-fixture scenario | `browser-smoke.log` |
| Clean final `scripts/size-budget.sh` | 0 | Validated locally; informational bytes because Go differs from CI | `final-size.log` |

The first aggregate attempt in `/tmp` was blocked by host VCS discovery through
a read-only `/tmp/.git`; its `check.log` is retained. Re-running the same frozen
tree and isolated caches under `/var/tmp` passed. Size was not read from
debug-tag bundles: all 11 budgeted examples were cleaned, generated, packaged
with TinyGo, repackaged with hashing/preload/gzip/Brotli, and only then measured.

### Browser Attribution Highlights

- Controlled Todo input: one flush, one rAF request/callback, no structural or
  listener operations, retained node/focus/selection, one legitimate runtime
  property write.
- Bursty Todo state: one flush and one rAF callback, proving coalescing for the
  characterized turn.
- Keyed reorder: one flush and three `insertBefore` calls with retained nodes.
- Dashboard search: 28 rows before and after; 4 retained as the same nodes, 24
  added, 24 removed, no retained rows recreated or reinserted. Exactly two
  listener additions/removals were attributed to each new/removed row.
- Dashboard aggregate counts (336 elements, 216 text nodes, 96 comments, 600
  `insertBefore`, 72 removals, 48 listener adds/removes) remain observations,
  not hard thresholds. Row replacement explains the measured listener and
  retained-placement classes; a buffered alternative was not measured.

### Local Evidence Locations

- Audit data: `/tmp/goframe-deep-audit.zjBONr/data`
- Validation logs: `/tmp/goframe-deep-audit.zjBONr/logs`

These paths are local, uncommitted evidence. They are not part of the durable
repository contract.

## Appendix C — Current Capability Inventory

| Layer | Current capability | Evidence boundary | Explicitly absent |
|---|---|---|---|
| Runtime | Retained nodes/components, state/reducer/effects/context, memo, events, keyed reconciliation, batching, focus restoration | Host tests plus Chrome browser smoke | Stable API, non-browser renderer, production support |
| Failure model | Global phase-aware reporting and render Error Boundaries | Unit and browser recovery tests | Async/effect exception capture by boundaries |
| Data | Component-scoped `UseResource`, `FetchText`, stale suppression, retry, cleanup | Resource, router-dashboard, server-backed | Cache, Suspense, mutations, server functions |
| Router | Hash routing, params, query, links, back, programmatic navigation, not-found | Router and router-dashboard | History fallback, transition state, loaders, blockers |
| Collections | Fixed-height virtual list/table | Unit, virtualized, dashboard | Dynamic heights, infinite loading, advanced a11y |
| GOX | Components, expressions, nested markup, fragments, package-qualified tags, source diagnostics | Goldens, errors, fuzzing, external builds | Formatter, LSP, semantic Go/TinyGo check |
| Toolchain | Check, generate, build, package, export, clean, size, doctor, version, local serve | CLI tests, aggregate check, package fixtures | Production server, JS bundler, deployment provider |
| Layout | Root, internal multipackage, and child command entrypoints | Multipackage/cmdapp smoke | Broad reusable multi-module ecosystem guarantee |
| Delivery | Static WASM/JS/CSS, manifests, hashes, preloads, gzip/Brotli sidecars | Artifact/package tests and size CI | Bundle splitting, multi-entry runtime, dynamic loading |
| Backend boundary | Plain Go `net/http` serving static package and same-origin API | Server-backed browser smoke | GoFrame server/fullstack framework |
| Editor | Syntax/snippets plus saved-source schema-v1 diagnostics, UTF mapping, multi-root isolation, Workspace Trust | 43 Node tests and VS Code CI | Unsaved semantic analysis, formatter, completion, LSP |
| Platforms | Go/GOX core on Linux/macOS/Windows; Chrome/Chromium browser evidence | CI and local Chrome smoke | Automated Firefox/WebKit/Safari support |
| Release | Signed preview tags, durable release notes, exact module/version verification | `v0.2.0-preview.6` and release process | Stable 1.0 or binary distribution promise |

The inventory is the present boundary, not a promise that every listed surface
is stable. API stability and release notes remain authoritative for exact
preview contracts.
