# GoFrame Roadmap

## Status

This roadmap is a planning document. It is not a compatibility promise, a
delivery schedule, or a commitment to ship every listed capability. Preview
slot numbers are planning handles that may be redesigned, split, deferred, or
rejected when implementation evidence changes the direction.

Current code, reference documentation, and release notes remain authoritative
for shipped behavior. A roadmap entry does not make a feature available or
expand the current preview contract.

## Product Identity

GoFrame is an experimental Go-first web application framework and toolchain.

The current validated layer is interactive browser/WASM applications. The
long-term north star is a broader Go-first web framework with staged client,
delivery, server-rendering, hydration, and backend-integration capabilities.
Those broader capabilities are directions to evaluate, not descriptions of the
current product.

GoFrame is not currently production-ready, SSR-capable, hydration-capable, or
fullstack. It does not have stable 1.0 APIs, and its strongest browser evidence
is Chrome/Chromium rather than broad cross-browser verification.

## Current Baseline

Status: **Current / shipped**. In this document, shipped means present on
current `main`; it does not imply a stable API or production-support promise.

The current baseline includes:

- the `pkg/goframe` browser/WASM runtime, retained component and DOM identity,
  component-scoped state, reducers, effects, context selectors, events,
  keyed reconciliation, explicit memoization, and batched dirty-component
  updates;
- the GOX language and compiler, including typed component boundaries,
  package-qualified component tags, fragments, expression-oriented rendering,
  nested markup in Go callbacks, and source-oriented diagnostics;
- the `goxc` check, generate, build, package, export, serve, size, clean,
  doctor, and version workflow for standard Go and TinyGo WebAssembly output;
- generated component identity based on Go package identity where available,
  plus multi-package and child-entry application evidence;
- a hash router with params and query helpers, component-scoped resources,
  render-only Error Boundaries, runtime error reporting, and fixed-height
  `VirtualList` and `VirtualTable` primitives;
- a server-backed reference fixture that serves a packaged browser/WASM app
  from plain Go `net/http` and uses a same-origin API, without introducing a
  GoFrame server framework;
- static package and export delivery with custom or generated `index.html`,
  asset manifests, package metadata, asset hashing, preload hints, and optional
  gzip/brotli sidecars;
- Chrome/Chromium browser smoke and DOM evidence, with minimal macOS and Windows
  core Go/toolchain CI evidence;
- read-only `goxc check` validation with text diagnostics and a schema-v1 JSON
  transport;
- a lightweight VS Code extension with syntax highlighting, snippets,
  language configuration, `goxc` commands, and CLI-backed saved-source inline
  diagnostics.

Current preview limitations remain part of this baseline:

- the router is hash-based; there is no history/path routing contract, route
  loader system, or server fallback automation;
- package output is a static browser/WASM delivery surface, not bundle
  splitting, SSR, hydration, or a production server;
- resources are component-scoped and explicit; there is no global cache,
  Suspense-style rendering, mutation framework, or server-function model;
- virtualization is fixed-height and does not provide dynamic measurement,
  infinite loading, or an advanced accessibility layer;
- Firefox and WebKit/Safari remain outside current automated browser evidence;
- GOX has no formatter, semantic language service, or Go/TinyGo type checking
  in `goxc check`;
- reusable external GOX package and broad multi-module ecosystem contracts are
  not established;
- no stable 1.0, production-readiness, SSR, hydration, or fullstack claim is
  made.

See [architecture](architecture.md), [API stability](api-stability.md), and
[platform support](platform-support.md) for the current contracts behind this
summary.

## Status Vocabulary

The roadmap uses these labels consistently:

| Status | Meaning |
|---|---|
| **Current / shipped** | Exists in current `main`. This does not imply stable or production-ready. |
| **Next checkpoint** | Intended immediate release or engineering checkpoint. |
| **Planned direction** | Selected direction, still subject to design and implementation evidence. |
| **Candidate** | Plausible preview slot or capability, but no implementation has been selected. |
| **Research** | Requires feasibility and cost evidence before a merge commitment. |
| **Inactive** | Explicitly outside the active roadmap. |

Future features do not become delivery promises by appearing in this document.

## Development Principles

1. Each minor line should open a new class of application or workflow.
2. Each preview should contain an observable capability, except for a bounded
   release or emergency-maintenance checkpoint.
3. Hardening supports a feature; it does not replace the feature train.
4. The preferred development chain is:

   ```text
   capability
       -> focused evidence
       -> bounded closeout
       -> release checkpoint
   ```

5. Avoid more than one pure-hardening stage in sequence unless a real blocker,
   security issue, or regression requires it.
6. Roadmap slots do not make production, delivery-date, or compatibility
   promises.
