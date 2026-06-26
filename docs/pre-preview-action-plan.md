# GoFrame Pre-Preview Action Plan

## Snapshot

- Audited SHA: `8c5744560dfad9ef4cb1bd3510c1fc6fb4ffaaa2`.
- Codex audit branch: `chore/pre-preview-deep-audit-i`.
- Codex report path: `docs/pre-preview-deep-audit-i.md`.
- Independent research report was also reviewed externally by the project owner.

This plan synthesizes both audits into the next pre-preview work. It does not
merge the report branch and does not change runtime, compiler, toolchain,
examples, scripts, or CI behavior.

## Operating Principle

Scope preview != scope project.

The first public preview should validate the browser/WASM application layer that
already has evidence: runtime components, GOX generation, `goxc`, packaging,
router, resources, Error Boundaries, examples, docs, and CI gates. That narrower
preview scope must not be confused with a smaller project vision.

GoFrame should not remove or hide working surfaces merely because they are not
perfect yet. Valuable but risky areas should be hardened, documented, tested, or
marked experimental. The project remains an ambitious Go-first application
platform; preview contracts should make maturity visible rather than amputating
the roadmap.

## Current Product Identity

GoFrame is an experimental Go-first application platform whose first validated
public layer is browser/WASM interactive applications.

Current real layers:

- `pkg/goframe` runtime;
- `pkg/gox` GOX compiler;
- `cmd/goxc` toolchain;
- examples and reference apps;
- package/export workflow;
- docs, CI, and release policy.

The current preview candidate is not a production platform and does not claim
SSR, hydration, history-mode routing, route loaders, server resources,
production deployment hosting, or Player/Engine delivery. Those boundaries are
preview boundaries, not statements that the future platform should avoid them.

## Maturity Tiers

| Tier | Meaning | Current surfaces |
|---|---|---|
| Core / public-candidate | Real user-facing surface with examples, tests, and intended direction, but no 1.0 guarantee. | Component model, state/reducer/effects hooks, context, basic events, GOX basic syntax and generation workflow, `goxc generate/build/package/export` basics, static browser/WASM package workflow. |
| Experimental frontier | Working and valuable, but contract details still need staged hardening before broad promises. | Resources, Error Boundaries, hash router details, runtime error APIs, virtualization beyond the current fixed-height contract, advanced GOX syntax and diagnostics, package metadata details. |
| Future vision | Strategic direction that should remain visible but is not promised by the first preview. | Player/Engine, richer app platform, broader host/runtime story, stronger editor tooling, future package ecosystem, future deployment/bundle contracts. |
| Internal / implementation detail | Needed by the implementation and tests, not an external compatibility promise. | Hidden `.goframe` workspace layout, mounted tree internals, dirty queues, debug probes, staging directories, smoke harness details. |

## Confirmed Risks

The audits did not identify a confirmed Critical blocker at the audited SHA.
They did confirm readiness risks that should be handled before broad public
preview or explicitly scoped in release notes:

- reusable/multi-module component identity contract;
- platform/browser evidence beyond Linux/Chrome;
- manifest/versioning decision for public preview;
- GOX parser/codegen fuzzing, now reduced by initial bounded fuzz targets but
  still not a formal exhaustive language verification story;
- `pkg/gox` file helper safety contract for direct library callers;
- package publication transactionality;
- supply-chain scanner evidence;
- CLI/helper direct coverage;
- docs/status clarity.

These are contract and evidence risks, not reasons to reduce GoFrame to a
smaller product category.

## What We Will Not Do

- No feature removal just to reduce audit surface.
- No runtime/toolchain rewrites in this docs/planning phase.
- No pretending experimental surfaces do not exist.
- No production claims.
- No SSR, hydration, history-router, route-loader, server-resource, or
  Player/Engine promises for the first preview.

## Roadmap

### 1. `docs/vision-preserving-preview-contract`

Scope: define the vision-preserving preview contract, maturity tiers, and
action plan.

Findings addressed: docs/status clarity, release framing, audit synthesis.

Acceptance criteria:

- `docs/pre-preview-action-plan.md` exists;
- readiness/API/platform/release docs clearly separate project vision from first
  preview scope;
- no code, examples, scripts, CI, manifests, or tests change.

