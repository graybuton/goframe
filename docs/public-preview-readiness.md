# Public Preview Readiness

## Executive Summary

Status: Needs hardening.

How to read this status: several subsystems are already "Ready with
limitations", but the project as a public preview remains "Needs hardening"
until High readiness risks are closed or explicitly scoped in release notes.

GoFrame has enough runtime, GOX, toolchain, examples, and CI coverage to start
formal public-preview preparation. It is not yet preview-ready because several
contracts still need final decisions before a preview tag:

- package/manifest schema version decision;
- multi-module/reusable package identity policy;
- cross-browser/platform support scope;
- first public migration notes and release notes;
- issue-tracker follow-up for remaining compatibility blockers.

MVP 30 closes part of the readiness gap by documenting API tiers, component
identity, manifest/package compatibility, symlink policy, platform support,
compatibility/deprecation policy, migration policy, and release checklist. The
filesystem/package corrective pass hardens the goxc package/export/generate/
build/clean/serve paths against common symlink traversal, false ownership,
physical alias overlap, authored-source output, partial package ownership, and
generated-asset collision mistakes.

The first preview scope is narrower than the project vision. GoFrame remains an
experimental Go-first application platform; the preview candidate validates the
current browser/WASM application layer rather than reducing the long-term
project to a small dashboard-only framework.

See `docs/pre-preview-action-plan.md` for the post-audit action plan.

## Project Vision And First Preview Scope

Project vision:

- GoFrame is an experimental Go-first application platform.
- Browser/WASM interactive applications are the first validated public layer.
- Runtime, GOX, `goxc`, packaging, router, resources, Error Boundaries,
  examples, docs, and release policy are real project layers.
- Player/Engine, richer host/runtime stories, stronger editor tooling, and a
  future package ecosystem remain future vision.

First preview scope:

- validate the current browser/WASM layer with clear maturity tiers;
- keep static hash-router deployment as the documented delivery path;
- label experimental frontier surfaces honestly instead of hiding them;
- avoid production, SSR/hydration, history-router, route-loader, server-resource,
  or Player/Engine promises.

Preview scope is not project scope. Risky but valuable surfaces should be
hardened, documented, tested, or marked experimental, not removed merely to make
the preview smaller.

## Current Status

| Area | Status | Evidence |
|---|---|---|
| Runtime primitives | Ready with limitations | `pkg/goframe/*_test.go`, `docs/runtime-model.md`, browser smoke. |
| GOX compiler | Ready with limitations | `pkg/gox` golden/error tests, source diagnostics, package-qualified component tests, and initial bounded fuzz seed targets. |
| Toolchain | Ready with limitations | `cmd/goxc` tests, browser smoke, size budget, package matrix, filesystem/package safety matrix. |
| Public docs | Ready with limitations | README, tutorial, API stability docs, docs-check. |
| Platform support | Needs hardening | Chrome/Linux remains the strongest evidence; macOS/Windows have minimal CI check evidence; Firefox/Safari remain unverified. |
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

## Maturity Tiers

| Tier | Meaning | Examples |
|---|---|---|
| Core / Public-Candidate | User-facing surfaces with real examples/tests and intended direction, still pre-1.0. | Component model, state/reducer/effects, GOX basic syntax/generation workflow, `goxc generate/build/package/export`, static browser/WASM packaging. |
| Experimental frontier | Working surfaces that should stay visible while contracts are hardened. | Resources, Error Boundaries, router details, runtime error APIs, virtualization beyond fixed-height guarantees, advanced GOX diagnostics, package metadata details. |
| Future vision | Strategic direction outside the first preview promise. | Player/Engine, broader host/runtime story, stronger editor tooling, future package ecosystem. |
| Internal / implementation detail | Implementation and harness details with no compatibility promise. | Hidden `.goframe` workspace layout, mounted tree internals, debug probes, staging directories, smoke harness internals. |

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
- `asset-manifest.json` and `goframe-package.json` are generated metadata, but
  only complete `goframe-package.json` metadata is the authoritative current
  package completion marker;
- package/export ownership is granted only by structured, regular, valid
  current metadata with matching companion asset manifest and regular
  referenced files; generic `{}` files, standalone `asset-manifest.json`, and
  generic web `manifest.json` files are not ownership markers;
- legacy ownership is fail-closed and limited to the historical GoFrame
  `manifest.json` shape found in repository history;
- manifest `wasm` values must be relative `.wasm` child paths;
- `assets` supports the preview-facing directory form (`"./assets"`), legacy
  explicit path lists, omitted/`null` auto mode, and explicit empty `[]`;
- package root `index.html` is always produced. It is rewritten from a custom
  template when selected, or generated by `goxc package` when no custom
  template exists;
- package assets are planned before publication so user assets cannot collide
  with the generated WASM, `wasm_exec.js`, or compressed sidecars;
- successful package/export commands verify that the published output is
  immediately recognized as a complete current GoFrame package;
- no mandatory manifest schema version is added for `v0.1.0-preview.1`.

## Filesystem And Symlink Safety

Status: Ready with limitations.

Evidence:

- `docs/security-symlink-policy.md`;
- `cmd/goxc/symlink_test.go`;
- `cmd/goxc/filesystem_safety_test.go`;
- root-aware validation rejects intermediate symlink components below declared
  roots;
- package/export validation rejects symlinked output roots and false ownership
  markers;
- entry/source/assets/package-source symlinks and non-regular files are
  rejected;
- lexical and physical symlink-alias output overlap is rejected for external
  workspaces and explicit build/generate/package/export outputs;
- build/generate explicit outputs cannot physically point back into authored
  source, and manifest `wasm` cannot name authored files such as `main.go` or
  `go.mod`;
- package completion requires complete current metadata, and partial
  publication without `goframe-package.json` is not considered owned;
- stale managed `index.html` is removed during package/export replacement, and
  invalid input preserves any previous complete package because required
  entrypoint validation runs before compilation and cleanup;
- output overlap and generated asset namespace collisions are rejected before
  publication.

Remaining risk:

- concurrent filesystem mutation between validation and operation remains out
  of scope;
- `goxc serve` remains development-server scoped.

## Platform Support Matrix

Status: Needs hardening.

Evidence:

- `docs/platform-support.md`;
- CI and local validation on Linux/Chrome and macOS/Windows for core Go/toolchain
  checks;
- TinyGo `0.41.1` size and smoke gates;
- Go/WASM recover-capable smoke fixtures.

Blocker:

- no dedicated Firefox/Safari browser evidence yet.

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
| High | Browser cross-platform support is partial. | `docs/platform-support.md` | Keep Linux/Chrome as strongest evidence; add focused Firefox/Safari/browser diversity or explicit deferred evidence notes. |
| Medium | Production deployment server remains out of scope. | `docs/deployment.md` | Keep preview messaging static-hosting/hash-router focused. |
| Medium | Package publication is hardened but not a full transactional installer. | `cmd/goxc/package.go` | Metadata is written last and sources are prevalidated; a future transaction/rollback design can further protect existing packages from mid-copy failures. |
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

The next branches should follow `docs/pre-preview-action-plan.md`: focus on
contract, platform evidence, identity, fuzzing, safety closeout, experimental
surface hardening, and preview release notes rather than deleting working
surfaces.

Immediate public-preview blockers remain:

1. decide multi-module/reusable package identity scope;
2. add focused Firefox/Safari browser evidence or keep them explicitly deferred.
3. draft release notes and migration notes for `v0.1.0-preview.1`;
4. optionally add an exported API classification gate if it remains small and
   robust.
