# WASM Size Headroom Audit

## Summary

This document began with a toolchain-sensitive dashboard size investigation.
The historical Go `1.24.4` and Go `1.22.12` results remain below for context.

The current supported release-size source of truth is Go `1.26.5` plus TinyGo
`0.41.1` on Linux amd64. The 2026-07-30 migration reproduced the frozen base
twice with isolated workspaces and caches. Both runs produced identical raw,
gzip, Brotli, and Zstandard outputs for all eleven applications. Three measured
cells exceeded their previous limits, so only those cells were aligned to the
new baseline.

The controlled-select correctness correction later added a measured shared raw
runtime cost of `514 B` to `526 B`. The 2026-07-31 closeout reproduced the base
and behavior-correct head twice, evaluated four bounded compactness candidates,
and aligned only four raw ceilings by one KiB. No compressed ceiling, ratio,
runtime behavior, application code, or package behavior changed in that
closeout.

## Source of Truth

- Workflow: `.github/workflows/ci-wasm-size.yml`
- Budget script: `scripts/size-budget.sh`
- CI Go version: `1.26.5`
- CI TinyGo version: `0.41.1`

The workflow installs `goxc`, generates and packages every listed example with
TinyGo, then repeats packaging with `--asset-hash --preload
--compress=gzip,br` before running `scripts/size-budget.sh`.

The budget script does not build examples. It checks existing package artifacts
under `examples/<app>/.goframe/package/standalone`. For each app, it selects
the first path matching:

1. `assets/bundle*.wasm`
2. `main.wasm`

If no match exists, it reports the default missing path
`assets/bundle.wasm`.

### Budgets

| app | raw | br | gzip | zstd |
| --- | ---: | ---: | ---: | ---: |
| counter | 97280 B | 40960 B | 56320 B | 49152 B |
| components | 107520 B | 43008 B | 56320 B | 49152 B |
| todo | 122880 B | 40960 B | 56320 B | 49152 B |
| dashboard | 171008 B | 53248 B | 71680 B | 61440 B |
| context | 117760 B | 36864 B | 46080 B | 40960 B |
| virtualized | 124928 B | 40960 B | 49152 B | 44032 B |
| multipackage | 110592 B | 43008 B | 56320 B | 49152 B |
| cmdapp | 110592 B | 43008 B | 56320 B | 49152 B |
| router | 117760 B | 45056 B | 58368 B | 51200 B |
| router-dashboard | 234496 B | 77824 B | 94208 B | 82944 B |
| resource | 157696 B | 58368 B | 68608 B | 61440 B |

## Supported Toolchain Baseline Migration - 2026-07-30

The frozen base is `ed88c68328ca0c969c816c2191732751f9af10dd`.
Measurements use Go `1.26.5`, TinyGo `0.41.1` with LLVM `20.1.1`, Linux
amd64, gzip `1.13`, Brotli `1.1.0`, and Zstandard `1.5.7`. Each isolated run
installed its own `goxc`, generated and packaged all eleven applications, and
then repeated packaging with `--asset-hash --preload --compress=gzip,br`.

Two frozen-base runs with separate workspaces, Go caches, module caches, TinyGo
caches, and output directories were byte-identical. Repeating the package
sequence on the final workflow and budget state produced the same artifacts.

