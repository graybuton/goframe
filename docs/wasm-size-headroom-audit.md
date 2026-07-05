# WASM Size Headroom Audit

## Summary

This audit exists because an attempted keyed-reorder LIS optimization was
blocked by the local WASM size gate. The local gate failed even after the
runtime changes were removed, which made the local size result unsuitable for
deciding whether the LIS implementation itself was too large.

Current `main` at `587c06d1345415960cd9100d42fa81df360fc1a0` behaves
differently across toolchains:

- Local Go `1.24.4` plus TinyGo `0.41.1` fails the dashboard raw budget.
- A clean containerized Go `1.22.12` plus TinyGo `0.41.1` reproduction passes
  the full size-budget workflow.

The dashboard result is still tight under the CI-like toolchain:

- Local current toolchain: `169789 B / 168960 B`, over by `829 B`.
- CI-like Go `1.22.12`: `168907 B / 168960 B`, under by `53 B`.

Top recommendation: keep budgets unchanged, keep CI Go `1.22.x` as the release
size source of truth for now, and recover at least 1-4 KB of dashboard raw
headroom before retrying LIS or updating the size workflow to a newer Go
toolchain.

## Source of Truth

- Workflow: `.github/workflows/ci-wasm-size.yml`
- Budget script: `scripts/size-budget.sh`
- CI Go version: `1.22.x`
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
| dashboard | 168960 B | 53248 B | 71680 B | 61440 B |
| context | 116736 B | 36864 B | 46080 B | 40960 B |
| virtualized | 124928 B | 40960 B | 49152 B | 44032 B |
| multipackage | 110592 B | 43008 B | 56320 B | 49152 B |
| cmdapp | 110592 B | 43008 B | 56320 B | 49152 B |
| router | 116736 B | 45056 B | 58368 B | 51200 B |
| router-dashboard | 230400 B | 77824 B | 94208 B | 81920 B |
| resource | 153600 B | 57344 B | 67584 B | 61440 B |

## Toolchain Matrix

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

## Recommendation

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
- The audit does not change runtime code, size budgets, workflows, examples, or
  generated artifacts.
