# GoFrame v0.4.0-preview.1

## Status / Scope

`v0.4.0-preview.1` is an experimental browser/WASM pre-release and a package
inspection and development-feedback checkpoint.

The product changes audited for this checkpoint span `v0.3.0-preview.1`
(`898179a15e2fe577992ad557b60cff490ddfa5aa`) through
`0ef6d45b3b715d890731e98010aedfa9458e2e30`.

The signed `v0.4.0-preview.1` Git tag is authoritative for the final release
boundary. Release-only documentation commits are outside the product-change
baseline above.

The primary capability is read-only inspection of one existing current
standalone package. The secondary capability presents post-start `goxc dev`
package and build failures in connected browser pages while preserving the
last successful interactive generation.

This versioned document records the `v0.4.0-preview.1` release contract. Git
tags and GitHub Releases are authoritative for publication and availability.
GoFrame remains pre-1.0, experimental, and focused on interactive browser/WASM
applications. This preview is not a production-readiness, stable API,
fullstack, SSR, hydration, or broad browser-support claim.

## Maturity And Contract Tiers

The existing [API stability tiers](api-stability.md) apply to this release:

- **Public-Candidate:** documented high-level `goxc inspect` and `goxc dev`
  command behavior. The `goxc inspect --format=json` report is a versioned
  schema-v1 tooling process contract; incompatible field or semantic changes
  require a schema version increment.
- **Experimental Frontier:** default human-readable inspect formatting and
  exact error wording; generated package metadata field stability and deeper
  preview package semantics; and `goxc dev` watcher timing, event names,
  private payload fields, presentation DOM/CSS, and build-number formatting.
- **Compiler-Facing / Low-Level:** existing `pkg/gox` compiler exports,
  including the in-memory generation boundary and trusted-filesystem
  convenience helpers, plus the documented `goframe.json` and generated
  package metadata tooling boundaries. This release adds no compiler-facing
  export.
- **Internal:** private inspect graph structs, package-root resolution,
  filesystem traversal, sorting, hashing, physical-alias registry,
  generation-fence implementation, and the development broker transport,
  storage, and presentation implementation.
- **Outside Current Contract / Inactive:** source dependency and bundle graphs,
  asset splitting, multi-entry WASM, route-lazy delivery, HMR, incremental
  compilation, SSR, hydration, fullstack APIs, LSP or formatter behavior,
  Player/Engine, and `.gfapp`.

Private implementation details are not compatibility contracts. These tiers
classify individual surfaces; they do not classify the entire release as
Public-Candidate.

## Highlights

### Package Graph Inspection

- `goxc inspect <app-or-package>` reports the declared graph of an existing
  current standalone package as deterministic text or schema-v1 JSON.
- Application workspaces, current package directories, exported packages, and
  explicit `--dir` package roots use the same read-only inspection contract.
- Reports include package metadata, artifacts, entrypoints, compression
  sidecars, byte sizes, full SHA-256 values, and metadata-declared edges.
- Inspection validates canonical logical names and package paths, entrypoint
  roles and media types, declared hashes, physical aliases, containment, and
  completion-marker consistency before emitting a report.
- A package generation change detected before output returns a retry error and
  emits zero report bytes. Inspection does not build, repair, or mutate the
  package.

### Browser Development Feedback

- After `goxc dev` has started successfully, a failed package or build attempt
  is reported in the terminal and presented in connected browser pages.
- The last successful generation remains served and interactive. Consecutive
  failures replace the development message, and successful recovery clears it
  before the normal full-page reload.
- The development presentation is owned outside the application mount root,
  preserves authored elements with similar attributes, and works across
  multiple pages and reconnects.
- Initial package failures and watch or scan failures remain terminal-only.
  The workflow remains serialized full-package rebuilding with full-page
  reload, not HMR or incremental compilation.

### Package And Delivery Contract

- Browser asset extensions use ASCII-only case-insensitive recognition.
  `.WASM`, `.JS`, `.CSS`, and `.HTML` variants retain their browser roles;
  Unicode lookalikes remain ordinary assets.
- Generated logical asset names and generated package paths have separate,
  slash-only validation contracts. Drive-looking logical names are namespace
  data under `assets/`; drive-prefixed package paths remain invalid.
- Current-package metadata readers shared by package verification, ownership,
  export, cleanup, and inspection reject malformed or non-canonical generated
  names and paths as incomplete package metadata.
- The `asset-manifest.json` and `goframe-package.json` schemas remain version
  1. The separate `goxc inspect --format=json` process contract is also schema
  version 1; it does not change either package metadata schema.
- Package completion-marker publication, ownership, export, cleanup, and
  serving behavior for valid current packages remain unchanged.

## Compatibility

- `pkg/goframe` and `pkg/gox` have no exported API additions, removals, or
  signature changes across the release range.
- No new deprecations are introduced. Existing legacy and deprecated surfaces
  retain the replacement guidance in [API stability](api-stability.md) and
  [compatibility policy](compatibility.md).
- This release does not expand the existing component-identity promise.
  Generated identity remains evidenced within the documented application and
  package composition boundaries of one app/module tree; a broad reusable
  external package ecosystem and full multi-module identity compatibility
  remain outside this preview contract.
- `goxc inspect` is the only CLI command addition. Executable comparisons found
  no help, flag, or unchanged-command exit-behavior differences for the
  previously available commands.
- All eleven ordinary package outputs matched `v0.3.0-preview.1`: package
  manifests and HTML were byte-identical, completion metadata was equivalent
  after excluding `generatedAt`, and all 44 raw, gzip, Brotli, and Zstandard
  WASM streams were byte-identical.
