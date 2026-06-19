# API Stability

## Current Status

GoFrame is experimental. APIs, GOX syntax, manifests, and toolchain behavior
may still change between MVPs.

This document classifies current surfaces so future changes can be discussed
with clear expectations. It does not create a stable 1.0 compatibility promise.

## Stability Tiers

### Experimental

Can change between MVPs. Changes should be documented, but migration support is
best-effort.

### Public-Candidate

Intended direction for user-facing APIs. Changes should include migration
notes, tests, and a compatibility reason.

### Internal

Runtime, compiler, toolchain, or smoke-test implementation details. These are
not user APIs and may change without compatibility guarantees.

### Legacy / Deprecated

Kept temporarily for migration or compatibility. New code should avoid these
surfaces.

## Compatibility Policy

Before public preview:

- compatibility is important but not absolute;
- breaking changes are allowed when they remove unsafe behavior, simplify a
  wrong API, or unblock the architecture;
- breaking changes should be documented in `CHANGELOG.md` and relevant docs;
- generated output and hidden workspace internals are not stable.

After public preview, this policy should be revisited and tightened.

## Current API Classification

### Public-Candidate

Runtime:

- `gf.El`
- `gf.Text`
- `gf.Fragment`
- `gf.Child`
- `gf.Key`
- `gf.Component`
- `gf.C`
- `gf.NewComponentType`
- `gf.ComponentT`
- `gf.UseState`
- `gf.UseReducer`
- `gf.UseEffect`
- `gf.UseUnmount`
- `gf.Deps`
- `gf.EveryRender`
- `gf.CreateContext`
- `gf.ProvideContext`
- `gf.UseContext`
- `gf.UseContextSelector`
- `gf.VirtualList`
- `gf.VirtualTable`
- basic event facades such as `gf.Event`, `gf.InputEvent`, and
  `gf.ScrollEvent`

Tooling:

- `goxc generate`
- `goxc build`
- `goxc package`
- `goxc export`
- `goxc serve`
- `goxc size`
- `goxc doctor`
- `goxc clean`
- `goxc version`

These are public-candidate because examples and docs rely on them, but their
exact shapes can still change before public preview.

### Experimental

- GOX syntax surface.
- GOX expression ergonomics.
- Context topology behavior and selector limitations.
- Virtualization details such as fixed-height range buffering and table spacer
  structure.
- Component identity id format for generated GOX component tokens.
- Package manifest field stability.
- Browser smoke scripts and debug probe output.
- VS Code extension commands and snippets.

### Internal

- dirty queue internals;
- mounted tree structures;
- component instance fields;
- state/effect/context slot storage;
- virtual range helper functions;
- `.goframe/work`, `.goframe/build`, `.goframe/package`, and `.goframe/cache`
  internal layout;
- `.goframe/gen` generated file layout;
- debug globals and browser probe object shapes;
- package staging directories;
- smoke harness implementation details.

### Legacy / Deprecated

- `goxc build --release`: accepted temporarily, but package/compress behavior
  belongs to `goxc package`.
- Explicit `"wasm": "main.wasm"` manifests: still supported, but examples and
  docs use `bundle.wasm`.
- Legacy `manifest.json` package marker: recognized for migration, but
  `goframe-package.json` is current.
- `goxc generate --in-place`: debug/legacy only. Generated `.gox.go` files
  should live under `.goframe/gen` or an explicit output directory.
- `UseMount`: deprecated alias for once-after-mount effect behavior.

## Deprecation Policy

For now, deprecations should:

- keep a clear warning or documentation note;
- have a replacement path;
- remain covered by tests if behavior is still accepted;
- be removed only in a planned cleanup MVP.

## What Is Not Stable Yet

Not stable:

- router design;
- SSR/hydration;
- Player/Engine or `.gfapp` format;
- child-entry and multi-module app support;
- final public component package identity policy;
- dynamic virtualization measurement;
- infinite loading;
- advanced accessibility/keyboard model for tables;
- LSP/formatter behavior;
- stable callback hook;
- error boundaries;
- production deployment server behavior.

## Road To Public Preview

Before public preview, GoFrame should have:

- a component identity decision for child-entry apps, multi-module workspaces,
  and reusable component packages;
- a symlink/file safety test matrix;
- clearer package manifest compatibility policy;
- stable migration notes for GOX syntax changes;
- documented browser support expectations;
- repeatable performance baseline updates;
- explicit public/deprecated/internal API review.
