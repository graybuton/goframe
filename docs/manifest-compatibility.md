# Manifest Compatibility

## Purpose

This document records the current manifest and generated package metadata
contracts for public-preview readiness. It separates user-authored input from
generated metadata and hidden workspace internals.

Status labels:

- Ready
- Ready with limitations
- Needs hardening
- Blocker
- Deferred / non-goal

## `goframe.json`

Status: Ready with limitations.

`goframe.json` is the user-authored application input contract. The file is
optional; when it is absent, `goxc` uses defaults.

Supported fields:

| Field | Default | Contract |
|---|---|---|
| `name` | app directory base name | Human-readable package name. Empty means default. |
| `entry` | `.` | Go package entry. Supports `.` and relative child package directories such as `./cmd/app`, `cmd/app`, `./src/app`, and `app`. |
| `output` | `dist` | Legacy/export-oriented output hint. Current package output defaults to `.goframe/package/standalone`; explicit package/export flags are preferred. |
| `compiler` | `go` | Must be `go` or `tinygo`. CLI `--compiler` overrides it. |
| `wasm` | `bundle.wasm` | Logical WASM filename. `main.wasm` remains accepted for legacy apps. |
| `assets` | `["index.html"]` | Static child paths copied by `goxc package`. Missing assets are skipped with a message. |

Validation evidence:

- `cmd/goxc/manifest.go`
- `cmd/goxc/cli_test.go`
- `cmd/goxc/workspace_test.go`
- `cmd/goxc/symlink_test.go`

Current input behavior:

- unknown fields are rejected with `DisallowUnknownFields`;
- malformed JSON and trailing JSON are rejected;
- empty explicit `entry` is rejected;
- path fields must be relative child paths;
- absolute paths, raw `..` components, parent traversal, and tool-owned entry
  roots such as `.goframe`, `build`, `dist`, `node_modules`, and `.git` are
  rejected;
- entry paths must point to directories, not files;
- symlinked entry directories and symlinked assets are rejected.

Compatibility policy:

- adding an optional manifest field is backward-compatible only when old
  manifests continue to load with the same behavior;
- changing a default, changing accepted values, or making an optional field
  required is a breaking change;
- tightening unsafe path behavior is allowed as a security fix, even if it
  rejects previously accepted unsafe layouts;
- tightening package ownership detection is allowed as a security fix, even if
  it rejects placeholder metadata such as `{}`;
- legacy `wasm: "main.wasm"` remains supported through public preview unless a
  migration note says otherwise.

## Schema Version Decision

Status: Needs hardening.

There is no required `version` field in `goframe.json` today. MVP 30 does not
add one because doing so would either be optional metadata with limited value
or a breaking input requirement.

Recommended follow-up:

- decide before public preview whether a future optional `version` marker is
  useful for diagnostics;
- do not make it mandatory without a migration note and compatibility window.

## `asset-manifest.json`

Status: Ready with limitations.

`asset-manifest.json` is generated package metadata, not a user-authored input
file. It records final asset paths and entrypoints for packaged output. It is
also considered package ownership evidence only when it is a regular file with
versioned, recognizable GoFrame structure; an empty or malformed file with the
same name is not enough.

Current fields:

- `version`;
- `assets`;
- `assets[*].path`;
- `assets[*].hash`;
- `assets[*].type`;
- `assets[*].compressed`;
- `entrypoints.wasm`;
- `entrypoints.runtime`;
- `entrypoints.styles`.

Evidence:

- `cmd/goxc/package.go`
- `docs/deployment.md`
- `scripts/size-budget.sh`

Compatibility policy:

- consumers may read existing fields after public preview;
- adding fields is backward-compatible;
- removing or renaming fields requires migration notes;
- hidden staging paths and package internals are not stable.

## `goframe-package.json`

Status: Ready with limitations.

`goframe-package.json` is generated package metadata and an ownership marker.
It also lets `goxc package` and `goxc export` distinguish previous GoFrame
output from arbitrary user directories.

Current fields:

- `version`;
- `name`;
- `compiler`;
- `toolchainVersion`;
- `assetsDir`;
- `hashAssets`;
- `preload`;
- `entrypoints.html`;
- `entrypoints.wasm`;
- `entrypoints.runtime`;
- `generatedAt`.

Compatibility policy:

- the ownership-marker role is part of the tooling contract;
- ownership recognition is fail-closed: the marker must be regular, parseable,
  versioned metadata with sane entrypoint paths;
- adding metadata fields is backward-compatible;
- removing the marker or changing ownership detection is breaking unless it is
  required for a safety fix;
- exact timestamps are not stable.

## Legacy Metadata

Status: Ready with limitations.

Legacy `manifest.json` is recognized only as a previous GoFrame package/export
marker when the surrounding package has an unmistakable GoFrame legacy
signature, such as a regular root WASM file plus a regular `wasm_exec.js`
runtime shim. A generic web app manifest or `{}` file does not grant ownership
and must not cause `dist/` or package output deletion.

Evidence:

- `cmd/goxc/package.go`
- `cmd/goxc/export.go`
- `cmd/goxc/clean_test.go`

## Breaking Changes

Breaking manifest/package changes require:

- a `CHANGELOG.md` entry;
- a migration note in `docs/migrations.md` when user action is needed;
- tests for old accepted input when compatibility is retained;
- explicit mention in release notes.

## Current Limitations

- No mandatory user-authored schema version yet.
- No signed package metadata.
- No machine-readable JSON schema file.
- Generated workspace layout under `.goframe` remains internal.
