# goframe

GoFrame is an experimental Go-first web application framework and toolchain for
interactive browser/WASM apps. It combines:

- the `goframe` runtime library;
- JSX-like `.gox` markup embedded in Go files;
- the `goxc` generate/build/package/serve toolchain;
- TinyGo and standard Go WebAssembly targets.

GoFrame is not production-ready. Some APIs are public-candidate, but the GOX
syntax, manifests, packaging details, and runtime internals may still change
between MVPs. The browser/WASM target is the current validated implementation;
Player/Engine and `.gfapp` are inactive directions, not shipped products or
preview promises.

The first public preview scope is intentionally narrower than the project
direction: it should validate the current browser/WASM application layer
without removing or hiding working experimental surfaces such as the router,
resources, and Error Boundaries. Staged Go backend integration is a direction
that requires evidence before it becomes a preview feature claim.

## Status

Current baseline includes:

- GOX expression ergonomics and source-oriented diagnostics;
- component boundaries, state, reducer dispatch, effects, context selectors,
  memoization, keyed reconciliation, fixed-height virtualization,
  component-scoped resources, and a small hash-based client router with tiny
  query helpers;
- runtime error reporting plus scoped render-only Error Boundaries for
  recover-capable builds;
- cache-safe packaging, hidden `.goframe` workspace output, export safety, and
  clean workspace commands;
- multi-package GOX workspaces, child entry packages such as `./cmd/app`, and
  import-aware generated component identity;
- documented forms/validation patterns and a resource-backed router-dashboard
  reference app;
- CI gates for Go/GOX tests, TinyGo size budgets, browser smoke, artifact
  checks, module path checks, and docs consistency.

Non-goals for the current project surface include SSR, hydration, fullstack
server APIs, Player/Engine, `.gfapp`, path/history-mode routing, file-based
routing, route loaders, Suspense-style resources, global resource caching,
dynamic virtualization, infinite loading, LSP, formatter, XML-style namespace
tags, spread props, schema validation framework, production deployment server,
and full multi-module monorepo support.

## What Is GoFrame?

GoFrame lets you write interactive UI in Go while keeping authored source
clean:

```gox
package main

func App(props AppProps) gf.Node {
	count, setCount := gf.UseState(0)

	return (
		<main>
			<h1>Counter</h1>
			<p>Count: {count}</p>
			<button onClick={func() { setCount(count + 1) }}>
				Increment
			</button>
		</main>
	)
}
```

`goxc generate` turns `.gox` into Go compiler output under `.goframe`; it does
not write generated `.gox.go` files next to source by default.

## Project Layers

| Layer | Role |
|---|---|
| GOX language | JSX-like declarative markup embedded in Go source. |
| `pkg/goframe` runtime | Nodes, component instances, hooks, context, events, reconciliation, and browser mounting. |
| `pkg/gox` compiler | GOX lexer/parser/codegen plus diagnostics and golden tests. |
| `cmd/goxc` toolchain | Generate, build, package, export, serve, size, clean, doctor, and version commands. |
| examples and scripts | Integration probes, browser smoke, size budgets, and DOM pressure gates. |
| VS Code extension | Lightweight syntax highlighting/snippets/commands over the same `goxc` workflow. |

## Installing goxc

Install the current published toolchain:

```bash
go install github.com/graybuton/goframe/cmd/goxc@latest
```

Make sure `$(go env GOPATH)/bin` is in `PATH`, then check the local toolchain:

```bash
goxc version
goxc doctor
```

When working inside this repository, install the local checkout instead:

```bash
go install ./cmd/goxc
```

`go run ./cmd/goxc` is useful while editing the CLI itself.

## Quick Start

Package and serve the counter example:

```bash
goxc doctor
goxc package ./examples/counter --compiler=tinygo
goxc size ./examples/counter
goxc serve ./examples/counter --port=8080
```

Open <http://127.0.0.1:8080>.

Use standard Go compatibility mode when TinyGo is unavailable or when binary
size is not the main concern:

```bash
goxc package ./examples/counter --compiler=go
```

## Start Here

Recommended first path:

