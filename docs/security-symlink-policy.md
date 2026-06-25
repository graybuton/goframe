# Security: Symlink and File Safety Policy

## Purpose

This document records the current file safety model for `goxc`. It is a
baseline policy before public preview, not a claim that every symlink edge case
has been hardened.

## Current Safety Model

`goxc` treats authored application source and tool-owned output as separate
spaces:

- authored files stay in the app directory;
- generated files go under `.goframe/gen` by default;
- build output goes under `.goframe/build`;
- package output goes under `.goframe/package/standalone`;
- deploy directories are created only through explicit `goxc export` or
  explicit package `--out`.

Normal `generate`, `build`, and `package` commands should not create visible
`*.gox.go`, `build/`, or `dist/` files next to source.

## Protected Operations

Current protection focuses on:

- rejecting manifest paths that are not relative child paths;
- rejecting unknown manifest fields;
- refusing non-empty non-GoFrame package/export output directories;
- validating GoFrame ownership markers as regular, structured metadata instead
  of trusting filenames alone;
- rejecting intermediate symlink components below declared app/workspace/output
  roots before reading, writing, copying, deleting, or serving files;
- publishing generated files through temporary sibling files and replacement so
  destination symlinks and hardlink peers are not truncated;
- planning package asset output names before publication so user assets cannot
  overwrite generated WASM, runtime shims, metadata, or compressed sidecars;
- using staging directories before publishing packages;
- cleaning only known GoFrame-owned artifacts;
- binding the development server to `127.0.0.1`.

## Manifest Paths

Manifest fields such as `entry`, `output`, `wasm`, and `assets` are validated
as relative child paths. Absolute paths, `..`, and paths escaping the
application directory are rejected.

`entry` supports `"."` and relative child package directories such as
`"./cmd/app"`, `"cmd/app"`, `"./src/app"`, and `"app"`. Entries that point to
tool-owned directories such as `.goframe`, `build`, `dist`, `node_modules`, or
`.git` are rejected. The entry must point to a directory inside the app root,
not a file.

## Workspace Paths

The default workspace is:

```text
<app>/.goframe/
```

For read-only source mounts, use:

```bash
GOFRAME_WORKSPACE=/work/goframe goxc package /src/app
goxc package /src/app --workspace /work/goframe
```

When an external workspace is used, `goxc` scopes output under an app slug to
avoid simple collisions. The workspace is tool-owned and should not contain
authored source.

## Package Output

Default package output is:

```text
<app>/.goframe/package/standalone/
```

Explicit `goxc package --out <dir>` treats `<dir>` as tool-owned package
output. If the directory is non-empty and does not look like a previous GoFrame
package, the command fails instead of deleting user files.

GoFrame-owned markers:

- `goframe-package.json`
- `asset-manifest.json`
- legacy `manifest.json`

These markers must be regular files with recognizable GoFrame package
structure. An empty `{}` file, malformed JSON, symlinked marker, or generic web
`manifest.json` is not enough to grant ownership.

## Export Output

`goxc export <app> --out <dir>` copies the latest standalone package to an
explicit deploy directory.

The output directory is considered tool-owned. If it is non-empty and has no
GoFrame package marker, export fails. `--force` tells `goxc` to treat the
directory as package output and overwrite known package artifacts.

Do not use `--force` against a directory that contains user-owned assets unless
that overwrite is intentional.

## Clean

`goxc clean <app>` removes tool-owned workspace output:

- `.goframe/work`
- `.goframe/build`
- `.goframe/package`

`--generated` also removes `.goframe/gen` and legacy adjacent `*.gox.go` files.

`--legacy` helps migrate old layouts. It removes legacy `build/` and adjacent
generated `.gox.go` files. It removes legacy `dist/` only when that directory
looks GoFrame-owned. User-owned `dist/` directories are left alone.

## Serve

`goxc serve` is development-only. It serves from `.goframe/package/standalone`
or an explicit `--dir`, binds to localhost, and sets common content types for
WASM, JavaScript, CSS, gzip, and brotli sidecars.

It is not a production server. Production compression negotiation, cache
headers, TLS, path hardening, and access controls belong to deployment
infrastructure.

## Symlink Policy

Current path validation is root-aware and symlink-aware at safety-sensitive
boundaries. `goxc` validates paths relative to the declared app, workspace,
package, export, or serve root and checks each existing component inside that
controlled subtree with `os.Lstat`.

Policy:

- app roots that are themselves symlinks are rejected;
- entry package directories must not be symlinks;
- `.go` and `.gox` source files discovered for workspace materialization must
  not be symlinks;
- manifest-declared package assets must not be symlinks, including broken
  symlinks;
- explicit package/export output directories and intermediate parents must not
  be symlinks;
