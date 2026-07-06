# Contributing

Thanks for your interest in contributing to goframe.

By contributing to this repository, you agree that your contributions are
licensed under the Apache License, Version 2.0, the same license as the
project.

GoFrame is an experimental Go-first application framework and toolchain. The
current preview validates the browser/WASM application layer; it is not a
production-readiness claim or a 1.0 API guarantee.

## Workflow

Use short-lived topic branches from current `main`, then open a pull request
before merge. Keep each pull request focused on one reviewable concern.

Branch names should make the work area obvious. Common prefixes include:

- `docs/...`
- `test/...`
- `fix/...`
- `ci/...`
- `design/...`
- `toolchain/...`
- `runtime/...`

Pull request titles should use:

```text
type(scope): action object
```

Examples:

- `docs(preview): clarify platform evidence`
- `test(runtime): cover resource cleanup`
- `toolchain(package): reject unsafe asset paths`
- `ci(preview): add minimal platform evidence`

## Development Checks

Run the local checks before opening a pull request when practical:

```bash
scripts/check.sh
```

For Go changes, also run:

```bash
go test ./...
```

For docs changes, run:

```bash
node scripts/docs-check.mjs
```

For browser, runtime, package, or toolchain behavior changes, run the browser
smoke checks when the local environment has the required browser tooling:

```bash
scripts/browser-smoke.sh
```

If a check cannot run locally, explain the blocker in the pull request.

## Documentation Policy

GoFrame documentation is fact-first. Product docs should describe current
behavior, current limitations, current evidence, and current non-goals.

Avoid roadmap-style promise wording such as `planned`, `will add`, `future
follow-up`, or `may be added later` in product docs. Prefer factual wording such
as `outside current preview scope`, `not part of the current browser/WASM
preview`, or `current non-goal`.

The VS Code extension lives in `extensions/vscode-gox`.
