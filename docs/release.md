# Release Hygiene

goframe is still an experimental MVP project. The current tags are milestone
tags, not stable public API releases.

## Tag Style

Current milestone tag:

```text
v0.1.0-mvp10
```

Recommended first public preview tag:

```text
v0.1.0-preview.1
```

Subsequent previews should increment the pre-release number:

```text
v0.1.0-preview.2
```

The current MVP tags remain historical milestone tags. They are not public API
release tags.

Use annotated, signed tags when possible:

```bash
git tag -s v0.1.0-preview.1 -m "GoFrame public preview 1"
git push origin v0.1.0-preview.1
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

## After Tagging

For now, release notes can be drafted from `CHANGELOG.md`. Keep milestone notes
clear about API instability and experimental status.

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
- browser smoke;
- TinyGo WASM size budgets;
- dashboard DOM pressure;
- artifact and module path gates;
- docs consistency;
- package matrix for all examples with TinyGo and selected Go compiler paths.

### Docs

- README current;
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
- compatibility/deprecation notes included;
- migration notes linked;
- rollback/revert plan noted for preview users.

## Versioning Decision

The recommended first public-preview tag is `v0.1.0-preview.1`. It is a Go
module pre-release tag. Later previews increment the numeric suffix.

`v0.1.0` final should wait until public-preview blockers in
`docs/public-preview-readiness.md` are closed or consciously accepted in release
notes.