- the standalone package directory used as export source must not be a symlink;
- `goxc clean` removes final tool-owned symlinks as links and rejects
  intermediate workspace symlinks instead of traversing targets;
- `goxc serve` rejects symlinked roots and symlinked entries inside the served
  tree;
- explicit external workspaces remain allowed because the user opted into that
  root directly, but the final app-scoped workspace root must not overlap the
  app source tree.

The preferred security direction is reject rather than follow when a symlink
could make source, asset, package, export, or cleanup operations escape the
declared root.

Evidence:

- `cmd/goxc/symlink_test.go`;
- `cmd/goxc/filesystem_safety_test.go`;
- `cmd/goxc/workspace.go`;
- `cmd/goxc/package.go`;
- `cmd/goxc/export.go`.

## Symlink Matrix

| Scenario | Status | Policy |
|---|---|---|
| app root is symlink | Ready | Rejected before mutation. |
| entry directory is symlink | Ready | Rejected. |
| intermediate entry component is symlink | Ready | Rejected before read/copy/generation. |
| `.go`/`.gox` source file is symlink | Ready | Rejected during discovery/materialization and direct-file generation. |
| asset is symlink | Ready | Rejected, including broken symlinks. |
| symlink target stays inside app root | Ready with limitations | Still rejected for entry/source/assets; the policy is simpler and safer. |
| symlink target escapes app root | Ready | Rejected for entry/source/assets/output roots. |
| broken symlink | Ready | Rejected for manifest assets and source discovery. |
| symlink loop | Ready with limitations | Rejected at Lstat boundaries; broader loops are not supported. |
| external `GOFRAME_WORKSPACE` | Ready | Allowed and scoped under an app-specific slug when it does not overlap the app tree. |
| external workspace inside app tree | Ready | Rejected before workspace refresh/copy. |
| workspace path collides between apps | Ready | External workspace slug includes a hash of app path. |
| read-only source tree | Ready with limitations | Use external workspace. |
| package output path is symlink | Ready | Rejected. |
| export destination is symlink | Ready | Rejected. |
| destination file is symlink | Ready | Rejected before atomic replacement. |
| package source contains symlink | Ready | Rejected before copy; external content is not published. |
| package source contains FIFO/socket/device | Ready with limitations | Non-regular entries are rejected; platform-specific special-file coverage may vary. |
| generic `manifest.json` in `dist` | Ready | Not considered GoFrame-owned; user files are preserved. |
| `clean --legacy` sees symlinked `dist` | Ready with limitations | Final symlink is removed as a link when it is a tool-owned cleanup target; relying on symlinked legacy `dist` is not a supported workflow. |
| explicit `--out` path | Ready | Must be empty, GoFrame-owned, or rejected; symlink root rejected. |
| generated `.goframe` path | Ready with limitations | Intermediate symlinks are rejected; final cleanup symlinks are removed as links. |
| user asset named `bundle.wasm` or `wasm_exec.js` | Ready | Rejected as a generated namespace collision. |
| user asset compressed sidecar collision | Ready | Rejected before package publication. |
| export source/output overlap | Ready | Rejected before cleanup/copy. |
| serve tree symlink entry | Ready with limitations | Dev server returns 404 for symlink entries; it remains development-only. |

## Package Publication Integrity

Before publishing, `goxc` validates the staged package tree and rejects symlinks
and non-regular file entries. Package metadata is copied last so a mid-copy
failure cannot leave `goframe-package.json` describing a newly completed tree.

This is not a full transactional installer. If a copy fails after old
package-owned files were cleaned, the destination may need another successful
`goxc package` or `goxc export` run to restore all files. The package is not
marked complete until metadata is present.

## Residual TOCTOU Risk

The policy is designed for static repository trees and ordinary user mistakes.
It does not attempt to sandbox a hostile local process that concurrently
replaces paths between validation and an operation. Obvious final symlink
write-through and cleanup traversal cases are still checked immediately at the
operation boundary, but full adversarial filesystem mutation is out of scope.

## Current Limitations

- No production static server hardening beyond local development needs.
- No signed package/export metadata.
- No permission model for future Player or `.gfapp` bundles.
- No full multi-module workspace model.
- `goxc serve` remains development-only and is not a hardened static server.
- Windows filesystem behavior is not CI-verified.

## Recommended Future Tests

Keep adding focused tests for:

- Windows path edge cases;
- package rollback behavior when replacing an existing valid package;
- additional special file types where the platform can create them safely;
- serve path traversal behavior on non-Linux platforms.

## Non-Goals

This policy does not add:

- sandboxed execution;
- a production web server;
- package signing;
- permission prompts;
- SSR/hydration;
- Player/Engine security boundaries.