| app | raw | gzip | br | zstd | raw WASM SHA-256 (base and final) |
| --- | ---: | ---: | ---: | ---: | --- |
| counter | 84536 B | 34007 B | 28303 B | 30571 B | `c503b7d0d8c11a3a11123626f972bb494b6acad1835c014c892d6427931d1d29` |
| components | 90176 B | 35788 B | 29714 B | 31968 B | `c6e2a7076a3a638f15d73ec8bd3e7fa5d9b21ffcaea0c20b38e0b06deeffb0b0` |
| todo | 119423 B | 46237 B | 38222 B | 41269 B | `54419f7df8c972241639453557c481da264ba35b45c96e1e2b8b5b3769379d22` |
| dashboard | 169794 B | 63522 B | 51527 B | 55534 B | `ccf1115134115770a3d2e18b3eb141394090c5e27e972021ef6fc88eec407cd2` |
| context | 116313 B | 43907 B | 35932 B | 38714 B | `74343c753dd9dfb36f3743856069c33d67b53990ee1d0ce0ceda29165015023d` |
| virtualized | 123248 B | 47631 B | 38998 B | 42209 B | `7265158603003d6ef0bb15034e48047cb757066689abac23c8497252d8383930` |
| multipackage | 95340 B | 37501 B | 31020 B | 33510 B | `a908e6fbe0b0efdffe7d50dcb9a606b46d951866794640f10212d9d15aeecdbd` |
| cmdapp | 95358 B | 37516 B | 31121 B | 33494 B | `5ec1dcb73d0e4c85abe9d6549cd8305934914ae034c2fa0dc97798429a5865d9` |
| router | 116602 B | 44598 B | 36780 B | 39614 B | `64008632fe639c1c6851ecfa1517f5294b8feeb715b779b200a8a35e7a7dd5e0` |
| router-dashboard | 233415 B | 93591 B | 77001 B | 82143 B | `24a53d9c4546d3dbb6354cd414048787edf2143b14d0a9ca97167f4c6b091f14` |
| resource | 156239 B | 68172 B | 57464 B | 61009 B | `5075f6dd01b1ee7e78a0c96c576827805fd1f7c059c5aec3cd61426b96c4fcde` |

All base-to-final raw, gzip, Brotli, and Zstandard sizes and SHA-256 values are
unchanged. The three-cell budget decision is:

| app/format | measured | old budget | overage | new budget | headroom |
| --- | ---: | ---: | ---: | ---: | ---: |
| dashboard raw | 169794 B | 168960 B | 834 B | 171008 B | 1214 B |
| router-dashboard zstd | 82143 B | 81920 B | 223 B | 82944 B | 801 B |
| resource br | 57464 B | 57344 B | 120 B | 58368 B | 904 B |

Every other raw and compressed budget remains unchanged. Compression ratio
limits remain gzip `52.00%`, Brotli `38.00%`, and Zstandard `46.00%`.
This evidence-backed migration is the only supersession of the older Go
`1.22.12` toolchain source-of-truth recommendation.

## Controlled Select Correctness Rebaseline — 2026-07-31

The frozen base is `6bcbde2318099f92f940b817a69c2e1f52ad4058`.
The behavior-correct branch head is
`9531d48ae64fd0db966dad4e03c7f67575e6ed27`. Measurements use Go `1.26.5`,
TinyGo `0.41.1`, Linux amd64, gzip `1.14`, Brotli `1.2.0`, and Zstandard
`1.5.7`.

The accepted runtime guarantee restores a controlled single-value select's
current value after option reconciliation. It preserves browser ownership for
uncontrolled selects, stable select DOM identity, and the absence of synthetic
`input` or `change` events. Dedicated Chrome evidence passes under both
standard Go and TinyGo.

Two isolated frozen-base builds and two isolated behavior-correct-head builds
were byte-identical within each ref for raw, gzip, Brotli, and Zstandard
outputs. The resulting raw matrix is:

