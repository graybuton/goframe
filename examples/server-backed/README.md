# Server-Backed Reference

This example shows a narrow integration pattern:

- a GoFrame browser/WASM app packaged by `goxc`;
- a plain Go `net/http` backend;
- static serving of the packaged standalone app;
- a same-origin `/api/greeting` endpoint;
- hash-routed home and greeting content inside a retained application shell;
- browser-side text loading through experimental `gf.FetchText` and
  `gf.UseResource`;
- cancellation when a greeting target is superseded or its route unmounts;
- a controlled backend failure and recovery path through later navigation;
- browser back/forward through the same router and resource lifecycle.

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
- The stable shell owns the controlled form and stays mounted while
  `gf.RouterView` switches route content.
- Form submission builds `/greeting?name=...` with `gf.WithQuery` and navigates
  with `gf.Navigate` instead of mutating a local resource key.
- The greeting route decodes `RouteContext.Query()` and owns its request with
  `gf.UseResource`.
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
controlled shell input
→ gf.WithQuery("/greeting", ...)
→ gf.Navigate(...)
→ RouterView observes hashchange
→ RouteContext.Query() derives the name
→ GreetingRoute derives /api/greeting?name=...
→ UseResource starts FetchText
→ loading, failed, or ready route UI
```

Routes exercised by the browser evidence are:

```text
/
#/greeting?name=Ada
#/greeting?name=slow
#/greeting?name=fail
```

The `/greeting` pattern stays mounted across query changes, so a new resource
key supersedes the previous generation. Navigating to `/` unmounts the route
owner. Both paths run the cleanup returned by `gf.FetchText` through
`gf.UseResource` ownership.

## Ownership And Coordination

| Concern | Current owner |
|---|---|
| form input | `ServerBackedShell` and one `gf.UseState` slot |
| URL target construction | the shell submit handler through `gf.WithQuery` |
| hash navigation | `gf.Navigate`; native history for back/forward |
| route matching | `gf.RouterView` and the example route table |
| query decoding | `RouteContext.Query()` |
| resource key derivation | `GreetingRoute` through `greetingPath` |
| request generation | `gf.UseResource` generation state |
| cancellation | `gf.UseResource` cleanup invoking `gf.FetchText` cleanup |
| stale completion suppression | `gf.UseResource` generation checks and `gf.FetchText` active state |
| loading/failed/ready UI | explicit branches in `GreetingRoute` |
| shell retention | `ServerBackedShell` composed outside `gf.RouterView` |
| old-screen retention during pending | not provided; the route shows loading and removes the previous ready result |
| atomic route + data commit | not provided; the hash target commits before the resource is ready |

Example-local coordination consists of one input state slot, one submit
handler, one route table, three small route handlers, and one route-owned
resource hook. Helpers normalize the name, format the route target and request
key, and render resource status/error text.

The application contains:

- manual generation counters: `0`;
- manual stale-result guards: `0`;
- app-owned `AbortController` instances: `0`;
- app-owned cleanup callbacks: `0`;
- duplicated loading/error state variables: `0`;
- custom effects for router/resource coordination: `0`.

## Executable Evidence

`scripts/server-backed-browser-smoke.mjs` installs browser-only instrumentation
before loading the app. It records greeting fetches and their exact abort
signals, debug-tag render/update flushes, structural DOM operations, route
targets, and retained shell nodes. Production runtime code is not
instrumented.

One deterministic run performs eight route-owned fetches:

- four successful `Ada` completions;
- two controlled `fail` completions;
- two active `slow` requests aborted, one on same-pattern supersede and one on
  route unmount;
- zero stale slow-result appearances;
- zero shell, form, input, or route-content identity changes.

Each complete greeting navigation uses two rAF-scheduled update flushes: one
for the route/loading state and one for completion. Starting a pending slow
route and unmounting it each use one update flush. The script prints DOM bridge
operation totals for review but does not make browser-version-dependent totals
part of the product contract.

The flow also demonstrates current semantic boundaries: the URL changes before
data is ready, the previous ready greeting is not retained during pending, and
route plus data do not commit atomically. Those are observations, not behavior
simulated with extra application state.

## Size Evidence

The frozen-base and route-driven versions were packaged with TinyGo `0.41.1`
using `--asset-hash --preload --compress=gzip,br`. The WASM entrypoint was
resolved through `asset-manifest.json` in both cases.

| Artifact | Frozen base | Route-driven flow | Delta |
|---|---:|---:|---:|
| raw WASM | 130,948 B | 156,668 B | +25,720 B (+19.64%) |
| gzip | 60,254 B | 68,269 B | +8,015 B (+13.30%) |
| Brotli | 50,504 B | 57,492 B | +6,988 B (+13.84%) |

This example is not part of the global hard size-budget list. The delta is
evidence for the current router/UI composition and does not authorize a budget
increase or another shared runtime abstraction.

## Evidence Verdict

Verdict: **SUFFICIENT**.

The existing router, ordinary component composition, `gf.UseResource`, and
`gf.FetchText` express route-driven loading, failure, recovery, same-pattern
supersession, unmount cancellation, stale-result suppression, and native
back/forward without duplicating asynchronous lifecycle state in the app. The
coordination is small and has one clear owner per concern.

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
loading, exact request aborts, stale-result suppression, controlled failure and
recovery, native back/forward, retained shell identity, and update/DOM bridge
evidence.

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
