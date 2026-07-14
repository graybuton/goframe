# GoFrame v0.2.0-preview.6 Release Notes

## Status / Scope

`v0.2.0-preview.6` is a published experimental preview for GoFrame's current
browser/WASM surface. This release is focused on structured GOX diagnostics and
saved-source editor feedback.

The validated surface remains interactive browser/WASM applications built with
the GoFrame runtime, GOX, `goxc`, and standard Go or TinyGo WebAssembly output.
The new tooling makes authored GOX failures available through a read-only CLI
contract and the repository's lightweight VS Code extension.

GoFrame is not production-ready and does not provide a stable 1.0 API
guarantee. This preview does not claim fullstack/server APIs, SSR/hydration,
formatter, or LSP support. The VS Code extension is not published to the
Marketplace.

## Highlights

### GOX Tooling

- `goxc check <file-or-directory> [--format=text|json]` validates authored
  `.gox` source without writing generated output.
- The command accepts one `.gox` file or recursively checks an authored
  directory tree, including nested packages. Files are processed in
  deterministic lexical order, and a diagnostic in one file does not prevent
  later files from being checked.
- Checks use the existing in-memory GOX generation path and discard successful
  generated Go. They do not create `.goframe`, materialize a workspace, write
  `.gox.go` files, or invoke Go or TinyGo type checking.

### Structured Diagnostics

- Text output remains the default, while `--format=json` provides the
  versioned schema-v1 tooling transport.
- A completed clean check exits `0`. A completed check with source diagnostics
  exits `1`. Operational failures also exit `1` through the normal CLI error
  path, but they do not produce a completed or partial schema-v1 report.
- Completed JSON output is one compact document followed by a newline.
  Completed JSON diagnostic output does not add human-readable text to
  stderr.
- Schema-v1 diagnostics contain an absolute authored file path. `line` and
  `column` are one-based when available and `0` when unavailable, with `column`
  measured in UTF-8 bytes. Diagnostics also contain the current `"error"`
  severity, a message, and the authored source line when available.
  Consumers must reject unsupported schema versions.
- Exact diagnostic wording remains experimental.

### VS Code Editor Diagnostics

- `GOX: Check Current Project` runs
  `goxc check <workspace-folder> --format=json` and applies source diagnostics
  to authored files.
- Saving an authored `.gox` file checks its owning workspace folder. The
  diagnostics process is started without a shell and honors the existing,
  resource-scoped `gox.goxcPath` executable setting.
- The extension validates schema v1 and treats diagnostic exit `1` as a
  completed report. Absolute diagnostic paths are mapped to file URIs owned by
  the checked workspace.
- One-based UTF-8 byte columns are converted to VS Code UTF-16 editor positions
  from the saved filesystem bytes that `goxc` checked. Unsafe location mapping
  falls back to a file-level diagnostic instead of using a misleading raw
  column.
- Per-workspace generations prevent stale runs from replacing newer results,
  previous child processes are cancelled, and multi-root workspaces keep
  process and diagnostic ownership isolated.
- A completed report clears stale diagnostics for its workspace. VS Code
  delete and rename events clear diagnostics for old paths; a rename or move
  may recheck the destination workspace. These hooks are not general
  filesystem watch mode.
- VS Code Workspace Trust is required before the extension may execute the
  configured `goxc` executable. Automatic save and rename failures do not
  produce popup spam, while operational details are recorded in the dedicated
  `GOX Diagnostics` Output Channel.
- Focused pure Node tests and the VS Code Extension CI workflow cover the
  diagnostics transport, source-position mapping, stale-run coordination,
  multi-root ownership, and file-lifecycle helpers.

### Browser Evidence

- Todo browser smoke characterizes controlled-input updates, including retained
  node identity, focus, selection, property writes, and structural DOM work.
- Synchronous burst updates coalesce through the current animation-frame
  scheduling path, and keyed reorder bridge operations are attributed
  separately from focus and selection restoration.
- Dashboard browser smoke attributes visible-row creation, placement, removal,
  retained identity, and listener lifecycle during a representative search
  update.
- The measured scenarios did not identify a concrete redundant operation class
  that justified an immediate broad commit-buffer or mutation-queue rewrite.
  A buffered alternative was not measured, and no production runtime mutation
  architecture changed in this release line.

### Project And Release Documentation

