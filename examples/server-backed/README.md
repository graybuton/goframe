# Server-Backed Reference

This example shows a narrow integration pattern:

- a GoFrame browser/WASM app packaged by `goxc`;
- a plain Go `net/http` backend;
- static serving of the packaged standalone app;
- a same-origin `/api/greeting` endpoint;
- a process-local saved-greeting store exposed through same-origin `GET` and
  `POST /api/saved-greeting`;
- hash-routed home and greeting content inside a retained application shell;
- route-owned controlled forms whose active greeting follows the route query;
- browser-side text loading through experimental `gf.FetchText` and
  `gf.UseResource`;
- same-target success reload and failed-request retry through the resource's
  existing `reload` function;
- cancellation when a greeting target is superseded or its route unmounts;
- a controlled backend failure and recovery path through later navigation;
- direct hash and browser back/forward navigation through the same router,
  resource, and route-to-form synchronization lifecycle;
- a separate same-pattern transition route that retains its last committed
  target and message while the requested target prepares;
- a route-owned mutation form with client validation, pending and failure
  states, duplicate-submit suppression, and server-confirmed recovery;
- committed-state confirmation through the existing `UseResource.reload`
  contract after each successful write.

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
- The `/transition-greeting` route keeps its requested target separate from one
  committed target/message snapshot, so the previous screen can remain visible
  while the next same-pattern query target prepares.
- `gf.FetchText` is a low-level text loader, not a server framework or data
  framework.
- The `/saved-greeting` route keeps the committed value in a read resource and
  keeps draft, pending, and mutation-error state local to the route.
- A successful form-encoded `POST` triggers the read resource's existing
  `reload` closure. The UI does not update the committed value optimistically.