```text
examples/counter
  -> examples/components
  -> examples/router-dashboard
```

`examples/router-dashboard` is the flagship reference app. It shows how the
current primitives fit together in a small Go-first SPA/dashboard/admin-style
application: stable shell, hash router, query filters, component-scoped
resource data, explicit loading/failed UI, controlled form validation, and a
render Error Boundary.

Focused deep dives:

- `examples/router`: routing only;
- `examples/resource`: resource lifecycle, stale completion, failure, and
  cleanup;
- `examples/context`: scoped providers and selector consumers;
- `examples/virtualized`: fixed-height bounded DOM;
- `examples/todo`: controlled inputs, effects, events, and list helpers.

Toolchain/layout examples: `examples/multipackage` and `examples/cmdapp`.
Pressure/performance example: `examples/dashboard`.

For the guided path, read [GoFrame Tutorial](docs/tutorial.md).

## Examples

### Quickstart

| Example | What it demonstrates | Command |
|---|---|---|
| `examples/counter` | Minimal state, events, TinyGo quickstart, package/serve workflow. | `goxc package ./examples/counter --compiler=tinygo` |
| `examples/components` | Typed props, children, fragments, component composition. | `goxc package ./examples/components --compiler=tinygo` |

### Application Primitives

| Example | What it demonstrates | Command |
|---|---|---|
| `examples/todo` | Controlled inputs, events, effects, keys, list helpers, localStorage. | `goxc package ./examples/todo --compiler=tinygo` |
| `examples/context` | Scoped providers, selector consumers, broad `UseContext`, nested providers. | `goxc package ./examples/context --compiler=tinygo` |
| `examples/virtualized` | `gf.VirtualList`, `gf.VirtualTable`, bounded DOM with 10,000 logical rows. | `goxc package ./examples/virtualized --compiler=tinygo` |
| `examples/router` | Hash router, query helpers, route params, not-found route, and stable shell layout. | `goxc package ./examples/router --compiler=tinygo` |
| `examples/resource` | Focused `gf.UseResource` lifecycle: loading/ready/failed state, stale completion guards, reload, and cleanup. | `goxc package ./examples/resource --compiler=tinygo` |

### Toolchain / Layout

| Example | What it demonstrates | Command |
|---|---|---|
| `examples/multipackage` | `entry: "."` app with root plus `internal/...` packages. | `goxc package ./examples/multipackage --compiler=tinygo` |
| `examples/cmdapp` | Child entry package with `"entry": "./cmd/app"` and Go-first layout. | `goxc package ./examples/cmdapp --compiler=tinygo` |

### Reference / Pressure

| Example | What it demonstrates | Command |
|---|---|---|
| `examples/dashboard` | Reducer dispatch, explicit memoization, virtual table, dashboard pressure smoke. | `goxc package ./examples/dashboard --compiler=tinygo` |
| `examples/router-dashboard` | Flagship reference app: router, query-state filters, component-scoped resource data, forms, validation, Error Boundary, and Go-first layout. | `goxc package ./examples/router-dashboard --compiler=tinygo` |
| `examples/server-backed` | Server-backed reference fixture: packaged browser/WASM app served by plain Go `net/http` with a same-origin API. | `goxc package ./examples/server-backed --compiler=go` |

Serve any packaged example with:

```bash
goxc serve ./examples/<name> --port=8080
```

For cache-safe deployment-style artifacts:

```bash
goxc package ./examples/<name> --compiler=tinygo --asset-hash --preload --compress=gzip,br
```

Use `goxc export` only when you want a visible deploy directory:

```bash
goxc export ./examples/counter --out ./dist
```

## Go-First App Layout

Single-package apps can keep `entry: "."`:

```text
app/
├── goframe.json
├── app.gox
├── main.go
└── assets/
    ├── index.html
    └── styles.css
```

For larger apps, the recommended Go-first layout is a child executable entry
package plus app-private internal packages:

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

```json
{
  "entry": "./cmd/app",
  "compiler": "tinygo",
  "assets": "./assets"
}
```