- README is a version-agnostic project entry point that links to GitHub
  Releases and the stable release process instead of advancing a hard-coded
  preview pointer.
- Release-specific state remains in GitHub Releases, release notes, the
  changelog, and intentionally versioned evaluator documents.
- The status-qualified roadmap is the current planning document. Future
  version slots are planning handles, not current capabilities, schedules, or
  compatibility promises.

## Compatibility

- `goxc check` is additive. Existing `generate`, `build`, `package`, `export`,
  `serve`, and other normal toolchain workflows are unchanged.
- This preview does not change GOX grammar, runtime APIs, package layout, or
  manifest behavior.
- Existing VS Code command wrappers remain available. Inline diagnostics
  require a `goxc` version that contains the `check` command.
- Valid existing applications are not expected to require migration.
- Schema v1 is a versioned tooling transport. Consumers should reject
  unsupported versions, while exact diagnostic wording remains experimental.
- The editor integration uses saved authored source. It does not change Go or
  TinyGo compiler diagnostics or type-checking behavior.

## Validation

The release candidate passed:

- `git diff --check`;
- `node scripts/docs-check.mjs`;
- `go test ./...`;
- `go test -race ./pkg/... ./cmd/...`;
- `go vet ./...`;
- `go test -tags=goframe_debug ./...`;
- `go test ./pkg/gox -run 'TestGolden|TestErrorGolden'`;
- `scripts/check.sh`;
- `scripts/artifact-check.sh`;
- `scripts/module-path-check.sh`;
- `scripts/size-budget.sh`;
- `scripts/browser-smoke.sh`;
- `npm ci --prefix extensions/vscode-gox`;
- `npm test --prefix extensions/vscode-gox`.

The corresponding GitHub Actions workflows passed on the tagged commit:

- Core;
- Browser Smoke;
- WASM Size;
- VS Code Extension.

## Known Limitations And Follow-Ups

- Diagnostics describe saved authored `.gox` source only. Unsaved buffer
  content is not checked, and checks do not run on every edit.
- `goxc check` validates GOX parsing and generation only. It does not run Go or
  TinyGo semantic type checking or present generated-Go compiler diagnostics.
- The editor integration is process-based. It does not provide an LSP,
  formatter, semantic highlighting, completion, code actions, or quick fixes.
- The VS Code extension is not published to the Marketplace.
- Delete and rename cleanup covers VS Code file-operation events, not arbitrary
  external filesystem changes or general watch mode.
- Chrome/Chromium remains the strongest browser evidence. Firefox and
  Safari/WebKit do not have equivalent automated browser-smoke coverage.
- This preview does not claim production readiness, stable 1.0 APIs,
  fullstack/server behavior, SSR, or hydration.

## Install

Install the exact preview with:

```bash
go install github.com/graybuton/goframe/cmd/goxc@v0.2.0-preview.6
```

## Verification

Run:

```bash
goxc version
goxc doctor
goxc check ./examples/counter --format=json
```

Expected first line from the exact tagged install:

```text
goxc version v0.2.0-preview.6
```

The check command should produce valid JSON with `schemaVersion: 1`, `ok: true`,
and no diagnostics for the valid counter example. Its absolute file paths, if
present in future schema-compatible output, are environment-specific.

For a quick browser/WASM check:

```bash
goxc package ./examples/counter --compiler=tinygo
goxc serve ./examples/counter --port=8080
```

Use `--compiler=go` when TinyGo is unavailable or when standard Go/WASM output
is preferred for local inspection.

## Links

- [README](../README.md)
- [Evaluator guide](evaluator-guide.md)
- [GOX language](gox-language.md)
- [API stability](api-stability.md)
- [Platform support](platform-support.md)
- [Release process](release.md)
- [Roadmap](roadmap.md)
- [Previous preview: v0.2.0-preview.5](release-notes-v0.2.0-preview.5.md)
- [This preview: v0.2.0-preview.6](release-notes-v0.2.0-preview.6.md)

## Non-Goals

This preview does not add GOX grammar, runtime, package, manifest, or public API
behavior. It does not add unsaved-buffer analysis, Go/TinyGo semantic checking,
an LSP, formatter, completion, code actions, general filesystem watch mode, or
Marketplace distribution. It also does not add a commit buffer, mutation queue,
production server, fullstack API, SSR, or hydration.
