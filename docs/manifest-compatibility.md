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
| `entry` | `.` | Go package entry. Supports `.` and canonical relative child package directories such as `cmd/app`, with authored `./cmd/app` and `cmd\app` spellings normalized at load time. |
| `output` | `dist` | Canonical relative legacy/export-oriented output hint. Current package output defaults to `.goframe/package/standalone`; explicit package/export flags are preferred. |
| `compiler` | `go` | Must be `go` or `tinygo`. CLI `--compiler` overrides it. |
| `wasm` | `bundle.wasm` | Canonical relative WASM child path. The final package still uses its basename. `main.wasm` remains accepted for legacy apps. |
| `assets` | auto | Static assets contract. Recommended form is a relative directory string such as `"./assets"`. Legacy explicit lists such as `["index.html", "styles.css"]` remain supported. Authored separators are canonicalized before either mode is consumed. Omitted or `null` auto-detects `./assets`, then root `index.html`, then generated default HTML. Empty `[]` means no user static assets and still generates runnable default HTML. |

Validation evidence:

- `cmd/goxc/manifest.go`
- `cmd/goxc/cli_test.go`
- `cmd/goxc/workspace_test.go`
- `cmd/goxc/symlink_test.go`

Current input behavior:

- unknown fields are rejected with `DisallowUnknownFields`;
- malformed JSON and trailing JSON are rejected;
- empty explicit `entry` is rejected;
- authored path fields use portable relative syntax. `/` is the canonical
  separator; `\` is accepted as alternate separator syntax and canonicalized
  to `/` exactly once during manifest loading. A successful load stores `entry`,
  `output`, `wasm`, the asset directory, and every listed asset in canonical
  slash form;
- a backslash in a manifest path is never a literal Unix filename byte. For
  example, `"assets\\styles.css"` selects `assets/styles.css`, not one file
  named `assets\styles.css`;
- absolute paths, raw `..` components, parent traversal, and tool-owned entry
  roots such as `.goframe`, `build`, `dist`, `node_modules`, and `.git` are
  rejected;
- canonical manifest paths cross into host filesystem operations through
  platform-native conversion below the application root. Logical package
  names, generated package paths, and browser URLs remain separate contracts;
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
- custom templates give GoFrame explicit ownership of preload, runtime, and
  bootstrap sections through complete, exact managed comment pairs. These are
  the universal ownership mechanism. The delimiters remain in package and
  development HTML. Preload markers must be direct children of one concrete
  `head`; runtime and bootstrap markers must be direct children of one concrete
  `head` or `body`. Arbitrary ordinary-container, document-level,
  direct-`html`, cross-parent, structurally uncertain, and
  SVG/MathML-ancestor placements fail before package publication because the
  rewriter does not infer arbitrary browser tree-builder recovery. Malformed,
  duplicate, nested, and interleaved marker shapes also fail;
- when GoFrame owns both runtime and bootstrap integrations, an executable
  parser-blocking owned runtime must precede the owned bootstrap. Reversed,
  async, deferred, module, and `nomodule` arrangements cannot prove that order.
  A paired legacy `event`/`for` form is eligible only for `window` `onload`
  execution after character-reference decoding and HTML ASCII trimming;
- managed ownership does not change browser URL resolution. Runtime, WASM,
  stylesheet, and enabled-preload references emitted by the current packager
  are package-relative, so a potentially active HTML `base[href]` rejects any
  managed or markerless operation that would emit one. A target-only `base`, a
  disabled preload block, and active-base documents with no package-owned
  relative URL output remain valid. Configurable deployment-base output is not
  part of the current schema or preview contract;
- without a managed section, compatibility rewriting is limited to a balanced
  simple historical profile: structural runtime script URLs, complete
  historical GoFrame bootstrap scripts, declared stylesheet links, and
  structural preload insertion. GoFrame does not infer markerless ownership
  through select/table/frameset/noscript insertion modes, declarative Shadow
  DOM, or ownership-affecting misnesting. A required rewrite in those contexts
  fails before publication rather than publishing a stale path;
- arbitrary direct fetch calls and scripts with additional authored statements
  remain unchanged. The recognizer does not validate arbitrary JavaScript;
  its closed historical grammar accepts ECMAScript whitespace, line
  terminators, and bounded comments as trivia and decodes static quoted WASM
  URLs with bounded ECMAScript string escapes while preserving their raw source
  span. Dynamic expressions, templates, concatenation, escaped identifiers,
  malformed or legacy octal and decimal escapes, and unsupported custom loaders
  remain byte-preserved and use an explicit managed block when rewriting is
  required. Asset-managed stylesheet rewriting inside declarative Shadow DOM
  is not part of the current preview contract;
- custom-index DOCTYPE source spans end at the first literal `>` even when a
  malformed public or system identifier contains an unmatched quote. A DOCTYPE
  through EOF remains opaque authored source;
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

Browser entrypoint extensions use ASCII-only case-insensitive matching. The
same private classification governs authored and generated WASM validation,
CSS entrypoint discovery, special browser media types, compression eligibility,
inspection, legacy WASM recognition, and static package serving. ASCII variants
such as `.WASM`, `.JS`, `.CSS`, and `.HTML` remain valid; Unicode simple-fold
lookalikes do not acquire those roles. This interpretation adds no generated
metadata field, changes neither generated metadata schema, and leaves the
inspection schema at version 1.

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
- ownership remains a cleanup-safety classification, so a current tool-owned
  package can still be removed when a declared ordinary asset or sidecar has
  subsequently been damaged;
- successful `goxc package` and `goxc export` runs apply the stronger complete
  graph integrity contract before printing success. If published verification
  fails, the completion marker is removed;
- `index.html` is a managed package artifact and is removed during package or
  export replacement so stale bootstraps cannot survive a later package run;
- adding metadata fields is backward-compatible;
- removing the marker or changing ownership detection is breaking unless it is
  required for a safety fix;
- exact timestamps are not stable.

## Package Inspection Schema

`goxc inspect --format=json` is a separate schema-version-1 tooling process
contract. It does not add fields to or change the schemas of
`asset-manifest.json` or `goframe-package.json`. Incompatible inspection field
or semantic changes require an inspection schema-version increment; consumers
should reject unsupported versions. No separate machine-readable JSON Schema
document is provided.

Inspection validates the declared current package graph, including assets,
entrypoints, hashes, compressed sidecars, physical identity, and the stable
completion-marker generation, but does not itself grant ownership. Package
publication and export reuse this strong validator without writing an inspect
report. Cleanup continues to use the narrower ownership classification, so
damaged tool-owned output remains removable. Shared metadata readers require
canonical generated logical names and paths; malformed or non-canonical
metadata is classified as incomplete. Unknown additive fields in generated
package metadata are not rejected merely because the report does not expose
them. `generatedAt` is intentionally omitted so the same package copied to
another safe root produces byte-identical JSON.

Schema-version-1 ordered strings use ascending lexical order of their raw UTF-8
bytes. Artifact paths and roles use that order, and edges compare `from`,
`kind`, `encoding`, and `to` field by field. Path fields and edge endpoints are
canonical slash-only package-relative paths; leading absolute and drive-prefix
forms are rejected. Consumers use POSIX/schema path semantics rather than
platform-native separator rules.

`artifact.logicalName` is a generated asset namespace key rather than an
independent package path. It is canonical and slash-only, but a colon or a
drive-looking prefix such as `C:logo.svg` remains literal key data. Its generated
package path is still contained below `assets/`, for example
`assets/C:logo.svg`. Platform-native separators are converted before
publication, while a remaining literal backslash is rejected. This does not
promise that every generated logical name is portable to filesystems that
reserve its characters. The inspection schema remains version 1, and its
fields and both generated metadata schemas remain unchanged. Authored
`goframe.json` paths have their own portable ingestion contract: backslashes
are separator syntax and successful loads store canonical slash paths before
filesystem or package consumers run.

For graph inspection, every asset logical key must use the exact canonical
relative-name representation returned by the current package producer helper
before an asset destination is derived. This tightens inspector acceptance of
damaged or externally produced metadata without changing either generated
package schema.

The default text report preserves all-graphic package-controlled values and
quotes a complete value when it contains a non-graphic rune. This is a
human-readable presentation rule only. Inspect JSON schema version 1 is
unchanged, and JSON escaping and decoded values continue to be defined by
`encoding/json`.

Stable package generation is guarded by the current `goframe-package.json`
completion marker. Inspection retains the marker's filesystem identity, exact
byte length, and full SHA-256, buffers the report, and verifies the marker again
before output. A missing, replaced, or changed marker returns a retry error with
no report. This does not add a package lock or cover external artifact mutation
that bypasses the completion-marker protocol.

Legacy `manifest.json` packages remain supported only for the documented
fail-closed cleanup and migration boundary. They are not supported inspection
inputs; `goxc inspect` reports migration guidance instead.

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

- No mandatory user-authored schema version in `v0.1.0-preview.1`.
- No signed package metadata.
- No machine-readable JSON schema file.
- Generated workspace layout under `.goframe` remains internal.
