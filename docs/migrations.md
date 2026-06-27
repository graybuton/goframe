# Migration Notes

## Purpose

This document defines when migration notes are required and provides a template
for future changes. It does not invent migrations for unreleased behavior.

## When A Migration Note Is Required

Add a migration note when a change affects:

- Public-Candidate runtime API;
- GOX syntax or generated-code expectations visible to users;
- `goframe.json` input behavior;
- `goxc` command or flag behavior;
- package/export output contracts;
- documented browser/runtime semantics.

Security hardening still needs a note when users may see new rejections.

## Template

```md
## <version or stage>: <short title>

### Affected Surface

### Reason

### Old Usage

### New Usage

### Compatibility Window

### Removal Timing

### Automated Migration
```

## Historical Examples

### `main.wasm` to `bundle.wasm`

Status: legacy compatibility retained.

New examples and docs use `bundle.wasm`. Explicit manifests that set
`"wasm": "main.wasm"` still load and package.

### Explicit asset lists to asset directory mode

Status: legacy compatibility retained.

New examples and docs use `"assets": "./assets"` and keep static files under
an app-local `assets/` directory. Existing manifests such as
`"assets": ["index.html", "styles.css"]` still load and package. If no custom
HTML template is selected, `goxc package` generates a default root
`index.html`.

### Adjacent generated files to `.goframe`

Status: legacy/debug compatibility retained.

Normal `goxc generate`, `build`, and `package` workflows write generated
`.gox.go` files under `.goframe`. `goxc generate --in-place` remains available
only for debugging or legacy workflows and prints a warning.

### Function-call cross-package components to package-qualified GOX tags

Status: old Go function calls remain valid.

Package-qualified GOX tags such as `<layout.Shell />` are now preferred for
component boundaries across packages. Ordinary function calls still work when
you intentionally do not need a component boundary.
