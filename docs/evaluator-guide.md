# Evaluator Guide

This guide is for evaluating the `v0.2.0-preview.6` browser/WASM layer. It is
not a production deployment guide.

## Prerequisites

Install:

- Go 1.22 or newer;
- TinyGo 0.41.1 for size-oriented WASM packages;
- Node.js 20 or newer for docs and browser smoke scripts;
- Chrome or Chromium for local browser smoke;
- gzip, brotli, and zstd for package size/compression checks.

Firefox and Safari are not part of the current CI evidence. macOS and Windows
have minimal Go/toolchain CI coverage; TinyGo/browser smoke evidence is
Linux/Chrome-first.

## Install goxc

For the published module path:

```bash
go install github.com/graybuton/goframe/cmd/goxc@latest
goxc doctor
```

Inside a checkout:

```bash
go install ./cmd/goxc
goxc doctor
```

From a repository checkout, verify the structured diagnostics path:

```bash
goxc check ./examples/counter --format=json
```

A clean counter check returns one schema-v1 JSON document with:

```text
schemaVersion: 1
ok: true
diagnostics: []
```

## Recommended Evaluation Path

Start with the smallest example, then move to the integrated reference app:

```text
examples/counter
  -> examples/components
  -> examples/router-dashboard
```

Package and serve the quickstart:

```bash
goxc package ./examples/counter --compiler=tinygo
goxc serve ./examples/counter --port=8080
```

Package and serve the reference app:

```bash
goxc package ./examples/router-dashboard --compiler=tinygo
goxc serve ./examples/router-dashboard --port=8080
```

Open <http://127.0.0.1:8080>.

The reference app demonstrates the current integrated browser/WASM story:
stable shell, hash router, URL query filters, one component-scoped resource
owner, explicit loading/failed UI, controlled form validation, and a scoped
render Error Boundary.

For the intentional ErrorBoundary panic route, use Go/WASM:

```bash
goxc package ./examples/router-dashboard --compiler=go
goxc serve ./examples/router-dashboard --port=8080
```

Then visit:

```text
#/issues/RD-2?panic=render
```

TinyGo's size-oriented trap behavior is not the containment proof path for this
demo.

## Package A Small App

The recommended preview manifest shape is:

```json
{
  "entry": "./cmd/app",
  "compiler": "tinygo",
  "assets": "./assets"
}
```

`goxc package` writes a runnable standalone package under:

```text
<app>/.goframe/package/standalone/
```

The package contains root `index.html`, generated metadata, and static files
under package `assets/`. If no custom HTML template is selected, `goxc package`
generates a default `index.html`.

Use `goxc export` only when you want a visible deploy directory:

```bash
goxc package ./examples/counter --compiler=tinygo --asset-hash --preload --compress=gzip,br
goxc export ./examples/counter --out ./dist
```

## Local Checks

Core checks:

```bash
node scripts/docs-check.mjs
go test ./...
go test ./pkg/gox -run 'TestGolden|TestErrorGolden'
```

Fuller local evidence, when TinyGo and Chrome are available:

```bash
scripts/size-budget.sh
scripts/browser-smoke.sh
```

The browser smoke covers focused examples and the router-dashboard reference
app. It is not a cross-browser certification suite.

## What To Look For

- GOX components should preserve normal Go package structure.
- State, effects, context, resources, and route changes should remain
  component-scoped and explicit.
- Router-dashboard should load packaged data once across route/query changes
  and reload only through the explicit reload control.
- Resource failures should render explicit failed UI, not ErrorBoundary
  fallback.
- Render failures in the Go/WASM ErrorBoundary demo should keep the outer shell
  and resource owner mounted.
- Package output should be static-host friendly and should not require a
  production server from GoFrame.

## Current Limits To Expect

- The current target is browser/WASM only.
- Hash routing is the documented router path.
- Error Boundaries are render-only and recover-based.
- Resources are component-scoped and do not include cache, dedupe, Suspense, or
  route loaders.
- `goxc serve` is development-only.
- Production deployment infrastructure, TLS, cache negotiation, and server
  fallback rules are outside this preview.

For the guided walkthrough, read the [tutorial](tutorial.md). For release scope
and limitations, read the
[v0.2.0-preview.6 release notes](release-notes-v0.2.0-preview.6.md).
