# GoFrame Tutorial

## What You Will Build

This tutorial follows the reference app in `examples/router-dashboard`: a small
Go-first browser/WASM dashboard with a stable shell, hash routes, URL query
filters, a component-scoped resource owner, explicit loading and failed states,
controlled form validation, and a scoped render Error Boundary.

The goal is not to introduce a new app framework layer. The app uses the
existing runtime primitives directly:

- GOX package-qualified components;
- `gf.NewHashRouter`, `gf.RouterView`, and `gf.RouterLink`;
- `gf.RouteContext.Query()` and `gf.WithQuery`;
- `gf.UseResource` for one component-scoped data load;
- `gf.UseReducer` for form state;
- `gf.ErrorBoundary` for render failures only.

## Prerequisites

Install Go, TinyGo, Node.js, Chrome or Chromium, gzip, brotli, and optionally
zstd. The repository CI currently uses Go `1.24.x`, TinyGo `0.41.1`, and
Node.js 20.

## Install goxc

For a published install:

```bash
go install github.com/graybuton/goframe/cmd/goxc@latest
```

Inside this repository, install the local checkout:

```bash
go install ./cmd/goxc
goxc doctor
```

## Run The Reference App

Package and serve the reference dashboard:

```bash
goxc package ./examples/router-dashboard --compiler=tinygo
goxc serve ./examples/router-dashboard --port=8080
```

Open <http://127.0.0.1:8080>.

Try these routes:

- `#/`
- `#/issues`
- `#/issues?status=open&q=auth`
- `#/issues/RD-2`
- `#/issues/RD-2/edit`
- `#/missing`

## Project Layout

The reference app uses the recommended Go-first child-entry layout:

```text
examples/router-dashboard/
├── goframe.json
├── index.html
├── styles.css
├── data/
│   └── issues.txt
├── cmd/app/
│   ├── main.go
│   └── app.gox
└── internal/
    ├── data/
    ├── filters/
    ├── forms/
    ├── layout/
    └── pages/
```

`cmd/app` is the executable entry package. `internal/...` packages keep UI,
data parsing/loading, forms, filters, and pages private to the app.

`goxc` generates `.gox.go` files under `.goframe`, not next to source files.

## GOX Components

GOX lowercase tags create DOM elements:

```gox
<section class="rd-card">
    <h2>Issues</h2>
</section>
```

Capitalized tags create component boundaries. Cross-package components use
ordinary Go imports and package-qualified GOX tags:

```gox
import layout "github.com/graybuton/goframe/examples/router-dashboard/internal/layout"

func App() gf.Node {
    return (
        <layout.Shell>
            {gf.RouterView(router)}
        </layout.Shell>
    )
}
```

The supported package-qualified form is `packageAlias.Component`. There are no
namespace tags, spread props, or file-based route components.

## Stable App Shell

The app keeps layout outside route content:

```text
App
  -> data.IssueProvider
    -> layout.Shell
      -> gf.RouterView(router)
```

The shell stays mounted while routes, query filters, forms, and data reloads
change the outlet. This is the recommended layout model for the current hash
router.

## Router And Route Params

Routes are declared in Go:

```go
var router = gf.NewHashRouter([]gf.Route{
    gf.RoutePath("/", homeRoute),
    gf.RoutePath("/issues", issuesRoute),
    gf.RoutePath("/issues/:id/edit", issueEditRoute),
    gf.RoutePath("/issues/:id", issueDetailsRoute),
    gf.NotFoundRoute(notFoundRoute),
})
```

Route params come from `gf.RouteContext`:

```go
id := ctx.Param("id")
```

The router is hash-based. `#/issues/RD-2` works on static hosting because the
server still serves `index.html`.

## URL Query State

The issues page reads query state from the route:

```go
query := ctx.Query()
filter := data.IssueFilter{
    Query:  query.Get("q"),
    Status: query.Get("status"),
}
```

Filter controls update the URL explicitly:

