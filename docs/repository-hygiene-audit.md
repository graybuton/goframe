# Repository Hygiene Audit I — Go Dead and Obsolete Code

## Frozen Snapshot

This audit uses commit `b71af16cb49a38a5c764b41721b6d719a1c36f0a`
on 2026-08-02. The repository, `origin/main`, and the local audit branch all
pointed to that commit before analysis. The worktree was clean, the remote
audit branch did not exist, and the task preserved the existing ignored VS
Code extension package, dependency, and output directories.

The task-owned declaration inventory contains 3,356 Go declarations plus its
header. Its SHA-256 is
`e941fd1067e5af7e54fb4a4bfdc0c09d5fe98e5880f203376190cf99d5fbcc89`.
The selected-file inventory contains 171 records plus its header and has
SHA-256
`c4792c3e5255af0c6bb5db241916614f297d06c803b1d0beb3b945b7917e16d8`.
Neither inventory nor any raw analyzer report is committed.

## Scope

The cleanup scope is unexported Go declarations in `cmd/goxc`, `pkg/gox`, and
`pkg/goframe`. The audit also reads first-party executables, tests, examples,
fixtures, generated-code boundaries, build tags, history, callbacks, and
interfaces to determine whether an analyzer signal represents removable code.

JavaScript, TypeScript, shell, CSS, assets, workflows, generated `.gox.go`
files, `.goframe` output, dependencies, public APIs, size budgets, and
unrelated refactors are outside the cleanup scope.

## Tool Versions

- Staticcheck 2026.1 (`v0.7.0`).
- `deadcode` from `golang.org/x/tools v0.48.0`, built with Go 1.26.5.
- Go 1.22.12, Go 1.25.12, and Go 1.26.5 for validation.
- TinyGo 0.41.1 with LLVM 20.1.1 for source selection and WASM output.
- actionlint 1.7.12, built with Go 1.26.5.
- Node.js 24.18.1 and npm 11.16.0 for documentation and extension checks.

## Build And Target Contexts

| Context | Target and tags | Packages | Tests |
|---|---|---|---|
| H1 | Linux/amd64, ordinary | `./...` | included and excluded |
| H2 | Linux/amd64, `goframe_debug` | `./...` | included and excluded |
| H3 | Windows/amd64, ordinary | `./cmd/goxc`, `./pkg/gox` | included and excluded |
| W1 | js/wasm, ordinary | `./pkg/goframe` | production only |
| W2 | js/wasm, `goframe_debug` | `./pkg/goframe` | production only |

H3 intentionally excludes `pkg/goframe`: the repository's supported Windows
selection for this audit is the toolchain and compiler packages. W1 and W2
test-inclusive analysis are not applicable because the protected teardown
tests select host-only helpers incorrectly under js/wasm. Production W1 and W2
type-check successfully.

## Method

The inventory parser records declaration kind, visibility, receiver, source
location, build constraints, selected contexts, and test-file status. Each
analyzer signal is then checked against direct references, selectors, function
values, interfaces, registrations, callbacks, reflection, `js.Func` use,
templates, generated references, string dispatch, `//go:linkname`, history,
tests, and build-tag alternatives.

Staticcheck ran with `GOTOOLCHAIN=go1.26.5`, task-owned `GOCACHE` and
`XDG_CACHE_HOME`, and these exact analyzer arguments:

```bash
staticcheck -checks=U1000 -tests=true ./...
staticcheck -checks=U1000 -tests=false ./...
staticcheck -checks=U1000 -tests=true -tags=goframe_debug ./...
staticcheck -checks=U1000 -tests=false -tags=goframe_debug ./...
GOOS=windows GOARCH=amd64 staticcheck -checks=U1000 -tests=true ./cmd/goxc ./pkg/gox
GOOS=windows GOARCH=amd64 staticcheck -checks=U1000 -tests=false ./cmd/goxc ./pkg/gox
GOOS=js GOARCH=wasm staticcheck -checks=U1000 -tests=false ./pkg/goframe
GOOS=js GOARCH=wasm staticcheck -checks=U1000 -tests=false -tags=goframe_debug ./pkg/goframe
```

`deadcode` used this display filter:

```text
^github\.com/graybuton/goframe/(cmd/goxc|pkg/gox|pkg/goframe)(/.*)?$
```

The exact host analyzer arguments were:

```bash
deadcode -json -filter="$FILTER" ./...
deadcode -test -json -filter="$FILTER" ./...
GOFLAGS='-tags=goframe_debug' deadcode -json -filter="$FILTER" ./...
GOFLAGS='-tags=goframe_debug' deadcode -test -json -filter="$FILTER" ./...
GOOS=windows GOARCH=amd64 deadcode -json -filter="$FILTER" ./...
GOOS=windows GOARCH=amd64 deadcode -test -json -filter="$FILTER" ./...
```

