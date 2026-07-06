# GoFrame v0.2.0-preview.4 Release Notes

## Summary

`v0.2.0-preview.4` is a maintenance, correctness, performance,
release-readiness, and public-surface preview for GoFrame's current
browser/WASM layer.

This preview improves runtime hot paths, recovers WASM size headroom, reduces a
common keyed placement move pattern, fixes scheduler reset correctness, improves
GOX diagnostics, tightens `goxc doctor` command behavior, and refreshes the
repository public presentation.

GoFrame remains experimental. The validated surface is interactive
browser/WASM applications built with the GoFrame runtime, GOX, `goxc`, and
standard Go or TinyGo WebAssembly output. This preview does not claim
production readiness, stable 1.0 APIs, fullstack/server APIs, SSR/hydration,
Player/Engine, `.gfapp`, formatter, or LSP support.

## Highlights

### Runtime And Performance

- `splitProps` hot paths were characterized and reduced, while preserving
  zero-allocation behavior for nil and empty props.
- `splitProps` now uses a smaller slice-backed internal shape for DOM and event
  prop output.
- `VirtualTable` runtime code was reduced to recover dashboard WASM size
  headroom.
- Keyed child placement now skips redundant moves for a stable suffix pattern.
  The characterized rotate-right case `[a b c d] -> [d a b c]` improves from
  three existing-node moves to one.
- This is not a full LIS reconciliation implementation. Broader keyed reorder
  optimization remains future work.

### Runtime Correctness

- The update scheduler reset path now invalidates stale queued callbacks. A
  callback scheduled before `reset()` can no longer run stale update work after
  a newer request is queued.
- Browser smoke coverage now includes focused input value and selection
  preservation during dirty updates.
- A full batching mutation queue or commit-buffer architecture remains future
  architecture work. Related issue: #70.

### GOX

- Component prop names that are Go keywords are rejected before codegen with a
  source-level diagnostic.
- DOM attributes such as `type` remain valid where they are DOM props rather
  than generated Go struct field names.
- GOX no longer emits invalid generated Go for keyword component prop names.

### goxc

- `goxc doctor` now returns a nonzero command result when required checks fail.
- Existing user-readable doctor diagnostics remain the primary explanation for
  failed checks.

### Public And Community Surface

- The README was rewritten into a newcomer-first landing page with Quick Start
  near the top.
- The README now includes a branded hero header and the GoFrame mark.
- Lightweight project branding assets were added under `assets/brand/`.
- `CODEOWNERS`, `NOTICE`, `SECURITY`, `CONTRIBUTING`, and other
  community-facing metadata were clarified.
- No GitHub social preview image is committed in the repository.

### Audits

- Runtime audit completed with no blocker or high-severity findings after the
  focused runtime fixes.
- GOX audit completed with no blocker or high-severity findings after the
  keyword component prop diagnostic fix.
- `cmd/goxc` audit completed with no blocker or high-severity findings after
  the doctor exit-status fix.

## Compatibility

- Browser/WASM remains the validated preview surface.
- Standard Go and TinyGo workflows remain supported as documented.
- No stable API guarantee is introduced.
- No production hosting or production runtime claim is introduced.
- Normal preview users do not need a migration.
- Users who relied on invalid GOX component props named with Go keywords must
  rename those props to valid exported Go field names.
- Documentation, community metadata, and branding changes do not affect runtime,
  GOX, or `goxc` code behavior.

## Known Limitations And Follow-Ups

- `splitProps` allocation work improved hot paths, but broader allocation
  elimination can continue.
- A batching mutation queue or commit-buffer architecture remains future work.
  Related issue: #70.
- Keyed placement improved a common stable-suffix case, but full LIS
  reconciliation remains future work. Tracked separately in #71.
- Retained focused-input selection restoration still has a focused follow-up.
  Follow-up area: #69.
- Malformed embedded GOX expression diagnostics can be improved.
- Workspace module directive fidelity is intentionally limited and can be
  refined later.
- Production readiness, SSR/hydration, fullstack/server APIs, Player/Engine,
  `.gfapp`, formatter, and LSP remain non-goals for this preview.

## Install

After the tag is published, install the exact preview with:

```bash
go install github.com/graybuton/goframe/cmd/goxc@v0.2.0-preview.4
```

## Verification

Run:

```bash
goxc version
goxc doctor
```

Expected first line after an exact tagged install:

```text
goxc version v0.2.0-preview.4
```

For a quick browser/WASM check:

```bash
goxc package ./examples/counter --compiler=tinygo
goxc serve ./examples/counter --port=8080
```

Use `--compiler=go` when TinyGo is unavailable or when standard Go/WASM output
is preferred for local inspection.
