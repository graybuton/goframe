# Repository Hygiene Audit I - Deep Go Cleanup

## Frozen Snapshot

This audit uses merge base `b71af16cb49a38a5c764b41721b6d719a1c36f0a`
on 2026-08-02. The cleanup branch began with a clean worktree and preserved
the ignored VS Code extension package, dependency, and output directories.
No analyzer binary, raw report, generated package, or comparison artifact is
committed.

The first pass removed `generateFileSafely` and recorded a broad inventory.
The continuation revisited every strong finding instead of treating the audit
document as a substitute for cleanup. It removes three more obsolete helpers,
resolves all confirmed `nilness`, `writestring`, and `unusedparams` findings in
`cmd/goxc`, attributes the `useState` experiment at the WASM-section level,
and records the remaining findings with concrete evidence.

## Scope

Code cleanup is limited to private Go declarations and expressions under
`cmd/goxc`, `pkg/gox`, and `pkg/goframe`. First-party Go code under `cmd`,
`pkg`, `examples`, and `scripts/fixtures` is included in the diagnostic
inventory. Examples and fixtures are evidence inputs and are not edited.

Public APIs, CLI behavior, generated Go format, examples, browser fixtures,
workflows, dependencies, lockfiles, size budgets, and non-Go code are outside
the cleanup boundary.

## Tool Versions

- Gopls `v0.23.0`, built with Go 1.26.5.
- Staticcheck 2026.1 (`v0.7.0`), built with Go 1.26.5.
- `deadcode` from `golang.org/x/tools v0.48.0`, built with Go 1.26.5.
- Go 1.22.12, Go 1.25.12, and Go 1.26.5 for validation.
- TinyGo 0.41.1 with LLVM 20.1.1 for browser and WASM evidence.

All task-owned tools and caches are outside the repository.

## Build And Target Contexts

| Context | Target and tags | Staticcheck packages | deadcode roots | Tests |
|---|---|---|---|---|
| H1 | Linux/amd64, ordinary | `./...` | all loadable first-party executables | included and excluded |
| H2 | Linux/amd64, `goframe_debug` | `./...` | all loadable first-party executables | included and excluded |
| H3 | Windows/amd64, ordinary | `./cmd/goxc`, `./pkg/gox` | `cmd/goxc`; test roots also include `pkg/gox` | included and excluded |
| W1 | js/wasm, ordinary | `./pkg/goframe` | `duplicate-keys`, `runtime-errors` | production only |
| W2 | js/wasm, `goframe_debug` | `./pkg/goframe` | `duplicate-keys`, `runtime-errors` | production only |

This corrects the earlier H3 wording. The Windows production deadcode run does
not use `./...` or treat a library package as an executable root. Its real
production root is `cmd/goxc`; with `-test`, the `cmd/goxc` and `pkg/gox` test
executables are real additional roots.

The W1 and W2 test suites are not valid analyzer roots because
`protected_teardown_test.go` selects host-only test helpers under js/wasm.
Production W1 and W2 type-check successfully. Most browser applications also
need generated GOX declarations before they load, so the two pure-Go fixture
executables provide bounded runtime reachability evidence rather than a claim
of whole-browser-program coverage.

## Method

Gopls first checked the five mandatory files with hint diagnostics enabled,
then checked all 222 tracked first-party, non-generated Go files in bounded
batches. Staticcheck ran `-checks=all`, with tests both included and excluded
where applicable. Findings were grouped as `SA*`, `S*`, `U1000`, `ST*`, and
`QF*`; no `SA*` finding was present. Ordinary and debug `go vet` runs passed.

Deadcode used real executable and test roots. Its display filter restricts
reported packages to `cmd/goxc`, `pkg/gox`, and `pkg/goframe`; it does not
create roots. Direct references, selectors, method expressions, callbacks,
interfaces, reflection, `js.FuncOf`, generated references, build tags,
`//go:linkname`, tests, and history were inspected before classifying a
declaration. No suspicious live declaration lacked a direct caller, so no
`-whylive` query was needed to explain an indirect edge.

## Finding States

- `FIX_NOW`: private and mechanically removable or simplifiable with preserved
  contracts and output.
- `PRESERVE_WITH_PROOF`: retained because a named caller, target role, test
  seam, compatibility contract, or measured output boundary exists.
- `SEPARATE_BEHAVIORAL_STAGE`: changing the finding requires a distinct
  behavioral contract and executable evidence.
- `EVIDENCE_GAP`: an identified target or generated-source limitation prevents
  a complete conclusion.

Style-only modernizations are recorded but are not treated as correctness or
reachability defects.

## Final Analyzer Inventory

### Gopls

