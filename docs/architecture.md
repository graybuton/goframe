# Architecture

## Platform boundaries

goframe is not intended to be a tiny replacement for React on static websites.
goframe is an experimental Go-first application platform for interactive apps.

The project now separates four concepts:

```text
GOX language
   declarative UI syntax, function components, typed props, and fragments

goframe runtime
   application state, nodes, browser DOM mounting, and events

goxc toolchain
   source generation, compilation, packaging, inspection, and local serving

VS Code GOX extension
   syntax highlighting, snippets, language configuration, and goxc commands

GoFrame Player / Engine
   a possible future host for portable application bundles
```

`goframe` is the library imported by applications. `goxc` is the installed
developer tool. This distinction avoids treating the runtime package as a CLI
product.

The VS Code extension is deliberately a thin developer-experience layer over
GOX and `goxc`. TextMate scopes provide heuristic highlighting, while terminal
commands invoke the same installed CLI used outside the editor. Semantic
analysis, formatting, and diagnostics remain future LSP responsibilities.

## Toolchain responsibilities

### Generate

```text
.gox source -> generated .goframe/gen/*.gox.go
```

Generation is deterministic source transformation. Generated files remain next
to the app in a hidden toolchain workspace, not next to authored source files.
`goxc build` and `goxc package` materialize a build workspace that normal Go
and TinyGo compilers can consume.

The MVP component model deliberately delegates type checking to Go:

- lowercase tags generate `gf.El`;
- capitalized tags generate `gf.ComponentT` boundaries using generated
  `gf.ComponentType` tokens and `<Name>Props`;
- package-qualified capitalized tags such as `<ui.Header />` generate the same
  kind of boundary with the selected package alias, imported props type, and
  import-path-aware component identity when available;
- component children populate `Children []gf.Node`;
- fragments generate `gf.Fragment`;
- child expressions generate `gf.Child`.
- GOX render expressions such as `condition && <Node />` and
  `condition ? <A /> : <B />` lower to runtime conditional primitives.
- `Key={...}` is a pseudo-prop that lowers to `gf.Key` instead of entering
  component props or DOM attributes.
- nested GOX markup in callback `return` expressions is rewritten before the
  generated Go is formatted.

Component bodies remain ordinary typed Go functions. The generated boundary
lets the runtime defer their call, preserve component identity, own state
slots, and update a dirty component subtree independently.

### Build

```text
materialized Go source -> raw .goframe/build/<compiler>/<profile>/bundle.wasm
```

Build only compiles. It does not copy HTML, create a distribution, or generate
gzip/brotli files. Both Go and TinyGo targets use the same raw output contract.

### Package

```text
application + selected compiler -> runnable .goframe/package/standalone bundle
```

Packaging compiles the selected target and combines `assets/bundle.wasm`, the
matching runtime shim, declared static assets, generated `asset-manifest.json`,
and generated `goframe-package.json`. Compiler-specific runtime names are
normalized to `assets/bundle.wasm` and `assets/wasm_exec.js`.

Packaging prepares artifacts in a staging directory before publishing them to
`.goframe/package/standalone`. This keeps failed compile/copy/compression
steps from damaging the currently runnable package and keeps the authored app
directory free of visible generated files.

Precompression is optional packaging assistance, never default compiler
behavior:

```bash
goxc package ./app --compress=gzip,br
```

Compression and content negotiation primarily belong to deployment
infrastructure: web servers, CDNs, and reverse proxies.

Release-style packages can opt into content-hashed asset filenames and preload
hints:

```bash
goxc package ./app --asset-hash --preload --compress=gzip,br
```

See `docs/deployment.md` for cache-safe package delivery and
`docs/manifest-compatibility.md` for manifest/package compatibility policy.

Use `goxc export ./app --out ./dist` to copy the latest standalone package to a
deployment directory. Export is intentionally explicit so normal build/package
commands do not create visible `dist/` output. Export destinations are treated
as tool-owned: non-empty directories without GoFrame export manifests are
rejected unless `--force` is passed.
Explicit `goxc package --out <dir>` is also treated as package-owned output and
is rejected when `<dir>` is a non-empty non-GoFrame directory.