The filter restricts displayed declarations; it does not create reachability
roots. All loadable first-party packages supplied the host executable roots.
The js/wasm commands used the two loadable pure-Go executable roots that reach
the runtime:

```bash
GOOS=js GOARCH=wasm deadcode -json -filter="$FILTER" \
  github.com/graybuton/goframe/scripts/fixtures/duplicate-keys \
  github.com/graybuton/goframe/scripts/fixtures/runtime-errors

GOOS=js GOARCH=wasm GOFLAGS='-tags=goframe_debug' \
  deadcode -json -filter="$FILTER" \
  github.com/graybuton/goframe/scripts/fixtures/duplicate-keys \
  github.com/graybuton/goframe/scripts/fixtures/runtime-errors
```

## Staticcheck Test Inclusion

Staticcheck finding exits are audit results, not passing test exits. Every
applicable final command reported U1000 findings without a load, type-check,
internal, or cache error. Empty stderr files all have SHA-256
`e3b0c44298fc1c149afbf4a4bfdc8996fb92427ae41e4649b934ca495991b7852b855`.

| Context | Staticcheck tests=true | Staticcheck tests=false | deadcode tests | deadcode production |
|---|---|---|---|---|
| H1 | 94, `917b2b066ef64bf2e378c2c95ce31c7d7d607549ae18ccd082f946630f532a22` | 233, `0d7a3ec8bdd7af16097e03a3a87646ad645916fee62a72d15213930672460368` | 26, `98edf318c17da840827d5af947db03c94970a9d9a4729f21bdb69d8cb35468f3` | 275, `3f7df1acfd1aa1128d55d1e61c340cd9e00784603937bbdd6331728ce5d92c23` |
| H2 | 94, `d551eb55ca9b400690f473fe4e0a294423e8eed65647db766df1c44edbb87097` | 233, `56e9b1fc1c0bf3929e1914087baf883d78577af77e58577a403fdfeb67f259a7` | 26, `98edf318c17da840827d5af947db03c94970a9d9a4729f21bdb69d8cb35468f3` | 275, `3f7df1acfd1aa1128d55d1e61c340cd9e00784603937bbdd6331728ce5d92c23` |
| H3 | 3, `08e1cc3af630422859507291033d7865707cf691d98cfc601df61496272e3dd4` | 15, `6bab4dba9b874f2222e10e2b71a2e0b2252953bb78359c19a4f61a3a1e4bd019` | 26, `98edf318c17da840827d5af947db03c94970a9d9a4729f21bdb69d8cb35468f3` | 275, `3f7df1acfd1aa1128d55d1e61c340cd9e00784603937bbdd6331728ce5d92c23` |
| W1 | not applicable | 5, `81023a6d6077ba91e81fc0355f5df73189d7649d4ea19fd7bb5f99dbe72f253d` | not applicable | 134, `6a71fed1a6f00b37df85451dab890c9764204468962d9954da09b60e99d19ad7` |
| W2 | not applicable | 5, `81023a6d6077ba91e81fc0355f5df73189d7649d4ea19fd7bb5f99dbe72f253d` | not applicable | 134, `6a71fed1a6f00b37df85451dab890c9764204468962d9954da09b60e99d19ad7` |

Compared with the frozen snapshot, every applicable H1, H2, and H3 report has
exactly one fewer finding. W1 and W2 are unchanged because the removed command
helper is not selected in the browser runtime package. No new U1000 or
deadcode finding appeared.

## Deadcode Reachability Roots

Host production analysis starts from all loadable first-party `main` packages;
`-test` adds their test executables. The same roots are loaded under ordinary,
debug, and Windows contexts. The H1, H2, and H3 result sets happen to have the
same counts and hashes, but they remain separate target observations.

The repository has 24 js/wasm `main` packages in ordinary and debug selection.
Most require generated GOX declarations before they type-check. The audit does
not fabricate those declarations or treat a failed load as reachability
evidence. W1 and W2 therefore use the two pure-Go fixture roots named above.

## Classification Rules

- `REMOVE_NOW`: private, analyzer-confirmed, unreferenced in every applicable
  context, historically superseded, and output-identical when removed.
- `TEST_ONLY_SEAM`: production declaration directly exercised by repository
  tests or test parity checks.
- `TARGET_OR_TAG_SPECIFIC`: selected or reached through another GOOS, GOARCH,
  or build-tag implementation.
- `EXPORTED_OR_COMPATIBILITY_SURFACE`: exported callable surface; limited
  executable roots do not establish removability.
- `OBSOLETE_BUT_SEPARATE`: apparently superseded, but subsystem boundaries or
  output evidence require a separate decision.
