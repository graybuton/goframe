# Multi-package GOX Workspace

MVP 20 adds the first `goxc` path for applications with more than one Go
package. This is toolchain support, not a router, namespace system, or app
framework convention.

## Supported Layout

The supported MVP 20 layout is an application root with `entry: "."` and any
number of child packages below that root:

```text
app/
├── goframe.json
├── app.gox
├── main.go
├── internal/ui/
│   ├── layout.gox
│   └── model.go
└── internal/features/
    ├── list.gox
    └── model.go
```

Child entry packages such as `entry: "./cmd/app"` are not implemented yet.
`goxc` rejects them with a clear error.

## Workspace Strategy

`goxc` uses a materialized hidden workspace, not a Go build overlay.

For apps inside this repository, the build workspace mirrors the module root:

```text
examples/multipackage/.goframe/work/dev/
├── go.mod
├── pkg/goframe/
└── examples/multipackage/
    ├── app.gox.go
    └── internal/ui/layout.gox.go
```

The authored source tree remains clean. Generated files are written under
`.goframe/gen` for `goxc generate` and under `.goframe/work/<profile>` for
build/package materialization.

This model is intentionally used instead of a Go overlay because TinyGo
support for overlays is not part of the current baseline.

## Import Paths

When `goxc` can find a nearest `go.mod`, it computes package import paths from:

```text
module path + relative package path from module root
```

For example:

```text
module:      github.com/graybuton/goframe
package dir: examples/multipackage/internal/ui
identity:    github.com/graybuton/goframe/examples/multipackage/internal/ui
component:   Header
component id github.com/graybuton/goframe/examples/multipackage/internal/ui.Header
```

If no module path can be determined, generation falls back to the package-name
identity path used by `GenerateNamed`.

## Cross-package Components

GOX does not add namespace tags in MVP 20. This remains unsupported:

```gox
<ui.Header />
```

Use ordinary Go imports and function calls for cross-package composition:

```go
import ui "github.com/example/app/internal/ui"

func App() gf.Node {
    return ui.Layout(ui.LayoutProps{Title: "Demo"})
}
```

Inside each package, local capitalized GOX tags still create component
boundaries with import-aware generated identities. The cross-package function
call itself is not a runtime component boundary unless that package exposes one
through its own local GOX-generated component calls or handwritten
`gf.ComponentT` wrapper.

## Commands

The multi-package example supports:

```bash
goxc generate ./examples/multipackage
goxc build ./examples/multipackage --compiler=go
goxc package ./examples/multipackage --compiler=go
goxc package ./examples/multipackage --compiler=tinygo
goxc size ./examples/multipackage
goxc serve ./examples/multipackage --port=8080
```

## Current Limitations

- `entry: "."` only.
- No namespace tags.
- No full multi-module monorepo story.
- No router/layout convention.
- External module dependencies in app packages are not deeply exercised yet.
  The current path is focused on packages below the app root plus the GoFrame
  runtime dependency.
- Generated component type variable names are internal and not stable API.

## Example

See `examples/multipackage` for a compact app with a root package,
`internal/ui`, `internal/issues`, and ordinary shared Go helpers.