The mandatory final check contains no `nilness`, `unusedparams`, `writestring`,
or `unusedfunc` diagnostic for F003-F009. Its remaining 13 output lines are
three multiline `runtime.GOROOT` deprecation diagnostics and one
`sort.Slice` modernization suggestion. The report SHA-256 is
`4d8f8a9246e4455894d119664b693edfce2d7e62acfe27849cf99fbfea8ee728`.

The repository-wide report has 163 lines and SHA-256
`a67c1022521223b224da5e46148184cea8c72dedee7b30e3c8d31fa8810f0113`.
It also records generated-GOX loading gaps, host/browser stub selection,
example and fixture declarations, and Go-version modernization hints.

### Staticcheck

| Context | `S*` | `ST*` | `U1000` | Total |
|---|---:|---:|---:|---:|
| H1 tests included | 1 | 56 | 91 | 148 |
| H1 tests excluded | 1 | 56 | 230 | 287 |
| H2 tests included | 1 | 56 | 91 | 148 |
| H2 tests excluded | 1 | 56 | 230 | 287 |
| H3 tests included | 0 | 10 | 0 | 10 |
| H3 tests excluded | 0 | 10 | 12 | 22 |
| W1 production | 2 | 22 | 5 | 29 |
| W2 production | 2 | 22 | 5 | 29 |

There are no `SA*` findings. The two runtime `S*` suggestions are evaluated
under F013. `ST1000` package-comment and `ST1005` diagnostic-capitalization
signals do not establish dead or incorrect behavior; changing exact CLI error
text is forbidden in this cleanup.

### Deadcode

| Context | Production functions | Test-inclusive functions |
|---|---:|---:|
| H1 | 272 | 23 |
| H2 | 272 | 23 |
| H3 | 20 | 3 |
| W1 | 134 | not applicable |
| W2 | 134 | not applicable |

The large production counts include exported package surfaces and features not
reached by the selected executables. Test-inclusive H1/H2 reduces the closed
set to debug stubs, exported compatibility surfaces, and `useState`; it does
not report the removed F001, F003, F004, or F005 declarations.

## Mandatory Finding Resolution

| ID | Finding | Decision | Evidence and result |
|---|---|---|---|
| F001 | `generateFileSafely` | `FIX_NOW`, complete | It remains removed in `172ee5bc8e011849c3b158129403667c17f6a889`; package-coordinated generation superseded it. |
| F002 | `useState` | `PRESERVE_WITH_PROOF` | No caller or compatibility role exists, but the fully attributed standard-Go removal grows Brotli by 300 bytes. Details are below. |
| F003 | `fileExists` | `FIX_NOW` | Declaration-only reference; structured metadata checks replaced its export-ownership callers. Removed in `f4958fd06c8f3eb7e7cecdad4517396827ae85e1`. |
| F004 | `removeDirectoryIfExists` | `FIX_NOW` | Declaration-only reference; all cleanup callers use root-aware `removeDirectoryIfExistsBelowRoot`. Removed in `f4958fd06c8f3eb7e7cecdad4517396827ae85e1`. |
| F005 | `(*devServer).activatePackage` | `FIX_NOW` | No method expression, interface, callback, or test caller; `activatePackageWithCommit` superseded it. Removed in `f4958fd06c8f3eb7e7cecdad4517396827ae85e1`. |
| F006 | redundant `err != nil` after failed `Lstat` | `FIX_NOW` | The `else` branch already proves non-nil. Simplified without changing the symlink or error path in `1ac9dd68a8dfd01e3f68895705c87a2e2346377b`. |
| F007 | redundant `err == nil` after early return | `FIX_NOW` | `pathsOverlap` now returns `relation != "separate"`; path and symlink tests preserve behavior. Fixed in `1ac9dd68a8dfd01e3f68895705c87a2e2346377b`. |
| F008 | concatenated `WriteString` calls | `FIX_NOW` | Every confirmed `writeWorkspaceGoMod` occurrence is split into sequential writes. Generated `go.mod` is byte-identical. Fixed in `1ac9dd68a8dfd01e3f68895705c87a2e2346377b`. |
| F009 | unused `entry` parameter | `FIX_NOW` | Both callers now pass only the relative path; embed discovery, unsafe-entry rejection, and overlay bytes are unchanged. Fixed in `1ac9dd68a8dfd01e3f68895705c87a2e2346377b`. |

The full Gopls pass found one additional `writestring` allocation in
`writePackageGoMod`. It is fixed in `1ac9dd68a8dfd01e3f68895705c87a2e2346377b`
with sequential writes, preserving exact generated bytes.

## Complete Grouped Inventory