- Existing raw and compressed WASM ceilings and ratio gates pass without a
  budget change.
- Valid current packages remain compatible. Manually edited, damaged, or
  externally produced package metadata can now be classified as incomplete
  when generated names or paths are malformed or non-canonical.
- `go.mod`, `go.sum`, and the VS Code extension lockfile are unchanged across
  the release range. See the
  [v0.4.0-preview.1 migration notes](migration-v0.4.0-preview.1.md) for the
  bounded compatibility actions.
- Supply-chain and tooling evidence remains lightweight: read-only repository
  contents permissions, Dependabot coverage for GitHub Actions, Go modules,
  and the VS Code extension, lockfile-based extension installation, artifact
  and module gates, and successful CodeQL code-scanning checks. This preview
  does not claim an SBOM, signed binary distribution, comprehensive dependency
  attestation, or a general supply-chain or vulnerability-scanner guarantee.

## Validation

The release baseline passed:

- ordinary, `goframe_debug`, and vet lanes on Go `1.22.12`, `1.25.12`, and
  `1.26.5`, plus race lanes on Go `1.25.12` and `1.26.5`;
- GOX golden and error-golden tests, `scripts/check.sh`, artifact, module-path,
  documentation, WASM-size, workflow-lint, and clean-diff gates;
- two complete Linux/Chrome `149` Browser Smoke runs and a focused
  development-loop browser pass covering failure replacement, retained
  interaction, two-page delivery, reconnects, body-root isolation, recovery,
  and zero runtime errors;
- VS Code extension installation and tests under Node.js `24.18.1`, with its
  lockfile unchanged;
- TinyGo `0.41.1` representative packages for counter and router-dashboard
  with asset hashing, preload, gzip, and Brotli, plus a standard-Go
  server-backed package;
- repeated text/JSON inspection, copied-root identity, exported-package graph
  identity, and valid completion metadata for all three representative
  packages;
- local Windows amd64 test-binary compilation and successful Go `1.26.5` Core
  host-evidence lanes on `macos-15-intel` and `windows-latest`. Each host lane
  runs formatting, ordinary tests, vet, debug-tag tests, and selected GOX
  golden tests; neither lane claims TinyGo or browser-smoke evidence.

Publication requires the release pull request to pass the normal Core, Browser
Smoke, WASM Size, and VS Code Extension workflows. Local evidence does not
substitute for those workflows.

## Known Limitations And Follow-Ups

- `goxc inspect` describes one declared current standalone package. It does not
  infer source dependencies, build a source or bundle graph, split assets or
  WASM, define lazy loading, or inspect legacy packages.
- The inspection generation fence follows GoFrame's completion-marker
  publication protocol. It is not a general package lock for arbitrary
  external artifact mutation.
- `goxc dev` presents post-start package and build failures only. Initial
  package failures and watch or scan failures remain terminal-only, and the
  workflow has no HMR, incremental compilation, or source maps.
- Linux with Chrome/Chromium remains the strongest combined host and browser
  evidence. macOS and Windows have minimal Core host evidence only; neither has
  TinyGo or browser-smoke evidence. Firefox and Safari/WebKit do not have
  equivalent automated coverage.
- Static package output is not a production server, deployment platform, SSR,
  hydration, bundle-splitting, or fullstack framework.
- The VS Code extension remains repository-local tooling and is not published
  to the Marketplace.
- Related external evaluation
  [#117](https://github.com/graybuton/goframe/issues/117) remains independent
  and non-blocking. It is not used as release evidence.
- Preview users can revert to `v0.3.0-preview.1` while evaluating a migration
  or regression. No stable 1.0 compatibility guarantee applies.

## Install

Install the exact preview with:

```bash
go install \
  github.com/graybuton/goframe/cmd/goxc@v0.4.0-preview.1
```

Availability of this exact command is determined by the published tag shown in
GitHub Releases.

## Verification

Run:

```bash
goxc version
goxc doctor
goxc inspect --help
```

The first line from an exact tagged install should be:

```text
goxc version v0.4.0-preview.1
```

Create and inspect a current package:

```bash
goxc package ./examples/counter \
  --compiler=tinygo \
  --asset-hash \
  --preload \
  --compress=gzip,br

goxc inspect ./examples/counter
goxc inspect ./examples/counter --format=json
```

For browser development feedback from a repository checkout:

```bash
goxc dev ./examples/counter --compiler=tinygo --port=8080
```

## Links

- [README](../README.md)
- [Evaluator guide](evaluator-guide.md)
- [Migration notes](migration-v0.4.0-preview.1.md)
- [Deployment and packaging](deployment.md)
- [Manifest compatibility](manifest-compatibility.md)
- [API stability](api-stability.md)
- [Platform support](platform-support.md)
- [Release process](release.md)
- [Roadmap](roadmap.md)
- [Previous preview: v0.3.0-preview.1](release-notes-v0.3.0-preview.1.md)
- [Release-range comparison](https://github.com/graybuton/goframe/compare/v0.3.0-preview.1...v0.4.0-preview.1)

## Non-Goals

This preview does not add a public inspection Go API, schema v2, bundle graph,
asset splitting, multi-entry WASM, route-lazy delivery, HMR, incremental
compilation, source maps, a production server, SSR, hydration, or a fullstack
API. It does not select Application Model II APIs, change Issue #117, publish
the VS Code extension, broaden automated browser coverage, or establish stable
1.0 or production-readiness guarantees.