7. A stage may be redesigned, split, deferred, or rejected when evidence
   contradicts it.

## Version Train Overview

| Version line | Theme | Intended capability class | Status |
|---|---|---|---|
| `v0.2.0-preview.6` | Diagnostics & Editor DX checkpoint | Publish the current CLI/editor diagnostic boundary as a bounded preview checkpoint. | **Next checkpoint** |
| `v0.3.0-preview.*` | Application Model II | Route-driven data, transitions, loaders, mutations, and document state. | **Planned direction** |
| `v0.4.0-preview.*` | Modular Delivery & Bundle Splitting | Explicit asset, multi-entry WASM, and route-lazy delivery boundaries. | **Candidate** |
| `v0.5.0-preview.*` | Server Rendering & Prerender | A DOM-independent HTML-rendering subset, static prerender, and evaluated SSR adapters. | **Research** |
| `v0.6.0-preview.*` | Hydration & Islands | Deterministic attachment, state handoff, recovery, and partial activation. | **Research** |
| `v0.7.0-preview.*` | Dev Loop & Language Services | Incremental development, source mapping, formatting, language services, and distribution. | **Candidate** |
| `v0.8.0-preview.*` | Fullstack Application Contracts | Explicit typed client/server application boundaries over Go and HTTP. | **Research** |
| `v0.9.0-preview.*` | Ecosystem & 1.0 Readiness | External packages, broader platform evidence, accessibility, and compatibility operations. | **Candidate** |
| `v1.0` | Readiness gate | Evidence-based compatibility and support criteria, not a scheduled release. | **Research** |

The ordering expresses dependencies and preferred focus. It is not a calendar.

## `v0.2.0-preview.6` - Diagnostics & Editor DX Checkpoint

Status: **Next checkpoint**.

This is the nearest tooling and diagnostics checkpoint. Its implementation
baseline already exists on current `main`:

- `goxc check <file-or-directory>` performs read-only GOX validation;
- text output and schema-v1 JSON transport report authored source diagnostics;
- completed checks exit `0` without diagnostics and `1` when source
  diagnostics exist; operational failures also exit `1` through the normal CLI
  error path and do not produce a completed schema-v1 report;
- the VS Code extension applies saved-source diagnostics without treating
  unsaved editor text as compiler input;
- one-based UTF-8 byte columns are converted to VS Code UTF-16 positions using
  saved filesystem bytes;
- stale runs cannot replace diagnostics from a newer run, and multi-root
  workspaces keep process and diagnostic ownership isolated;
- resource-scoped `gox.goxcPath` configuration and Workspace Trust bound
  configured executable use;
- delete and rename hooks clear old-path diagnostics and recheck the applicable
  destination workspace;
- focused pure Node tests and the VS Code extension CI lane cover the process
  contract and mapping helpers.

Release notes for `v0.2.0-preview.6` are prepared in the
[dedicated release document](release-notes-v0.2.0-preview.6.md). The tag,
GitHub Release body, publication, and exact-install verification remain
separate steps. The release remains a **Next checkpoint** until publication.

`v0.2.0-preview.7` should exist only for a real maintenance, compatibility, or
security need. Normal feature progression should move to
`v0.3.0-preview.1`.

## `v0.3.0-preview.*` - Application Model II

Status: **Planned direction**. The exact public API is not selected by this
roadmap.

Purpose:

```text
route-driven data applications with explicit transition,
loader, mutation, and document-state boundaries
```

Candidate preview slots:

| Planning slot | Candidate capability |
|---|---|
| `preview.1` | Route transition state and cancellation. |
| `preview.2` | Optional history/path routing and its deployment contract. |
| `preview.3` | Narrow route loaders. |
| `preview.4` | Actions and mutations. |
| `preview.5` | Document head/meta and an integrated reference flow. |
| `preview.6` | Application-model checkpoint. |

Design boundaries:

- the current hash router remains supported;
- history mode must not silently assume that a deployment provides an
  `index.html` fallback;
- route transition work needs explicit cancellation and stale-result rules;
- loaders are not automatically a global cache or Suspense clone;
- actions are not an ORM, hidden RPC layer, or magic server-function system;
- no file-based router is promised;
- no final route, loader, action, or mutation API signature is chosen here.

The line remains subject to focused design and executable evidence before any
public contract is selected.

## `v0.4.0-preview.*` - Modular Delivery & Bundle Splitting

Status: **Candidate**. This line must keep distinct delivery concepts separate.

### Asset Splitting

Candidate scope:

- route- or feature-specific static assets;
- explicit manifest entries and ownership;
- preload and prefetch policy;
- cache-safe immutable chunks.

Asset splitting does not by itself split the Go/WASM program.

### Multi-Entry WASM

Candidate scope:

