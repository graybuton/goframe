<div align="center">
  <picture>
    <img alt="GoFrame mark" src="assets/brand/goframe-mark.svg" width="112">
  </picture>

  <h1>GoFrame</h1>

  <p>
    <strong>Go-first web framework &amp; toolchain for browser/WASM apps.</strong>
  </p>

  <p>
    <a href="LICENSE">
      <img alt="License: Apache-2.0" src="https://img.shields.io/badge/license-Apache--2.0-2563EB.svg?style=for-the-badge&amp;labelColor=111827">
    </a>
    <a href="docs/evaluator-guide.md">
      <img alt="Preview scope: browser/WASM" src="https://img.shields.io/badge/preview-browser%2FWASM-00ADD8.svg?style=for-the-badge&amp;labelColor=111827">
    </a>
    <a href="docs/gox-language.md">
      <img alt="GOX language" src="https://img.shields.io/badge/GOX-language-5260FF.svg?style=for-the-badge&amp;labelColor=111827">
    </a>
    <a href="docs/deployment.md">
      <img alt="Toolchain: goxc" src="https://img.shields.io/badge/toolchain-goxc-248BFF.svg?style=for-the-badge&amp;labelColor=111827">
    </a>
  </p>
</div>

GoFrame is an experimental Go-first framework and toolchain for interactive
browser/WASM apps. It combines a small Go runtime, JSX-like `.gox` markup, the
`goxc` generate/build/package workflow, and standard Go or TinyGo WebAssembly
output.

The current validated surface is browser/WASM applications. GoFrame is not
production-ready, does not claim stable 1.0 APIs, and does not ship
SSR/hydration, fullstack/server APIs, Player/Engine, `.gfapp`, formatter, or
LSP support.

## Quick Start

Install `goxc`, add its install directory to `PATH`, check your local
toolchain, package the counter example, and serve it locally:

```bash
go install github.com/graybuton/goframe/cmd/goxc@latest

goxc_bin="$(go env GOBIN)"
if [ -z "$goxc_bin" ]; then
  goxc_bin="$(go env GOPATH)/bin"
fi
export PATH="$goxc_bin:$PATH"

goxc doctor
goxc package ./examples/counter --compiler=tinygo
goxc serve ./examples/counter --port=8080
```

Open <http://127.0.0.1:8080>.

TinyGo gives the smallest browser bundles when available. Standard Go remains
useful for compatibility and local inspection:

```bash
goxc package ./examples/counter --compiler=go
```

`goxc serve` is a development server for packaged examples, not production
hosting infrastructure.

## Tiny GOX Example

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

`goxc check` validates authored `.gox` files without writing generated output.
`goxc generate` turns `.gox` files into Go compiler output under `.goframe`;
it does not write generated `.gox.go` files next to authored source by default.

## What You Get

- `pkg/goframe`: browser/WASM runtime primitives for nodes, components, hooks,
  context, events, reconciliation, resources, routing, and fixed-height
  virtualization.
- `pkg/gox`: GOX lexer/parser/codegen with source-oriented diagnostics.
- `cmd/goxc`: check, generate, build, package, export, serve, size, clean,
  doctor, and version commands.
- Examples and scripts that exercise the runtime, compiler, package workflow,
  browser smoke paths, and WASM size budgets.
- A lightweight VS Code GOX extension in `extensions/vscode-gox`.

## Where To Go Next

- [GoFrame Tutorial](docs/tutorial.md) - guided router-dashboard walkthrough.
- [Evaluator guide](docs/evaluator-guide.md) - short path for trying the
  current browser/WASM preview.
- [GOX language](docs/gox-language.md) - syntax, diagnostics, and limits.
- [Runtime model](docs/runtime-model.md) - component identity, hooks,
  reconciliation, resources, and errors.
- [Deployment](docs/deployment.md) - package layout, asset hashing, preloads,
  export, and cache-safe static delivery.
- [API stability](docs/api-stability.md) and
  [platform support](docs/platform-support.md) - maturity and support
  boundaries.
