# GoFrame v0.2.0-preview.3 Release Notes

## Summary

`v0.2.0-preview.3` is a hotfix preview after `v0.2.0-preview.2`.

It fixes stale `goxc version` output. `v0.2.0-preview.2` installed from the
correct Go module tag, but the CLI self-reported:

```text
goxc version 0.1.0
```

`goxc version` now reports the module version recorded in Go build information
for tagged module installs. Local checkout builds report:

```text
goxc version devel
```

## What Changed

- `goxc version` no longer uses the hardcoded release constant for CLI
  self-reporting.
- Tagged module installs such as `@v0.2.0-preview.3` report
  `v0.2.0-preview.3`.
- Local checkout builds such as `go run ./cmd/goxc version` and
  `go install ./cmd/goxc` report `devel`.

## Compatibility

- No GoFrame runtime behavior changed.
- No GOX parser, codegen, or language behavior changed.
- No `goxc` package, build, export, serve, clean, doctor, workspace, or
  manifest behavior changed.
- No examples changed.
- No migration is required.

## Validation

Release-gate validation for this hotfix should include:

- `go test ./cmd/goxc`;
- `go test ./...`;
- `go vet ./...`;
- `node scripts/docs-check.mjs`;
- `scripts/artifact-check.sh`;
- `scripts/module-path-check.sh`;
- `scripts/size-budget.sh`;
- `scripts/browser-smoke.sh`.

The release readiness check should also verify local checkout behavior:

```bash
go run ./cmd/goxc version
tmpbin="$(mktemp -d)"
GOBIN="$tmpbin" go install ./cmd/goxc
"$tmpbin/goxc" version
```

Both local commands should print `goxc version devel` on the first line.

## Install

After the tag is published, install the exact hotfix preview with:

```bash
go install github.com/graybuton/goframe/cmd/goxc@v0.2.0-preview.3
```

## Verification

Run:

```bash
goxc version
```

Expected first line:

```text
goxc version v0.2.0-preview.3
```

## Non-Goals

This hotfix does not include:

- runtime changes;
- GOX parser, codegen, or language changes;
- `goxc` package/build/export/serve behavior changes;
- server/fullstack API changes;
- production readiness claims;
- route loader, JSON/data framework, or global cache changes;
- release tag creation in the release-notes PR.
