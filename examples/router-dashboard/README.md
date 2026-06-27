# Router Dashboard

## Purpose

This is the flagship GoFrame reference app for a small Go-first
SPA/dashboard/admin-style application.

It demonstrates how the current primitives fit together without adding a
larger framework layer:

- child entry package with `cmd/app + internal/...`;
- package-qualified GOX components;
- stable shell plus `gf.RouterView`;
- route params and not-found route;
- URL query filters;
- one component-scoped resource owner;
- explicit loading and failed state;
- manual resource reload;
- controlled form inputs;
- local validation with touched/dirty/submitted state;
- scoped render Error Boundary.

The larger `examples/dashboard` remains the DOM pressure test. The focused
`examples/resource` app remains the lifecycle/stale-completion resource probe.

## Run

```bash
goxc package ./examples/router-dashboard --compiler=tinygo
goxc serve ./examples/router-dashboard --port=8080
```

Open <http://127.0.0.1:8080>.

TinyGo is the normal path for the router, resource, query, and form scenarios.
The intentional panic recovery demo requires a recover-capable Go/WASM build:

```bash
goxc package ./examples/router-dashboard --compiler=go
goxc serve ./examples/router-dashboard --port=8080
```

Try with either build:

- `#/`
- `#/issues`
- `#/issues?status=open&q=auth`
- `#/issues/RD-2`
- `#/issues/RD-2/edit`
- `#/missing`

Try only with the Go/WASM build:

- `#/issues/RD-2?panic=render`

## What It Demonstrates

- `gf.NewHashRouter`, `gf.RouterView`, `gf.RouterLink`, params, and not-found
  routing.
- `gf.RouteContext.Query()` plus `gf.WithQuery` for small URL-driven filters.
- `gf.UseResource` as an app-local data owner, not a route loader or global
  cache.
- Browser fetch of a packaged local text asset.
- Form state with `gf.UseReducer`, `gf.InputEvent`, and `gf.Event`.
- Synchronous validation in ordinary Go.
- Render-only Error Boundary behavior separate from resource failure UI.

## Project Structure

```text
examples/router-dashboard/
├── goframe.json
├── assets/
│   ├── index.html
│   ├── styles.css
│   └── data/
│       └── issues.txt
├── cmd/app/
│   ├── app.gox
│   ├── main.go
│   └── model.go
└── internal/
    ├── data/
    ├── filters/
    ├── forms/
    ├── layout/
    └── pages/
```

Source `assets/data/issues.txt` is copied into the packaged app at the same
logical path and fetched from the same origin.

## Data Flow

```text
packaged assets/data/issues.txt
  -> example-local browser fetch loader
  -> data.ParseIssues
  -> data.IssueProvider
  -> app-local context
  -> route pages
  -> query filters / detail / edit form
```

There is one stable resource owner:

```text
App
  -> data.IssueProvider
    -> layout.Shell
      -> gf.RouterView(router)
```

The provider stays mounted across route changes, query changes, browser
back/forward, and form edits. Navigation does not start another loader.
Manual reload starts a new resource generation.

## Routing

Routes are declared in `cmd/app/app.gox`:

- `/`
- `/issues`
- `/issues/:id`
- `/issues/:id/edit`
- not found

Route content is wrapped in a render Error Boundary. The shell remains outside
the route subtree.

## Query State

The issues list reads `q` and `status` from the hash query:

```text
#/issues?status=open&q=auth
```

Filter controls update the URL explicitly with `gf.Navigate(gf.WithQuery(...))`.
This example does not add automatic query binding.

## Resource Ownership

The resource owner uses `gf.UseResource` with one stable data key. The UI
shows:

- `rd-resource-status`;
- `rd-resource-attempt`;
- `rd-resource-reload`;
- explicit loading panel;
- explicit failed panel with retry.

Resource failure is normal app state. It does not activate the Error Boundary.

## Forms And Validation

`internal/forms` keeps field state local to the form:

- value;
- error;
- touched;
- dirty;
- submitted;
- saved.

Save is local-only. It does not persist to a server and does not introduce a
mutation framework.

## Error Handling

Route render failures are handled by `gf.ErrorBoundary`. Ordinary resource
failure renders `rd-resource-failed` instead.

The detail route has an intentional render-failure trigger for smoke coverage:
`#/issues/RD-2?panic=render`. The boundary fallback exposes two actions:

- Retry current route: calls the boundary reset callback and rerenders the same
  protected subtree with the same URL state. If `panic=render` is still present,
  the route can fail again.
- Back to issues: navigates to `/issues`, removing the crashing query input and
  proving the shell and resource owner stay alive.

This demo depends on recover-based render containment. Use
`goxc package ./examples/router-dashboard --compiler=go` for it. The
size-oriented TinyGo build path does not guarantee recover-based containment
and may trap instead of rendering the fallback.

This app does not install route loaders, route-level automatic boundaries, or
server error pages.

## Tests

Focused checks:

```bash
go test ./examples/router-dashboard/...
node --check scripts/router-dashboard-browser-smoke.mjs
goxc package ./examples/router-dashboard --compiler=tinygo
```

The browser smoke verifies:

- shell identity remains stable;
- initial resource load starts once;
- navigation and query changes do not reload data;
- manual reload starts exactly one new generation;
- resource failure stays outside Error Boundary fallback;
- route render failure shows boundary fallback and safe navigation restores the
  issues list without reloading data;
- detail/edit/form/not-found routes still work after reload.

## Non-goals

This example intentionally does not provide:

- external network calls;
- route loaders;
- global resource cache;
- request deduplication;
- Suspense;
- server resources or server functions;
- schema validation framework;
- persistence or mutation framework;
- auth, middleware, or route guards;
- production deployment server.
