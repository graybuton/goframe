# Server-Backed Reference

This example shows a narrow integration pattern:

- a GoFrame browser/WASM app packaged by `goxc`;
- a plain Go `net/http` backend;
- static serving of the packaged standalone app;
- a same-origin `/api/greeting` endpoint;
- hash-routed home and greeting content inside a retained application shell;
- route-owned controlled forms whose active greeting follows the route query;
- browser-side text loading through experimental `gf.FetchText` and
  `gf.UseResource`;
- same-target success reload and failed-request retry through the resource's
  existing `reload` function;
- cancellation when a greeting target is superseded or its route unmounts;
- a controlled backend failure and recovery path through later navigation;
- direct hash and browser back/forward navigation through the same router,
  resource, and route-to-form synchronization lifecycle.

It is a reference fixture, not a GoFrame server framework.

## Run

Package the browser app:

```bash
goxc package ./examples/server-backed --compiler=go
```

Run the backend against the packaged output:

```bash
go run ./examples/server-backed/cmd/server \
  --package=./examples/server-backed/.goframe/package/standalone \
  --addr=127.0.0.1:8080
```

Open <http://127.0.0.1:8080>.

## What It Demonstrates

- `goxc package` can produce a browser/WASM bundle that a Go backend serves as
  static files.
- The backend can expose a same-origin API endpoint beside the packaged app.
- The stable outer shell stays mounted while `gf.RouterView` switches
  route-owned forms and content.
- Home and greeting routes own separate controlled drafts. The active greeting
  route query is the source of truth for the greeting form.
- A different-name submit builds `/greeting?name=...` with `gf.WithQuery` and
  navigates with `gf.Navigate`; submitting the current greeting calls the
  `reload` function returned by `gf.UseResource`.
- The greeting route decodes `RouteContext.Query()` and owns its request with
  `gf.UseResource`.
- A dependency-aware route effect synchronizes the greeting draft after direct,
  Back, or Forward query changes while the `/greeting` pattern stays mounted.
- The browser text fetch uses `gf.FetchText`; app-specific URL/key construction
  stays local to the example.
- The app renders the existing `gf.UseResource` failed state for a controlled
  backend error and recovers after later valid navigation.
- A delayed backend response is aborted when superseded or unmounted, and it
  cannot replace the current route result.
- `gf.FetchText` is a low-level text loader, not a server framework or data
  framework.

## Route Flow

The executable flow uses only existing GoFrame primitives:

```text
route-owned controlled input
→ gf.WithQuery("/greeting", ...)
→ gf.Navigate(...)
→ RouterView observes hashchange
→ RouteContext.Query() derives the name
→ GreetingRoute derives /api/greeting?name=...
→ UseResource starts FetchText
→ loading, failed, or ready route UI
```

When the normalized greeting draft already equals the active route name, the
submit path skips navigation and calls `UseResource`'s `reload` closure:

```text
same active name
→ URL remains unchanged
→ reload current resource generation
→ loading
→ ready or failed
```

Routes exercised by the browser evidence are:

```text
/
#/greeting?name=Ada
#/greeting?name=Lin
#/greeting?name=slow
#/greeting?name=fail
```

The `/greeting` pattern stays mounted across query changes, so a new resource
key supersedes the previous generation and the route effect synchronizes the
controlled draft. Navigating to `/` unmounts the greeting route, form, and
resource owner. Both cancellation paths run the cleanup returned by
`gf.FetchText` through `gf.UseResource` ownership.

## Ownership And Coordination

| Concern | Current owner |
|---|---|
| home form draft | `HomeRoute` and its `gf.UseState` slot |
| greeting form draft | `GreetingRoute` and its `gf.UseState` slot |
| active greeting source of truth | normalized `RouteContext.Query()` name passed to `GreetingRoute` |
| route-to-draft synchronization | one `GreetingRoute` effect keyed by the normalized route name |
| URL target construction | each route submit handler through `gf.WithQuery` |
| hash navigation | `gf.Navigate`; native history for back/forward |
| route matching | `gf.RouterView` and the example route table |
| query decoding | `RouteContext.Query()` |
| resource key | `GreetingRoute` through `greetingPath` |
| request generation | `gf.UseResource` generation state |
| same-target reload | the active `GreetingRoute` through `UseResource`'s returned `reload` function |
| cancellation | `gf.UseResource` cleanup invoking `gf.FetchText` cleanup |
| stale completion suppression | `gf.UseResource` generation checks and `gf.FetchText` active state |
| loading/failed/ready UI | explicit branches in `GreetingRoute` |
| global shell retention | `App`, `ServerBackedShell`, and the route-content container outside the matched route subtree |
| same-pattern form/input retention | pattern-keyed `RouterView` reconciliation retains the greeting form and input across query changes and reloads |
| old-screen retention during pending | not provided; the route shows loading and removes the previous ready result |
| atomic route + data commit | not provided; the hash target commits before the resource is ready |

Example-local coordination consists of two mutually exclusive route-owned state
slots, one Home submit handler, one Greeting submit/reload handler, one focused
query-to-draft synchronization effect, one route table, three small route
handlers, and one route-owned resource hook. A shared render helper emits the
form, while small helpers normalize the name, format the route target and
request key, and render resource status/error text.

