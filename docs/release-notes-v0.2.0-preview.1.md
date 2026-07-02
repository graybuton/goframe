# GoFrame v0.2.0-preview.1 Release Notes

## Summary

`v0.2.0-preview.1` is a narrow evaluator preview for GoFrame's web-first
project direction. GoFrame is an experimental Go-first web application
framework and toolchain whose validated layer today is browser/WASM interactive
applications.

This preview records post-`v0.1.0-preview.2` evidence around external Go
component packages resolved through an app module's local `require` / `replace`
configuration. It also closes the public strategy wording around GoFrame's
active direction: browser/WASM, GOX, `goxc`, packaging/export, examples, and CI
evidence remain the current release surface; Player/Engine and `.gfapp` are
inactive directions.

Scope preview != scope project. This preview does not claim production
readiness, stable 1.0 APIs, shipped fullstack/server APIs, SSR/hydration,
Player/Engine, `.gfapp`, or a broad reusable package ecosystem.

## Highlights

- Web-first positioning closeout: GoFrame is described as an experimental
  Go-first web application framework and toolchain.
- Player/Engine, `.gfapp`, portable host/runtime packaging, desktop/mobile
  shells, and custom app engine work are marked inactive and outside the
  preview promise.
- External component package identity evidence now covers package-qualified GOX
  tags for external imports, import aliases as debug labels rather than
  identity, same-symbol package separation, and versioned import path
  separation.
- `goxc` build workspaces now preserve app module `require` / `replace`
  directives needed for ordinary Go package resolution, while rewriting local
  relative replace targets to the original module locations.
- Go/WASM build-flow evidence covers a local-replace external Go component
  package imported by an app module.
- Documentation and release checklists now keep preview promises tied to
  current evidence, with Player/Engine and broader package ecosystem claims
  explicitly scoped out.

## What Changed Since v0.1.0-preview.2

### Strategy and positioning

GoFrame's public-facing identity is now web-first: an experimental Go-first web
application framework and toolchain. The current validated layer remains
browser/WASM interactive applications with the `pkg/goframe` runtime, GOX,
`goxc`, static package/export workflows, examples, and CI/smoke/size evidence.

The Player/Engine direction is inactive. `.gfapp`, portable host/runtime
packaging, desktop/mobile shells, and a custom application engine are not part
of this preview contract.

Fullstack or Go backend integration is not a shipped feature in this preview.
It remains a staged project direction that needs dedicated evidence before it
can become a preview claim.

### External Go component package path

The external component package path now has executable characterization for
ordinary Go component packages outside the app module:

- generated GOX identity uses the external package import path plus component
  symbol, such as `example.com/ui.Card`;
- import aliases affect the debug label, not runtime identity;
- two external packages exporting the same symbol remain distinct;
- versioned import paths such as `example.com/ui/v2.Card` remain distinct;
- raw `.gox` files inside external dependencies are not generated or
  materialized by the current workspace model.

### Toolchain workspace behavior

`goxc` build workspace materialization now preserves app module dependency
information needed for normal Go package resolution:

- app module `require` directives for external packages are written into the
  workspace `go.mod`;
- app module `replace` directives are written into the workspace `go.mod`;
- local relative replace targets such as `../ui` are rewritten so they still
  point to the original module location after `go.mod` is materialized under
  `.goframe/work/<profile>`;
- GoFrame runtime dependency resolution remains controlled by the existing
  local checkout or released-module fallback behavior.

This is ordinary Go module resolution support for the app's dependencies. It is
not a broad multi-module monorepo model and does not copy external modules into
GoFrame's generated workspace.

### Tests and evidence

Post-`v0.1.0-preview.2` tests add evidence for:

- external package-qualified component identity generation;
- alias, same-symbol, and versioned import-path identity boundaries;
- app module `require` / `replace` preservation in workspace `go.mod`;
- local relative replace target rewriting;
- `go list -deps .` from the materialized workspace with network access
  disabled for the local-replace fixture;
- the actual Go/WASM `buildApp` path for an app importing a local-replace
  external Go component package;
- unchanged unsupported behavior for raw external dependency `.gox` files.

### Documentation and release contract

The release-facing docs now distinguish the web-first project direction from
inactive Player/Engine options and from unsupported fullstack/server claims.
Component identity and multi-package workspace docs describe the current
external Go component package evidence without turning it into a stable
reusable package ecosystem promise.