Non-goals:

- no runtime/toolchain fixes;
- no new preview tag;
- no CI matrix expansion.

### 2. `design/component-identity-preview-contract`

Scope: define the first-preview component identity contract for reusable
packages and multi-module expectations.

Findings addressed: reusable/multi-module component identity contract.

Acceptance criteria:

- preview explicitly states whether reusable components are supported only
  within one module/app-root or across multiple modules;
- remount expectations for module path/version changes are documented;
- fixtures or design examples cover the accepted scope.

Non-goals:

- no broad monorepo support unless intentionally designed;
- no hidden identity migration scheme.

### 3. `ci/minimal-platform-evidence`

Scope: add or document minimal platform/browser evidence for the first preview.

Findings addressed: platform/browser evidence.

Acceptance criteria:

- Linux/Chrome remains the strongest evidence;
- macOS/Windows pure Go/toolchain checks are added or explicitly deferred with
  release-note wording;
- at least one non-Chrome browser lane is added or a clear external validation
  plan is recorded.

Non-goals:

- no production support matrix;
- no promise that every browser has equivalent behavior.

### 4. `test/gox-fuzz-seeds`

Scope: seed short fuzz targets or fuzz-ready corpora for GOX parsing/codegen.

Findings addressed: GOX fuzzing.

Acceptance criteria:

- fuzz targets exist for parser/codegen entry points or a documented initial
  corpus is checked in;
- existing golden/error golden fixtures are reused as seeds;
- CI impact is bounded.

Current status: initial bounded fuzz targets exist for whole-file generation and
element parser/codegen entry points. Longer fuzz campaigns remain manual and
future evidence should be added as the language surface grows.

Non-goals:

- no full language redesign;
- no LSP/formatter work.

### 5. `toolchain/goxc-preview-safety-closeout`

Scope: close remaining toolchain contract gaps that are still relevant after
MVP 30.

Findings addressed: `pkg/gox` file helper safety contract, package publication
transactionality, command/helper direct coverage, supply-chain evidence where it
belongs to tooling.

Acceptance criteria:

- decide whether exported `pkg/gox` file helpers are hardened or documented as
  trusted-filesystem convenience helpers;
- package publication limitation is either accepted in release notes or moved
  toward transactional replacement;
- command-level coverage is improved where it protects user-facing behavior.

Non-goals:

- no runtime API changes;
- no package ecosystem design.

### 6. `runtime/experimental-surface-hardening`

Scope: harden working experimental surfaces without demoting them from the
project vision.

Findings addressed: experimental router/resource/Error Boundary/runtime error
contract clarity.

Acceptance criteria:

- resources, router details, Error Boundaries, and runtime error APIs have
  precise preview wording;
- any risky edge behavior is tested or documented;
- examples continue to show the integrated platform path.

Non-goals:

- no Suspense, route loaders, global cache, server resources, or full
  ErrorBoundary framework.

### 7. `docs/v0.1.0-preview-release-story`

Scope: prepare release notes and evaluator guidance for `v0.1.0-preview.1`.

Findings addressed: release clarity, maturity tiers, known limitations,
platform evidence, identity scope.

Acceptance criteria:

- release notes state maturity tiers;
- known risks are linked to follow-up branches;
- first-preview claims are limited to the validated browser/WASM layer;
- future Player/Engine/platform direction remains visible but explicitly not
  promised by the preview.

Non-goals:

- no production support claim;
- no final `v0.1.0` stability promise.

## Preview Verdict

| Decision target | Verdict | Meaning |
|---|---|---|
| Continued development | Go | The project direction is coherent and the audited baseline is strong enough for continued feature and hardening work. |
| Narrow evaluator preview | Conditional Go | Reasonable after contract, platform, and identity closeout, with Linux/Chrome evidence and explicit maturity tiers. |
| Broad public preview | No-Go until High risks are addressed or explicitly scoped | Multi-module/reusable identity and platform evidence are not yet broad-preview complete. |
| Production use | No-Go | GoFrame remains experimental and does not provide production server, SSR/hydration, history routing, route loaders, or a 1.0 API promise. |

The broad-preview No-Go is not a rejection of GoFrame's ambition. It means the
project should harden and label its current browser/WASM layer before widening
public promises.
