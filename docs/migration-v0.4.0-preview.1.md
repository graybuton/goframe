# Migrating to v0.4.0-preview.1

## Status / Scope

Valid applications and standalone packages produced by
`v0.3.0-preview.1` require no source or package-layout migration for
`v0.4.0-preview.1`. The public `pkg/goframe` and `pkg/gox` export sets,
generated package metadata schemas, valid package publication protocol, and
ordinary package artifacts remain unchanged.

This preview adds the `goxc inspect` command and tightens validation of
malformed generated package metadata and browser-extension spellings. The
actions below apply only where an existing workflow relied on the previously
accepted edge behavior.

## Valid Applications

No application-source migration is required for valid applications using the
documented runtime, GOX, manifest, package, export, serve, and development
contracts.

`goxc inspect` is additive. Existing commands retain their previous flags and
help contracts. To inspect an existing current package, first create or locate
the package and then run:

```bash
goxc package ./examples/counter --compiler=tinygo
goxc inspect ./examples/counter
goxc inspect ./examples/counter --format=json
```

## Generated Package Metadata

### Affected Surface

Manually edited standalone packages, external tools that produce GoFrame-like
package metadata, and damaged or non-canonical package directories.

### Current Behavior

Current-package readers require generated logical names and package paths to
use the canonical slash-only forms produced by `goxc package`. Malformed or
non-canonical metadata is classified as incomplete. Inspection also rejects
invalid entrypoint roles, declared hashes, sidecars, collisions, containment,
physical aliases, and changed completion markers before report output.

The generated `asset-manifest.json` and `goframe-package.json` schemas remain
version 1. The separate `goxc inspect --format=json` report is a schema-v1
tooling process contract and does not alter either package metadata schema.

### Migration

- Regenerate the standalone package with the current `goxc package` command.
- Do not repair generated package metadata by hand.
- Use `goxc inspect` to validate the resulting current package.
- Treat an incomplete-package result as a request to regenerate, not as
  destructive ownership evidence.

Valid current packages and exported copies require no metadata rewrite.

## Browser Extension Recognition

### Affected Surface

Package inputs or generated metadata that relied on Unicode case-fold
lookalikes such as `.waſm`, `.jſ`, or `.csſ` being treated as WASM,
JavaScript, or CSS.

### Current Behavior

Browser asset extensions are matched case-insensitively for ASCII letters
only. ASCII forms such as `.WASM`, `.JS`, `.CSS`, and `.HTML` keep their
browser roles. Unicode lookalikes remain ordinary assets and do not become
browser entrypoints through case folding.

### Migration

Rename a browser entrypoint to an ASCII extension with the intended role, then
regenerate the package. No migration is needed for lowercase or mixed-case
ASCII extensions.

## Development Failure Presentation

After `goxc dev` has started successfully, package and build failures continue
to be printed in the terminal and are also presented in connected browser
pages. The last successful generation remains served and interactive. A later
failure replaces the development message; successful recovery clears it before
the normal full-page reload.

No application-source migration is required. Automation that reads terminal
diagnostics may continue to do so. Initial package failures and watch or scan
failures remain terminal-only. This behavior does not add HMR, incremental
compilation, or source maps.

## Rollback

During preview evaluation, pin the previous published module when a migration
or regression cannot be evaluated immediately:

```bash
go install github.com/graybuton/goframe/cmd/goxc@v0.3.0-preview.1
```

Keep the CLI and application module on the same intended preview while
comparing behavior. GoFrame remains pre-1.0 and experimental.
