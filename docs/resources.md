# Resources

## Purpose

Resources are a minimal component-scoped loading primitive for asynchronous
work. They model a single explicit state machine:

- loading;
- ready;
- failed.

They do not add Suspense, async rendering, route loaders, server resources, a
global cache, request deduplication, retries, polling, JSON loading, or a data
framework. The runtime stays transport-light: `gf.FetchText` covers the narrow
browser/WASM text-fetch case, while custom loaders may use timers, local
storage, host bridges, or other callback sources.

## API

```go
type ResourceStatus uint8

const (
    ResourceLoading ResourceStatus = iota
    ResourceReady
    ResourceFailed
)

type Resource[T any] struct {
    Status ResourceStatus
    Value  T
    Err    error
}

func (resource Resource[T]) Loading() bool
func (resource Resource[T]) Ready() bool
func (resource Resource[T]) Failed() bool

type ResourceLoader[T any] func(
    key string,
    resolve func(T),
    reject func(error),
) Cleanup

func UseResource[T any](
    key string,
    loader ResourceLoader[T],
) (Resource[T], func())

func FetchText(
    key string,
    resolve func(string),
    reject func(error),
) Cleanup
```

The returned function requests a manual reload for the current component
instance. Calling an old reload closure after later renders uses the latest
component resource state. Calling it after unmount is a no-op.

`key` is a string and may be empty. A nil loader is a runtime invariant panic.

## Browser Text Loader

`gf.FetchText` is an experimental browser/WASM helper for the
`ResourceLoader[string]` shape. It uses browser `fetch` for the provided key,
reads successful responses with `response.text()`, and resolves the text.

The cleanup returned by `FetchText` aborts the in-flight browser request with
`AbortController` and prevents later promise callbacks from resolving or
rejecting the resource generation.

Non-OK HTTP responses reject with an ordinary error whose text includes the
HTTP status code. Fetch/network failures also reject with an ordinary error.
The helper does not expose response bodies for non-OK responses.

Host builds compile with a stub. The stub rejects with a clear error that
`FetchText` is available only in browser/WASM builds and returns a safe no-op
cleanup.

`FetchText` is intentionally text-only. It does not provide JSON parsing,
request caching, retry/backoff, deduplication, route loaders, server APIs,
auth/session behavior, SSR, hydration, or production server behavior.

## Status Model

The first render returns:

- `Status: ResourceLoading`;
- zero `Value`;
- nil `Err`.

The loader has not run yet during that render. It starts after the DOM patch
through the existing effect lifecycle.

A valid `resolve(value)` transitions the current generation to:

- `Status: ResourceReady`;
- `Value: value`;
- nil `Err`.

A valid `reject(err)` transitions the current generation to:

- `Status: ResourceFailed`;
- zero `Value`;
- non-nil `Err`.

If `reject(nil)` is called, the runtime stores a small internal non-nil error
instead of panicking.

For one generation, the first completion wins. Later `resolve` or `reject`
calls are ignored.

## Loader Contract

The loader receives the current key and two completion callbacks. It may return
a cleanup. Nil cleanup is allowed.

Callbacks are intended to be called from the browser/WASM event loop or another
single-threaded host callback path. GoFrame does not add synchronization or a
cross-thread callback model in MVP 28.

If the loader function identity changes while the key and reload generation are
unchanged, the current load is not restarted. The latest loader is used on the
next key change or manual reload.

## Key Changes

Changing the key invalidates the current generation immediately during render,
before the next loader starts. This means late completions from the old key are
ignored before they can update resource state or dirty the component.

The next loader starts after the new render is patched. The previous generation
cleanup runs before that next loader body.

## Manual Reload

Manual reload creates a new generation using the current key. It immediately
invalidates old completion callbacks, switches public state back to loading,
marks the component dirty, and lets the next effect pass start the new loader
after patch.

Reload does not use a global cache or dedupe shared keys. Two components using
the same key have independent resources.

## Cancellation And Cleanup

The cleanup returned by the loader runs:

- before starting a new generation after key change;
- before starting a new generation after manual reload;
- during component unmount.

Each started generation cleanup runs at most once. Before cleanup calls user
code, the generation is marked inactive so callbacks fired by cleanup cannot
complete that generation.