| app | frozen base | reviewed head | delta | reviewed raw SHA-256 | old raw budget result |
| --- | ---: | ---: | ---: | --- | --- |
| counter | 84536 B | 85052 B | +516 B | `33f72247577f8f2b6f5d6ee889e267d48548236e1cdca0208d9c46efdfbb91b1` | pass |
| components | 90176 B | 90691 B | +515 B | `8a94c41998279827c415d6f4e3fa16c4c74389f43f62d57a1e85a7278858c66c` | pass |
| todo | 119423 B | 119938 B | +515 B | `54b7308666ddb090bf5ae3a93120e1158a7fa218b0f8698ab78a5462b06e1664` | pass |
| dashboard | 169794 B | 170308 B | +514 B | `bc59482b4746f5a9100e8072769289d6e34d99fc57121c7358bce8e44afa2aa9` | pass |
| context | 116313 B | 116828 B | +515 B | `f4d4c19c691aa91a568fa361dc0fd017812fe1cb81c040443611627346168e8a` | fail by 92 B |
| virtualized | 123248 B | 123762 B | +514 B | `1c36299671986e8aef8c2203ccd6a5bb2159a2246677a1189483c3dd0efa5378` | pass |
| multipackage | 95340 B | 95854 B | +514 B | `83f3402aae1b8a0bd358616810e0574e0a6cf02ff3ef197cca47c5f38c78c3f0` | pass |
| cmdapp | 95358 B | 95872 B | +514 B | `f0ae215386be33e6b9f35b4e62a19a32cabca8d11c7aaa9689a0e5e5b66fcbae` | pass |
| router | 116602 B | 117117 B | +515 B | `f49b180f0a523a2ec5e3d5e9cf8152f799c3f8ffbcdf312e73516844c2792978` | fail by 381 B |
| router-dashboard | 233415 B | 233941 B | +526 B | `9dd248b23ebc527037eb53e504e9f90fd5bfa52cbcc94e45ea23943b8ed578d8` | fail by 469 B |
| resource | 156239 B | 156764 B | +525 B | `03f7f32e9c9e422a78b7aa8bf131d8575c4b0a8471be106adc1062d7703a2aa1` | fail by 92 B |

Deterministic compression used gzip `-n -9`, Brotli quality 11, and
Zstandard level 19. Every compressed size remained within its existing ceiling
and every gzip `52.00%`, Brotli `38.00%`, and Zstandard `46.00%` ratio limit
passed.

| app | gzip | br | zstd | result |
| --- | ---: | ---: | ---: | --- |
| counter | 34008 B | 28491 B | 30675 B | pass |
| components | 35899 B | 29739 B | 32074 B | pass |
| todo | 46336 B | 38399 B | 41370 B | pass |
| dashboard | 63874 B | 51504 B | 55609 B | pass |
| context | 44024 B | 35925 B | 38825 B | pass |
| virtualized | 47750 B | 39062 B | 42290 B | pass |
| multipackage | 37642 B | 31123 B | 33610 B | pass |
| cmdapp | 37651 B | 31289 B | 33610 B | pass |
| router | 44739 B | 36877 B | 39773 B | pass |
| router-dashboard | 93747 B | 77105 B | 82271 B | pass |
| resource | 68351 B | 57508 B | 61140 B | pass |

Four bounded private renderer shapes preserved the browser contract but failed
at least one mandatory raw sentinel ceiling:

| candidate | shape | counter | context | router | router-dashboard | resource | rejection |
| --- | --- | ---: | ---: | ---: | ---: | ---: | --- |
| A | return and reuse normalized DOM props | 85013 B | 116789 B | 117087 B | 233877 B | 156700 B | four blockers remained |
| B | mount-only helper plus local patch synchronization | 84813 B | 116590 B | 116892 B | 233689 B | 156513 B | router cells remained over |
| C1 | compact value/presence handoff with local synchronization | 84715 B | 116492 B | 116788 B | 233584 B | 156408 B | router by 52 B; router-dashboard by 112 B |
| C2 | compact handoff plus shared synchronization | 84920 B | 116690 B | 116994 B | 233797 B | 156626 B | router cells remained over |

The decision is to preserve the simpler shared post-children runtime
implementation instead of adopting compiler-shaped Candidate C1 solely for a
smaller binary. The measured correctness cost is accepted through four aligned
one-KiB raw ceiling increases:

| app | measured | old ceiling | new ceiling | headroom |
| --- | ---: | ---: | ---: | ---: |
| context | 116828 B | 116736 B | 117760 B | 932 B |
| router | 117117 B | 116736 B | 117760 B | 643 B |
| router-dashboard | 233941 B | 233472 B | 234496 B | 555 B |
| resource | 156764 B | 156672 B | 157696 B | 932 B |

This rebaseline does not authorize general renderer optimization, compressed
budget changes, ratio changes, workflow changes, or unrelated application
headroom. No compressed ceiling or compression command changed.

## Targeted ErrorBoundary Correctness Rebaseline — 2026-07-28

