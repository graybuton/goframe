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
- using staging directories before publishing packages;
- cleaning only known GoFrame-owned artifacts;
- binding the development server to `127.0.0.1`.

## Manifest Paths

Manifest fields such as `entry`, `output`, `wasm`, and `assets` are validated
as relative child paths. Absolute paths, `..`, and paths escaping the
application directory are rejected.

`entry` currently supports only `"."` for the hidden workspace builder. This
is a single-package app limitation, not a security feature.

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

Current path validation is lexical: paths must be relative child paths and must
not contain escapes such as `..` or absolute roots.

The current baseline does not yet deeply specify all symlink behavior. Treat
the following as not fully hardened:

- symlinked asset files;
- symlinked app directories;
- symlinked export directories;
- symlinks inside `.goframe`;
- symlinked legacy `dist/` during `clean --legacy`;
- symlinks that resolve outside the app after lexical validation.

Until explicit tests are added, avoid relying on symlinks for package assets or
tool-owned output directories.

## Current Limitations

- No full `EvalSymlinks` policy for every path boundary yet.
- No production static server hardening beyond local development needs.
- No signed package/export metadata.
- No permission model for future Player or `.gfapp` bundles.
- No multi-package app workspace model.

## Recommended Future Tests

Add focused tests before public preview for:

- asset symlink pointing outside app;
- app directory symlink;
- external `GOFRAME_WORKSPACE` symlink;
- export target symlink;
- `clean --legacy` with symlinked `dist`;
- package staging failure with existing package output preserved;
- serve path traversal behavior through `http.FileServer`.

## Non-Goals

This policy does not add:

- sandboxed execution;
- a production web server;
- package signing;
- permission prompts;
- SSR/hydration;
- Player/Engine security boundaries.