- Client validation and server failure leave the previous committed value
  visible, and a later valid submit clears the mutation error.

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
#/transition-greeting?name=Ada
#/transition-greeting?name=Lin
#/transition-greeting?name=Mia
#/transition-greeting?name=slow
#/transition-greeting?name=fail
#/saved-greeting
```

The `/greeting` pattern stays mounted across query changes, so a new resource
key supersedes the previous generation and the route effect synchronizes the
controlled draft. Navigating to `/` unmounts the greeting route, form, and
resource owner. Both cancellation paths run the cleanup returned by
`gf.FetchText` through `gf.UseResource` ownership.

## Async Navigation Transition Pressure Test

The separate `/transition-greeting?name=...` route measures whether current
primitives can retain a committed screen while a same-pattern query target
prepares. The current hash and requested target drive one `gf.UseResource`
instance. One example-owned `committedGreeting` value holds the displayed name,
route target, message, and ready marker. An effect copies the requested target
and ready resource value into that snapshot through one state setter.

### Proven

Two browser runs demonstrated that:

- a slow same-pattern transition keeps the previous committed target and
  message visible while the hash and requested target already identify `slow`;
- superseding slow work aborts its request, and `gf.UseResource` prevents the
  stale result from committing;
- displayed target and displayed data commit as one observed snapshot, with no
  mixed target/message pair;
- controlled failure and retry remain separate from the committed screen;
- recovery, browser Back, and browser Forward use the same retained-screen and
  paired-commit path;
- unmounting the route aborts active slow work and produces no late route
  activity or commit.

The two runs produced identical behavioral counters:

| Counter | Run 1 | Run 2 |
|---|---:|---:|
| transition requests started | 10 | 10 |
| successful completions | 6 | 6 |
| failed completions | 2 | 2 |
| aborted requests | 2 | 2 |
| transition commits | 6 | 6 |
| retry attempts | 1 | 1 |
| stale-result appearances | 0 | 0 |
| invalid committed pairs | 0 | 0 |
| old-screen losses after initial commit | 0 | 0 |

### Measured Coordination

| Application-owned concern | Count |
|---|---:|
| state slots | 2 |
| effects | 2 |
| `UseResource` instances | 1 |
| reload handoff points | 1 |
| manual generation counters | 0 |
| manual attempt IDs | 0 |
| manual stale-result guards | 0 |
| manual cleanup callbacks | 0 |
| app-owned `AbortController` instances | 0 |
| example-local helper types/functions | 2 |
| route/data commit handoff points | 1 |

The transition production change adds 161 Go/GOX lines. The browser harness
adds 907 lines and removes 14, a net increase of 893 lines. These are source
measurements, not API or size-budget thresholds.

### Remaining Limitation

`location.hash` changes before data readiness. The displayed target and data
commit together, but the example does not provide an atomic URL + screen + data
transaction.

The retained-screen evidence covers same-pattern query transitions. The
cross-pattern scenario proves unmount cancellation only; it does not prove
cross-pattern old-screen retention. The evidence does not cover redirects,
blockers, route loaders, a global cache, or prefetch. The existing `/greeting`
route remains the baseline where pending navigation removes the previous ready
message.

### Decision

Result A: Current primitives are sufficient for the demonstrated same-pattern
retained transition flow; no public transition or loader API is selected.

## Mutation Flow

The saved-greeting route composes ordinary route state, one example-local
browser transport helper, and the existing read-resource reload contract:

```text
GET /api/saved-greeting through UseResource and FetchText
-> committed value is ready
-> edit the controlled route-owned draft
-> trim and validate on submit
-> POST form data through the example-local fetch helper
-> keep the previous committed value visible while pending
-> reject a duplicate submit while the POST is active
-> on success, call the read resource's reload closure
-> GET confirms and renders the committed server value
```

Whitespace-only input fails client validation without a request. The exact
value `fail` reaches the backend and returns a controlled non-empty HTTP 500
error without changing committed state. The exact value `slow` holds the POST
for the deterministic backend delay; a second submit during that interval does
not start another POST. A later valid `Grace` submission recovers from failure,
reloads the read resource, and clears the prior error.

The route reports `idle`, `pending`, `validation failed`, `server failed`, and
`success` mutation states independently from the read resource's loading,
failed, and ready states.

## Backend Contract

The server owns a mutex-protected in-memory value initialized to `GoFrame`.
`GET /api/saved-greeting` returns the current value as `text/plain` without
mutating it. `POST /api/saved-greeting` accepts an
`application/x-www-form-urlencoded` `name`, trims surrounding whitespace, and
commits valid values. Empty input returns HTTP 400, `fail` returns HTTP 500,
and neither path changes the store. A canceled `slow` request exits through its
request context before commit. Unsupported methods return HTTP 405 with
`Allow: GET, POST`. Every endpoint response sets `Cache-Control: no-store`.

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
| transition requested target | current `/transition-greeting` route props and resource key |
| transition committed screen | one `TransitionGreetingRoute` state slot containing target and message |
| transition commit handoff | one effect commits a ready resource for the current requested target |
| transition retry | the active resource's returned `reload` function |
| global shell retention | `App`, `ServerBackedShell`, and the route-content container outside the matched route subtree |
| same-pattern form/input retention | pattern-keyed `RouterView` reconciliation retains the greeting form and input across query changes and reloads |
| baseline greeting old-screen retention during pending | not provided; `GreetingRoute` shows loading and removes the previous ready result |
| atomic URL + screen + data commit | not provided; the hash target commits before the resource is ready |
| controlled mutation draft | `SavedGreetingRoute` and its dedicated `gf.UseState` slot |
| client validation | `SavedGreetingRoute` submit handler trims the draft and rejects an empty name |
| mutation pending state | route-owned mutation status plus the active request owner |
| duplicate-submit suppression | the request owner's synchronous `active` guard; the button also reflects pending state |
| POST transport | example-local `postSavedGreeting` browser helper |
| server validation | the saved-greeting HTTP handler |
| committed server state | mutex-protected server store, observed by the route's read resource |
| mutation error | `SavedGreetingRoute` mutation-error state |
| successful commit confirmation | POST success callback followed by a fresh resource GET |
| read-resource reload | `SavedGreetingRoute` calls the closure returned by `gf.UseResource` |
| stale mutation completion protection | the route request owner's mounted/active guard and the transport helper's active guard |
| route/component lifetime | `RouterView` mounts the route; `gf.UseUnmount` cancels its active POST helper |

Example-local coordination now consists of one Home draft slot, one Greeting
draft slot, and four SavedGreetingRoute slots for draft, mutation status,
mutation error, and a stable request-owner pointer. The three routes have one
submit handler each. Greeting keeps one query-to-draft effect and one read
resource; SavedGreetingRoute keeps one read resource and one unmount callback.
Four route functions feed one route table. The write path adds one browser POST
helper and one success-to-reload coordination point.

The application contains:

- manual generation counters: `0`;
- mutation attempt IDs or generation tokens: `0`;
- manual mutation completion guards: two bounded boolean layers, the
  route-owned `mounted`/`active` owner and the transport helper's `active`
  flag;
- app-owned `AbortController` instances: one per active POST, inside the
  example-local helper;
- app-owned mutation cleanup callbacks: one transport cleanup retained by the
  route owner and one `gf.UseUnmount` callback that invokes it;
- mutation lifecycle state: one status slot and one error slot, separate from
  the read resource's loading/error state;
- resource lifecycle effects outside `gf.UseResource`: `0`;
- route/form synchronization effects: `1`;
- successful-mutation reload coordination points: `1`.

## Executable Evidence

`scripts/server-backed-browser-smoke.mjs` installs browser-only instrumentation
before loading the app. It records greeting GETs and their exact abort signals,
saved-state GET causes, completed, failed, and aborted mutation POSTs, active
writes, duplicate submit attempts, committed values, debug-tag render/update
flushes, structural DOM operations, route targets, global shell identity, and
per-scenario form/input identity.
Before a form scenario starts, the harness waits for the state-owning route's
render and patch counts to advance, verifies the controlled value, and observes
two additional stable frames. Production runtime code is not instrumented.

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

Each run then performs ten isolated transition-route fetches: six successful
completions, two controlled failures, and two aborts. Six valid committed
target/message pairs were observed, including Back and Forward. One failed
target was retried. No stale result, invalid pair, or old-screen loss after the
initial commit was observed. The transition route mounted once and unmounted
once, while app, shell, and route-content-container identity remained stable.

Direct, Back, and Forward navigation each prove that route target, resource key,
controlled input, and result refer to the same normalized name. Same-target
success and failure submissions keep the hash unchanged while starting a new
resource generation and exposing loading before the new result.

The flow also demonstrates current semantic boundaries: the URL changes before
data is ready, the previous ready greeting is not retained during pending, and
route plus data do not commit atomically. Those are observations, not behavior
simulated with extra application state.

The same two final runs produced identical mutation request evidence:

- four saved-state GETs started and completed, with zero GET failures: two
  ordinary route loads and two successful-mutation reloads;
- four mutation POSTs: two completed, one controlled failure, and one aborted
  when the saved-greeting route unmounted;
- one duplicate submit attempt during the slow write and exactly one POST in
  that scenario;
- two read-resource reloads, one after each successful mutation;
- zero read-resource reloads after the canceled mutation;
- committed values observed in order: `GoFrame`, `slow`, `Grace`;
- zero stale or contradictory committed-value appearances;
- zero app-root, outer-shell, or route-content-container identity changes;
- the previous committed value remained visible during pending, client
  validation, and controlled server failure.

The browser executed the saved route's unmount-cancellation path while a slow
POST was active. The POST's real `AbortSignal` fired once, active mutation work
returned to zero, the route received no late render or patch, and a direct
backend GET remained `Grace` after the slow delay. Returning to
`/saved-greeting` performed an ordinary route-load GET, not a mutation reload,
and loaded `Grace` without appending a false committed version. Scheduling and
DOM-operation totals remain printed observations. The mutation assertions fix
request deltas, visible lifecycle states, committed-value order, reload cause,
abort behavior, and identity invariants rather than browser-specific operation
counts.

Focused backend tests cover the initial GET, a successful trimmed POST and
subsequent GET, validation and controlled failure without state changes, the
405/`Allow` contract, canceled slow work without commit, and concurrent
reads/writes. The server package passes the same coverage under the race
detector.

## Size Evidence

The frozen mutation baseline and this branch were packaged with TinyGo `0.41.1`
using `--compiler=tinygo --asset-hash --preload --compress=gzip,br`. The WASM
entrypoint was resolved through `asset-manifest.json` in both cases.

| Artifact | Baseline | Final | Delta |
|---|---:|---:|---:|
| raw WASM | 160,236 B | 178,589 B | +18,353 B (+11.45%) |
| gzip | 69,233 B | 75,669 B | +6,436 B (+9.30%) |
| Brotli | 58,185 B | 62,755 B | +4,570 B (+7.85%) |

This example is not part of the global hard size-budget list. The incremental
cost covers the saved route, mutation state and owner, browser POST transport,
and visible lifecycle branches. It does not authorize a budget increase or a
shared runtime abstraction.

The transition pressure-test branch was also compared with its frozen base
using Go `1.22.12` and TinyGo `0.41.1`. The raw WASM was resolved through
`asset-manifest.json`; compressed measurements use `gzip -n -9`, Brotli quality
11, and Zstandard level 19.

| Artifact | Frozen base | Transition branch | Delta |
|---|---:|---:|---:|
| raw WASM | 180,336 B | 194,393 B | +14,057 B |
| gzip | 76,325 B | 80,475 B | +4,150 B |
| Brotli | 63,513 B | 66,896 B | +3,383 B |
| Zstandard | 67,426 B | 71,131 B | +3,705 B |

These values are informational. No server-backed threshold or repository size
budget changed.

## Evidence Verdict

Verdict: **SUFFICIENT**.

Ordinary component state and handlers, one route-local request owner, one
example-local browser POST helper, and `gf.UseResource.reload` express the
required write lifecycle. Client and server failures preserve committed state,
pending synchronously blocks duplicate writes, browser-proven unmount cleanup
aborts active work without committing or reloading, and a fresh read confirms
each successful commit. The backend tests, two matching browser runs, and the
incremental compressed size show a bounded flow without a second read-resource
lifecycle or cross-route state bridge.

The app does manually own the write-specific pending/error state, one
`AbortController`, and completion guards. In this single workflow those concerns
remain explicit and local rather than repeated enough to justify a private or
public mutation abstraction. This verdict does not select an Action, Mutation,
cache-invalidation, RPC, transition, or loader API, and it does not authorize a
later stage or roadmap change.

## Project Structure

```text
examples/server-backed/
├── goframe.json
├── assets/
│   ├── index.html
│   └── styles.css
└── cmd/
    ├── app/
    │   ├── app.gox             # routes, resources, transition UI, and mutation owner
    │   ├── mutation_js.go      # example-local browser POST transport
    │   ├── transition.go       # committed transition snapshot helper
    │   └── transition_test.go  # pure snapshot handoff coverage
    └── server/
        ├── main.go             # plain Go net/http server
        ├── saved_greeting.go   # synchronized store and endpoint
        └── saved_greeting_test.go