| ID | Finding group | Decision | Concrete basis |
|---|---|---|---|
| F010 | `writePackageGoMod` builder allocation | `FIX_NOW` | Gopls `writestring`; fixed with byte-identical output in `1ac9dd68a8dfd01e3f68895705c87a2e2346377b`. |
| F011 | command helpers reported only without tests | `PRESERVE_WITH_PROOF` | Direct callers exist in generation, workspace, dev, reload, source-selection, package, path, and symlink tests. |
| F012 | runtime placement, teardown, and virtual-range helpers | `PRESERVE_WITH_PROOF` | Direct host test or non-js bridge callers exist; browser roots use the decomposed production paths. |
| F013 | router S1017 and S1021 suggestions | `PRESERVE_WITH_PROOF` | S1017 grows standard-Go WASM by 47 bytes; S1021 is output-active style-only churn. |
| F014 | `runtime.GOROOT` deprecation | `SEPARATE_BEHAVIORAL_STAGE` | Replacement changes compiler discovery and doctor behavior and needs relocated-binary evidence. |
| F015 | `slices`, `strings.Cut`, range, `max`, and switch modernizations | `PRESERVE_WITH_PROOF` | No correctness or reachability defect; applying style-only changes would violate the narrow output-preserving cleanup boundary. |
| F016 | generated-GOX application load failures | `EVIDENCE_GAP` | Analyzers cannot type-check missing generated `App`, props, and helper declarations before goxc materialization. |
| F017 | example and browser-fixture U1000 findings | `PRESERVE_WITH_PROOF` | Generated GOX, target-specific code, and successful package/browser executions establish current evidence roles; these paths are read-only here. |
| F018 | exported runtime and GOX declarations reported by deadcode | `PRESERVE_WITH_PROOF` | Closed-world executable roots do not supersede exported compatibility contracts. |

## Preserved Findings With Proof

### Command test seams

The following private command declarations are reported only when tests are
excluded. Direct repository test callers preserve them as intentional seams:

- `devSnapshot.paths` in `dev_test.go`;
- `devGenerationManager.activeID` and `devGenerationHandler` in
  `dev_generation_test.go`;
- `devReloadBroker.subscriberCount` in `dev_reload_test.go`;
- `browserGenerationSourceSelection` and `browserTargetToolTags` in
  `generation_constraints_test.go`;
- `resolvePackageAssetSource` in `symlink_test.go`;
- `prepareBuildWorkspace` in `workspace_test.go` and
  `filesystem_safety_test.go`;
- `generateIntoDirectory`, `generateIntoDirectoryForCompiler`,
  `generateFilesIntoDirectory`, and
  `generateFilesIntoDirectoryWithSelection` in generation and workspace tests.

With tests included, `cmd/goxc` has no U1000 finding. `pkg/gox` also has no
U1000 finding with tests included.

### Runtime target and test roles

Ordinary and debug host stub functions are selected to satisfy the same
private call surface as the js/wasm implementations. Their apparent U1000
status in one context is offset by the alternate target implementation and
browser call sites.

`stableChildPlacementStart` is called by the keyed-reorder tests.
`finalizePendingProtectedSubtreeTeardown` is called by the non-js teardown
bridge. `validateVirtualRangeDimensions` and `calculateVirtualRange` are called
by virtual-range tests, while their browser production decomposition uses the
lower-level helpers. These are concrete target/test roles rather than inferred
future uses.

Deadcode reports exported runtime and GOX functions when the selected
executables do not call them. Exported compatibility surfaces are not removed
from a closed-world executable graph.

### Staticcheck runtime simplifications

Staticcheck suggests unconditional `strings.TrimPrefix` in
`normalizeRouteTarget` (S1017) and merging a `js.Func` declaration with its
assignment (S1021). Both were tested independently and restored. S1017 grew
the standard-Go router-dashboard WASM from 2,654,439 to 2,654,486 bytes.
S1021 kept the size at 2,654,439 bytes but changed the complete artifact hash
without adding correctness, reachability, or allocation value. S1021 is
style-only; S1017 violates the no-growth boundary.

## `useState` WASM Attribution

`useState` has no direct or indirect caller in the inspected source and is
reported by Staticcheck and deadcode in every applicable runtime context. Its
history shows that `UseState` began calling `useStateSlot` directly when
reducer support was introduced. The function was nevertheless restored after
the required output attribution, rather than after a complete-hash comparison
alone.

For the standard-Go counter package, alternating restored and removed builds
are deterministic:

| Evidence | Restored | Removed | Delta |
|---|---:|---:|---:|
| raw | 2,192,823 | 2,192,823 | 0 |
| gzip | 642,700 | 642,695 | -5 |
| Brotli | 494,467 | 494,767 | +300 |
| Zstandard | 523,295 | 523,295 | 0 |

