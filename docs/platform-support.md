# Platform Support

## Purpose

This document records what GoFrame currently tests, what is expected but not
verified, and what remains unsupported. It is a public-preview readiness matrix,
not a production support promise.

The current strongest evidence is Linux plus Chrome/Chromium. macOS currently
has lightweight Intel-runner CI evidence for core Go/toolchain behavior, and
Windows has lightweight CI evidence for the same layer. Firefox and Safari are
not rejected platforms, but they are outside the current preview evidence.
The browser smoke harness is Chrome DevTools Protocol based, so equivalent
non-Chrome behavior is not claimed by the current pre-1.0 preview line.

Labels:

- CI-tested
- expected
- unverified
- unsupported

## Toolchain Hosts

| Host | Status | Evidence |
|---|---|---|
| Linux amd64 | CI-tested | Core runs Go `1.22.12`, `1.25.12`, and `1.26.5`. The supported Go entries run full correctness gates; Go `1.26.5` also owns the TinyGo size and Chrome browser baselines. |
| macOS | CI-tested (minimal, Intel runner) | Core runs Go `1.26.5` on `macos-15-intel`: formatting, ordinary tests, vet, debug-tag tests, and selected GOX golden tests. TinyGo/browser smoke remain Linux-only checks. |
| Windows | CI-tested (minimal) | Core runs Go `1.26.5`: formatting, ordinary tests, vet, debug-tag tests, and selected GOX golden tests. TinyGo/browser smoke remain Linux-only checks. |

Symlink safety tests skip when `os.Symlink` is unavailable or restricted.

## Compilers

| Compiler target | Status | Notes |
|---|---|---|
| Go/WASM | CI-tested for selected smoke fixtures | Current heavy browser baseline uses Go `1.26.5`. It covers recover-capable runtime error and Error Boundary scenarios. Larger bundle size is expected. |
| TinyGo/WASM | CI-tested for packaging, size, and most browser smoke | Current heavy baseline uses Go `1.26.5` with TinyGo `0.41.1`. Focused source-selection parity also runs under Go `1.25.12`. Default package path uses `-panic=trap`. |
| Native Go runtime | CI-tested for pure tests | Pure runtime/compiler/tooling tests run with normal Go. Browser runtime requires `js/wasm`. |

Minimum module declaration is `go 1.22`, with Go `1.22.12` in Core CI as the
minimum-version check. Go `1.25.12` and `1.26.5` are the supported full Core
toolchains. The browser and size workflows use Go `1.26.5`, TinyGo `0.41.1`,
and Node.js `24.18.1`; Go `1.25.12` does not currently have a separate heavy
browser or size lane.

## Browser Targets

| Browser | Status | Evidence |
|---|---|---|
| Chrome/Chromium | CI-tested | Browser smoke and dashboard DOM pressure use Chrome/CDP. |
| Firefox | unverified | Runtime uses standard browser APIs, but current CI has no Firefox smoke or package-load evidence. |
| Safari/WebKit | unverified | Runtime uses standard browser APIs, but current CI has no Safari/WebKit smoke or package-load evidence. |
| Non-browser WASM hosts | unsupported | Current runtime assumes browser DOM APIs. |

Current non-Chrome boundary:

- browser smoke scripts launch Chrome/Chromium and talk to the Chrome DevTools
  Protocol over WebSocket;
- the repository does not currently include a Playwright, WebDriver, Marionette,
  or WebKit automation harness;
- the current pre-1.0 preview line does not claim equivalent Firefox or Safari
  behavior.

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