- `TEST_CONTEXT_MISMATCH`: analyzer loading is invalid because a test and its
  helper select different target contexts.
- `EVIDENCE_GAP`: the tool cannot construct an applicable executable graph.
- `OUT_OF_SCOPE`: finding is outside cleanup-authorized paths.

No declaration is removed from analyzer output alone.

## Confirmed Removals

| ID | Declaration | Package/path | Static evidence | Reference evidence | Historical role | Output evidence | Cleanup commit |
|---|---|---|---|---|---|---|---|
| F001 | `generateFileSafely` | `cmd/goxc/workspace.go` | U1000 in H1/H2/H3 with and without tests; deadcode in H1/H2/H3 production and test roots | Declaration only; no interface, callback, registration, reflection, linkname, generated, tagged, or string-dispatch caller | Added in `ac311fa8` for per-file safe generation; callers moved to package-coordinated generation in `22f46532` and that pipeline expanded in `cb5ce576` | CLI, generated Go, standard-Go package, and all 44 TinyGo output streams remain byte-identical | `172ee5bc8e011849c3b158129403667c17f6a889` |

The removal deletes one declaration and 18 Go lines. The current path remains:
source selection, package grouping, authored-source reservation,
`GeneratePackageWithOptions`, sibling verification, atomic publication, and
inactive-output cleanup. The public single-file generation APIs remain live.

## Preserved Findings

| ID | Declaration | Tool signal | Final classification | Evidence for preservation |
|---|---|---|---|---|
| F002 | `useState` | U1000 and deadcode in applicable host/browser contexts | `OBSOLETE_BUT_SEPARATE` | Removal preserved all TinyGo streams but changed the representative standard-Go `bundle.wasm` SHA-256 from `b832d74221f7476303ee685fdbb4c2dc1eeb1644d937823261d2859ec0f1f010` to `b819abad55c6e0bc8d62d844f452ae86d23d91497570a003521521d5b885704b`; it was restored |
| F003-F005 | `removeDirectoryIfExists`, `(*devServer).activatePackage`, `fileExists` | U1000 and deadcode in host contexts | `OBSOLETE_BUT_SEPARATE` | Each belongs to a separate cleanup, dev-lifecycle, or filesystem-helper review; this audit does not combine unrelated subsystems |
| F006-F017 | command generation, dev, reload, source-selection, package, and workspace inspection helpers | U1000 only when tests are excluded | `TEST_ONLY_SEAM` | Direct repository test callers establish their current role |
| F018-F020 | stable placement, protected teardown finalization, and virtual range helpers | browser production U1000/deadcode or host production deadcode | `TEST_ONLY_SEAM` | Host tests call them directly or through the non-js teardown bridge |
| F021-F022 | host/debug diagnostic stubs and host-only runtime graph | context-dependent U1000 | `TARGET_OR_TAG_SPECIFIC` | js/wasm implementations and browser call sites are selected in other contexts |
| F023-F024 | exported runtime and GOX helpers | deadcode from limited roots | `EXPORTED_OR_COMPATIBILITY_SURFACE` | Executable-root reachability does not supersede exported API compatibility |
| F025 | protected teardown test/helper selection | W1/W2 test-inclusive type-check failure | `TEST_CONTEXT_MISMATCH` | The js/wasm test selects host-only helper references; no production failure is demonstrated |
| F026 | generated-GOX js/wasm main packages | deadcode load failures before materialization | `EVIDENCE_GAP` | The analyzer cannot type-check generated `App` and props declarations without the goxc generation stage |
| F027 | example and script-fixture declarations | H1/H2 U1000 | `OUT_OF_SCOPE` | These are evidence/application paths and are not cleanup-authorized |

## Test Context Mismatches

| Context | File/package | Failure | Production impact demonstrated | Audit treatment |
|---|---|---|---|---|
| W1 tests | `pkg/goframe/protected_teardown_test.go` | Undefined host-only teardown helpers under js/wasm | No; production W1 type-checks and reports normally | `NOT APPLICABLE`, preserved as F025 |
| W2 tests | `pkg/goframe/protected_teardown_test.go` with `goframe_debug` | Same helper-selection mismatch | No; production W2 type-checks and reports normally | `NOT APPLICABLE`, preserved as F025 |

The mismatch is not repaired in this audit and contributes no removal
evidence.

## False Positives And Indirect Entrypoints

Staticcheck production-only reports include deliberate test seams. Host
analysis reports runtime declarations whose browser call graph is excluded;
browser analysis reports host-tested helpers that are absent from production
WASM roots. `deadcode` also reports exported declarations that its selected
executables do not call. These are context limits, not proofs of obsolescence.

