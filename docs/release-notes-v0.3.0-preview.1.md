# GoFrame v0.3.0-preview.1

## Status / Scope

`v0.3.0-preview.1` is an experimental browser/WASM pre-release and an
application-workflow and development-loop checkpoint. It covers the merged
changes after `v0.2.0-preview.6`, from
`9548345776e6398cd70e8fc58435dd5dab687c7d` through the release baseline at
`c9983314aec7bc8bc8eb08a9687681da2c24cb2b`.

The validated surface remains interactive browser/WASM applications built with
the GoFrame runtime, GOX, `goxc`, and standard Go or TinyGo WebAssembly output.
This preview adds a watched full-package development loop, strengthens runtime
and toolchain ownership boundaries, and records integrated application evidence
without adding public route-loader, transition, mutation, document-metadata, or
history-routing APIs.

This versioned document records the `v0.3.0-preview.1` release contract. Git
tags and GitHub Releases are authoritative for publication and availability.
GoFrame remains pre-1.0 and experimental. This preview is not a
production-readiness, fullstack, SSR, hydration, or broad browser-support claim.

## Highlights

### Watched Development Loop

- `goxc dev <app-directory>` packages an application, serves only a verified
  completed generation on `127.0.0.1`, watches effective authored inputs, and
  performs serialized full-package rebuilds.
- A failed rebuild preserves the last successful generation. A later
  successful rebuild activates a complete replacement generation and reloads
  connected pages.
- The command provides full-page reload, not HMR, incremental compilation, or
  an in-browser compiler-error overlay. Compiler and generation failures remain
  terminal output.
- Generated-workspace compiler invocations now isolate the app from a parent
  `go.work` and ambient `GOFLAGS`. Application dependencies must be represented
  by the app module's `require` and `replace` directives.

### Application Workflow Evidence

- The server-backed reference app now exercises route-like async data,
  transition-style request ownership, saved mutations, stale-result rejection,
  failed request recovery, document-state ownership, and unmount cleanup using
  existing public primitives and example-local coordination.
- History-routing browser evidence characterizes direct navigation, refresh,
  subpath fallback, asset/API 404 boundaries, browser history, and listener
  cleanup. Hash routing remains the documented public router contract; no
  history-router API or server fallback automation was added.
- Integrated browser evidence covers direct and repeated application
  replacement, controlled selects, context-selector topology changes, state and
  reducer render transactions, protected ErrorBoundary teardown, generated
  package reload, and document ownership pressure.

### GOX And Toolchain Correctness

- `pkg/gox` adds coordinated package generation through
  `GeneratePackageWithOptions`, `PackageGenerateOptions`, and `PackageSource`.
  These exports are compiler-facing and experimental, not a package loader or
  build-system API.
- Package-aware generation reserves authored declarations and allocates
  deterministic private component identifiers across one supplied Go package.
- GOX validation rejects duplicate props, normalized HTML attribute and event
  collisions, and explicit `Children` combined with renderable nested content.
  Valid audited examples retain byte-identical generated Go.
- Generated-source publication stages coordinated outputs and restores the
  previous managed set after detected publication failures.
- `go:embed` inputs are discovered and materialized through the generated
  workspace while preserving authored source selection and symlink policy.
- Adjacent generated-file cleanup now revalidates ownership, filesystem
  identity, and content immediately before removal.

### Runtime Correctness

- Recover-capable renders stage new state slots and reducer replacements until
  commit. Discarded slots do not become live, and committed dispatch closures
  keep the latest successfully committed reducer.
- Context selectors rebind to the new nearest provider even when selector
  evaluation fails, allowing a later safe provider update to recover.
- Controlled select values are restored after option reconciliation.
- Repeated `gf.Mount` calls release the previous application, remove its exact
  mounted range, preserve unrelated host siblings when moving roots, and reject
  a different target inside the active root before teardown.
- ErrorBoundary and lifecycle fixes preserve cleanup ordering, protected
  subtree ownership, runtime reporting, and replacement behavior across the
  focused standard-Go and TinyGo evidence boundaries.

### Toolchain Baseline And Private Research

- The supported release validation baseline is Go `1.22.12`, `1.25.12`, and
  `1.26.5`; Node.js `24.18.1`; TinyGo `0.41.1`; and Chrome/Chromium browser
  evidence on Linux.
- All eleven ordinary TinyGo applications remain covered by raw, gzip, Brotli,
  and Zstandard absolute ceilings and the existing compression-ratio limits.
  Relative to `v0.2.0-preview.6`, absolute ceilings were evidence-backed
  rebaselined for seven applications across fifteen cells to reflect the
  supported toolchain baseline and measured runtime correctness costs.
  Compression-ratio limits remained unchanged, and this release-preparation
  branch changes no size budget. See the
  [WASM size headroom audit](wasm-size-headroom-audit.md) for per-stage
  measurements and attribution.
- Private document-state research confirmed that a bounded transactional
  ownership handoff is technically viable. A follow-up comparison did not
  select a public hook, component, or owner-handle API. The experiment remains
  build-tagged and absent from ordinary builds.

## Compatibility

