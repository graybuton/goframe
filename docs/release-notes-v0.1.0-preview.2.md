# Release Notes: v0.1.0-preview.2

## Summary

`v0.1.0-preview.2` is a maintenance preview after
`v0.1.0-preview.1`. It focuses on public repository hygiene and security/test
harness hardening that landed on `main` after the first evaluator preview.

GoFrame remains an experimental Go-first application platform. This maintenance
preview does not expand the runtime/API stability promise, does not make
`goxc serve` a production server, and keeps the preview scope centered on
browser/WASM evaluator use.

Scope preview != scope project. The first preview remains narrower than the
project vision without removing or hiding working experimental surfaces.

## What Changed Since v0.1.0-preview.1

### Community Health

The repository now includes minimal public-preview community files:

- issue templates for bug reports and feature requests;
- a pull request template;
- updated contribution guidance;
- a short Code of Conduct.

These files describe contribution expectations for an experimental public
preview repository. They do not change runtime, toolchain, package, or example
behavior.

### `goxc serve` Path Hardening

`goxc serve` now hardens static request path handling:

- request paths are sanitized before filesystem resolution and before handing
  the request to `http.FileServer`;
- traversal and backslash paths return 404;
- symlinked served entries remain fail-closed;
- existing static asset headers for WASM, JavaScript, CSS, gzip, and brotli
  sidecars are preserved.

`goxc serve` remains development-only. Production static hosting, cache
headers, TLS, access control, and broader server hardening remain outside the
current preview contract.

### Browser Smoke Harness Hardening

Browser smoke scripts no longer construct dynamic executable JavaScript from
test-controlled selectors, labels, or expected values in the targeted CodeQL
alert patterns. The harness passes those values as Chrome DevTools Protocol
function arguments instead.

This is test-harness hardening only. Browser runtime behavior and smoke
assertions are unchanged.

## Compatibility

This maintenance preview has no intended:

- runtime API changes;
- GOX language changes;
- manifest, package, or export semantics changes;
- workflow support expansion;
- preview promise expansion.

The `v0.1.0-preview.1` compatibility boundaries still apply: public-candidate
API shapes remain pre-1.0, experimental semantics remain explicitly scoped, and
production readiness is not claimed.

## Validation

Expected release-branch validation includes:

```bash
git diff --check
node scripts/docs-check.mjs
go test ./...
```

Additional checks may be run by CI or release maintainers, but this maintenance
release note only records commands that are part of the documented local
release-prep validation set.

## Notes For Evaluators

After release publication, install the exact CLI tag when exact preview
selection matters:

```bash
go install github.com/graybuton/goframe/cmd/goxc@v0.1.0-preview.2
```

`@latest` may depend on Go module proxy and cache timing. Use the exact tag
when immediate reproducibility matters after publication.

## Non-Goals

`v0.1.0-preview.2` does not include:

- production readiness;
- stable 1.0 API compatibility;
- runtime feature changes;
- GOX language changes;
- manifest/package/export contract changes;
- browser/platform support expansion;
- turning `goxc serve` into a production static server;
- SSR, hydration, history routing, Player/Engine, or `.gfapp` packaging.
