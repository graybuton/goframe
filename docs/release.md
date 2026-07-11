# Release Hygiene

goframe is still an experimental MVP project. The current tags are milestone
tags, not stable public API releases.

## Tag Style

Current milestone tag:

```text
v0.1.0-mvp10
```

Latest published preview tag:

```text
v0.2.0-preview.5
```

Subsequent previews should increment the pre-release number.

The current MVP tags remain historical milestone tags. They are not public API
release tags.

Use annotated, signed tags when possible for future previews:

```bash
git tag -s vX.Y.Z-preview.N -m "GoFrame vX.Y.Z-preview.N"
git push origin vX.Y.Z-preview.N
```

## Pre-Release Checklist

Before creating a tag:

- working tree is clean;
- `main` contains the intended merge commit;
- module path is `github.com/graybuton/goframe`;
- `go test ./...` passes;
- `go test -race ./pkg/... ./cmd/...` passes;
- `go vet ./...` passes;
- `go test -tags=goframe_debug ./...` passes;
- GOX golden tests pass;
- `scripts/check.sh` passes locally or in CI;
- `scripts/size-budget.sh` passes;
- `scripts/browser-smoke.sh` passes on a machine with Chrome and localhost bind;
- VS Code extension JSON validation and compile pass;
- `scripts/artifact-check.sh` passes;
- `scripts/module-path-check.sh` passes;
- release package layout is checked with `goxc package --asset-hash --preload`
  for affected examples;
- filesystem/package safety spot checks pass for symlink rejection, ownership
  markers, physical output overlap aliases, asset collisions, partial publish,
  clean, legacy ownership, and serve behavior;
- README, docs, and changelog are updated;
- no `dist/`, `build/`, `.goframe`, `.gox.go`, `.wasm`, `.wasm.gz`,
  `.wasm.br`, `.wasm.zst`, `node_modules`, `.vsix`, or `.test` files are
  tracked.

## Current Publishing Policy

The project does not publish binaries yet.

The VS Code extension is not published to the Marketplace yet. Local extension
development remains under `extensions/vscode-gox`.

The recommended install path for the CLI is:

```bash
go install github.com/graybuton/goframe/cmd/goxc@latest
```

### README release references

README is a version-agnostic project entry point. It links to GitHub Releases
and this release process instead of hard-coding the latest preview tag or a
current release-note filename. Version-specific scope and history belong in
GitHub Releases, `docs/release-notes-*.md`, `CHANGELOG.md`, and intentionally
versioned release or evaluator documents. Release-prep and post-publish work
should not modify README solely to advance a current-preview pointer.

Because the Quick Start installs `goxc` with `@latest`, every command in that
sequence must exist in the latest published module tag. Commands merged on
`main` may appear in current-main reference sections before their next release,
but should enter the `@latest` Quick Start only after a release containing them
is published.

## After Tagging

Use the dedicated release notes document for the preview tag being released.
Current preview release note documents:

- `docs/release-notes-v0.1.0-preview.1.md`
- `docs/release-notes-v0.1.0-preview.2.md`
- `docs/release-notes-v0.2.0-preview.1.md`
- `docs/release-notes-v0.2.0-preview.2.md`
- `docs/release-notes-v0.2.0-preview.3.md`
- `docs/release-notes-v0.2.0-preview.4.md`
- `docs/release-notes-v0.2.0-preview.5.md`

`CHANGELOG.md` remains the factual change log; release notes should summarize
preview scope, maturity tiers, validation evidence, compatibility notes, and
known limitations. `docs/evaluator-guide.md` is the evaluator-facing quick path
for trying the preview.

## GitHub Release Body Style

Preview release titles should stay short and version-first:

```text
GoFrame vX.Y.Z-preview.N
```

Stable release titles should use the same shape without the preview suffix:

```text
GoFrame vX.Y.Z
```

Put scope and caveats in the body, not in a long title.

Preview GitHub Release bodies should be concise publish-facing contracts, not
marketing announcements. They should answer what was released, what surface is
safe to evaluate, what was validated, what the release does not promise, how to
install it, and how to verify it.

Preferred preview body order:

- `Status / Scope`;
- `Highlights`;
- `Compatibility`;
- `Validation`;
- `Known limitations and follow-ups`;
- `Install`;
- `Verification`;
- `Links`;
- `Non-goals`, when useful.

