# Multi-package GOX Example

This example exercises the MVP 20 hidden workspace path for applications with
more than one Go package.

It contains:

- a root app package with `app.gox`;
- `internal/ui` with GOX layout/header components;
- `internal/issues` with GOX list/row components;
- `internal/shared` with ordinary Go helpers.

Generated `.gox.go` files stay under `.goframe`. Authored source directories
should remain free of adjacent generated files, `build/`, and `dist/` output.

## Run

```bash
goxc package ./examples/multipackage --compiler=tinygo
goxc serve ./examples/multipackage --port=8080
```

For a cache-safe package:

```bash
goxc package ./examples/multipackage --compiler=tinygo --asset-hash --preload --compress=gzip,br
```

## What It Proves

The root package imports internal packages through normal Go imports. GOX does
not add namespace tags such as `<ui.Header />`; cross-package composition uses
ordinary Go function calls.

GOX files inside each package still use local capitalized tags. Generated
component identity uses the package import path when `goxc` can determine it,
for example:

```text
github.com/graybuton/goframe/examples/multipackage/internal/ui.Header
```

`entry: "."` remains supported for apps with packages under the app root. For
a child-entry layout such as `entry: "./cmd/app"`, see `examples/cmdapp`.
