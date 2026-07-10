# GoFrame v0.2.0-preview.5 Release Notes

## Status / Scope

`v0.2.0-preview.5` is an experimental preview release for GoFrame's current
browser/WASM surface.

This preview focuses on post-preview.4 correctness, diagnostics, and
reconciliation behavior. It improves retained input selection restoration, GOX
source diagnostics, and keyed child placement.

GoFrame remains experimental. The validated surface is interactive browser/WASM
applications built with the GoFrame runtime, GOX, `goxc`, and standard Go or
TinyGo WebAssembly output.

This preview does not claim production readiness, stable 1.0 APIs,
fullstack/server APIs, SSR/hydration, Player/Engine, `.gfapp`, formatter, or
LSP support.

## Highlights

### Runtime

- Retained focused inputs now restore `selectionStart` and `selectionEnd` after
  dirty component updates.
- Existing focus behavior is preserved: retained active inputs do not receive
  unnecessary `focus()` calls only to restore selection.
- No broad focus manager or id-less focus capture expansion was introduced.

### Reconciliation

- Keyed child placement now keeps the longest increasing keyed subsequence
  stable during placement under the current matching semantics.
- The implementation is exact O(n log n), not unbounded O(n^2).
- The characterized middle-backward keyed reorder improves from two
  existing-node moves to one.
- The characterized rotate-right keyed reorder remains at one move.
- Long keyed list behavior has focused test and benchmark coverage.
- Keyed matching, duplicate-key behavior, keyed/unkeyed behavior, and component
  identity semantics are unchanged.

### GOX

- Malformed embedded child expressions now report GOX/source-level diagnostics
  before generated Go parsing.
- Malformed attribute expressions now report GOX/source-level diagnostics
  before generated Go parsing.
- Nested GOX markup inside embedded Go expressions preserves inner source
  locations for diagnostics.
- No GOX grammar expansion, formatter, LSP, or semantic/type-checking
  diagnostics were added.

### Release And Docs Process

- Repository docs distinguish published-preview state from prepared-preview
  state across the release cycle.
- Release documentation includes GitHub Release title/body style guidance for
  future preview releases.
- These were process/docs cleanups, not runtime behavior changes.

## Compatibility

- Browser/WASM remains the validated preview surface.
- Standard Go and TinyGo workflows remain supported as documented.
- No stable API guarantee is introduced.
- No production hosting or production runtime claim is introduced.
- Normal preview users should not need migration for valid code.
- Invalid GOX embedded expressions now fail earlier with source-level
  diagnostics.
- Keyed reorder DOM placement may perform fewer moves, but matching and state
  semantics are unchanged.
- Documentation/process changes do not affect runtime, GOX, or `goxc`
  behavior.

## Validation

Release-gate validation for this preview should include:

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
- VS Code extension compile validation;
- GitHub Actions Core;
- GitHub Actions Browser Smoke;
- GitHub Actions WASM Size;
- GitHub Actions VS Code Extension.

## Known Limitations And Follow-Ups

- `splitProps` allocation work improved hot paths earlier, but broader
  allocation elimination can continue. Related issue: #69.
- A batching mutation queue or commit-buffer architecture remains future work.
  Related issue: #70.
- Keyed placement now has exact O(n log n) LIS-aware stable placement under the
  current matching semantics; broader reconciler architecture changes remain
  separate. Related area: #71.
- Broader focus capture for id-less active elements remains separate.
- Semantic/type-checking diagnostics for GOX remain separate.
- Workspace module directive fidelity remains intentionally limited and can be
  refined later.
- Production readiness, SSR/hydration, fullstack/server APIs, Player/Engine,
  `.gfapp`, formatter, and LSP remain non-goals for this preview.

## Install

Install the exact preview with:

```bash
go install github.com/graybuton/goframe/cmd/goxc@v0.2.0-preview.5
```

## Verification

Run:

```bash
goxc version
goxc doctor
```

Expected first line after an exact tagged install:

```text
goxc version v0.2.0-preview.5
```

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
- [API stability](api-stability.md)
- [Platform support](platform-support.md)
- [Deployment](deployment.md)
- [Previous preview: v0.2.0-preview.4](release-notes-v0.2.0-preview.4.md)
- [This preview: v0.2.0-preview.5](release-notes-v0.2.0-preview.5.md)