This targeted rebaseline covers complete mounted-subtree teardown ownership
during protected ErrorBoundary reconciliation. The frozen main base is
`15a0b8fe4f5d3f0da79b57acc10e1fab4e6cbac5`; the PR head before this
correction is `ee68d49296586a42d142b9b4031fcb173f127b2d`. The runtime correction
is `a3463d9d88837a3adb46d30f1d03e1e8c1947d1c`, and the browser evidence is
`481d8ea5945caec08ac57668af49e3dc456e2762`.

Measurements use Go `1.22.12` and TinyGo `0.41.1` on Linux amd64. Compression
uses the unchanged budget-script commands. Head-to-final deltas compare
artifacts with the same `bundle.wasm` basename.

| app/format | PR head size | final size | delta | old budget | new budget | final headroom |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| router-dashboard raw | 230316 B | 232586 B | +2270 B | 230400 B | 233472 B | 886 B |
| router-dashboard gzip | 92577 B | 93271 B | +694 B | 94208 B | 94208 B | 937 B |
| router-dashboard br | 76183 B | 76703 B | +520 B | 77824 B | 77824 B | 1121 B |
| router-dashboard zstd | 81281 B | 81813 B | +532 B | 81920 B | 81920 B | 107 B |
| resource raw | 153200 B | 155484 B | +2284 B | 153600 B | 156672 B | 1188 B |
| resource gzip | 67158 B | 67897 B | +739 B | 67584 B | 68608 B | 711 B |
| resource br | 56578 B | 57182 B | +604 B | 57344 B | 57344 B | 162 B |
| resource zstd | 59968 B | 60748 B | +780 B | 61440 B | 61440 B | 692 B |

The nine non-ErrorBoundary applications (`counter`, `components`, `todo`,
`dashboard`, `context`, `virtualized`, `multipackage`, `cmdapp`, and `router`)
remain byte-identical at the raw WASM SHA-256 boundary; their compressed sizes
are also unchanged. The final compression ratios remain within the existing
limits: gzip `52.00%`, Brotli `38.00%`, and Zstandard `46.00%`.

At that historical stage, the earlier recommendation to keep budgets unchanged
was superseded only for this accepted ErrorBoundary correctness guarantee. Raw
budgets increased by 3 KiB for `router-dashboard` and `resource`. Resource gzip
increased by 1 KiB under the aligned compressed-budget rule; the other
compressed budgets remained unchanged because their final artifacts fit. No
workflow or ratio limit changes, no unrelated application budget changes, and
no general runtime headroom were authorized by this rebaseline.

## Historical Toolchain Matrix

This matrix records the original audit environment. The supported baseline
migration above supersedes it as current CI guidance.

| environment | Go version | TinyGo version | goxc version | reproduction method | status |
| --- | --- | --- | --- | --- | --- |
| local current toolchain | `go1.24.4 linux/amd64` | `0.41.1`, using Go `1.24.4` | `devel` in temp clone; installed host `goxc` reported `v0.2.0-preview.3` | clean temp clone, full workflow package sequence | failed dashboard raw by `829 B` |
| CI-like container | `go1.22.12 linux/amd64` | `0.41.1`, using Go `1.22.12` | `devel` in container clone | Docker `golang:1.22-bookworm`, TinyGo `0.41.1`, full workflow package sequence | passed |

Local Go `1.22.x` binaries were not installed. Docker was available, so the
CI-like reproduction used a self-contained container clone. Host bind mounts
from `/tmp` were not visible inside the Docker daemon namespace, so the
container cloned `https://github.com/graybuton/goframe.git` directly and report
files were copied out with `docker cp`.

## Size Results

### Local Current Toolchain

Toolchain: Go `1.24.4`, TinyGo `0.41.1` using Go `1.24.4`.

