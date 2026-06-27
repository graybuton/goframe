# Release Notes: v0.1.0-preview.1

Status: draft release notes. No `v0.1.0-preview.1` tag is created by this
document.

## Summary

GoFrame is an experimental Go-first application platform. This preview
validates the current browser/WASM interactive application layer: the
`pkg/goframe` runtime, GOX generation, the `goxc` toolchain, static packaging,
examples, docs, and CI evidence.

The preview scope is narrower than the project vision. Router, resources, Error
Boundaries, package/export workflow, and the reference app are real working
surfaces. Player/Engine, broader host/runtime targets, richer editor tooling,
and a package ecosystem remain outside this preview promise.

## Preview Scope

This preview is for evaluators who want to inspect and try the current
browser/WASM layer:

- Go-authored interactive browser apps;
- GOX components and package-qualified component tags;
- component-scoped state, effects, context, memoization, fixed-height
  virtualization, resources, and hash routing;
- static `goxc package` output with generated package metadata;
- focused examples plus the router-dashboard reference app;
- Linux/Chrome browser smoke evidence and minimal macOS/Windows Go/toolchain
  evidence.

It is not a production-ready release and does not create a stable 1.0 API
promise.

## What Is Included

- `pkg/goframe`: nodes, typed component identity, hooks, context, events,
  virtualized collections, hash router, component-scoped resources, runtime
  error reporting, and scoped render Error Boundaries.
- `pkg/gox`: GOX parsing/code generation, source-oriented diagnostics,
  package-qualified component tags, golden/error golden tests, and bounded fuzz
  seeds.
- `cmd/goxc`: generate, build, package, export, serve, size, clean, doctor, and
  version commands.
- Package workflow: versionless `goframe.json`, recommended
  `"assets": "./assets"`, legacy explicit asset lists, generated or custom root
  `index.html`, versioned `asset-manifest.json`, and authoritative
  `goframe-package.json` completion metadata.
- Examples: quickstart, focused primitives, toolchain/layout examples,
  `examples/router-dashboard` as the reference app, and `examples/dashboard` as
  the pressure/performance example.

## How To Try It

Install the current toolchain:

```bash
go install github.com/graybuton/goframe/cmd/goxc@latest
goxc doctor
```

From a local checkout, install the local `goxc` instead:

```bash
go install ./cmd/goxc
goxc doctor
```

Run the quickstart:

```bash
goxc package ./examples/counter --compiler=tinygo
goxc serve ./examples/counter --port=8080
```

Run the reference app:

```bash
goxc package ./examples/router-dashboard --compiler=tinygo
goxc serve ./examples/router-dashboard --port=8080
```

For the intentional ErrorBoundary panic demo in the reference app, use the
recover-capable Go/WASM package path:

```bash
goxc package ./examples/router-dashboard --compiler=go
goxc serve ./examples/router-dashboard --port=8080
```

See the [tutorial](tutorial.md) for the guided reference-app walkthrough.

## Compatibility Notes

- Public-Candidate API shapes are documented in
  [API stability](api-stability.md). They remain pre-1.0 and can change with
  migration notes.
- Some exported API shapes are public-candidate while deeper lifecycle,
  routing, fallback, and edge-case semantics remain Experimental Frontier.
- Generated typed component identity is stable enough for one app/module tree.
  Broad reusable multi-module package identity is not promised by this preview.
- `goframe.json` remains versionless for `v0.1.0-preview.1`.
- `assets: "./assets"` is the recommended static asset form. Legacy explicit
  asset lists remain supported.
- Package root `index.html` is always produced: selected custom templates are
  rewritten, and `goxc package` generates a default entrypoint when no custom
  template exists.
- Generated `asset-manifest.json` and `goframe-package.json` are versioned
  tooling metadata. `goframe-package.json` is the authoritative current package
  completion marker.

## Known Limitations

- Browser/WASM DOM target only.
- Linux/Chrome is the strongest browser evidence. Firefox and Safari are
  unverified in current CI evidence.
- macOS and Windows have minimal Go/toolchain CI evidence, not full browser or
  TinyGo smoke coverage.
- TinyGo size-oriented builds use trap-style panic behavior by default.
  Recover-based ErrorBoundary demos use Go/WASM.
- Resources are component-scoped. There is no global cache, deduplication,
  automatic retry, Suspense behavior, route loader, or runtime fetch API.
- The router is hash-based. History-mode routing, file-based routing,
  middleware, route guards, and production fallback automation are outside this
  preview.
- Package publication is metadata-last and fail-closed, but not a transactional
  rollback installer.
- `goxc serve` is development-only and not a production static server.

## Validation Evidence

Current repository evidence includes:

- `go test ./...`;
- race tests for `./pkg/... ./cmd/...`;
- `go vet ./...`;
- debug-tag tests;
- GOX golden/error golden tests;
- bounded GOX fuzz seed targets through normal tests;
- TinyGo size budgets;
- browser smoke on Linux/Chrome;
- dashboard DOM pressure checks;
- artifact/module path gates;
- docs consistency checks;
- VS Code extension compile checks.

See [CI and regression gates](ci.md), [platform support](platform-support.md),
and [public preview readiness](public-preview-readiness.md) for the current
evidence matrix.

## Non-Goals

`v0.1.0-preview.1` does not include:

- production readiness;
- stable 1.0 compatibility;
- SSR or hydration;
- history-mode router or server fallback automation;
- route loaders, middleware, auth guards, or route-level data APIs;
- Suspense, server resources, or a global resource cache;
- schema validation framework or mutation framework;
- production deployment server;
- LSP/formatter;
- Player/Engine or `.gfapp` packaging.
