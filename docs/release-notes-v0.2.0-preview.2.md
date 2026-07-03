# v0.2.0-preview.2 Release Notes

## Status And Scope

`v0.2.0-preview.2` is an experimental preview for GoFrame's web-first
browser/WASM direction. The validated target remains interactive browser/WASM
applications built with the GoFrame runtime, GOX, and `goxc`.

This preview focuses on server-backed evidence and the smallest browser-side
text loader now present in the runtime: experimental `gf.FetchText`. It does
not claim production readiness, fullstack/server APIs, server functions, SSR,
hydration, route loaders, JSON/data framework behavior, global caching, or a
production server.

Scope preview != scope project. The server-backed work records a narrow
integration pattern: a packaged browser/WASM app can be served by a plain Go
`net/http` backend and can read from a same-origin API endpoint.

## Highlights

- `examples/server-backed` is now a persistent reference fixture for a packaged
  GoFrame browser/WASM app served by a plain Go `net/http` backend.
- The fixture exposes a same-origin `/api/greeting` endpoint and exercises a
  resource/form flow from the browser app.
- Browser smoke covers initial backend data, form-driven updates, controlled
  backend HTTP 500 failure UI, recovery after a valid submission, and delayed
  stale-response no-overwrite behavior.
- Experimental `gf.FetchText` provides a browser/WASM text-only helper that
  fits the `gf.UseResource` / `ResourceLoader[string]` shape.
- `examples/server-backed` uses `gf.FetchText`, and `examples/resource` is now
  the second adoption point for the helper while keeping parsing and lifecycle
  demonstration local to the example.
- The `cmd/goxc` workspace dependency characterization test now keeps the inner
  `go list` baseline robust under `GOFLAGS=-buildvcs=false`.

## Server-Backed Evidence

The server-backed reference fixture demonstrates a narrow integration boundary:

- `goxc package ./examples/server-backed --compiler=go` produces the
  browser/WASM app bundle;
- `examples/server-backed/cmd/server` is a plain Go `net/http` server;
- the backend serves the packaged standalone directory as static files;
- the backend exposes `/api/greeting` on the same origin as the app;
- the browser app renders the backend greeting through `gf.UseResource` and a
  small form flow.

The browser smoke verifies the app through Chrome/CDP after the backend starts:

- initial `GoFrame` data renders from `/api/greeting`;
- submitting `Ada` renders updated backend data;
- submitting `fail` renders the resource failed state for an HTTP 500;
- submitting a valid value after the failure returns the app to ready state;
- submitting `slow` starts a delayed request, then a newer `Ada` submission
  wins and remains visible after the slow delay has passed.

This evidence does not add a GoFrame server framework. The server is ordinary
application code in the example, and `goxc serve` remains development-only.

## Experimental `gf.FetchText`

`gf.FetchText` is an experimental browser/WASM text loader for the
`ResourceLoader[string]` contract:

```go
func FetchText(
    key string,
    resolve func(string),
    reject func(error),
) Cleanup
```

In browser/WASM builds, it:

- calls browser `fetch` with the provided key as the URL;
- uses `AbortController` for cleanup;
- reads successful responses with `response.text()`;
- rejects non-OK HTTP responses with an ordinary error whose text includes the
  status code;
- rejects fetch/network failures with an ordinary error;
- prevents resolve/reject callbacks after cleanup.

Host builds compile with a stub that rejects with a clear ordinary error and
returns a safe no-op cleanup.

`gf.FetchText` is text-only. It does not provide JSON parsing, `UseFetch`,
public `HTTPError`, a browser subpackage, caching, retry/backoff, deduplication,
route loaders, server APIs, auth/session behavior, SSR, hydration, or
production server behavior.

## Resource Example Adoption

`examples/resource` now composes `gf.FetchText` for packaged issue text
transport. The example still owns its app-specific behavior locally:

- issue text parsing remains in `examples/resource/internal/data`;
- `slow:` key normalization remains local to the loader;
- delayed completion and timer cleanup remain local to the example;
- stale completion and cleanup-after-unmount behavior remain covered by browser
  smoke;
- missing asset failures still render explicit resource failed state.

`examples/router-dashboard` was not migrated in this preview. It still uses its
example-local loader.

## Toolchain Test Baseline

The `cmd/goxc` workspace dependency check now preserves any parent `GOFLAGS`
and adds `-mod=mod -buildvcs=false` for the inner `go list -deps .` subprocess.
This keeps the local `GOFLAGS=-buildvcs=false go test ./...` baseline from
failing on VCS stamping inside the nested workspace dependency check.

This is a test robustness fix. It does not change `goxc` workspace generation,
module handling, package behavior, or runtime behavior.

## Compatibility

- Existing `gf.UseResource` semantics did not change.
- Existing custom `ResourceLoader` implementations remain valid.
- `gf.FetchText` is Experimental Frontier, not Public-Candidate.
- `examples/server-backed` is an example/integration fixture, not a server
  framework.
- Existing browser/WASM app workflows remain experimental preview workflows.
- Existing release boundaries for Player/Engine, `.gfapp`, production
  readiness, SSR/hydration, fullstack/server APIs, route loaders, raw external
  `.gox` dependency generation, and broad reusable package ecosystem stability
  still apply.

## Validation

Release-gate validation for this preview should include:

- `git diff --check`;
- `node scripts/docs-check.mjs`;
- `go test ./...`;
- `go vet ./...`;
- `scripts/browser-smoke.sh`;
- `scripts/size-budget.sh`;
- `scripts/artifact-check.sh`;
- `scripts/module-path-check.sh`;
- GitHub Actions Core;
- GitHub Actions Browser Smoke;
- GitHub Actions WASM Size;
- GitHub Actions VS Code Extension.

Browser Smoke is the strongest automated browser evidence and remains
Chrome/Chromium/CDP-based. The WASM Size gate remains TinyGo-oriented for the
size-budgeted examples. The server-backed fixture uses the Go compiler path for
the focused backend integration smoke.

## Non-Goals And Limitations

`v0.2.0-preview.2` does not claim:

- production readiness;
- stable 1.0 API compatibility;
- fullstack/server APIs;
- server functions;
- SSR or hydration;
- route loaders;
- JSON/data framework behavior;
- global resource cache;
- auth/session helpers;
- production server behavior;
- history-mode router or server fallback automation;
- broad reusable package ecosystem stability;
- Player/Engine;
- `.gfapp`.

Firefox, Safari/WebKit, remote module dependency behavior, and broad reusable
component package ecosystem stability remain outside current automated preview
evidence unless separately documented.

## Upgrade Notes From `v0.2.0-preview.1`

No migration is required for existing custom resource loaders.

Applications may opt into `gf.FetchText` for browser/WASM text loading when the
`ResourceLoader[string]` shape fits their app. Keep app-specific URL
construction, parsing, validation, and data shaping local to the application.

If an app replaces a custom loader with `gf.FetchText`, visible error text may
change. Non-OK HTTP responses use ordinary errors such as
`goframe: fetch returned HTTP 500`.

Install the exact tag after publication when exact preview selection matters:

```bash
go install github.com/graybuton/goframe/cmd/goxc@v0.2.0-preview.2
```

`@latest` may depend on Go module proxy and cache timing immediately after tag
publication.

## Links

- [README](../README.md)
- [API stability](api-stability.md)
- [Resources](resources.md)
- [CI and regression gates](ci.md)
- [Platform support](platform-support.md)
- [Server-backed reference example](../examples/server-backed/README.md)
- [Resource example](../examples/resource/README.md)
- [v0.2.0-preview.1 release notes](release-notes-v0.2.0-preview.1.md)