| app | selected wasm path | raw size | raw budget | raw delta | gzip | br | zstd | overall |
| --- | --- | ---: | ---: | ---: | --- | --- | --- | --- |
| counter | `examples/counter/.goframe/package/standalone/assets/bundle.26d7a4ef.wasm` | 84710 B | 97280 B | -12570 B | pass | pass | pass | pass |
| components | `examples/components/.goframe/package/standalone/assets/bundle.c61bdba9.wasm` | 90357 B | 107520 B | -17163 B | pass | pass | pass | pass |
| todo | `examples/todo/.goframe/package/standalone/assets/bundle.a278fa54.wasm` | 118568 B | 122880 B | -4312 B | pass | pass | pass | pass |
| dashboard | `examples/dashboard/.goframe/package/standalone/assets/bundle.66835bbd.wasm` | 169789 B | 168960 B | +829 B | pass | pass | pass | fail |
| context | `examples/context/.goframe/package/standalone/assets/bundle.d0e2edab.wasm` | 116505 B | 116736 B | -231 B | pass | pass | pass | pass |
| virtualized | `examples/virtualized/.goframe/package/standalone/assets/bundle.653e65fe.wasm` | 124306 B | 124928 B | -622 B | pass | pass | pass | pass |
| multipackage | `examples/multipackage/.goframe/package/standalone/assets/bundle.146e4abe.wasm` | 95514 B | 110592 B | -15078 B | pass | pass | pass | pass |
| cmdapp | `examples/cmdapp/.goframe/package/standalone/assets/bundle.14826b8b.wasm` | 95540 B | 110592 B | -15052 B | pass | pass | pass | pass |
| router | `examples/router/.goframe/package/standalone/assets/bundle.22ebe49b.wasm` | 115879 B | 116736 B | -857 B | pass | pass | pass | pass |
| router-dashboard | `examples/router-dashboard/.goframe/package/standalone/assets/bundle.2b9c87c5.wasm` | 227447 B | 230400 B | -2953 B | pass | pass | pass | pass |
| resource | `examples/resource/.goframe/package/standalone/assets/bundle.c9c1cb5d.wasm` | 150210 B | 153600 B | -3390 B | pass | pass | pass | pass |

### CI-Like Go 1.22 Toolchain

Toolchain: Go `1.22.12`, TinyGo `0.41.1` using Go `1.22.12`.

| app | selected wasm path | raw size | raw budget | raw delta | gzip | br | zstd | overall |
| --- | --- | ---: | ---: | ---: | --- | --- | --- | --- |
| counter | `examples/counter/.goframe/package/standalone/assets/bundle.313897da.wasm` | 83867 B | 97280 B | -13413 B | pass | pass | pass | pass |
| components | `examples/components/.goframe/package/standalone/assets/bundle.0e720a2b.wasm` | 89507 B | 107520 B | -18013 B | pass | pass | pass | pass |
| todo | `examples/todo/.goframe/package/standalone/assets/bundle.97a210dc.wasm` | 117700 B | 122880 B | -5180 B | pass | pass | pass | pass |
| dashboard | `examples/dashboard/.goframe/package/standalone/assets/bundle.eb4676f0.wasm` | 168907 B | 168960 B | -53 B | pass | pass | pass | pass |
| context | `examples/context/.goframe/package/standalone/assets/bundle.4b92c0e0.wasm` | 115663 B | 116736 B | -1073 B | pass | pass | pass | pass |
| virtualized | `examples/virtualized/.goframe/package/standalone/assets/bundle.431d82c8.wasm` | 123454 B | 124928 B | -1474 B | pass | pass | pass | pass |
| multipackage | `examples/multipackage/.goframe/package/standalone/assets/bundle.b81c6cab.wasm` | 94671 B | 110592 B | -15921 B | pass | pass | pass | pass |
| cmdapp | `examples/cmdapp/.goframe/package/standalone/assets/bundle.dcec4282.wasm` | 94689 B | 110592 B | -15903 B | pass | pass | pass | pass |
| router | `examples/router/.goframe/package/standalone/assets/bundle.324f2b8d.wasm` | 114856 B | 116736 B | -1880 B | pass | pass | pass | pass |
| router-dashboard | `examples/router-dashboard/.goframe/package/standalone/assets/bundle.529f28a9.wasm` | 226171 B | 230400 B | -4229 B | pass | pass | pass | pass |
| resource | `examples/resource/.goframe/package/standalone/assets/bundle.4b41b5b6.wasm` | 148985 B | 153600 B | -4615 B | pass | pass | pass | pass |

