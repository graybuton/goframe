# Compatibility and Deprecation Policy

## Purpose

GoFrame is still pre-preview. This document defines compatibility expectations
for the first public preview preparation phase.

It does not promise SemVer 1.0 stability.

## Pre-Preview Rules

Status: Ready with limitations.

Before public preview:

- breaking changes are allowed when they fix unsafe behavior, correct a wrong
  public shape, or unblock the architecture;
- breaking changes must be documented in `CHANGELOG.md`;
- user-facing changes should include migration notes when user action is
  required;
- generated workspace internals may change without migration support;
- security/path-safety hardening may reject previously accepted unsafe input.

## Post-Preview Expectations

After public preview:

- Public-Candidate APIs should not break without a migration note;
- deprecated APIs should remain for at least one documented release stage unless
  safety requires faster removal;
- CLI command and flag changes should include replacement commands;
- user-authored manifest compatibility should be stricter than generated
  metadata compatibility;
- generated output should remain an implementation detail unless documented as
  a tooling contract.

## Deprecation Requirements

A deprecation must include:

- a GoDoc `Deprecated:` comment for exported Go symbols when applicable;
- replacement guidance;
- test coverage while the deprecated behavior remains accepted;
- documentation in `docs/api-stability.md`;
- a `CHANGELOG.md` note when visible to users.

Current deprecated/legacy surfaces:

- `gf.UseMount`: use `gf.UseEffect` with no dependency argument or `gf.Once`;
- `gf.NoDeps`: use `gf.Once` or omit deps;
- `gf.AlwaysDeps`: use `gf.EveryRender`;
- `gf.DepsOf` and explicit `gf.Dep*` helpers: prefer `gf.Deps`;
- `gf.For` and `gf.ForIndexed`: use `gf.Map` and `gf.MapIndexed`;
- `goxc build --release`: use `goxc package`;
- `goxc generate --in-place`: debug/legacy only;
- explicit `wasm: "main.wasm"` manifests: use `bundle.wasm`;
- legacy package `manifest.json` marker: current metadata is
  `goframe-package.json`; legacy ownership is fail-closed and only recognized
  for the historical GoFrame package manifest shape.

## Migration Policy

Migration notes are required when:

- a Public-Candidate API changes;
- GOX syntax changes in a way that invalidates existing source;
- manifest input changes require user edits;
- CLI command/flag behavior changes;
- package output contract changes affect deployment.

Migration notes should follow `docs/migrations.md`.

## Exceptions

The project may break compatibility without a full deprecation window for:

- path traversal or symlink escape fixes;
- destructive output behavior fixes;
- security-sensitive package/export ownership behavior;
- manifest path canonicalization that rejects ambiguous raw `..` components;
- package asset namespace collision rejection;
- physical path overlap rejection for explicit build/generate/package/export
  outputs and external workspaces;
- requiring manifest `wasm` values to end in `.wasm`;
- replacing the old required-root-`index.html` manifest behavior with
  directory-mode assets and generated default HTML;
- treating `asset-manifest.json` as companion metadata rather than standalone
  destructive ownership evidence;
- CI-only smoke harness internals;
- generated workspace internals.

## Non-Goals

This document does not define:

- SemVer 1.0 guarantees;
- npm/VS Code Marketplace publishing policy;
- production server support;
- compatibility for private debug globals or smoke harness variables.
