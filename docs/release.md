# Release Hygiene

goframe is still an experimental MVP project. The current tags are milestone
tags, not stable public API releases.

## Tag Style

Current milestone tag:

```text
v0.1.0-mvp10
```

Future public experimental releases should use normal semantic tags such as:

```text
v0.1.0
```

Use annotated tags:

```bash
git tag -a v0.1.0-mvp10 -m "MVP 10: GOX expression ergonomics and public API cleanup"
```

Push tags explicitly:

```bash
git push origin v0.1.0-mvp10
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