Generic child entries such as `./app` or `./src/app` work as ordinary Go
package directories, but `cmd/app + internal/...` is the primary Go convention.
GOX discovery remains app-root-wide, so imported internal packages get
generated files too. Cross-package component composition can use normal Go
imports with package-qualified tags such as `<ui.Header />`. Ordinary Go
function calls are still useful for helper functions and low-level composition.

## GOX In One Minute

Lowercase tags create DOM elements:

```gox
<button onClick={increment}>Increment</button>
```

Capitalized tags create runtime component boundaries using
`<ComponentName>Props`:

```gox
<Button Label="Increment" OnClick={increment} />
```

```go
type ButtonProps struct {
	Label   string
	OnClick func()
}
```

Package-qualified component tags use ordinary Go import aliases and keep
cross-package composition declarative:

```gox
import ui "github.com/example/app/internal/ui"

func App() gf.Node {
	return (
		<ui.Shell Title="Dashboard">
			<p>Hello</p>
		</ui.Shell>
	)
}
```

The supported form is exactly `packageAlias.Component`. It is not an XML
namespace system and does not support arbitrary selector chains.

Generated GOX uses typed component identity tokens:

```go
var _goxComponent_app_Button = gf.NewComponentType("main.Button", "Button")

gf.ComponentT(_goxComponent_app_Button, ButtonProps{
	Label:   "Increment",
	OnClick: increment,
}, Button)
```

`goxc` can emit import-aware ids for generated components when the package path
is known, for example:

```text
github.com/graybuton/goframe/examples/cmdapp/internal/ui.Header
```

Children use `Children []gf.Node`. Fragments use `<>...</>`.
`Key={...}` is a pseudo-prop for reconciliation and is not passed to component
props or emitted as a DOM attribute.

GOX supports expression-oriented rendering:

```gox
{ready && <ReadyView />}

{len(items) == 0 ? (
	<EmptyState />
) : (
	<ItemList Items={items} />
)}

{gf.Map(items, func(item Item) gf.Node {
	return <ItemRow Key={item.ID} Item={item} />
})}
```

Unsupported syntax is intentionally rejected with source diagnostics:

- XML-style namespace tags such as `<ui:Header />`;
- arbitrary selector chains such as `<foo.bar.Baz />`;
- spread props such as `<Button {...props} />`;
- template-block `if`/`for`;
- arbitrary JavaScript-like expressions.

See [GOX language](docs/gox-language.md).

## Runtime Primitives

State is component-scoped and positional:

```go
count, setCount := gf.UseState(0)
setCount(count + 1)
```

Reducer dispatch applies an action to the latest state slot at event time,
which avoids stale render captures in retained event handlers:

```go
type CounterAction int

count, dispatch := gf.UseReducer(0, func(state int, action CounterAction) int {
	return state + int(action)
})
dispatch(1)
```

Effects run after DOM patching:

```go
gf.UseEffect(func() gf.Cleanup { return nil })
gf.UseEffect(func() gf.Cleanup { return nil }, gf.Deps(value))
gf.UseUnmount(func() {})
```

Context is scoped by the component tree:

```go
var PreferencesContext = gf.CreateContext(Preferences{})

gf.ProvideContext(PreferencesContext, preferences)

density := gf.UseContextSelector(PreferencesContext, func(value Preferences) string {
	return value.Density
})
```

Memoization is explicit and opt-in: implement `MemoEqual` on props when a
component can safely skip render/patch for equivalent props. The runtime also
protects against dirty descendants hidden under memoized ancestors.

Large fixed-height collections should use `gf.VirtualList` or
`gf.VirtualTable` instead of mounting hidden offscreen rows.

Client-side page changes can use the small hash router:

```go
var router = gf.NewHashRouter([]gf.Route{
	gf.RoutePath("/", homeRoute),
	gf.RoutePath("/issues/:id", issueRoute),
	gf.NotFoundRoute(notFoundRoute),
})

func App() gf.Node {
	return gf.RouterView(router)
}
```

The MVP router is hash-based. Path/history mode, file-based routing, loaders,
route middleware, and automatic route-level boundaries are not part of the
current preview contract.

For small URL-driven state, routes expose query helpers:

