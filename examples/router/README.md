# Router Example

This example demonstrates the MVP 24 hash router with a Go-first child entry
package layout.

It shows:

- `"entry": "./cmd/app"`;
- a stable shell layout;
- `gf.NewHashRouter`;
- `gf.RouterView`;
- `gf.RouterLink`;
- route params;
- a not-found route;
- programmatic navigation with `gf.Navigate`.

## Run

```bash
goxc package ./examples/router --compiler=tinygo
goxc serve ./examples/router --port=8080
```

Open <http://127.0.0.1:8080>.

Try:

- `#/`
- `#/issues`
- `#/issues/1`
- `#/missing`

## Notes

MVP 24 routing is hash-based. It works on static hosting because route changes
stay after `#` and do not require a server rewrite rule.

There is no file-based routing, route loader system, route-level error
boundary, or history-mode server fallback in this MVP. Layout composition is
the Go-first pattern used here: a stable shell plus `gf.RouterView(router)` as
the outlet.
