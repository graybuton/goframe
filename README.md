# goframe

`goframe` is an experimental Go-first application platform for interactive
apps. It combines a Go runtime library, JSX-like `.gox` markup, the `goxc`
compiler toolchain, and WebAssembly browser targets.

goframe is not intended to be a tiny replacement for React on static websites.
The browser target is the current experiment; a future GoFrame Player remains
an architectural direction rather than an implemented product.

> Status: experimental MVP. APIs, GOX syntax, manifests, and toolchain behavior
> may change.

## Project layers

```text
GOX language       JSX-like declarative markup embedded in Go
goframe runtime    state, virtual nodes, DOM mounting, browser events
goxc toolchain     generate, build, package, inspect, and serve applications
GoFrame Player     possible future host for portable .gfapp bundles
```

## Installing goxc

Install the latest published toolchain:

```bash
go install github.com/jin-wu/goframe/cmd/goxc@latest
```

Make sure `$(go env GOPATH)/bin` is in `PATH`, then verify the installation:

```bash
goxc version
goxc doctor
```

## Local development install

From this repository:

```bash
go install ./cmd/goxc
```

Alternatively, use `go run ./cmd/goxc` while changing the CLI itself.

## Quick start

The counter manifest selects TinyGo by default:

```bash
goxc doctor
goxc generate ./examples/counter
goxc package ./examples/counter
goxc size ./examples/counter
goxc serve ./examples/counter --port=8080
```

Open <http://127.0.0.1:8080>. Add `?sw=1` to opt into the service-worker cache
experiment. Browser console logs show WASM instantiation. Render and patch
probes require a `goframe_debug` build.

Use the standard Go compiler explicitly when compatibility matters more than
download size:

```bash
goxc package ./examples/counter --compiler=go
```

The components example demonstrates typed props, children, fragments, and
state:

```bash
goxc package ./examples/components
goxc serve ./examples/components --port=8080
```

The Todo example exercises application-level primitives, controlled inputs,
typed events, conditional/list helpers, and keys:

```bash
goxc generate ./examples/todo
goxc package ./examples/todo --compiler=tinygo
goxc size ./examples/todo
goxc serve ./examples/todo --port=8080
```

The Todo reconciliation smoke test uses a separate instrumented build so debug
probes do not increase production WASM:

```bash
tinygo build -target=wasm -no-debug -panic=trap -tags=goframe_debug \
  -o ./examples/todo/dist/main.wasm ./examples/todo
goxc serve ./examples/todo --port=18080
node --experimental-websocket scripts/todo-browser-smoke.mjs
```

## GOX component model

Lowercase tags create HTML elements:

```gox
<button onClick={increment}>Increment</button>
```

Capitalized tags create runtime component boundaries using the
`<ComponentName>Props` convention:

```gox
<Button Label="Increment" OnClick={increment} />
```

```go
type ButtonProps struct {
	Label   string
	OnClick func()
}
```

The generated Go keeps the component visible to the runtime:

```go
gf.Component("Button", ButtonProps{
	Label:   "Increment",
	OnClick: increment,
}, Button)
```

Component children use a `Children []gf.Node` props field. Fragments use
`<>...</>`. Child expressions are generated through `gf.Child`, which supports
primitive values, a `gf.Node`, or `[]gf.Node`.

Calling `Button(ButtonProps{...})` directly remains valid Go, but it is an
ordinary function call and does not create component identity.

See [GOX language](docs/gox-language.md) and the
[components example](examples/components/README.md).

## Application primitives

GOX keeps control flow in Go. Runtime helpers make common render choices
concise without adding a second template language:

```go
gf.If(show, node)
gf.IfElse(show, thenNode, elseNode)
gf.For(items, func(item Item) gf.Node { ... })
gf.ForIndexed(items, func(index int, item Item) gf.Node { ... })
```

`gf.Empty()` is a nil-safe empty result. `gf.Key("item-1", node)` provides
stable identity for keyed child reuse and movement.

Browser props accept both lowercase and exported-style common names, including
`value`/`Value`, `type`/`Type`, `placeholder`/`Placeholder`, and
`onInput`/`OnInput`. Event handlers may use:

```go
func()
func(gf.Event)
func(gf.InputEvent)
```

`gf.Event` provides `PreventDefault` and `StopPropagation`.
`gf.InputEvent.Value()` supports controlled inputs. `gf.UseState` slots belong
to the component instance currently rendering. State `Set` calls mark that
owner dirty and are coalesced into one browser animation-frame update.

The MVP patch layer updates text and props in place, keeps one stable listener
per event name, patches unkeyed children positionally, and matches keyed
children by key. Dirty component updates start directly at their mounted
subtree, so unrelated ancestors and siblings are not traversed. If a parent and
child are both dirty in the same batch, ancestor pruning keeps only the parent
update. Descendant components encountered inside an updated parent subtree
rerender; there is no automatic props memoization.

Duplicate sibling keys are a user error. Production builds keep the smallest
behavior and do not diagnose them; `goframe_debug` builds record and warn about
duplicate keys for browser smoke tests.

## VS Code support

The repository includes an MVP `.gox` extension in
`extensions/vscode-gox`. It provides syntax highlighting, language
configuration, snippets, `.gox` icons, and command-palette actions for `goxc`.

Local extension development:

```bash
cd extensions/vscode-gox
npm install
npm run compile
code .
```

Press `F5` and select `Run GOX Extension` to launch an Extension Development
Host. Open `samples/demo.gox` in that host to inspect highlighting and snippets.

Available commands:

- `GOX: Generate Current Project`
- `GOX: Package Current Project`
- `GOX: Serve Current Project`
- `GOX: Run Doctor`

The commands require an installed `goxc`. Configure `gox.goxcPath` when it is
not directly available from the integrated terminal's `PATH`.

See [VS Code GOX extension](extensions/vscode-gox/README.md).

## Command responsibilities

### Generate

`generate` transforms `.gox` source into adjacent `.gox.go` files:

```bash
goxc generate ./examples/counter
goxc generate ./examples/counter/app.gox
```

### Build

`build` only compiles raw WASM. It does not copy web assets, create a
distribution, or compress files:

```bash
goxc build ./examples/counter --compiler=tinygo
goxc build ./examples/counter --compiler=go
```

Default output:

```text
examples/counter/build/main.wasm
```

`--out=directory` overrides the build directory.

### Package

`package` creates a runnable normalized bundle:

```bash
goxc package ./examples/counter --compiler=tinygo
```

```text
examples/counter/dist/
├── index.html
├── main.wasm
├── manifest.json
├── service-worker.js
└── wasm_exec.js
```

Compiler-specific filenames are internal details. A packaged application uses
`main.wasm` and `wasm_exec.js` for both Go and TinyGo.

Compression is a deployment, web-server, CDN, or reverse-proxy responsibility.
`goxc package` does not compress by default. Precompression is available only
as an explicit packaging helper:

```bash
goxc package ./examples/counter --compress=gzip,br
```

Production servers must return the matching `Content-Encoding` when serving
precompressed files.

### Size

```bash
goxc size ./examples/counter
goxc size ./examples/counter/build
goxc size ./examples/counter/dist
```

The command prefers `dist/`, then `build/`, when passed an application path.

TinyGo package budgets can be checked after packaging the examples:

```bash
scripts/size-budget.sh
```

Pure runtime benchmarks and budgets:

```bash
scripts/perf-report.sh
```

### Serve

```bash
goxc serve ./examples/counter --port=8080
goxc serve --dir=./examples/counter/dist --port=8080
```

The local server correctly serves `.wasm` as `application/wasm`. It does not
perform gzip or brotli content negotiation.

### Clean

```bash
goxc clean ./examples/counter
goxc clean ./examples/counter --generated
```

The default removes `build/` and the manifest output directory. Generated
`.gox.go` files are removed only with `--generated`.

### Doctor and version

```bash
goxc version
goxc doctor
goxc help
```

`doctor` checks Go, optional TinyGo, gzip, brotli, `wasm_exec.js`, the working
directory, and temporary build storage.

## Project manifest

`goframe.json` is optional. Without it, `goxc` uses conservative defaults.
Unknown manifest fields are rejected so misspelled toolchain settings fail
early.

