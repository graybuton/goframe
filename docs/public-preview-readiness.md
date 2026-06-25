# Public Preview Readiness

## Executive Summary

Status: Needs hardening.

GoFrame has enough runtime, GOX, toolchain, examples, and CI coverage to start
formal public-preview preparation. It is not yet preview-ready because several
contracts still need final decisions before a preview tag:

- package/manifest schema version decision;
- multi-module/reusable package identity policy;
- Windows/macOS support evidence;
- first public migration notes and release notes;
- issue-tracker follow-up for remaining compatibility blockers.

MVP 30 closes part of the readiness gap by documenting API tiers, component
identity, manifest/package compatibility, symlink policy, platform support,
compatibility/deprecation policy, migration policy, and release checklist.

## Current Status

| Area | Status | Evidence |
|---|---|---|
| Runtime primitives | Ready with limitations | `pkg/goframe/*_test.go`, `docs/runtime-model.md`, browser smoke. |
| GOX compiler | Ready with limitations | `pkg/gox` golden/error tests, source diagnostics, package-qualified component tests. |
| Toolchain | Ready with limitations | `cmd/goxc` tests, browser smoke, size budget, package matrix. |
| Public docs | Ready with limitations | README, tutorial, API stability docs, docs-check. |
| Platform support | Needs hardening | Chrome/Linux are tested; macOS/Windows/Firefox/Safari are not CI-tested. |
| Public preview release process | Needs hardening | `docs/release.md` now contains a preview checklist; no preview tag has been cut. |

## Public Preview Definition

A first public preview should mean:

- users can evaluate GoFrame with the tutorial and examples;
- Public-Candidate APIs have documented compatibility expectations;
- known Experimental APIs are clearly labelled;
- manifests and package metadata have a documented compatibility model;
- safety-sensitive filesystem behavior has tests;
- the release tag and notes make limitations explicit.

It should not mean production readiness, 1.0 API stability, SSR/hydration,
history routing, route loaders, Suspense, server resources, or a production
deployment server.

## API Surface

Status: Ready with limitations.

Evidence:

- `docs/api-stability.md`;
- `go doc -all ./pkg/goframe`;
- `go doc -all ./pkg/gox`;
- `pkg/goframe` and `pkg/gox` tests.

Findings:

- app-author APIs and compiler-facing helpers were mixed in older docs;
- low-level node helpers remain exported for GOX and handwritten low-level Go;
- resource, router, ErrorBoundary, and runtime error APIs are promising but
  still Experimental/Public-Candidate, not stable 1.0.

Decision:

- keep API shape unchanged in MVP 30;
- classify surfaces more explicitly;
- require future public exports to be classified before preview.

## Component Identity

Status: Ready with limitations.

Canonical policy:

- generated typed identity uses canonical Go import path plus component symbol;
- import aliases are debug labels, not identity;
- generated variable names and file discriminators are not runtime identity;
- legacy `gf.Component` string identity lives in a separate namespace;
- module path/version changes are identity changes and may remount state.

Evidence:

- `docs/component-identity.md`;
- `pkg/goframe/component_identity_test.go`;
- `pkg/gox/generate_test.go`;
- `cmd/goxc/workspace_test.go`.

Blocker:

- full multi-module workspace/reusable package identity is not promised yet.

## Manifest And Package Contracts

Status: Ready with limitations.

Evidence:

- `docs/manifest-compatibility.md`;
- `cmd/goxc/manifest.go`;
- `cmd/goxc/package.go`;
- `cmd/goxc/cli_test.go`;
- `cmd/goxc/symlink_test.go`.

Decision:

- `goframe.json` is the public input contract;
- `asset-manifest.json` and `goframe-package.json` are generated metadata and
  package/export ownership markers;
- no mandatory manifest schema version is added in MVP 30.

## Filesystem And Symlink Safety

Status: Ready with limitations.

Evidence:

- `docs/security-symlink-policy.md`;
- `cmd/goxc/symlink_test.go`;
- package/export validation rejects symlinked output roots;
- entry/source/assets symlinks are rejected.

Remaining risk:

- symlinked app roots are not a promised workflow;
- serve path hardening remains development-server scoped.

## Platform Support Matrix

Status: Needs hardening.

Evidence:

- `docs/platform-support.md`;
- CI and local validation on Linux/Chrome;
- TinyGo `0.41.1` size and smoke gates;
- Go/WASM recover-capable smoke fixtures.

Blocker:

- no macOS/Windows/Firefox/Safari CI evidence yet.

## Browser Support

Status: Ready with limitations.

Chrome/Chromium is CI-tested. Firefox and Safari are unverified. The runtime
requires browser DOM APIs, WebAssembly, `requestAnimationFrame`, `hashchange`,
and example-specific APIs such as `fetch` and `AbortController`.

## Compatibility And Deprecation Policy

Status: Ready with limitations.

Evidence:

- `docs/compatibility.md`;
- `docs/migrations.md`;
- deprecated GoDoc comments for current aliases.

Decision:

- pre-preview breaking changes remain allowed but documented;
- post-preview Public-Candidate changes require migration notes;
- deprecated APIs should survive at least one planned release stage unless
  safety requires immediate removal.

## Migration Policy

Status: Ready.

Evidence:

- `docs/migrations.md`;
- historical examples for `main.wasm`, `.goframe` generation, and
  package-qualified GOX.

## Release Checklist

Status: Ready with limitations.

Evidence:

- `docs/release.md`;
- current local baseline and package matrix.

Recommendation:

- first preview tag: `v0.1.0-preview.1`;
- later previews: `v0.1.0-preview.2`, etc.;
- final `v0.1.0` only after blockers are closed.

## Known Blockers

| Severity | Finding | Evidence | Recommendation |
|---|---|---|---|
| High | Multi-module/reusable package identity is not final. | `docs/component-identity.md` | Focused MVP before claiming reusable package ecosystem. |
| High | Platform support is Linux/Chrome-heavy. | `docs/platform-support.md` | Add macOS/Windows and Firefox/Safari verification or mark preview scope narrowly. |
| Medium | `goframe.json` has no schema version decision. | `docs/manifest-compatibility.md` | Decide optional schema marker before preview. |
| Medium | Production deployment server remains out of scope. | `docs/deployment.md` | Keep preview messaging static-hosting/hash-router focused. |
| Documentation-only | Public API classification must be kept current. | `docs/api-stability.md` | Optional exported-symbol classification gate can be added later. |

## Deferred Non-goals

- SSR/hydration;
- history-mode router;
- route loaders;
- Suspense/server resources;
- global resource cache;
- formatter/LSP;
- Player/Engine and `.gfapp`;
- production deployment server;
- full multi-module monorepo support.

## Recommended Next Stage

MVP 31 should focus on public-preview blockers, not new app features:

1. decide manifest schema/version strategy;
2. decide multi-module/reusable package identity scope;
3. add platform verification or narrow preview support claims;
4. draft release notes and migration notes for `v0.1.0-preview.1`;
5. optionally add an exported API classification gate if it remains small and
   robust.
