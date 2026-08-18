# Evaluator Guide

This guide is for evaluating the `v0.4.0-preview.1` browser/WASM layer. GitHub
Releases remains authoritative for whether that exact preview is published. It
is not a production deployment guide.

## Prerequisites

The release validation baseline is:

- Go `1.22.12`, `1.25.12`, and `1.26.5`, with Go `1.26.5` recommended for the
  guided path;
- TinyGo 0.41.1 for size-oriented WASM packages;
- Node.js `24.18.1` for docs, extension, and browser smoke scripts;
- Chrome `149` or the repository's current documented Chrome/Chromium
  equivalent for local browser smoke;
- gzip, brotli, and zstd for package size/compression checks.

Firefox and Safari are not part of the current CI evidence. macOS and Windows
have minimal Go/toolchain CI coverage; TinyGo/browser smoke evidence is
Linux/Chrome-first.

## Install goxc

Git tags and GitHub Releases determine availability of the exact preview.
Install it with:

```bash
go install github.com/graybuton/goframe/cmd/goxc@v0.4.0-preview.1
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

## Evaluate Package Inspection

Create one current standalone package, then inspect its declared graph:

```bash
goxc package ./examples/counter \
  --compiler=tinygo \
  --asset-hash \
  --preload \
  --compress=gzip,br

goxc inspect ./examples/counter
goxc inspect ./examples/counter --format=json
```

The text report is human-oriented. The JSON report is a separate schema-v1
tooling process contract; it does not change the schema-v1
`asset-manifest.json` or `goframe-package.json` files. Repeating the JSON
command against unchanged package bytes should produce byte-identical output.

Inspection reads one existing current standalone package. It validates the
declared artifacts, entrypoints, compressed sidecars, hashes, paths, and
completion marker without building, repairing, or mutating the package. It is
not a source dependency graph, bundle splitter, or legacy-package reader.

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

## Evaluate The Development Loop

Run the watched full-package workflow from a repository checkout:

```bash
goxc dev ./examples/counter --compiler=tinygo --port=8080
```

Open the printed loopback URL, edit `examples/counter/app.gox`, and save. A
successful full-package rebuild activates one verified generation and reloads
the page. Introduce a temporary GOX or Go compiler error to confirm that the
terminal reports the failure, the connected page presents it, and the previous
successful generation remains served and interactive. Correct the source to
confirm that the presentation clears before recovery reload.

This is serialized full-package rebuilding with full-page reload. It is not
incremental compilation or HMR. Initial package failures and watch or scan
failures remain terminal-only. The generated compiler workspace does not
inherit a parent `go.work` or ambient `GOFLAGS`.

## Evaluate The Server-Backed Reference

The server-backed example keeps the backend in ordinary Go `net/http` and the
application in packaged browser/WASM output:

```bash
goxc package ./examples/server-backed --compiler=go
go run ./examples/server-backed/cmd/server \
  --package=./examples/server-backed/.goframe/package/standalone \
  --addr=127.0.0.1:8080
```

Use the URL printed by the server. The fixture demonstrates same-origin data,
failed and stale request handling, saved mutations, and explicit application
ownership without adding a GoFrame server, loader, transition, or mutation API.

## Task-Based Evaluation

This guide is a guided walkthrough of an existing repository checkout. To
evaluate independent application construction with the published preview, give
participants the [GoFrame Task-Based Evaluation](task-based-evaluation.md)
brief instead. Facilitators should retain the separate
[facilitator protocol](task-based-evaluation-facilitator.md), distribute only
the participant brief, and avoid exposing observation prompts as implementation
hints during the task.

For each study series, facilitators must pin exact revisions for the study kit
and product documentation/examples, record those revisions for every session,
and avoid moving `main` links as study material. All participants in one series
should receive the same immutable snapshots.

No external task-based results are recorded yet. A maintainer mechanical pilot
may verify the instructions, but it is not independent participant evidence or
an adoption claim.

The existing task-based study kit remains intentionally pinned to
`v0.2.0-preview.6` and its recorded immutable snapshots. Do not substitute this
current guide for those pinned participant or facilitator materials within an
existing study series.

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
scripts/check.sh
scripts/browser-smoke.sh
scripts/size-budget.sh
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
- `goxc inspect` reports one declared current standalone package; it does not
  infer source dependencies or produce a split bundle graph.
- `goxc dev` performs full package rebuilds and full-page reload. Browser
  presentation covers post-start package and build failures; initial package
  failures and watch or scan failures remain terminal-only. There is no HMR,
  incremental compilation, or source-map contract.
- Production deployment infrastructure, TLS, cache negotiation, and server
  fallback rules are outside this preview.

For the guided walkthrough, read the [tutorial](tutorial.md). For release scope
and limitations, read the
[v0.4.0-preview.1 release notes](release-notes-v0.4.0-preview.1.md).