```go
gf.Navigate(gf.WithQuery("/issues", gf.QueryValues{
    "q":      {query},
    "status": {status},
}))
```

There is no automatic query-state manager. The URL is just ordinary app state
that components read and write deliberately.

## Component-Scoped Resource

The reference app has one stable resource owner. It loads the packaged
`data/issues.txt` asset once and distributes the explicit resource state
through app-local context.

Important properties:

- initial app load starts one generation;
- route navigation does not reload data;
- query changes do not reload data;
- browser back/forward does not reload data;
- manual reload starts a new generation;
- failed state is rendered explicitly;
- there is no global cache or route loader.

The data loader is example-local browser code. GoFrame does not provide a
runtime fetch API.

## Loading And Failure UI

Resource state is normal UI state:

- loading shows a small loading panel while the shell remains mounted;
- ready pages render list, detail, and edit views from the loaded data;
- failed shows an error panel plus retry control.

Ordinary resource failure is not a render failure and does not activate an
Error Boundary.

## Forms And Validation

The edit page uses controlled inputs and a reducer:

- `Value={state.Title.Value}`;
- `OnInput={func(event gf.InputEvent) { ... }}`;
- `OnSubmit={func(event gf.Event) { event.PreventDefault(); ... }}`;
- local touched/dirty/submitted flags;
- synchronous validation functions in the app package.

There is no schema validation framework, server mutation API, optimistic
mutation layer, or persistence story in this tutorial.

## Error Boundaries

Route content is wrapped in a scoped render Error Boundary. It protects route
rendering from render panics and keeps the outer shell alive.

The reference app includes an intentional route-render failure for integration
testing:

```text
#/issues/RD-2?panic=render
```

That query parameter makes the detail route panic during render. The boundary
shows fallback UI, while the outer shell and component-scoped resource owner
stay mounted. The resource remains `ready`; this is not an ordinary
`ResourceFailed` state and it does not reload the data.

The fallback exposes two different actions:

- retry the current route, which rerenders the same protected subtree with the
  same route/query input;
- navigate safely back to `/issues`, which removes the crashing query input and
  returns the app to a normal route.

Reset does not fix the cause of an error automatically. If the same route props
or query still trigger a panic, fallback UI can appear again.

Error Boundaries do not catch:

- ordinary `ResourceFailed` state;
- event handler panics;
- effect panics;
- async resource failures;
- route loader failures, because route loaders do not exist yet.

## Package And Serve

For local development:

```bash
goxc package ./examples/router-dashboard --compiler=tinygo
goxc serve ./examples/router-dashboard --port=8080
```

For cache-safe deploy-style artifacts:

```bash
goxc package ./examples/router-dashboard \
  --compiler=tinygo \
  --asset-hash \
  --preload \
  --compress=gzip,br
```

Use `goxc export` only when you want a visible deploy directory.

## Where To Go Next

Recommended learning path:

1. `examples/counter`: minimal state and packaging.
2. `examples/components`: GOX components, props, children, and fragments.
3. `examples/router`: focused routing.
4. `examples/resource`: focused resource lifecycle, stale completion, failure,
   and cleanup behavior.
5. `examples/router-dashboard`: integrated reference app.
6. `examples/dashboard`: pressure/performance example.

Focused deep dives:

- `examples/todo`: controlled inputs, effects, keys, and list helpers.
- `examples/context`: scoped providers and selector consumers.
- `examples/virtualized`: bounded DOM for fixed-height lists and tables.
- `examples/multipackage` and `examples/cmdapp`: workspace and layout shapes.

## Current Limitations

GoFrame is experimental and not production-ready. The tutorial intentionally
does not cover or provide:

- SSR or hydration;
- history-mode routing;
- route loaders;
- server resources or server functions;
- global resource cache;
- request deduplication;
- Suspense-style async rendering;
- schema validation;
- mutation framework;
- auth guards or route middleware;
- production deployment server;
- LSP or formatter;
- Player/Engine runtime.
