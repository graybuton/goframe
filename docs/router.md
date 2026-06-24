# Client Router

## Purpose

MVP 24 adds a small client-side router for browser/WASM applications. It is a
hash-based router designed for static hosting, Go-first route declarations, and
stable layout composition.

This is not a full application framework. There is no file-based routing,
server routing, route loader system, async resource model, Suspense-like
behavior, middleware, auth guard, or automatic route-level boundary policy.

## Scope

The router supports:

- hash-based navigation;
- static and parameterized route patterns;
- route params;
- small query helpers for URL-driven route state;
- a not-found route;
- `RouterView`;
- `RouterLink`;
- programmatic `Navigate`;
- browser back/forward through hash changes;
- a stable shell layout pattern.

The router intentionally does not support:

- path/history mode;
- server fallback automation;
- full query-state management;
- route loaders or data fetching;
- nested route/layout DSL;
- scroll restoration;
- code splitting;
- file-system route discovery.

## Hash Routing

Hash routing is the default and only router mode in MVP 24. It works on static
hosting because the browser requests `index.html` once, and route changes happen
inside the URL fragment:

```text
#/
#/issues
#/issues/42
```

The empty hash, `#`, and `#/` normalize to `/`.

`gf.HashHref("/issues")` returns `#/issues`. `gf.Navigate("/issues")` updates
`window.location.hash` in browser builds.

Query strings stay inside the hash target:

```text
#/issues?status=open&q=auth
```

Path/history mode remains future work. It would require server or CDN fallback
to `index.html`, which GoFrame does not configure in this MVP.

## Route Matching

Routes are declared in Go:

```go
var router = gf.NewHashRouter([]gf.Route{
    gf.RoutePath("/", homeRoute),
    gf.RoutePath("/issues", issuesRoute),
    gf.RoutePath("/issues/:id", issueDetailsRoute),
    gf.NotFoundRoute(notFoundRoute),
})
```

Supported patterns:

```text
/
/issues
/issues/:id
/projects/:projectID/issues/:issueID
```

Matching is deterministic and declaration-order based. The first route that
matches wins.

Supported semantics:

- static path segments match exactly;
- `:param` matches one non-empty segment;
- trailing slash is normalized away except for `/`;
- query text after `?` is stored as raw query text and can be parsed with
  `RouteContext.Query()`;
- wildcard, optional, regex, and splat params are not supported.

## Params

Route handlers receive a `RouteContext`:

```go
func issueDetailsRoute(ctx gf.RouteContext) gf.Node {
    return pages.IssueDetails(pages.IssueDetailsProps{
        ID: ctx.Param("id"),
    })
}
```

`RouteContext.Param(name)` returns the matching route param or an empty string.
`RouteContext.RawQuery` contains the query text after `?`.

## Query Helpers

MVP 25 adds a tiny query helper layer:

```go
query := ctx.Query()
status := query.Get("status")

target := gf.WithQuery("/issues", gf.QueryValues{
	"status": {"open"},
	"q":      {"auth"},
})
gf.Navigate(target)
```

API:

```go
type QueryValues map[string][]string

func (ctx RouteContext) Query() QueryValues
func ParseQuery(raw string) QueryValues
func (values QueryValues) Get(name string) string
func (values QueryValues) Has(name string) bool
func (values QueryValues) Encode() string
func WithQuery(path string, values QueryValues) string
```

Semantics:

- `Get` returns the first value or an empty string;
- repeated keys are preserved in `QueryValues`;
- keys without `=` parse as present with an empty first value;
- `WithQuery` replaces any existing query on the path;
- `Encode` orders keys deterministically and preserves per-key value order;
- malformed percent escapes are preserved literally instead of panicking.

This is not a full query-state manager. There are no typed codecs, no automatic
state binding, no route loaders, and no external data fetching story in MVP 25.

## RouterView

`gf.RouterView(router)` is a component boundary that:

- reads the current hash path on first render;
- subscribes to browser `hashchange`;
- updates internal state when the hash changes;
- matches the current route;
- renders the matched route handler;
- cleans up its browser listener when unmounted.

If no route matches and no `NotFoundRoute` exists, `RouterView` renders
`gf.Empty()`.

Route content is keyed by matched route pattern. Moving between different
patterns remounts the route subtree. Moving between the same pattern with
different params updates `RouteContext` and may preserve route-local state.
Applications that want a per-param reset can add their own keyed component
inside the route handler.

## RouterLink

`gf.RouterLink` renders a normal hash link. In GOX, use the package-qualified
component tag:

```gox
<gf.RouterLink To="/issues">Issues</gf.RouterLink>
```

The output is an anchor with `href="#/issues"`. MVP 24 does not intercept
clicks, compute active link classes, or manage focus/scroll restoration.

## Layout Composition

The MVP layout model is a Go-first composition pattern. In multi-package GOX
apps, the shell is usually rendered with a package-qualified component tag:

```gox
<layout.Shell>
	{gf.RouterView(router)}
</layout.Shell>
```

The shell is an ordinary component that stays mounted while `RouterView`
changes the route content inside it. This gives applications a stable layout
without a nested route DSL.

## Browser Back/Forward

Browser back and forward work through the native hash history stack. Since
`RouterView` listens for `hashchange`, user navigation, `RouterLink`, and
`gf.Navigate` all converge on the same update path.

## Error Semantics

Route handlers run as render work. If a route handler panics, MVP 23 runtime
error containment reports it as a render error and uses the normal render
fallback behavior.

MVP 27 adds scoped render-only `gf.ErrorBoundary`. The router does not install
one automatically, but route handlers can wrap page content explicitly:

```go
func issuesRoute(ctx gf.RouteContext) gf.Node {
	return gf.ErrorBoundary(gf.ErrorBoundaryProps{
		ResetKey: ctx.Path,
		Fallback: routeFallback,
		Children: []gf.Node{
			pages.Issues(pages.IssuesProps{}),
		},
	})
}
```

This keeps route matching separate from application error UI policy. Automatic
route-level error elements, route loaders, async resources, and Suspense-like
behavior remain future work.

## Deployment Notes

Hash routing works with static package output because the server always serves
`index.html` for the initial request. The route lives after `#`, so it is not
sent as a separate server path.

Path/history mode would require server or CDN fallback rules. `goxc serve` is
development-only and does not implement a production fallback policy.

## Limitations

- Hash routing only.
- No file-based routes.
- No XML-style namespace tags with `:` and no arbitrary selector-chain GOX
  tags beyond `packageAlias.Component`.
- No route loaders or async resources.
- No automatic route-level Error Boundary installation.
- No middleware or auth guards.
- No scroll restoration.
- No active link helper.
- No typed query-state manager.
- No route ranking beyond declaration order.

## Future Work

Potential future router work should be separate from this MVP:

- path/history mode with documented deployment fallback;
- route-level error UI helpers if repeated application patterns justify them;
- loader/resource integration;
- nested route layout DSL if Go-first composition proves insufficient;
- active link helpers;
- scroll restoration;
- route-aware code splitting.