- `pkg/goframe` has no exported API additions or removals in this release
  range.
- `pkg/gox` adds the three compiler-facing package-generation exports named
  above. Existing exported identifiers remain present.
- `goxc dev` is additive. Help output for the previous `check`, `generate`,
  `build`, `package`, `export`, `serve`, `size`, `clean`, `doctor`, and
  `version` commands is unchanged in the release-range comparison.
- Valid audited GOX applications generate byte-identical Go. Newly rejected
  sources contain duplicate or effectively colliding destinations that could
  previously produce duplicate Go fields or ambiguous DOM behavior.
- The standalone package layout, `asset-manifest.json` schema,
  `goframe-package.json` completion marker, hashed entrypoints, preload hints,
  and gzip/Brotli sidecars remain compatible in the representative package
  comparison.
- `go.mod`, `go.sum`, and the VS Code extension lockfile are unchanged across
  the release range.
- Two user-visible safety changes can require action: generated-workspace
  compiler environment isolation and rejection of nested repeated-Mount
  targets. See the
  [v0.3.0-preview.1 migration notes](migration-v0.3.0-preview.1.md).

## Validation

The release baseline passed the local release gate with:

- ordinary, `goframe_debug`, and vet lanes on Go `1.22.12`, `1.25.12`, and
  `1.26.5`;
- race lanes on Go `1.25.12` and `1.26.5`;
- focused GOX golden and error-golden tests;
- `scripts/check.sh`, `scripts/artifact-check.sh`,
  `scripts/module-path-check.sh`, `scripts/size-budget.sh`, docs checks,
  `actionlint`, and `git diff --check`;
- two complete Chrome Browser Smoke runs;
- the VS Code extension compile and Node tests under Node.js `24.18.1`, with
  its lockfile unchanged;
- a focused Windows amd64 compile of the `cmd/goxc` test package;
- representative TinyGo packages for counter and router-dashboard and a
  standard-Go package for server-backed, each with asset hashing, preload
  hints, matching manifests and completion metadata, and verified gzip/Brotli
  sidecars.

Publication requires the release pull request to pass the normal Core, Browser
Smoke, WASM Size, and VS Code Extension workflows. The local evidence above
does not substitute for those workflows.

## Known Limitations And Follow-Ups

- `goxc dev` uses serialized full-package rebuilds and full-page reload. It does
  not provide HMR, incremental compilation, source maps, or a browser build
  error overlay.
- Hash routing remains the public router path. History/path routing requires an
  explicit deployment fallback and remains evidence, not a selected API.
- Async navigation, mutation, cache invalidation, and document metadata remain
  application-local patterns or private research. No public API shape was
  selected for them.
- Standard Go/WASM supplies recover-based failure and ErrorBoundary evidence.
  TinyGo's normal trap-style panic mode covers successful paths only.
- Chrome/Chromium remains the strongest browser evidence. Firefox and
  Safari/WebKit do not have equivalent automated coverage.
- Static package output is not a production server, deployment platform, SSR,
  hydration, bundle-splitting, or fullstack framework.
- The VS Code extension remains local repository tooling and is not published
  to the Marketplace.
- Related issue [#117](https://github.com/graybuton/goframe/issues/117) remains
  an independent, non-blocking external evaluator task. No external submission
  from that issue is used as release evidence.
- Preview users can revert to `v0.2.0-preview.6` while evaluating a migration
  or regression. No stable 1.0 compatibility guarantee applies.

## Install

Install the exact preview with:

```bash
go install \
  github.com/graybuton/goframe/cmd/goxc@v0.3.0-preview.1
```

Availability of this exact command is determined by the published tag shown in
GitHub Releases.

## Verification

Run:

```bash
goxc version
goxc doctor
goxc check ./examples/counter --format=json
```

The first line from an exact tagged install should be:

```text
goxc version v0.3.0-preview.1
```

The counter check should return schema-v1 JSON with `ok: true` and no
diagnostics. For a packaged browser/WASM check:

```bash
goxc package ./examples/counter --compiler=tinygo \
  --asset-hash --preload --compress=gzip,br
goxc serve ./examples/counter --port=8080
```

For the watched development loop:

```bash
goxc dev ./examples/counter --compiler=tinygo --port=8080
```

## Links

- [README](../README.md)
- [Evaluator guide](evaluator-guide.md)
- [Migration notes](migration-v0.3.0-preview.1.md)
- [GOX language](gox-language.md)
- [API stability](api-stability.md)
- [Compatibility policy](compatibility.md)
- [Platform support](platform-support.md)
- [Release process](release.md)
- [Roadmap](roadmap.md)
- [Previous preview: v0.2.0-preview.6](release-notes-v0.2.0-preview.6.md)
- [Release-range comparison](https://github.com/graybuton/goframe/compare/v0.2.0-preview.6...v0.3.0-preview.1)

## Non-Goals

This preview does not add a public route transition, loader, action, mutation,
cache, document metadata, history-router, server, SSR, hydration, or fullstack
API. It does not select a public shape from private document-state research,
publish the VS Code extension, add HMR or an LSP, broaden browser support, or
establish stable 1.0 or production-readiness guarantees.
