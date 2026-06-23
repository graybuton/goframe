# Router Dashboard Example

This reference example demonstrates how to combine GoFrame's current
public-candidate app primitives without adding a larger framework layer.

It shows:

- Go-first child entry layout with `"entry": "./cmd/app"`;
- `gf.NewHashRouter`, `gf.RouterView`, `gf.RouterLink`, route params, and a
  not-found route;
- URL-driven filters with `gf.RouteContext.Query()` and `gf.WithQuery`;
- controlled form inputs with `gf.InputEvent`;
- synchronous validation with touched/dirty/submitted state;
- a stable shell layout around route content;
- local deterministic data only.

## Run

```bash
goxc package ./examples/router-dashboard --compiler=tinygo
goxc serve ./examples/router-dashboard --port=8080
```

Open <http://127.0.0.1:8080>.

Try:

- `#/issues`
- `#/issues?status=open&q=auth`
- `#/issues/RD-2`
- `#/issues/RD-2/edit`
- `#/missing`

## Notes

This is not a data-loading example. It does not use server calls, async
resources, schema validation, route loaders, middleware, or a global store.
The form validation logic is ordinary application Go code.

The larger `examples/dashboard` remains the DOM pressure test. This example is
the smaller reference app for router + query state + form patterns.

Stateful cross-package GOX components are exposed through small GOX wrapper
functions in their own package. The wrapper renders the local capitalized tag,
so generated code creates a component boundary before other packages call it.