## Dashboard Findings

- Current local dashboard raw size under Go `1.24.4` is `169789 B`, which is
  `829 B` over the raw budget.
- A clean temp clone under the same local Go `1.24.4` and TinyGo `0.41.1`
  reproduced the same `169789 B` dashboard raw size. This rules out stale
  ignored artifacts as the primary cause of the local failure.
- The CI-like Go `1.22.12` reproduction produced `168907 B`, which is `53 B`
  under the dashboard raw budget.
- The most likely explanation is Go-version-sensitive TinyGo output, not local
  artifact drift.
- The remaining dashboard raw headroom under the CI-like toolchain is too small
  for runtime experiments. A small production helper can plausibly consume more
  than `53 B` even when it is well scoped.

## Runtime Size Surface

The dashboard example uses `gf.VirtualTable`, state, effects, prop
normalization, component rendering, focus preservation, and event handlers. It
does not appear to use router, resource, fetch, or context APIs directly.

### Event Wrapper Path

- Files: `pkg/goframe/render_js.go`, `pkg/goframe/event.go`
- Why it may affect dashboard: dashboard has click, input, change, and scroll
  handlers. `eventHandler` currently constructs `Event`, `InputEvent`, and
  `ScrollEvent` wrappers before switching on the callback type.
- Risk: medium. Event callback behavior is user-facing and browser-smoke
  sensitive.
- Expected size impact: medium estimate. This is unmeasured.
- Follow-up recommendation: measure a type-switch-first event path that creates
  only the wrapper needed by the actual callback type.

### Virtual Table Helpers

- Files: `pkg/goframe/virtual.go`
- Why it may affect dashboard: dashboard renders its table through
  `gf.VirtualTable`.
- Risk: medium. Virtualization behavior is visible and already covered by tests
  and browser smoke.
- Expected size impact: medium estimate if string/style construction or
  callback recovery paths can be simplified. This is unmeasured.
- Follow-up recommendation: audit `VirtualTable` style/key helpers and render
  callback recovery paths, keeping fixed-row semantics unchanged.

### Runtime Error Recovery Metadata

- Files: `pkg/goframe/component.go`, `pkg/goframe/effects.go`,
  `pkg/goframe/render_js.go`, `pkg/goframe/virtual.go`,
  `pkg/goframe/error_boundary.go`, `pkg/goframe/errors.go`
- Why it may affect dashboard: component render, memo, effect, event, and
  virtual render callbacks include recovered-panic reporting paths and
  operation strings.
- Risk: medium to high. Error reporting and Error Boundary behavior are part of
  the runtime contract.
- Expected size impact: medium estimate. This is unmeasured.
- Follow-up recommendation: measure whether repeated operation strings or
  per-callback recovery wrappers can be compacted without changing
  `ErrorInfo`, Error Boundary, or panic containment behavior.

### Prop Normalization and Primitive Conversion

- Files: `pkg/goframe/props.go`
- Why it may affect dashboard: dashboard emits many DOM props and events.
  `splitProps`, `eventNameForProp`, `normalizeAttributeName`, and `ToString`
  are hot and linked.
- Risk: medium. Previous PRs already characterized prop behavior and
  allocations; behavior must remain exactly compatible.
- Expected size impact: low to medium estimate. This is unmeasured.
- Follow-up recommendation: measure ASCII-specialized normalization and a
  narrower conversion path only if existing tests remain unchanged.

### Focus Preservation

- Files: `pkg/goframe/mount_js.go`
- Why it may affect dashboard: every dirty flush captures and restores focus
  around DOM patches.
- Risk: high. This is visible UX behavior for focused inputs and selection.
- Expected size impact: unknown.
- Follow-up recommendation: do not optimize first. Only revisit with browser
  smoke coverage for focused input and selection behavior.

### Router Query Helpers