```go
query := ctx.Query()
status := query.Get("status")

gf.Navigate(gf.WithQuery("/issues", gf.QueryValues{
	"status": {"open"},
	"q":      {"auth"},
}))
```

Forms use ordinary runtime primitives: controlled inputs, `gf.InputEvent`,
`gf.Event.PreventDefault`, and application-owned validation state. GoFrame
does not include a schema validation framework.

Render failures can be contained with an experimental scoped Error Boundary:

```gox
<gf.ErrorBoundary ResetKey={routeKey} Fallback={fallback}>
	<pages.Content />
</gf.ErrorBoundary>
```

Boundaries catch descendant render failures, report through
`gf.SetErrorHandler`, and render fallback UI until reset. They do not catch
event, effect, cleanup, memo comparator, or context update failures.

## Toolchain Workflow

Generated, build, and package outputs live under an app-local hidden workspace:

```text
<app>/.goframe/
├── gen/
├── work/
├── build/
└── package/
```

Use `GOFRAME_WORKSPACE=/work/goframe` or `--workspace /work/goframe` when the
source tree is read-only.

Common commands:

| Command | Responsibility |
|---|---|
| `goxc generate <app>` | Generate `.gox.go` compiler output under `.goframe/gen`. |
| `goxc build <app>` | Compile raw WASM under `.goframe/build/<compiler>/dev`. |
| `goxc package <app>` | Build a runnable `.goframe/package/standalone` bundle. |
| `goxc export <app> --out <dir>` | Copy the latest package to an explicit deploy directory. |
| `goxc size <app>` | Report size from `.goframe/package/standalone`. |
| `goxc serve <app>` | Serve the local package for development. |
| `goxc clean <app>` | Remove tool-owned workspace/build/package artifacts. |
| `goxc doctor` | Check local Go/TinyGo/compression/runtime-shim tools. |

`goxc package` normalizes output to:

```text
<app>/.goframe/package/standalone/
├── index.html
├── asset-manifest.json
├── goframe-package.json
└── assets/
    ├── bundle.wasm
    └── wasm_exec.js
```

Use `"assets": "./assets"` for app static files. If `assets/index.html` exists,
it is rewritten as the package root HTML entrypoint; otherwise `goxc package`
generates a default `index.html`.

`goxc serve` is development-only. Production compression negotiation, cache
headers, TLS, access control, and static-server hardening belong to deployment
infrastructure.

## Documentation Map

Start here:

- [GoFrame Tutorial](docs/tutorial.md)
- [Evaluator guide](docs/evaluator-guide.md)
- [v0.1.0-preview.1 release notes](docs/release-notes-v0.1.0-preview.1.md)
- [v0.1.0-preview.2 release notes](docs/release-notes-v0.1.0-preview.2.md)
- [v0.2.0-preview.1 release notes](docs/release-notes-v0.2.0-preview.1.md)
- [Architecture and toolchain boundaries](docs/architecture.md)
- [Runtime model](docs/runtime-model.md)
- [GOX language and diagnostics](docs/gox-language.md)
- [API stability tiers](docs/api-stability.md)
- [Public Preview Readiness](docs/public-preview-readiness.md)
- [Pre-preview action plan](docs/pre-preview-action-plan.md)
- [Compatibility and deprecation policy](docs/compatibility.md)
- [Migration note template](docs/migrations.md)
- [Platform support matrix](docs/platform-support.md)
- [CI and regression gates](docs/ci.md)

Runtime topics:

- [Lifecycle and effects](docs/effects.md)
- [Runtime error semantics](docs/runtime-errors.md)
- [Error Boundaries](docs/error-boundaries.md)
- [Forms and validation patterns](docs/forms.md)
- [Explicit memoization](docs/memoization.md)
- [Context selectors](docs/context.md)
- [Virtualized collections](docs/virtualization.md)
- [Client router](docs/router.md)
- [Component-scoped resources](docs/resources.md)
- [Component identity strategy](docs/component-identity.md)
- [Component identity next steps](docs/component-identity-next.md)

Toolchain and delivery:

