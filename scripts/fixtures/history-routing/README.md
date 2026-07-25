# History Routing Deployment Pressure Fixture

This fixture measures the browser ownership and deployment contract required to
use clean History API paths with the current GoFrame primitives. It is evidence
for one bounded application shape, not a production server or a public routing
API proposal.

## Baseline

The existing hash router keeps the route after `#`, so the HTTP server only
needs to serve the package index at `/`. In the baseline browser run, `/`,
`/#/issues/1`, and a refresh of that hash route loaded successfully from the
ordinary static server. A direct request for `/users/42` returned `404`.

Hash routing therefore avoids an HTML navigation fallback, deployment-base
stripping, and deep-path document-base handling.

## Proven

The deterministic Go and browser checks prove the following behavior:

- strict static serving supports a clean-path client push after the root page
  has booted, but a direct load or refresh of that deep path returns `404`;
- bounded fallback serving restores root and subpath direct loads and refreshes;
- push, replace, Back, and Forward synchronize the browser location and the
  rendered route;
- route parameters, query text, percent decoding, trailing-slash normalization,
  and application not-found rendering are deterministic;
- the retained application root does not remount during navigation;
- the router owner adds one `popstate` listener and removes that exact listener
  when unmounted;
- real static files are served as files, including
  `application/wasm` for WASM;
- missing assets, API paths, traversal attempts, and paths outside the
  deployment base remain non-fallback responses;
- the package index is not modified on disk.

The two focused browser runs produced identical counters:

| Counter | Run 1 | Run 2 |
|---|---:|---:|
| push navigations | 4 | 4 |
| replace navigations | 2 | 2 |
| popstate events | 4 | 4 |
| direct deep-link boots | 2 | 2 |
| refresh recoveries | 2 | 2 |
| application not-found renders | 1 | 1 |
| HTML fallback responses | 2 | 2 |
| static file responses | 4 | 4 |
| missing-asset 404 responses | 5 | 5 |
| API 404 responses | 2 | 2 |
| incorrect fallback responses | 0 | 0 |
| router mounts | 1 | 1 |
| router unmounts | 1 | 1 |
| listener additions | 1 | 1 |
| listener removals | 1 | 1 |
| application-root identity changes | 0 | 0 |
| route-target mismatches | 0 | 0 |

## Required Deployment Contract

The demonstrated clean-path deployment needs all of these policies:

1. Explicit HTML navigation fallback for missing extensionless application
   routes and trailing-slash routes.
2. Asset and API exclusions so missing resources remain `404` rather than
   receiving the package index.
3. Static-file precedence and correct content types, including
   `application/wasm`.
4. One normalized deployment base, with fallback and static responses confined
   to that base.
5. A document asset base for deep URLs. This fixture injects one response-only
   `<base href>` element and rejects an index that already contains one.

The fixture server accepts only `GET` and `HEAD`. HTML fallback additionally
requires an HTML `Accept` value. It is a controlled evidence server, not a
production-hardened hosting implementation.

## Measured Browser Coordination

- Fixture application Go/GOX: 343 lines.
- Pure route model: 268 lines.
- State or reducer slots: 3 total, including 1 current-target state slot.
- Effects: 1.
- `popstate` listeners: 1.
- Cleanup callbacks: 1.
- Navigation handoffs: 2, one push helper and one replace helper.
- Route-model operations: 5 exported boundary/match operations and 7 private
  normalization or parsing helpers.
- JavaScript-specific helpers: 6.
- Global mutable route-state variables: 0. The package has one immutable
  component-type token stored at package scope.
- Browser harness: 773 lines.

## Measured Server Coordination

- Fixture server production code: 392 lines.
- Fixture server tests: 252 lines.
- Server modes: 2 (`strict` and `fallback`).
- Named fallback predicates: 4 (`allowsHTMLFallback`, `acceptsHTML`,
  `isAPIPath`, and `looksLikeAssetPath`).
- Explicit fallback exclusion classes: 5 (unsupported methods, unsafe or
  outside-base paths, API paths, asset-shaped paths, and non-HTML requests).
- Index-response transformations: 1 response-only document-base injection.
- Base-path rules: 2 (normalize the leading/trailing slash form, then constrain
  all serving and fallback decisions to that base).

The fixture WASM was packaged with TinyGo 0.41.1 on Linux amd64 using Go 1.24.4.
The sizes are informational and introduce no threshold:

| Encoding | Bytes |
|---|---:|
| raw | 123147 |
| gzip (`-n -9`) | 48428 |
| Brotli (`-q 11`) | 39732 |
| Zstandard (`-19`) | 42996 |

## Limitations

The evidence covers root deployment, `/app/` subpath deployment, static-file
hosting, direct deep links, refresh, push, replace, Back, Forward, query
handling, trailing-slash normalization, application not-found rendering, and
listener teardown.

It does not cover redirects, navigation blockers, route data loading, SSR,
hydration, CDN- or provider-specific rewrite syntax, service workers, or broad
browser compatibility. The fixture server does not establish production
hosting support.

## Decision

**Result A:** An example-local History API adapter and explicit deployment
contract are sufficient for the demonstrated fixture. No public API is selected
yet.

The repeated browser ownership is one current-target state slot, one effect,
one listener with one cleanup, and two explicit push/replace handoffs. The
deployment requirement is more substantial: a bounded HTML navigation fallback
with API and asset exclusions, static-file precedence, correct MIME types,
deployment-base ownership, and deep-URL document-base handling.

Hash routing remains supported. This evidence does not select a public history
router, production server, redirect API, blocker API, or data-loading API.