## Supported In This Preview

This preview supports evaluating:

- browser/WASM interactive apps;
- the current `pkg/goframe` runtime surfaces documented as public-candidate or
  experimental;
- GOX generation, package-qualified component tags, and diagnostics covered by
  current tests;
- `goxc generate`, `build`, `package`, `export`, `serve`, `size`, `clean`,
  `doctor`, and `version` as preview toolchain commands;
- static package/export workflow for browser deployment artifacts;
- hash-router/static-host deployment model;
- app module local `require` / `replace` external Go component package path for
  ordinary Go packages;
- current examples and reference apps under the documented preview limitations.

## Not Supported / Not Claimed

`v0.2.0-preview.1` does not claim:

- production readiness;
- stable 1.0 API compatibility;
- fullstack/server APIs;
- SSR or hydration;
- history-mode router, file routing, route loaders, or production server
  fallback automation;
- a production static server in `goxc serve`;
- raw `.gox` generation inside external dependencies;
- broad reusable component package ecosystem stability;
- full multi-module monorepo support;
- remote or non-local external module evidence for the new external component
  package path;
- equivalent Firefox or Safari support;
- Player/Engine, `.gfapp`, portable host/runtime packaging, desktop/mobile
  shells, or a custom app engine;
- stable LSP/formatter behavior.

## Platform Evidence

Current platform evidence remains preview-scoped:

- Chrome/Chromium on Linux is the strongest automated browser evidence because
  browser smoke and DOM pressure scripts use Chrome/CDP.
- macOS has lightweight Intel-runner CI evidence for core Go/toolchain checks.
- Windows has lightweight CI evidence for core Go/toolchain checks.
- Firefox and Safari remain outside current automated preview evidence.
- TinyGo/WASM remains the size-oriented browser build path; Go/WASM remains the
  recover-capable proof path for intentional panic/ErrorBoundary scenarios.

See [platform support](platform-support.md) for the current evidence matrix.

## Validation

Expected release-gate validation includes:

- `git diff --check`;
- `node scripts/docs-check.mjs`;
- `go test ./...`;
- GitHub Actions Core;
- GitHub Actions Browser Smoke;
- GitHub Actions WASM Size;
- GitHub Actions VS Code Extension.

Release maintainers may run broader local checks from [release hygiene](release.md)
when preparing the tag.

## Upgrade Notes

No migration is required for current preview users based on the documented
post-`v0.1.0-preview.2` changes.

Existing browser/WASM app workflows remain preview and experimental. External
Go component package support is limited to ordinary Go packages resolved
through the app module's dependencies, especially local `require` / `replace`
fixtures covered by current tests.

Use the exact tag after publication when exact preview selection matters:

```bash
go install github.com/graybuton/goframe/cmd/goxc@v0.2.0-preview.1
```

`@latest` may depend on Go module proxy and cache timing immediately after tag
publication.

## Known Limitations

- Browser/WASM DOM target only.
- Firefox and Safari are not covered by current automated evidence.
- macOS and Windows have minimal Go/toolchain evidence, not full browser/TinyGo
  smoke coverage.
- The router is hash-based; history-mode routing and server fallback automation
  are outside this preview.
- Resources are component-scoped; global resource cache, Suspense behavior,
  route loaders, automatic retry policy, and runtime fetch/JSON helpers are not
  part of this preview.
- Error Boundaries are render-only and recover-based; TinyGo trap builds are
  not the proof path for boundary containment.
- Package publication is metadata-last and fail-closed, but not a transactional
  rollback installer.
- `goxc serve` is development-only and not a production static server.
- External Go component package evidence uses local replace fixtures. Remote
  module fetching and broad ecosystem compatibility are not covered.
- Raw `.gox` files inside external dependencies are not generated by `goxc`.
- Player/Engine and `.gfapp` are inactive directions.

## Links

- [README](../README.md)
- [API stability](api-stability.md)
- [Platform support](platform-support.md)
- [Multi-package GOX workspace](multi-package-workspace.md)
- [Component identity](component-identity.md)
- [Player/Engine inactive direction note](player-vision.md)
- [v0.1.0-preview.2 release notes](release-notes-v0.1.0-preview.2.md)