The complete SHA-256 changes from
`01153ff699ca1c06297670f5a72a1a13b4b160abc122456bfe951cd8b2633c6f`
to
`b10c788f52d9b6735c1f6a90bc3f139b331b4b324afe5f0c00b94f4bd45eaa2f`.
Section IDs, lengths, and counts are identical. The code-section payload hash
remains
`2dc6cde2ac71a11fbd62217ddded24a589da5a32783b96ab3fffadeff7b3f69a`;
the custom `producers` section remains
`0c1b39ac92c76e8b5a75b57d492e9a58bd894bfa2787e6def3e2b8f7b7336f48`.
Only 28 bytes change, all in the data section, whose payload hash changes from
`24459ba7c0dbd4a69dc6f68482463d0d7771ef5f73aa17b25405d5075706ba3b`
to
`a287489142997c773fbff216cbff9c0bd81c5eae690393a57d38f6c7f9f8822e`.
Extracted strings are identical. Goxc intentionally links with an empty Go
build ID, so no build-ID payload is available to attribute.

All 44 TinyGo raw, gzip, Brotli, and Zstandard streams across the eleven
budgeted applications remain byte-identical after temporary removal. Every
budget and ratio passes. Aggregate browser phases using standard Go and TinyGo
also preserve behavior. The removal is rejected solely because its
reproducible Brotli output grows, which fails this task's explicit acceptance
boundary.

## Separate Behavioral Stage

Gopls deprecates `runtime.GOROOT` use in `cmd/goxc/helpers.go` and
`cmd/goxc/doctor.go`. Replacing it changes how goxc locates the Go executable
and reports GOROOT after a built binary is relocated. That is a compiler
discovery contract, not a dead-code edit. A separate
`fix/goxc-goroot-discovery` branch would need executable tests for relocated
binaries, `PATH` precedence, explicit `GOROOT`, Go/TinyGo selection, doctor
output, and exact failure diagnostics before changing those files.

No other strong finding requires a separate behavioral stage. Modernization
hints for `slices`, `strings.Cut`, range-over-int, `max`, and tagged switches
are style or minimum-Go-version changes without a demonstrated defect.

## Evidence Gaps And Read-Only Findings

- Test-inclusive js/wasm analysis remains invalid while the protected teardown
  tests select host-only helpers.
- GOX applications do not type-check before generated declarations are
  materialized. Their analyzer errors do not prove the authored helpers dead.
- Example and fixture U1000 findings are frequently references from generated
  GOX, js-only implementations, or browser evidence. These paths are read-only
  in this cleanup and their successful package/browser execution is preserved.
- TinyGo reachability is established through real source selection, packaging,
  byte comparison, size gates, and browser execution; host analyzers do not
  model the TinyGo linker.
- Dynamic third-party use of exported APIs is outside closed-world deadcode
  analysis.

## Behavior And Output Identity

The starting-head and cleanup `goxc` binaries were exercised against the same
fixed-path fixture. Directory, single-file, and in-place generation each
produce the same 409-byte output with SHA-256
`c3135e3aa47f59e2464018af8e7d6398fa587991c8a8c748affaea7f60abb973`.
Text and JSON check output, doctor output, exit status, stdout, stderr,
generated filenames, generated `go.mod`, embed selection, managed package
output, and package bytes are identical. The timestamp-bearing package
metadata differs only in `generatedAt`; removing that volatile field produces
an identical document.

The generated workspace `go.mod` SHA-256 is
`acfb7d984d87252705eda22e8828b2a882ba835a2f200140b8698577df467e1f`.
The fixed CLI package bundle SHA-256 is
`2bc7a85314fdf0d6ef1411e9800547ceae4687e81a709b3eb47b98e6db2bd1a0`.

With the final runtime source restored, all eleven TinyGo applications and all
44 deterministic compression streams remain byte-identical to the starting
head. The baseline hash-manifest SHA-256 is
`c5a566ba572082ed9aadf08d887003846f97d71203cf557053b67379dd0a6211`.
No runtime, GOX, example, fixture, dependency, workflow, lockfile, or budget is
changed by the cleanup commits.

## Cleanup Commits

- `172ee5bc8e011849c3b158129403667c17f6a889` removes
  `generateFileSafely`.
- `f4958fd06c8f3eb7e7cecdad4517396827ae85e1` removes `fileExists`,
  `removeDirectoryIfExists`, and `(*devServer).activatePackage`.
- `1ac9dd68a8dfd01e3f68895705c87a2e2346377b` resolves the two
  tautologies, all confirmed builder-write allocations, and the unused embed
  parameter.

Each cleanup commit is signed. The final documentation commit records the
completed finding set without altering package inputs.

## Limitations

This is a bounded repository snapshot, not a permanent proof that every
retained private declaration is necessary. Closed-world analysis cannot prove
third-party API use, and generated/target-specific limitations remain explicit.
Output identity applies to the recorded CLI fixture, package workflow, browser
evidence, and eleven budgeted applications under the recorded toolchains.

The audit installs no permanent analyzer gate. It does not redesign APIs,
change runtime behavior, rewrite generated output, alter exact diagnostics,
modify examples, rebaseline WASM, or begin the separate GOROOT discovery
contract.