```

## Tests

Focused checks:

```bash
goxc package ./examples/server-backed --compiler=go
go test ./examples/server-backed/...
go test -race ./examples/server-backed/cmd/server
go vet ./examples/server-backed/...
node --experimental-websocket scripts/server-backed-browser-smoke.mjs
```

The browser smoke packages the example, starts the Go backend on a dynamic
localhost port, opens the app through Chrome/CDP, and verifies route-driven
loading, same-target reload/retry, direct and history-driven input
synchronization, exact request aborts, stale-result suppression, controlled
failure and recovery, global shell retention, same-pattern form/input retention,
expected cross-pattern remounts, retained transition screens, paired committed
target/data observations, transition failure/retry/recovery, saved-state
loading, validation without a POST, duplicate-write suppression, failure
preservation, successful reload confirmation, mutation unmount cancellation,
route-load versus reload attribution, final backend/UI consistency, and
update/DOM bridge evidence.

## Non-goals

This example intentionally does not provide:

- a GoFrame server framework;
- production server behavior;
- fullstack/server APIs;
- server functions;
- a public Action or Mutation API;
- optimistic updates or cache invalidation;
- transactions, offline writes, or persistence;
- RPC or a data transport framework;
- SSR or hydration;
- route loaders;
- atomic URL + screen + data transitions;
- auth/session helpers;
- a global resource cache;
- JSON/data framework behavior.
