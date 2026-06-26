# Platform Support

## Purpose

This document records what GoFrame currently tests, what is expected but not
verified, and what remains unsupported. It is a public-preview readiness matrix,
not a production support promise.

The current strongest evidence is Linux plus Chrome/Chromium. macOS currently
has lightweight Intel-runner CI evidence for core Go/toolchain behavior, and
Windows has lightweight CI evidence for the same layer. Firefox and Safari are
not rejected platforms; they are explicitly deferred until dedicated non-Chrome
validation is added.

Labels:

- CI-tested
- expected
- unverified
- unsupported

## Toolchain Hosts

| Host | Status | Evidence |
|---|---|---|
| Linux amd64 | CI-tested | GitHub Actions and local validation run Go, TinyGo, Node, Chrome, size, smoke, and DOM pressure gates. |
| macOS | CI-tested (minimal, Intel runner) | Core Go/toolchain gates run in CI on `macos-15-intel`: `go fmt`, `go test ./...`, `go vet`, `go test -tags=goframe_debug`, and selected GOX golden tests. TinyGo/browser smoke remain Linux-first checks. |
| Windows | CI-tested (minimal) | Core Go/toolchain gates run in CI: `go fmt`, `go test ./...`, `go vet`, `go test -tags=goframe_debug`, and selected GOX golden tests. TinyGo/browser smoke remain Linux-first checks. |

Symlink safety tests skip when `os.Symlink` is unavailable or restricted.

## Compilers

| Compiler target | Status | Notes |
|---|---|---|
| Go/WASM | CI-tested for selected smoke fixtures | Used for recover-capable runtime error and Error Boundary scenarios. Larger bundle size is expected. |
| TinyGo/WASM | CI-tested for packaging, size, and most browser smoke | Preferred size-oriented path. Current CI uses TinyGo `0.41.1`. Default package path uses `-panic=trap`. |
| Native Go runtime | CI-tested for pure tests | Pure runtime/compiler/tooling tests run with normal Go. Browser runtime requires `js/wasm`. |

Minimum module declaration is `go 1.22`. Current local/CI baselines use newer Go
toolchains.

## Browser Targets

| Browser | Status | Evidence |
|---|---|---|
| Chrome/Chromium | CI-tested | Browser smoke and dashboard DOM pressure use Chrome/CDP. |
| Firefox | unverified | Runtime uses standard browser APIs but has no CI coverage. |
| Safari/WebKit | unverified | Runtime uses standard browser APIs but has no CI coverage. |
| Non-browser WASM hosts | unsupported | Current runtime assumes browser DOM APIs. |

Required browser APIs:

- WebAssembly;
- DOM node creation and event listeners;
- `requestAnimationFrame`;
- `hashchange`;
- `fetch` for example-local resource transports;
- `AbortController` for resource example cancellation;
- localStorage for the Todo example.

## Runtime Behavior Matrix

| Capability | Go/WASM | TinyGo/WASM | Notes |
|---|---|---|---|
| rendering/state/effects/context | CI-tested | CI-tested | Main browser app path. |
| hash router/query helpers | CI-tested | CI-tested | Router smoke uses TinyGo; reference ErrorBoundary route uses Go. |
| resources | expected | CI-tested | Go toolchain tests cover resource example packages; lifecycle smoke remains Linux/Chrome-hosted TinyGo. |
| runtime panic containment | CI-tested | limited | Recover-based containment is asserted with Go/WASM. TinyGo trap builds may terminate instead. |
| scoped render Error Boundaries | CI-tested | compile-compatible but not containment proof | Use Go/WASM for intentional panic demos. |
| fixed-height virtualization | expected | CI-tested | DOM pressure and virtualized smoke use TinyGo. |

## Deployment

Status: Ready with limitations.

Supported deployment shape:

- static hosting of `goxc package` output;
- hash-based routing, so app routes stay after `#` and do not need server
  rewrites;
- correct `application/wasm` MIME type;
- optional gzip/brotli sidecars from `goxc package --compress=gzip,br`;
- long-cache immutable headers for hashed assets when deployment
  infrastructure supports them.

Unsupported:

- production server;
- TLS/cache/compression negotiation automation;
- history-mode fallback configuration;
- SSR/hydration.