- Files: `pkg/goframe/router.go`, `pkg/goframe/router_js.go`
- Why it may affect dashboard: likely not linked into the plain dashboard
  example, but relevant to `router` and `router-dashboard` budgets.
- Risk: medium. Query encoding and matching behavior are public API.
- Expected size impact: low for dashboard, unknown for router examples.
- Follow-up recommendation: keep separate from dashboard headroom work.

### Resource and Fetch Helpers

- Files: `pkg/goframe/resource.go`, `pkg/goframe/fetch_js.go`
- Why it may affect dashboard: likely not linked into dashboard, but relevant
  to `resource` and `server-backed` evidence.
- Risk: medium. Cleanup, stale completion, and abort behavior are behavioral
  contracts.
- Expected size impact: low for dashboard, unknown for resource examples.
- Follow-up recommendation: do not use resource/fetch cleanup to recover
  dashboard headroom.

### Context Selector Topology

- Files: `pkg/goframe/context.go`
- Why it may affect dashboard: dashboard does not appear to use context.
- Risk: high. Provider topology and selector invalidation behavior are subtle.
- Expected size impact: low for dashboard, unknown for context example.
- Follow-up recommendation: not a first dashboard-size candidate.

## Cleanup Candidates

| priority | candidate | files | expected size impact | risk | proposed PR title | validation |
| ---: | --- | --- | --- | --- | --- | --- |
| 1 | Construct only the event wrapper required by the callback type | `pkg/goframe/render_js.go`, `pkg/goframe/event.go` if needed | medium estimate | medium | `perf(wasm): slim event wrapper construction` | `go test ./pkg/goframe`; `go test ./...`; `go vet ./...`; `scripts/size-budget.sh`; `scripts/browser-smoke.sh` |
| 2 | Slim `VirtualTable` helper/style construction without changing fixed-row behavior | `pkg/goframe/virtual.go` | medium estimate | medium | `perf(wasm): reduce virtual table runtime size` | `go test ./pkg/goframe -run 'TestVirtual'`; `go test ./...`; `scripts/size-budget.sh`; `scripts/browser-smoke.sh` |
| 3 | Compact runtime error operation metadata while preserving `ErrorInfo` semantics | `pkg/goframe/component.go`, `pkg/goframe/effects.go`, `pkg/goframe/render_js.go`, `pkg/goframe/virtual.go`, `pkg/goframe/errors.go` | medium estimate | medium/high | `perf(wasm): compact runtime error reporting paths` | error-boundary/error-handling focused tests; `go test ./...`; `scripts/browser-smoke.sh`; `scripts/size-budget.sh` |
| 4 | Measure ASCII-specialized prop event/attribute normalization | `pkg/goframe/props.go` | low/medium estimate | medium | `perf(wasm): simplify prop normalization` | splitProps tests and benchmarks; `go test ./...`; `scripts/size-budget.sh`; `scripts/browser-smoke.sh` |
| 5 | Router query helper size pass for router examples only | `pkg/goframe/router.go` | low for dashboard, unknown elsewhere | medium | `perf(router): audit query helper size` | router tests; router browser smoke; `scripts/size-budget.sh` |

The first follow-up should target dashboard-linked code and should measure size
after each production change. Test-only and benchmark-only changes do not help
the dashboard WASM size.

## Historical Recommendation (Superseded 2026-07-30)

The following recommendation records the earlier Go `1.22.12` source-of-truth
decision. The evidence-backed supported toolchain migration above is the only
supersession of its toolchain source-of-truth guidance; it does not alter the
historical runtime cleanup observations.

- Do not update the size workflow to Go `1.24` yet. The local Go `1.24.4`
  reproduction fails the dashboard raw budget by `829 B`.
- Keep budgets unchanged for now. The CI-like Go `1.22.12` reproduction passes,
  but dashboard has only `53 B` of raw headroom.
- Pause LIS and other runtime feature work that adds production code until at
  least 1-4 KB of dashboard raw headroom is recovered under the CI-like
  toolchain.
- Use Go `1.22.x` plus TinyGo `0.41.1` for local release-size decisions, or
  treat the GitHub WASM Size workflow as authoritative when local Go differs.
