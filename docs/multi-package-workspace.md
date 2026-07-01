# Multi-package GOX Workspace

MVP 20 added the first `goxc` path for applications with more than one Go
package. MVP 22 extends that path so the executable entry package can live in
a child directory such as `./cmd/app`. MVP 26 adds Go-like package-qualified
component tags, so cross-package component composition can remain declarative.
This is toolchain and language support, not a router, XML namespace system, or
app framework convention.

## Supported Layout

The app-root entry layout remains supported:

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

Child entry packages are also supported when they are relative package
directories inside the app root:

```json
{
  "entry": "./cmd/app"
}
```

The recommended Go-first layout is:

```text
app/
├── goframe.json
├── assets/
│   ├── index.html
│   └── styles.css
├── cmd/app/
│   ├── main.go
│   └── app.gox
└── internal/
    ├── ui/
    │   └── layout.gox
    └── features/
        └── tasks/list.gox
```

Generic child entry paths such as `./app` or `./src/app` work as ordinary Go
package directories. Frontend-flavored layouts such as `src/app` are allowed,
but the Go-first recommendation is `cmd/app` plus `internal/...`.

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

For a child entry app, the workspace still mirrors the app root and then builds
the selected child package:

```text
examples/cmdapp/.goframe/work/dev/
├── go.mod
├── pkg/goframe/
└── examples/cmdapp/
    ├── cmd/app/app.gox.go
    └── internal/ui/layout.gox.go
```

The build package is `examples/cmdapp/.goframe/work/dev/examples/cmdapp/cmd/app`.
GOX discovery remains app-root-wide so imported internal packages get their
generated files too.

When an app imports ordinary Go packages from external modules, `goxc`
preserves the nearest app module's `require` and `replace` directives in the
materialized workspace. Relative local `replace` targets are rewritten so they
still point at the original target after `go.mod` is written under
`.goframe/work/<profile>`. This supports normal Go package resolution for
external Go component packages declared by the app module.

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

For a child entry package:

```text
package dir: examples/cmdapp/cmd/app
component id github.com/graybuton/goframe/examples/cmdapp/cmd/app.Header

package dir: examples/cmdapp/internal/ui
component id github.com/graybuton/goframe/examples/cmdapp/internal/ui.Header
```

If no module path can be determined, generation falls back to the package-name
identity path used by `GenerateNamed`.

## Diagnostics

GOX generation errors in child packages keep the original source path. For
example, a broken `internal/ui/layout.gox` should report
`examples/multipackage/internal/ui/layout.gox:line:column` instead of only a
generated `.goframe/work/.../layout.gox.go` file.

Lower-level Go or TinyGo type-checking errors can still mention the hidden
workspace because the compiler is checking materialized generated Go files.
When that happens, inspect the matching `.gox` source and the generated file
under `.goframe/work/<profile>` together.

## Cross-package Components

GOX supports package-qualified component tags:

```gox
import ui "github.com/example/app/internal/ui"

func App() gf.Node {
    return (
        <ui.Layout Title="Demo">
            <p>Hello</p>
        </ui.Layout>
    )
}
```

The tag form is exactly `packageAlias.Component`; `packageAlias` is the normal
Go import alias in the current file. Generated component identity uses the
resolved import path plus component name:

```text
github.com/example/app/internal/ui.Layout
```

The generated shape uses the package-qualified props struct and render
function:

```go
gf.ComponentT(_goxComponent_app_ui_Layout, ui.LayoutProps{
    Title: "Demo",
    Children: []gf.Node{...},
}, ui.Layout)
```

Ordinary Go function calls remain valid for helpers and low-level composition,
but package-qualified tags are the recommended way to create cross-package
component boundaries in GOX.

Unsupported forms:

```gox
<ui:Layout />       // XML-style namespace syntax
<foo.bar.Layout />  // nested selector chain
<ui.layout />       // selected component is not exported
<.Layout />         // dot imports are not supported for tags
```

Inside each package, local capitalized GOX tags still create component
boundaries with import-aware generated identities.

## Entry Path Rules

Allowed examples:

```text
.
./cmd/app
cmd/app
./src/app
src/app
./app
app
```

Rejected examples include empty paths, absolute paths, paths containing `..`,
and tool-owned directories such as `.goframe`, `build`, `dist`,
`node_modules`, and `.git`.

The entry must point to a directory, not a file. Symlink behavior is still
covered by the broader file safety policy rather than a full symlink-hardening
claim.

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

The child-entry example supports:

```bash
goxc generate ./examples/cmdapp
goxc build ./examples/cmdapp --compiler=go
goxc package ./examples/cmdapp --compiler=go
goxc package ./examples/cmdapp --compiler=tinygo
goxc size ./examples/cmdapp
goxc serve ./examples/cmdapp --port=8080
```

The router-dashboard reference example uses the same child-entry model while
also demonstrating router query filters and local form validation:

```bash
goxc package ./examples/router-dashboard --compiler=tinygo
goxc serve ./examples/router-dashboard --port=8080
```

## Current Limitations

- No XML-style namespace tags with `:`.
- No arbitrary selector chains beyond `packageAlias.Component`.
- No full multi-module monorepo story.
- No generated GOX materialization for raw `.gox` files inside external
  dependencies. External dependencies are resolved as ordinary Go modules.
- No file-based router or framework-level layout convention. The hash router
  uses ordinary Go route declarations and stable shell composition.
- App module `require` / `replace` directives are preserved for normal Go
  dependency resolution, but that is not a stable reusable package ecosystem or
  full multi-module workspace claim.
- Generated component type variable names are internal and not stable API.

## Example

See `examples/multipackage` for a compact `entry: "."` app with a root
package, `internal/ui`, `internal/issues`, and ordinary shared Go helpers.
See `examples/cmdapp` for a compact `entry: "./cmd/app"` app.
See `examples/router-dashboard` for a larger child-entry reference app that
combines routing, query-state filters, and controlled forms.
