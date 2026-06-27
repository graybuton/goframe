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
| `wasm` | `bundle.wasm` | Logical WASM filename. Must be a relative `.wasm` child path. `main.wasm` remains accepted for legacy apps. |
| `assets` | auto | Static assets contract. Recommended form is a relative directory string such as `"./assets"`. Legacy explicit lists such as `["index.html", "styles.css"]` remain supported. Omitted or `null` auto-detects `./assets`, then root `index.html`, then generated default HTML. Empty `[]` means no user static assets and still generates runnable default HTML. |

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
- `wasm` must end in `.wasm`; names such as `main.go`, `go.mod`,
  `bundle.wasm.gz`, and `wasm_exec.js` are rejected;
- `assets` accepts omitted/`null`, a directory string, or an explicit path
  list. Other JSON shapes are rejected;
- directory mode walks a regular non-symlink asset directory recursively. Asset
  logical names are relative to that directory, so `assets/data/issues.txt`
  packages as `assets/data/issues.txt`;
- `assets/index.html` in directory mode is a custom root HTML template and is
  written to package root `index.html`, not `assets/index.html`;
- if no custom HTML template is selected, `goxc package` generates a default
  standalone `index.html` with a `root` mount element and final runtime/WASM
  paths;
- custom `index.html` templates must exist before compilation and must be
  regular non-symlink files;
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

Status: Ready with limitations.

There is no required `version` field in `goframe.json` for
`v0.1.0-preview.1`. The preview contract keeps user-authored manifests
versionless. Absence of a user-authored manifest version is supported preview
behavior, not a warning or deprecation signal.

Generated `asset-manifest.json` and `goframe-package.json` remain versioned
tooling metadata. User-authored schema or version markers are not part of the
current preview contract. Making such a marker mandatory would be a breaking
manifest change and would require migration notes and a compatibility window.

## `asset-manifest.json`

Status: Ready with limitations.

`asset-manifest.json` is generated package metadata, not a user-authored input
file. It records final asset paths and entrypoints for packaged output.

It is not an authoritative ownership or completion marker. A standalone
`asset-manifest.json`, even when valid, does not let `goxc package`, `goxc
export`, or `goxc clean --legacy` treat a directory as GoFrame-owned. Current
ownership requires complete `goframe-package.json` metadata plus matching
regular companion files.

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
- the file is companion metadata, not destructive ownership evidence;
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
- current package ownership is fail-closed: the marker must be regular,
  parseable, versioned metadata with sane entrypoint paths;
- the companion `asset-manifest.json` must be regular, parseable, versioned,
  and must match the WASM/runtime entrypoints in `goframe-package.json`;
- referenced HTML, WASM, and runtime files must exist as regular files inside
  the package root;
- `goframe-package.json` is published last and removed first during destructive
  package cleanup so partial packages are not marked complete;
- successful `goxc package` and `goxc export` runs verify the published
  directory as a complete current package before printing success. If
  verification fails, the completion marker is removed;
- `index.html` is a managed package artifact and is removed during package or
  export replacement so stale bootstraps cannot survive a later package run;
- adding metadata fields is backward-compatible;
- removing the marker or changing ownership detection is breaking unless it is
  required for a safety fix;
- exact timestamps are not stable.

## Legacy Metadata

Status: Ready with limitations.

Repository history shows the historical GoFrame package manifest was a
`manifest.json` containing GoFrame-specific fields:

- `name`;
- `compiler`;
- `wasm`;
- `assets`;
- `toolchainVersion`.

Legacy ownership is recognized only for that shape with supported compiler
value, safe `.wasm` path, regular WASM/runtime companion files, and regular
declared assets. A generic web manifest, empty `{}`, malformed JSON, symlinked
manifest, or generic Go/WASM dist containing only `manifest.json`,
`main.wasm`, and `wasm_exec.js` does not grant ownership and must not cause
`dist/` or package output deletion.

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
