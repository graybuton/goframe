# Post-preview v0.2 Technical Focus

## Purpose

This document records the selected technical focus after the first public
preview line and maintenance hardening. It is a planning/evidence document, not
a release note, compatibility promise, or implementation plan that changes
behavior by itself.

Scope preview != scope project. The current preview contract remains narrower
than the broader GoFrame project vision.

## Current Baseline

`v0.1.0-preview.*` established an evaluator preview for the browser/WASM
application layer:

- `pkg/goframe` runtime primitives for nodes, components, hooks, context,
  events, reconciliation, resources, router, runtime errors, and Error
  Boundaries;
- `pkg/gox` lexer, parser, code generator, diagnostics, golden tests, and
  bounded fuzz seeds;
- `cmd/goxc` generate, build, package, export, serve, size, clean, doctor, and
  version commands;
- static package/export flow with versionless `goframe.json`, directory-mode
  assets, generated/custom root `index.html`, and versioned package metadata;
- focused examples plus the router-dashboard reference app;
- Linux/Chrome browser smoke, TinyGo size budgets, Go tests, docs checks, and
  minimal macOS/Windows Go/toolchain CI evidence;
- public preview docs, release notes, evaluator guide, community health files,
  `goxc serve` path hardening, and browser smoke CodeQL cleanup.

## Closed Post-preview Follow-ups

The first post-preview hardening pass has recorded these boundaries:

- residual fact-first wording polish keeps product docs focused on current
  behavior, current limitations, current non-goals, and current evidence;
- non-Chrome browser behavior remains outside the current preview promise
  because browser smoke evidence is Chrome/Chromium-based;
- reusable component package identity is documented as evidenced inside the
  current app/workspace model, not as broad reusable multi-module package
  ecosystem stability.

## Candidate Technical Directions

| Direction | Current status |
|---|---|
| Reusable component/package identity model | Selected v0.2 technical focus. Current evidence covers generated typed tokens, package-qualified GOX tags, and app/workspace composition; broad reusable multi-module identity remains outside the current preview contract. |
| Actual non-Chrome browser evidence | Deferred from this v0.2 focus. Firefox and Safari remain outside current preview evidence. |
| Production deployment/server story | Outside the current preview contract. `goxc serve` remains development-only. |
| Router/history/SSR/hydration | Outside this v0.2 focus. The current router is hash-based and does not claim SSR, hydration, path/history routing, or server fallback automation. |
| Player/Engine / `.gfapp` | Inactive direction, outside the current preview contract and outside this v0.2 focus. |
| Performance/DOM pressure evidence expansion | Deferred from this v0.2 focus. Existing size, browser smoke, and DOM pressure evidence remain the current baseline. |

## Selected v0.2 Focus

The selected `v0.2.0-preview.1` technical focus is:

```text
Reusable component/package identity model
```

The first step is design and evidence, not implementation. The sequence is:

1. Design the reusable package identity model.
2. Add tests, fixtures, and evidence for current and desired boundaries.
3. Implement behavior only if the design and tests show an actual gap.
4. Update docs and release notes only after behavior and evidence exist.

## Why This Focus

Reusable component/package identity is the most important current contract
boundary because it affects state preservation, remount behavior, package
composition, and whether component packages can be evaluated safely outside one
app/workspace tree.

This focus builds directly on the current evidence:

- `gf.ComponentT` and `gf.NewComponentType`;
- generated GOX component tokens;
- package-qualified tags such as `<layout.Shell />`;
- import-path-aware generation in `goxc`;
- `multipackage` and `cmdapp` evidence inside the current app/workspace model.

It also prevents GoFrame from claiming broad reusable package ecosystem
stability before dedicated tests and contracts exist.

## Explicit Non-goals For v0.2 Planning

This planning document does not include:

- immediate runtime rewrite;
- GOX behavior change;
- `goxc` behavior change;
- broad package ecosystem promise;
- stable cross-module identity promise;
- production deployment server;
- SSR or hydration;
- Player/Engine or `.gfapp`;
- equivalent Firefox/Safari support claim;
- `v0.2.0-preview.1` tag creation.

## Proposed PR Sequence

These are candidate review units, not shipped behavior:

| Branch | PR title | Scope |
|---|---|---|
| `design/reusable-package-identity-model` | `docs(identity): define reusable package identity model` | Define the model, terms, expected boundaries, and compatibility constraints. |
| `test/identity-reusable-package-fixtures` | `test(identity): cover reusable package identity boundaries` | Add fixtures/evidence for current app/workspace behavior and reusable-package boundary cases. |
| `feature/identity-reusable-package-support` | `feat(identity): support reusable package component identity` | Optional implementation branch only if the design/test pass proves an implementation gap. |
| `docs/v0.2.0-preview.1-release-notes` | `docs(release): prepare v0.2.0 preview notes` | Release prep only after behavior, evidence, and docs match. |

## Exit Criteria For `v0.2.0-preview.1`

`v0.2.0-preview.1` needs:

- identity model documented;
- reusable package boundary tests present;
- any implementation changes, if needed, covered by tests;
- docs and API stability pages matching actual behavior;
- no unsupported package ecosystem claims;
- CI green;
- release notes written in fact-first language.