If cleanup panics, the existing effect cleanup containment reports
`gf.ErrorPhaseEffectCleanup`. The resource does not switch status because of
cleanup panic.

## Stale Completion Protection

Every generation has an internal token. Resolve and reject callbacks check that
their token is still the current active generation before touching public
resource state.

Stale callbacks do not:

- change `Resource[T]`;
- dirty the component;
- replace the current value or error;
- start another load;
- report a runtime error.

This guard is immediate. It does not rely on hiding stale state on a later
render.

## Error Semantics

Ordinary loader rejection is application data state. It becomes
`ResourceFailed` and does not activate an Error Boundary.

Loader panic happens while the loader starts inside an effect. In
recover-capable builds, GoFrame records a failed resource state with an
internal error when the generation has not already completed, reports one
`gf.ErrorPhaseEffect`, and treats that resource effect as finished for the
current key. Same-key rerenders do not automatically retry a panicking loader.
Retry is explicit: call the returned `reload` function or change the resource
key.

For one generation, the first completion still wins even when the loader later
panics. If the loader calls `resolve(value)` and then panics, the resource
remains `ResourceReady` with that value and the panic is still reported once.
If the loader calls `reject(err)` and then panics, the resource remains
`ResourceFailed` with the original error and the panic is still reported once.
The internal loader-panic error is used only when panic is the first
completion.

No cleanup is registered when the loader panics before returning one. The
resource wrapper handles this panic itself so the generic `UseEffect` panic
semantics are unchanged.

MVP 28 does not add `ErrorPhaseResource`.

## Error Boundaries

`gf.ErrorBoundary` remains render-only. It can catch render failures in UI that
displays a resource, but it does not catch normal `ResourceFailed` state.

Applications should render loading, ready, and failed UI explicitly.

## Synchronous Completion

A loader may call `resolve` or `reject` synchronously during effect setup. The
resource update is still scheduled through normal state/dirty mechanics after
the current DOM patch and effect flush. The first render remains loading.

## Component Lifetime

Resources are owned by the component instance that calls `UseResource`.

Unmount invalidates the active generation, runs its cleanup once, and makes
future completion or reload callbacks no-ops. The runtime does not keep a
global registry of resources.

The implementation is intentionally composed from existing component-scoped
hook machinery: one internal state slot stores a small mutable control object,
and one effect slot starts/cleans generations. No new `componentInstance`
fields are required.

## Reference App Pattern

`examples/resource` is the focused lifecycle example. It covers reload,
failure, stale completion, and cleanup-after-unmount scenarios.

The example composes `gf.FetchText` with local issue parsing, `slow:` key
normalization, delayed completion, and timer cleanup. Browser text transport is
shared through the experimental helper; example-specific parsing and lifecycle
demonstration remain local to the example.

`examples/router-dashboard` shows the integration pattern for a small app:

```text
App
  -> data.IssueProvider
    -> stable shell
      -> RouterView
```

The resource owner sits outside the route subtree, so navigating between list,
detail, edit, not-found routes, query filter changes, and browser back/forward
do not start another loader. Manual reload is explicit and starts a new
generation. Failed resource state renders application UI and does not activate
an Error Boundary.

## TinyGo And Browser Notes

Ordinary loading, resolving, rejecting, cleanup, key changes, manual reload,
and stale completion protection work in Go/WASM and TinyGo/WASM.

Recover-based loader panic containment depends on the build mode. The normal
size-oriented TinyGo path may use trap-style panics, so it should not be
treated as proof that loader panic recovery executes. This matches the existing
runtime error and Error Boundary panic-mode note.

## Non-goals

MVP 28 intentionally does not provide:

- Suspense or thrown promises;
- async components;
- global resource cache;
- request deduplication;
- retry/backoff;
- polling;
- stale-while-revalidate;
- optimistic mutations;
- route loaders;
- server resources or server functions;
- `context.Context` cancellation;
- JSON helpers or a higher-level fetch/data framework in `pkg/goframe`.

## Future Work

Future stages may consider:

- route-level loader composition;
- optional cache/deduplication layers outside the core primitive;
- recover-capable TinyGo package mode;
- richer debug probes for resource generations;
- typed resource helpers for common app patterns;
- integration guidance for external data sources.