- [Releases](https://github.com/graybuton/goframe/releases) - published
  previews and release notes.
- [Release process](docs/release.md) - versioning, validation, and publishing
  policy.
- [Roadmap](docs/roadmap.md) - status-qualified direction for the application
  model, modular delivery, SSR/hydration research, tooling, and fullstack
  contracts beyond the current shipped surface.

For the fastest tour, start with `examples/counter`, then
`examples/components`, then `examples/router-dashboard`.

<details>
<summary>Examples</summary>

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
| `examples/resource` | `gf.UseResource` loading, ready, failed, stale completion, reload, and cleanup behavior. | `goxc package ./examples/resource --compiler=tinygo` |

### Reference And Pressure Examples

| Example | What it demonstrates | Command |
|---|---|---|
| `examples/dashboard` | Reducer dispatch, explicit memoization, virtual table, dashboard pressure smoke. | `goxc package ./examples/dashboard --compiler=tinygo` |
| `examples/router-dashboard` | Reference app: hash router, query-state filters, component-scoped resource data, forms, validation, Error Boundary, and Go-first layout. | `goxc package ./examples/router-dashboard --compiler=tinygo` |
| `examples/server-backed` | Browser/WASM app served by plain Go `net/http` with a same-origin API. | `goxc package ./examples/server-backed --compiler=go` |

### Toolchain And Layout

| Example | What it demonstrates | Command |
|---|---|---|
| `examples/multipackage` | `entry: "."` app with root plus `internal/...` packages. | `goxc package ./examples/multipackage --compiler=tinygo` |
| `examples/cmdapp` | Child entry package with `"entry": "./cmd/app"` and Go-first layout. | `goxc package ./examples/cmdapp --compiler=tinygo` |

Serve any packaged example with:

```bash
goxc serve ./examples/<name> --port=8080
```

Create cache-safe deployment-style artifacts with:

```bash
goxc package ./examples/<name> --compiler=tinygo --asset-hash --preload --compress=gzip,br
```

Use `goxc export` when you want a visible deploy directory:

```bash
goxc export ./examples/counter --out ./dist
```

</details>

<details>
<summary>Project Layers</summary>

| Layer | Role |
|---|---|
| GOX language | JSX-like declarative markup embedded in Go source. |
| `pkg/goframe` runtime | Nodes, component instances, hooks, context, events, reconciliation, resources, routing, and browser mounting. |
| `pkg/gox` compiler | GOX lexer/parser/codegen plus diagnostics and golden tests. |
| `cmd/goxc` toolchain | Generate, build, package, export, serve, size, clean, doctor, and version commands. |
| examples and scripts | Integration probes, browser smoke, size budgets, and package/artifact gates. |
| VS Code extension | Syntax highlighting, snippets, file icons, and commands over the same `goxc` workflow. |

Single-package apps can use `entry: "."`:

```text
app/
├── goframe.json
├── app.gox
├── main.go
└── assets/
    ├── index.html
    └── styles.css
```

Larger apps can use a Go-style child executable entry package:

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
    └── ui/
        └── layout.gox
```

```json
{
  "entry": "./cmd/app",
  "compiler": "tinygo",
  "assets": "./assets"
}
```

GOX discovery is app-root-wide, so imported internal packages get generated
files too. Cross-package component composition uses normal Go imports with
package-qualified tags such as `<ui.Header />`.

</details>

<details>
<summary>GOX In One Minute</summary>

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

Package-qualified component tags use ordinary Go import aliases:

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

Unsupported syntax is intentionally rejected with source diagnostics, including
XML-style namespace tags, arbitrary selector chains, spread props,
template-block `if`/`for`, and arbitrary JavaScript-like expressions.

See [GOX language](docs/gox-language.md).

</details>

<details>
<summary>Runtime Primitives</summary>

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
component can safely skip render/patch for equivalent props. Large fixed-height
collections should use `gf.VirtualList` or `gf.VirtualTable` instead of
mounting hidden offscreen rows.

The small client router is hash-based:

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

Forms use ordinary runtime primitives: controlled inputs, `gf.InputEvent`,
`gf.Event.PreventDefault`, and application-owned validation state. GoFrame does
not include a schema validation framework.

Render failures can be contained with an experimental scoped Error Boundary:

```gox
<gf.ErrorBoundary ResetKey={routeKey} Fallback={fallback}>
	<pages.Content />
</gf.ErrorBoundary>
```

Boundaries catch descendant render failures, report through
`gf.SetErrorHandler`, and render fallback UI until reset. They do not catch
event, effect, cleanup, memo comparator, or context update failures.

</details>

<details>
<summary>Toolchain Workflow</summary>

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
| `goxc check <file-or-directory>` | Validate authored `.gox` source without writing generated files. |
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

</details>

<details>
<summary>Current Limitations</summary>

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
- Resources are component-scoped and explicit-state only. `gf.FetchText` is a
  low-level experimental browser text loader; there is no global resource
  cache, Suspense-style render blocking, route loader framework, JSON/data
  framework, or higher-level fetch API.
- Duplicate key diagnostics are debug-only.
- TinyGo compatibility remains version- and feature-dependent.

</details>

<details>
<summary>Development Checks</summary>

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

CI runs core Go/GOX checks, docs consistency, TinyGo WASM size budgets, browser
smoke, dashboard DOM pressure, artifact/module gates, and VS Code extension
compile checks.

</details>

<details>
<summary>VS Code Support</summary>

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

</details>

<details>
<summary>Full Documentation Map</summary>

Start here:

- [GoFrame Tutorial](docs/tutorial.md)
- [Evaluator guide](docs/evaluator-guide.md)
- [Published releases and release notes](https://github.com/graybuton/goframe/releases)
- [Release process](docs/release.md)
- [Architecture and toolchain boundaries](docs/architecture.md)
- [Runtime model](docs/runtime-model.md)
- [GOX language and diagnostics](docs/gox-language.md)
- [API stability tiers](docs/api-stability.md)
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

</details>

## License

goframe is licensed under the Apache License, Version 2.0.

See [LICENSE](LICENSE) and [NOTICE](NOTICE).