### Serve

`goxc serve` is a small development server for a packaged directory. It serves
WASM with `application/wasm`, but intentionally does not attempt to be a
production deployment system.

By default, `serve <app>` serves `.goframe/package/standalone`. `serve --dir`
continues to serve an explicit exported directory.

## Project manifest

An optional `goframe.json` describes application defaults:

- application name;
- Go package entry;
- package output directory;
- preferred compiler;
- normalized WASM filename, defaulting to `bundle.wasm`;
- static assets copied by packaging. The recommended preview shape is an
  asset directory such as `"assets": "./assets"`; legacy explicit path lists
  remain supported.

CLI flags override manifest defaults. Paths in the manifest must remain inside
the application directory. Unknown manifest fields are rejected so typos fail
early instead of silently falling back to defaults.

The hidden workspace builder supports `"entry": "."` apps and child entry
packages such as `"./cmd/app"`, `"cmd/app"`, `"./src/app"`, and `"app"` when
they stay inside the app root. During build/package it materializes a
module-root mirror under `.goframe/work/<profile>` so generated `.gox.go`
files can sit beside the corresponding package sources without polluting the
authored tree. GOX discovery remains app-root-wide even when the executable
entry is a child package.

## Build targets

### Standard Go compatibility mode

The standard Go compiler supports the broadest language and standard-library
surface. Its runtime includes garbage collection, scheduling, stack
management, panic machinery, and type metadata, producing a comparatively
large counter binary.

Recorded Go 1.24.4 output on June 16, 2026:

```text
counter bundle.wasm     1,928,333 bytes
components bundle.wasm  1,942,473 bytes
todo bundle.wasm        2,007,086 bytes
```

### TinyGo lightweight mode

TinyGo is the preferred lightweight experiment. It supports a smaller runtime
surface but dramatically reduces the counter:

```text
counter bundle.wasm          83,550 bytes
components bundle.wasm       89,198 bytes
todo bundle.wasm            117,409 bytes
dashboard bundle.wasm       168,628 bytes
context bundle.wasm         115,354 bytes
virtualized bundle.wasm     123,144 bytes
multipackage bundle.wasm     94,354 bytes
cmdapp bundle.wasm           94,380 bytes
router bundle.wasm          114,716 bytes
router-dashboard bundle.wasm 225,649 bytes
resource bundle.wasm        147,673 bytes
```

MVP 8.1 removed reflective props comparison and compiles browser
instrumentation only under the `goframe_debug` build tag. MVP 9 adds lifecycle
hooks with about 1 KiB of runtime growth on the smaller examples. MVP 10 adds
GOX expression ergonomics without meaningful raw WASM growth. Todo is larger
than Counter because it also demonstrates compact localStorage persistence.

The repository includes `scripts/size-budget.sh` as a regression gate for raw,
gzip, brotli, and optional zstd packaged TinyGo examples, including the
dashboard-sized pressure-test example, the context selector example, and the
virtualized collections, multi-package workspace, child-entry workspace,
hash-router, router-dashboard reference, and resource loading examples.
`scripts/perf-report.sh` runs pure runtime benchmarks plus the same size
budgets, and `scripts/browser-smoke.sh` runs the optional headless Chrome
regression probes.

The current target keeps TinyGo's scheduler enabled because the example and
browser event runtime keep `main` alive. A scheduler-free profile requires a
different lifetime model.

## Why counter remains a poor benchmark

Counter is valuable as an integration test: it proves GOX generation, state,
events, browser startup, compilation, packaging, and serving. It is not a good
measure of platform value because it contains almost no application behavior.

`examples/dashboard` is the first dashboard-sized pressure test. It keeps all
components in one Go package, but splits layout, metrics, filters, table, and
detail components across multiple GOX files. It models 300 deterministic rows
and exercises search, filters, sorting, keyed row identity, selection, metric
updates, and a small document-title effect. The table is physically
virtualized with `gf.VirtualTable`, so the logical row count can stay
dashboard-sized while the mounted DOM remains bounded.