```json
{
  "name": "counter",
  "entry": ".",
  "output": "dist",
  "compiler": "tinygo",
  "wasm": "main.wasm",
  "assets": [
    "index.html",
    "service-worker.js"
  ]
}
```

CLI flags override manifest compiler and output choices.

## Size experiment

Measured on June 16, 2026 with Go 1.24.4 and TinyGo 0.41.1:

| Artifact | Bytes | Approximate size |
|---|---:|---:|
| Counter, Go `main.wasm` | 1,928,333 | 1.8 MiB |
| Counter, TinyGo `main.wasm` | 76,767 | 75.0 KiB |
| Components demo, Go `main.wasm` | 1,942,473 | 1.9 MiB |
| Components demo, TinyGo `main.wasm` | 82,066 | 80.1 KiB |
| Todo demo, Go `main.wasm` | 2,007,086 | 1.9 MiB |
| Todo demo, TinyGo `main.wasm` | 101,752 | 99.4 KiB |
| Go `wasm_exec.js` | 16,992 | 16.6 KiB |
| TinyGo `wasm_exec.js` | 16,715 | 16.3 KiB |

MVP 8.1 removed `reflect.DeepEqual` and production debug probes from the
runtime. Compared with the MVP 8 regression, TinyGo decreased from
`111,148 / 116,447 / 135,344` bytes to `76,767 / 82,066 / 101,752` bytes for
counter, components, and Todo.
Counter remains an integration probe rather than a representative application
benchmark.

## Legacy CLI

The CLI moved from `cmd/goframe` to `cmd/goxc`. The old command path was
removed:

```text
go run ./cmd/goframe ...   removed
goxc ...                   preferred
```

`goxc build --release` is accepted temporarily but only prints a deprecation
warning. It no longer packages or compresses; use `goxc package`.

## When should I use goframe?

Use goframe for experiments with Go-first dashboards, admin panels,
local-first apps, visual editors, developer tools, internal tools, and future
desktop-like runtimes.

## When should I not use goframe?

Do not use goframe for tiny static websites, landing pages, blogs, or pages
where a few kilobytes of JavaScript would be enough. Do not use the current MVP
where stable APIs, mature accessibility tooling, SSR, or hydration are
required.

## Current limitations

- Minimal mounted-tree and component reconciliation; no concurrent or
  lifecycle system.
- One mounted app and one browser thread. State is component-scoped and
  positional within each component, so hook order must remain stable.
- There is no automatic props memoization. Components encountered during a
  parent subtree update rerender.
- Duplicate key diagnostics are debug-only and do not run in production builds.
- GOX has no dedicated loop/conditional syntax or spread props; use Go-native
  runtime helpers.
- No routing, SSR, hydration, CSS-in-Go, or hot reload.
- TinyGo compatibility remains version- and feature-dependent.
- The local server is for development, not production deployment.

## Documentation

- [Architecture and toolchain boundaries](docs/architecture.md)
- [Foundation audit](docs/foundation-audit.md)
- [Component identity strategy](docs/component-identity.md)
- [GOX language and component model](docs/gox-language.md)
- [Runtime model](docs/runtime-model.md)
- [VS Code GOX extension](extensions/vscode-gox/README.md)
- [Future GoFrame Player vision](docs/player-vision.md)
- [Counter example](examples/counter/README.md)
- [Components example](examples/components/README.md)
- [Todo example](examples/todo/README.md)

## Development

```bash
GOCACHE=/tmp/goframe-go-cache go fmt ./...
GOCACHE=/tmp/goframe-go-cache go test ./...
GOCACHE=/tmp/goframe-go-cache go vet ./...

go install ./cmd/goxc
goxc doctor
goxc package ./examples/counter
goxc serve ./examples/counter
scripts/size-budget.sh
scripts/perf-report.sh

# Optional headless Chrome regression gate.
scripts/browser-smoke.sh

cd extensions/vscode-gox
npm install
npm run compile
```

## License

goframe is licensed under the Apache License, Version 2.0.

See [LICENSE](LICENSE) and [NOTICE](NOTICE).
