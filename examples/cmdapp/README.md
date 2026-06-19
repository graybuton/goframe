# Child Entry Package Example

This example exercises `goxc` support for an executable entry package below
the application root:

```json
{
  "entry": "./cmd/app"
}
```

The layout follows Go package conventions:

```text
examples/cmdapp/
├── cmd/app/                  # executable entry package
├── internal/ui/              # app-private UI package with GOX files
├── internal/features/tasks/  # app-private feature package with GOX files
└── internal/shared/          # ordinary Go helpers
```

Generated `.gox.go` files stay under `.goframe`; the authored source tree
should remain free of adjacent generated files, `build/`, and `dist/` output.

## Run

```bash
goxc package ./examples/cmdapp --compiler=tinygo
goxc serve ./examples/cmdapp --port=8080
```

For a cache-safe package:

```bash
goxc package ./examples/cmdapp --compiler=tinygo --asset-hash --preload --compress=gzip,br
```

## What It Proves

`goxc` builds the child entry package under `cmd/app` while still discovering
GOX files across the whole app root, including `internal/ui` and
`internal/features/tasks`.

GOX does not add namespace tags such as `<ui.Header />`. Cross-package
composition uses ordinary Go imports and function calls. Local capitalized tags
inside each package still get import-aware component identities, for example:

```text
github.com/graybuton/goframe/examples/cmdapp/cmd/app.Header
github.com/graybuton/goframe/examples/cmdapp/internal/ui.Header
```