Use calm, factual, scope-bounding language. Prefer terms such as `preview`,
`experimental`, `pre-release`, `validated surface`, `current browser/WASM
layer`, `not production-ready`, `no stable API guarantee`, `known limitations`,
and `tracked separately` when they apply. Avoid stable or production wording
unless the release is actually stable and production-ready, and keep limitations
explicit rather than scattered.

Use neutral issue references, for example `Related issue: #70`, `Tracked
separately in #71`, or `See also #69`. Avoid the auto-closing verbs `Fixes`,
`Closes`, and `Resolves` unless the release body intentionally wants GitHub
auto-closing behavior.

For preview releases in the GitHub UI:

- mark the release as a pre-release;
- use manual notes rather than generated notes;
- do not upload binaries or assets unless GoFrame starts supporting an official
  binary distribution surface;
- do not mark preview releases as stable/latest when the UI supports avoiding
  that.

The GitHub Release body is the concise publication entrypoint.
`docs/release-notes-*.md` is the durable detailed release record in the
repository. `CHANGELOG.md` is factual cumulative history, not a narrative
announcement.

## Public Preview Checklist

### Repository

- `main` is clean and contains the intended merge commit;
- no generated artifacts are tracked;
- module path is `github.com/graybuton/goframe`;
- release commit and tag are signed;
- `CHANGELOG.md` is current;
- no unresolved Critical or High readiness findings remain without explicit
  release-note acknowledgement.

### API

- exported `pkg/goframe` and `pkg/gox` API inventory reviewed;
- `pkg/gox` in-memory APIs and trusted-filesystem file helpers classified;
- every new public export is classified in `docs/api-stability.md`;
- deprecated APIs have replacement guidance;
- migration notes exist for user-visible breaking changes;
- manifest/package compatibility policy is current.

### Tests

- `go fmt ./...` and clean diff check;
- `go test ./...`;
- `go test -race ./pkg/... ./cmd/...`;
- `go vet ./...`;
- `go test -tags=goframe_debug ./...`;
- GOX golden and error golden tests;
- focused runtime experimental-surface tests for resources, ErrorBoundaries,
  runtime error reporting, and router matching/remount policy;
- browser smoke;
- TinyGo WASM size budgets;
- dashboard DOM pressure;
- artifact and module path gates;
- docs consistency;
- package matrix for all examples with TinyGo and selected Go compiler paths.

### Docs

- README current;
- evaluator guide current;
- release notes current;
- tutorial current;
- API stability current;
- platform support current;
- public-preview readiness document current;
- known limitations current;
- examples aligned with docs.

### Packaging

- `goxc package --asset-hash --preload --compress=gzip,br` checked for
  representative examples;
- `asset-manifest.json` and `goframe-package.json` present, with
  `goframe-package.json` published as the authoritative completion marker;
- root `index.html` present as the standalone HTML entrypoint, either rewritten
  from a custom template or generated by `goxc package`;
- package/export success verified against current package ownership metadata;
- package/export ownership safety checked with complete structured metadata,
  not placeholder files or standalone asset manifests;
- manifest assets checked for collisions with generated WASM, runtime, and
  compressed sidecar names;
- explicit output paths checked for physical overlap with authored source or
  package source;
- clean workspace checked;
- sample static-host deploy notes current.

### Release Notes

- tag format documented;
- install command documented;
- maturity tiers included: public-candidate, experimental frontier,
  compiler-facing, internal, and outside-current-contract / inactive-direction
  scope;
- platform evidence stated explicitly, including Linux/Chrome as the strongest
  current evidence, minimal macOS/Windows CI check evidence, and non-Chrome
  browser platforms as explicit deferred scope when unverified;
- reusable/multi-module component identity scope stated explicitly;
- experimental surfaces named rather than hidden;
- Player/Engine bounded as inactive and not a preview promise;
- compatibility/deprecation notes included;
- supply-chain/tooling evidence stated as lightweight CI/Dependabot/lockfile
  coverage, with no SBOM or scanner claim in the current preview contract;
- migration notes linked;
- rollback/revert plan noted for preview users.

## Versioning Decision

Preview tags are Go module pre-release tags. Later previews increment the
numeric suffix.

The eventual non-preview release should wait until public-preview blockers in
`docs/public-preview-readiness.md` are closed or consciously accepted in release
notes.