The application contains:

- manual generation counters: `0`;
- manual stale-result guards: `0`;
- app-owned `AbortController` instances: `0`;
- app-owned cleanup callbacks: `0`;
- duplicated loading/error state variables: `0`;
- resource lifecycle effects outside `gf.UseResource`: `0`;
- route/form synchronization effects: `1`.

## Executable Evidence

`scripts/server-backed-browser-smoke.mjs` installs browser-only instrumentation
before loading the app. It records greeting fetches and their exact abort
signals, debug-tag render/update flushes, structural DOM operations, route
targets, global shell identity, and per-scenario form/input identity. Before a
form scenario starts, the harness waits for the state-owning `HomeRoute` or
`GreetingRoute` render and patch counts to advance, verifies the controlled
value, and observes two additional stable frames. Production runtime code is
not instrumented.

Two deterministic runs each perform eleven route-owned fetches:

- one direct `Lin` completion and five successful `Ada` completions, including
  same-target reload and Forward history;
- three controlled `fail` completions, including same-target retry and Back
  history;
- two active `slow` requests aborted, one on same-pattern supersede and one on
  route unmount;
- zero stale slow-result appearances;
- zero app-root, outer-shell, or route-content-container identity changes;
- retained greeting form/input nodes for every same-pattern query change and
  same-target reload;
- expected form/input remounts when the route pattern changes between `/` and
  `/greeting`.

Direct navigation, different-target form navigation, same-target reload, failure,
retry, recovery, and resource completion were observed with balanced rAF
requests/callbacks and no microtask fallback. Pending slow starts and route
unmount each used one update flush. Back and Forward each used three update
flushes: route/loading, route-to-draft synchronization, and completion. These
counts and DOM bridge totals are printed observations; the assertions require
behavioral outcomes, balanced scheduling, attributable component work, and no
input update leaking into the next scenario rather than fixing incidental DOM
totals as product contracts.

Direct, Back, and Forward navigation each prove that route target, resource key,
controlled input, and result refer to the same normalized name. Same-target
success and failure submissions keep the hash unchanged while starting a new
resource generation and exposing loading before the new result.

The flow also demonstrates current semantic boundaries: the URL changes before
data is ready, the previous ready greeting is not retained during pending, and
route plus data do not commit atomically. Those are observations, not behavior
simulated with extra application state.

## Size Evidence

The frozen-base and route-driven versions were packaged with TinyGo `0.41.1`
using `--asset-hash --preload --compress=gzip,br`. The WASM entrypoint was
resolved through `asset-manifest.json` in both cases.

| Artifact | Frozen base | Reviewed head | Final | Delta from base | Follow-up delta |
|---|---:|---:|---:|---:|---:|
| raw WASM | 130,948 B | 156,668 B | 160,236 B | +29,288 B (+22.37%) | +3,568 B (+2.28%) |
| gzip | 60,254 B | 68,269 B | 69,233 B | +8,979 B (+14.90%) | +964 B (+1.41%) |
| Brotli | 50,504 B | 57,492 B | 58,185 B | +7,681 B (+15.21%) | +693 B (+1.21%) |

This example is not part of the global hard size-budget list. The delta is
evidence for the current router/UI composition. The follow-up cost covers
route-owned forms, query synchronization, and same-target reload evidence; it
does not authorize a budget increase or another shared runtime abstraction.

## Evidence Verdict

Verdict: **SUFFICIENT**.

The existing router, ordinary component composition, `gf.UseResource`, and
`gf.FetchText` express route-driven loading, same-target reload/retry, coherent
URL/input state, failure, recovery, same-pattern supersession, unmount
cancellation, stale-result suppression, direct hash navigation, and native
back/forward without duplicating asynchronous lifecycle state in the app. The
coordination remains small: route/form synchronization adds one ordinary
effect, while request generation, cancellation, and stale suppression keep one
existing resource owner.

This flow does not establish a need for a framework-level transition or loader
API. It also does not prove that old-screen retention or atomic route/data
commit would never be valuable; those stronger semantics were not required by
this reference flow and remain separate design questions.

## Project Structure

```text
examples/server-backed/
├── goframe.json
├── assets/
│   ├── index.html
│   └── styles.css
└── cmd/
    ├── app/     # browser/WASM GoFrame app
    └── server/  # plain Go net/http backend
```

## Tests

Focused checks:

```bash
goxc package ./examples/server-backed --compiler=go
go test ./examples/server-backed/...
node --experimental-websocket scripts/server-backed-browser-smoke.mjs
```

The browser smoke packages the example, starts the Go backend on a dynamic
localhost port, opens the app through Chrome/CDP, and verifies route-driven
loading, same-target reload/retry, direct and history-driven input
synchronization, exact request aborts, stale-result suppression, controlled
failure and recovery, global shell retention, same-pattern form/input retention,
expected cross-pattern remounts, and update/DOM bridge evidence.

## Non-goals

This example intentionally does not provide:

- a GoFrame server framework;
- production server behavior;
- fullstack/server APIs;
- server functions;
- SSR or hydration;
- route loaders;
- auth/session helpers;
- a global resource cache;
- JSON/data framework behavior.
