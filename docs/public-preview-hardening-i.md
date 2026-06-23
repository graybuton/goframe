# Public Preview Hardening I

## Purpose

MVP 25 narrows and strengthens GoFrame's public surface before the next wave of
features. It is a product-hardening pass, not a framework expansion.

The goal is to make the current experimental platform easier to evaluate:

- clarify which APIs are meant for users, generated code, or internals;
- add a small router query helper layer for URL-driven state;
- document form and validation patterns without adding a form framework;
- add one reference-grade app that combines router, query filters, forms,
  validation, and the Go-first child-entry layout;
- keep CI, size, smoke, and docs aligned with that surface.

## Strategic Context

The current project direction is:

> GoFrame is an experimental Go-first browser/WASM framework and toolchain for
> SPA-style and dashboard/admin-class applications.

That direction makes router/query/forms polish more useful than jumping
directly to data loading, LSP, history routing, Player/Engine work, or a broad
application convention layer. Small real apps need URL-driven filters and
forms before they need a large app framework.

## Scope

In scope:

- public API surface review;
- router query parsing and URL building helpers;
- form patterns based on existing event, state, and reducer primitives;
- reference example using local deterministic data only;
- browser smoke and size budget coverage for the reference example;
- README and docs alignment.

Out of scope:

- server resources, server functions, or external data fetching;
- async resource framework or Suspense-like behavior;
- history-mode router or server fallback automation;
- route loaders, middleware, auth guards, or route-level Error Boundary API;
- schema validation library or large form registry;
- new GOX syntax, namespace tags, or spread props;
- LSP, formatter, Player/Engine, `.gfapp`, or production deployment server.

## Public API Surface Review

MVP 25 keeps the API small and classifies it more clearly.

Public-candidate core APIs are the primary surface for application authors:
components, hooks, context selectors, fixed-height virtualization, hash router
primitives, runtime error reporting, and browser event facades.

Low-level node builders such as `El`, `Text`, `Fragment`, `Child`, `Key`,
`Props`, `If`, `IfElse`, `Map`, and `MapIndexed` remain exported because GOX
and handwritten low-level code need them. They are documented as
compiler-facing or low-level helpers; most app code should prefer GOX markup.

Internal runtime details, generated workspace layout, smoke harness objects,
and debug probe globals remain intentionally unstable.

## Router Query / Route State

MVP 24 exposed `RouteContext.RawQuery` but left parsing and writing to user
code. MVP 25 adds a tiny query helper layer for common URL-driven state:

- parse `RawQuery` into `QueryValues`;
- read the first value with `Get`;
- check presence with `Has`;
- preserve repeated values through `map[string][]string`;
- build route targets with `WithQuery`.

This is not a query-state manager. There are no typed codecs, no automatic
binding to component state, no route store, and no loader integration.

## Forms and Validation Patterns

MVP 25 does not add a form framework. The recommended model is:

- controlled inputs with `gf.UseReducer` or `gf.UseState`;
- `gf.InputEvent.Value()` for input changes;
- `gf.Event.PreventDefault()` for submit handling;
- example-level field state for value, error, touched, dirty, and submitted
  state;
- validation as ordinary Go functions close to the form.

This keeps the runtime small while documenting a canonical pattern that users
can copy.

## Reference Example

`examples/router-dashboard` is the integrated reference example for this pass.
It demonstrates:

- `entry: "./cmd/app"` and `cmd/app + internal/...`;
- `gf.NewHashRouter`, `gf.RouterView`, `gf.RouterLink`, and params;
- query-driven filters for an issues list;
- controlled edit form with local validation;
- touched/dirty/submit state;
- local deterministic data only;
- not-found route and stable shell layout.

It is intentionally smaller than `examples/dashboard`. The existing dashboard
remains the pressure test; the new example is the user-facing reference app.

## CI / Smoke / Size Gates

The reference example is added to:

- TinyGo package checks;
- size budgets;
- browser smoke;
- docs/example consistency checks;
- CI WASM size workflow.

Smoke gates should verify behavior, not exact timing. The dashboard DOM
pressure audit remains the structural performance gate for large tables.

## Non-goals

MVP 25 does not introduce:

- a global store;
- a schema validation package;
- external data loading;
- async submit/data resource model;
- route loaders or route middleware;
- history-mode routing;
- production server behavior.

## Remaining Risks

- Query helpers intentionally cover a small URL-encoding subset. They are
  suitable for simple route-state strings, not arbitrary web form encoding.
- Forms remain patterns rather than a library. This is deliberate, but users
  who want schema validation still need app-level code.
- Router state remains hash-based only.
- The reference app is local and deterministic; it does not answer the future
  external data/resource story.

## Next Steps

After this pass, likely next work should focus on one of:

- a careful external data/resource model;
- browser support and deployment documentation hardening;
- public-preview compatibility policy;
- route-level error UI after Error Boundaries are designed;
- deeper accessibility review for forms and virtualized tables.