The reference audit checked method expressions, selectors, callback storage,
`js.FuncOf`, registration tables, interfaces, reflection, generated code,
template/string references, build tags, and `//go:linkname`. No hidden caller
was found for F001. Those checks preserve indirect entrypoints elsewhere.

## Evidence Gaps

- Test-inclusive js/wasm Staticcheck and deadcode are not applicable while the
  protected teardown test-context mismatch exists.
- Full js/wasm deadcode cannot load GOX applications before their generated Go
  files are materialized. The two pure-Go roots provide bounded runtime
  evidence, not whole-program coverage for every browser app.
- TinyGo reachability is established by source-selection parity, feature-tagged
  builds, real packaging, Browser Smoke, and byte comparison, not by host
  Staticcheck or `deadcode`.
- Dynamic third-party use of exported APIs is outside closed-world analyzer
  reachability and is preserved.

## Behavior And Output Identity

The base and final code were extracted and built sequentially from the same
absolute path. The CLI comparison covers directory, single-file, and in-place
generation; text and JSON check output; package output; and doctor output.
Exit status, stdout, stderr, generated filenames, generated Go bytes, and
managed package artifacts are byte-identical. Both CLI I/O manifests have
SHA-256
`491236bcad9c764f7bf5dcfb771a0a2a34854d0f1433e7a7c5304515dd3f2801`.

The generated Go hashes are:

- directory output: 409 bytes,
  `c3135e3aa47f59e2464018af8e7d6398fa587991c8a8c748affaea7f60abb973`;
- single-file and in-place output: 329 bytes,
  `9da63d42ff2dccdccfcfa6999527866840da4e15df2818f8673840f4ec3ae540`.

The representative standard-Go package is also byte-identical, including its
2,192,869-byte WASM file with SHA-256
`b832d74221f7476303ee685fdbb4c2dc1eeb1644d937823261d2859ec0f1f010`.
The timestamp-bearing metadata was compared from base and final builds that
completed in the same RFC3339 second.

With Go 1.26.5 and TinyGo 0.41.1, all raw, gzip, Brotli, and Zstandard streams
for `counter`, `components`, `todo`, `dashboard`, `context`, `virtualized`,
`multipackage`, `cmdapp`, `router`, `router-dashboard`, and `resource` are
byte-identical. The 44-stream manifest SHA-256 is
`1fe7d11780482ca77b04bba33f9453d93917456f54d80165f534245f9517428b`.
The unchanged size-budget output has SHA-256
`5a8854eb1e68af0bbb9362244bb3c1b45fab22348442a06be0cc63281db15523`
and every current budget passes.

## Validation

- Focused `cmd/goxc` tests pass on Go 1.22.12, 1.25.12, and 1.26.5.
- The generation/workspace/package/source-selection/check group passes 20
  repetitions; Go 1.25 and 1.26 race runs and the Go 1.26 debug run pass.
- Full ordinary and debug suites plus vet pass on Go 1.22.12, 1.25.12, and
  1.26.5. Required race suites pass on Go 1.25.12 and 1.26.5.
- TinyGo source-selection parity and the feature-tagged TinyGo build pass.
- `scripts/check.sh`, artifact, module-path, documentation, diff, and actionlint
  gates pass.
- Two complete Browser Smoke runs pass with Chrome 149 and the exact Go 1.26.5
  binary. An earlier setup-only attempt used `/usr/bin/go` with
  `GOTOOLCHAIN=go1.26.5` and `GOSUMDB=off`; it failed before an app build while
  Go tried to verify the cached toolchain. Pinning the exact binary removed the
  host-bootstrap ambiguity.
- VS Code extension install and tests pass with Node.js 24.18.1 and npm 11.16.0;
  `package-lock.json` is unchanged.
- Windows/amd64 `cmd/goxc` test compilation passes.
- Final Staticcheck and deadcode reports contain no F001 occurrence and no new
  finding.

## Limitations

This is a bounded snapshot, not a permanent proof that every retained private
declaration is necessary. Closed-world analyzers cannot establish third-party
API use, and the js/wasm limitations above remain explicit. Output identity
applies to the recorded fixtures, package command, and 11 budgeted
applications under the recorded toolchains.

The audit installs no permanent Staticcheck or deadcode CI gate. Raw reports,
inventories, tool binaries, caches, generated packages, browser profiles, and
comparison workspaces remain task-owned temporary evidence and are not
committed.

## Non-Goals

This audit does not redesign APIs, remove exported declarations, repair the
protected teardown test selection, materialize GOX applications for analyzer
convenience, change runtime behavior, alter generated output, rebaseline WASM,
change dependencies or workflows, or clean examples and non-Go code. Findings
classified as `OBSOLETE_BUT_SEPARATE` require their own bounded evidence and
are not authorized for removal here.