- [Multi-package GOX workspace](docs/multi-package-workspace.md)
- [Cache-safe package delivery](docs/deployment.md)
- [Manifest compatibility](docs/manifest-compatibility.md)
- [Symlink and file safety policy](docs/security-symlink-policy.md)
- [Performance baseline](docs/performance-baseline.md)
- [Release hygiene](docs/release.md)

Project audits and future design:

- [Foundation audit](docs/foundation-audit.md)
- [Foundation Audit IV](docs/foundation-audit-iv.md)
- [Public Surface Audit V](docs/public-surface-audit-v.md)
- [Public Preview Hardening I](docs/public-preview-hardening-i.md)
- [Post-preview v0.2 technical focus](docs/post-preview-v0.2-focus.md)
- [Inactive GoFrame Player/Engine note](docs/player-vision.md)

Examples:

- [Counter example](examples/counter/README.md)
- [Components example](examples/components/README.md)
- [Todo example](examples/todo/README.md)
- [Dashboard pressure-test example](examples/dashboard/README.md)
- [Context selectors example](examples/context/README.md)
- [Virtualized collections example](examples/virtualized/README.md)
- [Multi-package GOX example](examples/multipackage/README.md)
- [Child entry package example](examples/cmdapp/README.md)
- [Router example](examples/router/README.md)
- [Router dashboard reference example](examples/router-dashboard/README.md)
- [Resource loading example](examples/resource/README.md)
- [Server-backed reference example](examples/server-backed/README.md)
- [VS Code GOX extension](extensions/vscode-gox/README.md)

## Current Limitations

- Experimental browser/WASM target only.
- Hash router only; no SSR, hydration, path/history-mode server fallback,
  file-based routing, route loaders, hot reload, CSS-in-Go, or production
  deployment server.
- Player/Engine and `.gfapp` are inactive directions outside the current
  preview contract.
- No XML-style namespace tags, arbitrary selector chains, spread props,
  template-block loops, formatter, or LSP.
- No full multi-module monorepo story.
- State/effect/context hooks are positional and require stable call order.
- Context selectors require comparable selected values.
- Memoization is explicit; there is no automatic deep equality or function-prop
  equality.
- Virtualized collections are fixed-height only; no dynamic measurement,
  infinite loading, or advanced keyboard/accessibility layer yet.
- Error Boundaries are render-only and recover-based. Default TinyGo
  `panic=trap` builds compile the API but are not the proof path for boundary
  containment.
- Resources are component-scoped and explicit-state only. There is no global
  resource cache, Suspense-style render blocking, route loader framework, or
  runtime fetch/JSON API.
- Duplicate key diagnostics are debug-only.
- TinyGo compatibility remains version- and feature-dependent.

## Development Checks

Core local checks:

```bash
go fmt ./...
go test ./...
go test -race ./pkg/... ./cmd/...
go vet ./...
go test -tags=goframe_debug ./...
go test ./pkg/gox -run 'TestGolden|TestErrorGolden'
```

Full local gate:

```bash
scripts/check.sh
scripts/size-budget.sh
scripts/perf-report.sh
scripts/browser-smoke.sh
node --experimental-websocket scripts/dashboard-dom-pressure.mjs
scripts/artifact-check.sh
scripts/module-path-check.sh
```

Package all examples:

```bash
for app in examples/*; do
	if [ -f "$app/goframe.json" ]; then
		compiler=tinygo
		case "$app" in
			examples/server-backed) compiler=go ;;
		esac
		goxc package "$app" --compiler="$compiler" --asset-hash --preload --compress=gzip,br
	fi
done
```

CI runs core Go/GOX checks, docs consistency, TinyGo WASM size budgets,
browser smoke, dashboard DOM pressure, artifact/module gates, and VS Code
extension compile checks.

## VS Code Support

The MVP `.gox` extension lives in `extensions/vscode-gox`. It provides syntax
highlighting, snippets, file icons, and command-palette actions that shell out
to `goxc`.

Local extension development:

```bash
cd extensions/vscode-gox
npm install
npm run compile
code .
```

## License

goframe is licensed under the Apache License, Version 2.0.

See [LICENSE](LICENSE) and [NOTICE](NOTICE).