`examples/router-dashboard` is the flagship reference-grade integrated app. It
uses a child entry package, internal packages, the hash router, query-driven
filters, one component-scoped resource owner for packaged issue data,
controlled form state, synchronous validation, scoped render Error Boundary
composition, and stable shell composition without adding route loaders, a
global cache, server resources, or a schema validation library.

`examples/resource` is the first resource-loading probe. It uses
component-scoped `gf.UseResource` with example-local browser fetch, text
parsing, delayed responses, and abort cleanup. It intentionally does not add a
runtime fetch API, JSON helper, global resource cache, Suspense model, or route
loader framework.

Future editor-sized experiments should measure startup, update time, memory,
compressed transfer size, and development ergonomics before broader conclusions
are drawn.

## Runtime model

The MVP runtime currently has:

- one mounted application;
- one browser thread;
- explicit `ComponentNode` boundaries generated by GOX capitalized tags;
- component instances identified by typed component identity or legacy name,
  plus key and sibling position;
- component-scoped positional state slots, including the root `App`;
- reducer dispatch that applies actions to the latest component state slot;
- scoped context providers with selector-based consumer dirtying;
- component-scoped lifecycle/effect slots;
- component-scoped explicit-state resources;
- fixed-height `VirtualList` and `VirtualTable` collection primitives;
- hash-based client routing through `RouterView`, `RouterLink`, and Go-declared
  routes;
- dirty component updates coalesced into one animation-frame flush;
- direct dirty-owner subtree updates without root traversal;
- dirty queue ancestor pruning when parent and child are dirty in the same
  flush;
- fragment nodes rendered through DOM `DocumentFragment`;
- empty, conditional, list, and keyed node primitives;
- expression children accepting primitives, `Node`, or `[]Node`;
- a retained mounted tree and minimal DOM patch layer;
- text and element prop patching;
- positional unkeyed children and key-based child reuse/movement;
- typed event facades for input and form interaction;
- stable event listeners with replaceable current callbacks;
- optional browser probes compiled only with `goframe_debug`;
- debug-only duplicate key warnings compiled only with `goframe_debug`.
- debug-only lifecycle warnings for Set-after-unmount, Set-during-render, and
  effect update-loop guard trips.

Direct function calls remain possible but do not create a component boundary.
The single-thread assumption must be revisited before worker-driven updates.
See [component identity](component-identity.md) for the typed generated-token
strategy, legacy string identity compatibility, and import-path-aware GOX
identity limits.
See [lifecycle and effects](effects.md) for the MVP 9 side-effect model.

## Limitations

- Minimal reconciliation only; no Fiber, concurrent rendering, or priorities.
- Component state slots are positional and require stable call order.
- Lifecycle/effects are minimal; there is no Fiber scheduler or error-boundary
  model.
- Context is scoped and selector-based, but has no async/server bridge or
  custom non-comparable selector equality.
- Virtualized collections are fixed-height only; there is no dynamic
  measurement, infinite loading, or advanced keyboard navigation yet.
- No automatic props memoization; descendants rerender with their parent unless
  components explicitly implement `MemoEqual` on their props and the framework can
  perform a deterministic memoized bailout.
- Duplicate key diagnostics are debug-only.
- GOX has expression-oriented conditional rendering, but no template-block
  loops/conditionals, spread props, or component namespaces.
- No spread props or component namespaces.
- Hash routing only; no path/history-mode server fallback, file-based routing,
  route loaders, SSR, hydration, or accessibility abstraction.
- TinyGo compatibility is experimental and feature-dependent.
- `goxc serve` is a development server without compression negotiation.
- Debug-tag performance probes are observations, not rigorous benchmarks.

## Roadmap

1. Keep public docs, examples, and regression gates aligned with the actual
   toolchain surface.
2. Harden component identity and workspace behavior before claiming reusable
   package ecosystem support.
3. Add deeper symlink/path-safety tests before public preview.
4. Continue measuring size, DOM pressure, and browser smoke invariants before
   taking on larger features.
5. Explore path/history routing, Player/Engine, SSR/hydration, and `.gfapp`
   only as separate design efforts, not incidental cleanup.