- Recommended next PR: `perf(wasm): slim event wrapper construction`.

## Appendix

### Commands Run

Base and validation:

```sh
git fetch origin --tags --prune
git switch main
git pull --ff-only origin main
git status --short
git rev-parse HEAD
git rev-parse origin/main
git rev-parse v0.2.0-preview.3^{commit}
git switch -c audit/wasm-runtime-size-headroom
go test ./pkg/goframe
go test ./...
go vet ./...
git diff --check
```

Source-of-truth inspection:

```sh
sed -n '1,240p' .github/workflows/ci-wasm-size.yml
sed -n '1,260p' scripts/size-budget.sh
```

Local toolchain inspection:

```sh
go version
tinygo version
which go
which tinygo
go env GOOS GOARCH GOVERSION
goxc version || true
which goxc || true
command -v go1.22 || true
command -v go1.22.12 || true
ls "$HOME/sdk" 2>/dev/null || true
command -v docker || true
command -v podman || true
```

Current-toolchain reproduction:

```sh
git clone --no-local /home/jin-wu/solutions/repos/goframe "$tmpdir/goframe-size-go-current"
git checkout 587c06d1345415960cd9100d42fa81df360fc1a0
GOBIN="$tmpbin" go install ./cmd/goxc
goxc generate ./examples/<app>
goxc package ./examples/<app> --compiler=tinygo
goxc package ./examples/<app> --compiler=tinygo --asset-hash --preload --compress=gzip,br
scripts/size-budget.sh | tee /tmp/goframe-size-current-report.txt
```

CI-like Go `1.22` reproduction:

```sh
docker run --name "$name" golang:1.22-bookworm bash -lc '...'
docker cp "$name:/reports/goframe-size-go122-report.txt" /tmp/goframe-size-go122-report.txt
docker cp "$name:/reports/goframe-size-go122-paths.txt" /tmp/goframe-size-go122-paths.txt
docker rm "$name"
```

Runtime surface inspection:

```sh
rg -n "panic\(|fmt\.|reflect\.|recover\(|ErrorInfo|report|debug|runtimeComponentName|strings\.|strconv\.|syscall/js" pkg/goframe
rg -n "func .*\(" pkg/goframe
rg -n "go:build|goframe_debug" pkg/goframe
rg -n "goframe|UseResource|FetchText|Router|Virtual|UseContext|UseEffect|UseState|On[A-Z]|on[A-Z]|Component|ErrorBoundary" examples/dashboard
```

Optional artifact tools checked:

```sh
command -v wasm-objdump || true
command -v wasm-tools || true
command -v twiggy || true
command -v llvm-size || true
command -v wasm2wat || true
```

None were available locally during this audit.

### Temporary Reports

- `/tmp/goframe-size-current-paths.txt`
- `/tmp/goframe-size-current-report.txt`
- `/tmp/goframe-size-go122-paths.txt`
- `/tmp/goframe-size-go122-report.txt`

### Important Output Excerpts

Local current toolchain:

```text
go version go1.24.4 linux/amd64
tinygo version 0.41.1 linux/amd64 (using go version go1.24.4 and LLVM version 20.1.1)
dashboard    raw     169789 B /   168960 B
dashboard    raw over budget by 829 B
```

CI-like Go `1.22` toolchain:

```text
go version go1.22.12 linux/amd64
tinygo version 0.41.1 linux/amd64 (using go version go1.22.12 and LLVM version 20.1.1)
dashboard    raw     168907 B /   168960 B
```

### Known Limitations

- Cleanup candidate size impacts are estimates, not measured results.
- Optional WASM symbol-inspection tools were not installed, so this audit did
  not attribute bytes to individual functions.
- The CI-like reproduction used Docker and cloned from GitHub inside the
  container because host `/tmp` bind mounts were not visible inside the Docker
  daemon namespace.
- The original audit did not change runtime code, size budgets, workflows,
  examples, or generated artifacts. The later supported-toolchain migration
  changes only workflow versions and the three measured budget cells documented
  above.