- multiple independently compiled WASM entrypoints;
- a deterministic package and bundle graph;
- explicit DOM root, lifecycle, and artifact ownership;
- no implied shared Go heap or shared in-memory application state.

Independent WASM entrypoints must not be described as shared-runtime code
splitting.

### Route-Lazy Bundles

Candidate scope:

- a route boundary as an explicit load point;
- loading and failure UI;
- stale-load cancellation;
- an observable prefetch policy.

### Research

Status: **Research** for shared-runtime designs, dynamic linking, shared
heap/state, and WebAssembly component-model implications. These require
feasibility, size, lifecycle, and compatibility evidence before an
implementation commitment.

Candidate preview slots:

| Planning slot | Candidate capability |
|---|---|
| `preview.1` | Package/bundle graph and inspection. |
| `preview.2` | Lazy static chunks. |
| `preview.3` | Multi-entry WASM packaging. |
| `preview.4` | Route-lazy bundles. |
| `preview.5` | Independent roots/islands prototype. |
| `preview.6` | Delivery checkpoint. |

## `v0.5.0-preview.*` - Server Rendering & Prerender

Status: **Research**. Static prerender is intended to be useful before a full
request-time SSR contract is selected.

Candidate sequence:

| Planning slot | Candidate capability |
|---|---|
| `preview.1` | DOM-independent render-to-HTML subset. |
| `preview.2` | Static prerender / SSG. |
| `preview.3` | Go `net/http` route adapter. |
| `preview.4` | Loader data and document-head output. |
| `preview.5` | Streaming feasibility evaluation. |
| `preview.6` | SSR checkpoint, only if prior evidence supports it. |

Required design boundaries include:

- correct HTML escaping and deterministic output;
- fragments and component composition without a browser DOM;
- a server-safe lifecycle subset;
- explicit handling for browser-only props and events;
- status codes, redirects, request context, and cancellation;
- loader-data and document-head output without hidden global state;
- no conversion of `goxc serve` into a production server.

Streaming remains **Research** until measured. This roadmap does not claim that
GoFrame can currently render application HTML on a server.

## `v0.6.0-preview.*` - Hydration & Islands

Status: **Research**. This line depends on deterministic server output and
delivery boundaries from earlier lines.

Candidate sequence:

| Planning slot | Candidate capability |
|---|---|
| `preview.1` | Deterministic hydration markers. |
| `preview.2` | Full-root hydration. |
| `preview.3` | Initial state and loader-data handoff. |
| `preview.4` | Mismatch detection and recovery. |
| `preview.5` | Partial hydration / islands. |
| `preview.6` | Hydration checkpoint. |

Required concerns include:

- attaching to existing DOM instead of remounting it;
- component, fragment, and keyed-child identity;
- deterministic event attachment and effect lifecycle;
- initial state handoff without duplicate data loading;
- actionable mismatch diagnostics;
- safe subtree fallback when attachment cannot continue;
- explicit lazy island activation and ownership.

Resumability, server components, and selective or streaming hydration remain
**Research**, not planned guarantees.

## `v0.7.0-preview.*` - Dev Loop & Language Services

Status: **Candidate**.

The CLI-backed saved-source diagnostics in current `main` are
**Current / shipped**. Semantic tooling remains future language-service work.

Candidate sequence:

| Planning slot | Candidate capability |
|---|---|
| `preview.1` | Incremental development command. |
| `preview.2` | Browser reload and error overlay. |
| `preview.3` | GOX-to-generated source maps. |
| `preview.4` | Deterministic formatter. |
| `preview.5` | LSP baseline. |
| `preview.6` | Extension distribution checkpoint. |

Potential surface, subject to design and evidence:

- `goxc dev` with incremental GOX generation and a bounded build cache;
- browser reload with compiler and source error presentation;
- GOX-to-generated source maps shared by compiler and editor tooling;
- a deterministic formatter with explicit syntax-governance rules;
- diagnostics, hover, completion, definition, and references;
- VSIX and Marketplace publication policy.

State-preserving hot module replacement is not promised before dedicated
runtime architecture and browser evidence exist. A formatter or LSP must not
be inferred from the current process-based diagnostics.

## `v0.8.0-preview.*` - Fullstack Application Contracts

Status: **Research**. This is an explicit-contract direction, not a current
fullstack claim.

Candidate areas:

- typed route loaders;
- typed actions and mutations;
- explicit HTTP transport and error mapping;
- validation, status, and redirect contracts;
- auth/session adapters over Go `net/http`;
- SSE and WebSocket adapters;
- multipart upload flows;
- deployment adapters;
- a static frontend plus Go API binary workflow.

Unless separately selected and evidenced, this line does not include:

- a built-in ORM or mandatory database layer;
- a hidden RPC protocol;
- a production server disguised as `goxc serve`;
- an implicit security, authentication, or authorization model;
- a broad "fullstack-ready" claim.

Each candidate boundary needs independent threat, deployment, cancellation,
versioning, and interoperability evidence.

## `v0.9.0-preview.*` - Ecosystem & 1.0 Readiness

Status: **Candidate**.

Candidate areas:

- externally authored `.gox` packages and remote module evidence;
- module and package graph inspection;
- reusable component package authoring and distribution;
- compatibility metadata across runtime, toolchain, generated output, and
  editor versions;
- Firefox and WebKit/Safari browser evidence;
- accessibility hardening;
- dynamic-height virtualization;
- long-running memory and listener evidence;
- CSP, integrity, and deployment guidance;
- an exercised migration and deprecation policy.

This line is not a promise that every ecosystem or platform area will become a
1.0 requirement. Selection depends on the application classes GoFrame chooses
to advertise before 1.0.

## `v1.0` Readiness Gate

Status: **Research**. `v1.0` is a gate, not a scheduled release.

Readiness criteria include:

- selected public APIs are classified and stable enough for an explicit
  compatibility promise;
- migration and deprecation policy has been exercised across releases;
- package and manifest evolution policy is defined and tested;
- advertised SSR and hydration status is unambiguous, including an explicit
  statement when either remains unsupported;
- browser and platform evidence matches public claims;
- runtime, package, editor, and toolchain version compatibility is documented;
- any production claims are supported by evidence beyond showcase examples;
- reference applications prove every advertised application class;
- security, release, incident, and deprecation procedures are active.

`v1.0` does not require every candidate feature in this roadmap. It requires a
coherent supported surface and evidence strong enough for the promises actually
made.

## Dependency Graph

The preferred dependency shape is:

```text
v0.2 diagnostics/editor boundary
        ↓
v0.3 routes/data/actions
        ├───────────────┐
        ↓               ↓
v0.4 modular delivery   v0.5 render-to-HTML / SSR
        └───────┬───────┘
                ↓
        v0.6 hydration / islands
                ↓
        v0.8 fullstack contracts
                ↓
        v0.9 ecosystem / 1.0 readiness
```

Parallel evidence tracks are:

```text
source mapping -> formatter -> LSP -> editor distribution
package identity -> external GOX packages -> component ecosystem
platform evidence -> broader browser claims
```

`v0.7` can advance alongside application and delivery work when its source-map
and package-identity dependencies are explicit. The graph records likely
dependencies, not a guarantee that every node proceeds.

## Feature Reservoir

Status: **Candidate** or **Research** as stated when an item is selected. These
entries are deliberately not sequenced or assigned to releases.

### Runtime

- portals;
- transitions and deferred updates;
- state transactions;
- dynamic-height virtualization;
- accessibility primitives;
- focus management;
- component test renderer;
- devtools timeline;
- workers and off-main-thread execution research.

### GOX

- source maps;
- deterministic formatter;
- import-aware analysis;
- syntax governance;
- named children/slots evaluation;
- style ergonomics;
- typed event completion;
- package-level analysis.

### Toolchain

- incremental build cache;
- development server workflow;
- bundle graph;
- multi-entry packaging;
- prerender;
- SSR profiles;
- deployment adapters;
- reproducible packages;
- SBOM and signing evaluation;
- package inspection.

### Browser Platform

- PWA and offline behavior;
- service workers;
- Web Workers;
- file and clipboard APIs;
- realtime transport;
- performance tooling;
- Firefox and WebKit automation.

### Ecosystem

- component package template;
- design-system example;
- CSS and theme strategy;
- testing utilities;
- component demo/story harness;
- package distribution evaluation.

Reservoir entries become version-train work only through an explicit selection
that defines behavior boundaries, evidence, cost, and non-goals.

## Inactive Directions

Status: **Inactive**.

The following are historical strategic options, not current roadmap items:

- Player/Engine;
- `.gfapp`;
- a portable custom host/runtime;
- desktop or mobile shells;
- a custom application engine.

Reopening any of these directions requires an explicit maintainer decision.
Architecture lessons about authored source, runtime APIs, toolchain packaging,
and deployment contracts may still be reused without reactivating the original
product direction. Historical references remain available in
[the Player/Engine inactive direction note](player-vision.md).

## Immediate Sequence

1. Complete and merge preview.6 release preparation.
2. Run release gates and publish `v0.2.0-preview.6`.
3. Begin `v0.3` with an executable Application Model II capability.

The first post-release engineering PR should not be another broad audit or a
wording-only cleanup. It should provide a bounded, observable capability with
focused evidence. This roadmap does not choose final `v0.3` API signatures.
