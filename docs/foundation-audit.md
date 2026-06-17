# Foundation Audit

This document captures the Foundation Hardening Audit after MVP 8.1. The goal
is to keep `goframe` small, understandable, and safe enough for the next
runtime stages without adding product features.

## Summary

`goframe` now has clear project layers:

- `pkg/goframe`: runtime/framework, virtual nodes, component identity,
  component-scoped state, dirty updates, DOM patching, events, and browser
  mounting.
- `pkg/gox`: GOX lexer/parser/codegen. It emits Go calls into the runtime but
  does not mount, compile, or package applications.
- `cmd/goxc`: toolchain CLI for generate/build/package/size/serve/doctor/clean.
- `examples`: reference applications and integration smoke targets.
- `scripts`: regression probes that are intentionally outside production
  runtime.
- `extensions/vscode-gox`: editor support.

The most important architectural boundary is still healthy: `goframe` is the
runtime, `goxc` is the compiler/toolchain, and GOX is source syntax.

The audit found three small but real hardening issues and fixed them:

- `splitProps` compared `any` values directly with `false`, which could panic
  for uncomparable values.
- `goframe.json` silently ignored unknown fields, making typo mistakes hard to
  diagnose.
- `goxc package` prepared some artifacts directly in `dist/`; it now stages the
  package before publishing, and MVP 13.1 keeps normal package output inside
  `.goframe/package/standalone`.

Foundation Hardening II then closed two runtime risks:

- dirty queue ancestor pruning removes redundant child updates when an ancestor
  is dirty in the same flush;
- duplicate sibling keys now produce debug-only browser diagnostics.

MVP 9 closes the next lifecycle foundation gap:

- component-scoped `UseEffect` and `UseUnmount`, with `UseMount` retained as a
  deprecated compatibility alias;
- explicit primitive dependency helpers without reflection;
- cleanup on component unmount/replacement;
- debug-only warnings for Set-after-unmount and Set-during-render;
- a small debug guard for effect-triggered update loops.

MVP 10 is a language/API cleanup rather than a new runtime feature. It changes
the canonical state/effect surface to tuple state and optional effect deps,
adds expression-oriented GOX conditional rendering, adds `Key={...}` as a
pseudo-prop, and keeps low-level constructors as generated-code primitives.

## Architecture Review

The current separation of responsibilities is good for an MVP platform:

- Runtime code does not import the GOX compiler or CLI.
- GOX codegen knows the public runtime API (`gf.El`, `gf.Component`, `gf.Child`)
  but does not know browser DOM implementation details.
- CLI code owns build/package/serve concerns and keeps compression out of
  `build`.
- Examples use public APIs and are good integration probes rather than hidden
  runtime dependencies.

Temporary decisions to keep visible:

- Component identity is based on component name plus key/position, not Go
  function identity.
- Direct function component calls are valid Go but do not create runtime
  component identity.
- State slots are component-scoped but still positional.
- There is no automatic props memoization.
- The browser runtime assumes it owns the mounted DOM subtree.

These are acceptable now, but they are the areas most likely to be touched by a
future component lifecycle/effects stage.

## Runtime Review

Reviewed areas:

- Node model: `VNode`, `TextNode`, `FragmentNode`, `EmptyNode`, `KeyedNode`,
  `ComponentNode`.
- Mounted tree: DOM ranges are retained through `first`/`last` references.
- Fragment/component anchors: comments provide stable ranges for multi-node
  values.
- Keyed nodes: keys are stored on mounted boundaries and used for child
  matching.
- State slots: scoped to the currently rendering component instance.
- Dirty queue: state updates enqueue owner component instances and coalesce via
  the browser scheduler.
- DOM patch: text, props, events, positional children, keyed children, fragments,
  and incompatible replacement.
- Events: a stable browser listener is retained per event name and the Go
  callback pointer is updated.
- Controlled inputs: `value` is set as a property only when the browser value
  differs.
- Debug hooks: browser probes compile only with `goframe_debug`.

Fixed runtime issue:

- `Props` no longer compares `any` values directly to `false`. That direct
  comparison could panic if a user accidentally passed an uncomparable value
  such as a slice or map.

Runtime risks that remain:

- Duplicate key diagnostics are debug-only and do not include source locations.
- `UseState` call order must remain stable. Changing the order is undefined and
  may panic if types change.
- `Set` after unmount mutates the detached state slot but does not schedule.
  This is safe for now but there is no warning.
- Lifecycle hooks now cover mount/effect/unmount cleanup, but there is still no
  context, error boundary, async lifecycle, or external DOM mutation recovery.
- Event listener release depends on runtime unmount/replace paths; external
  DOM mutation remains outside runtime ownership.

## GOX Compiler Review

GOX remains deliberately small:

- Lowercase tags generate `gf.El`.
- Capitalized tags generate `gf.Component`.
- Props follow `<ComponentName>Props`.
- Children become `Children: []gf.Node{...}`.
- Expressions become `gf.Child(expr)`.
- Fragments generate `gf.Fragment`.
- GOX render expressions lower `condition && <Node />` to `gf.If`.
- GOX ternary render expressions lower `condition ? <A /> : <B />` to
  `gf.IfElse`.
- `Key={...}` lowers to `gf.Key` and is removed from props/attributes.
- Nested callback returns can contain GOX markup.

Golden tests cover simple elements, component props, component children,
fragments, helper expressions, keyed items/components, nested components, input
events, and diagnostics for mismatched/unclosed/unsupported syntax.

Remaining compiler risks:

- The parser is purpose-built and not a full Go parser. It works for current
  GOX blocks but may need a proper source-aware parser before larger syntax.
- Diagnostics are human-readable but not yet structured machine diagnostics.
- Component namespace tags and spread props are intentionally rejected.
- GOX does not validate whether a referenced props struct or component function
  exists; the Go compiler remains the type checker.

## CLI Review

`goxc` responsibilities are now clean:

- `generate`: `.gox` to `.goframe/gen/*.gox.go`.
- `build`: raw WASM into `.goframe/build/`.
- `package`: runnable `.goframe/package/standalone` bundle.
- `export`: explicit deploy directory copy.
- `size`: artifact inspection.
- `serve`: localhost static server with `application/wasm`.
- `doctor`: toolchain diagnostics.
- `clean`: build/package artifact cleanup.

Fixed CLI issues:

- Manifest parsing now rejects unknown fields with a useful JSON error.
- `package` builds the bundle in a staging directory first, then publishes
  prepared artifacts into `.goframe/package/standalone`.
- MVP 13 package publishing removes stale root bundle artifacts and the
  package-owned `assets/` directory before publishing the next staged bundle.
- Explicit package/export output directories are treated as tool-owned. Non-empty
  directories without GoFrame package markers are rejected before package-owned
  `assets/` cleanup.
- Legacy `manifest.json` is still recognized as a GoFrame-owned package marker
  for migration cleanup.

Remaining CLI risks:

- `serve` is a local development server. It binds to `127.0.0.1` and is not a
  hardened production static server.
- `--out` is intentionally user-directed and may point outside the application
  directory, but non-empty unmarked package/export directories are rejected.
- Compression remains an explicit package helper; production deployments should
  prefer web-server or CDN compression.

## Size Review

Production `pkg/goframe` is guarded against heavyweight imports:

- no `reflect` import in production runtime;
- no `fmt` import in production runtime;
- no `encoding/json`, `regexp`, `net/http`, `runtime/debug`, or `log` import in
  production runtime;
- debug browser probes compile only with `goframe_debug`;
- `ToString` uses `strconv` instead of `fmt.Sprint`;
- props comparison does not use `reflect.DeepEqual`.

TinyGo size budgets:

| Example | Budget |
|---|---:|
| Counter | 95 KiB |
| Components | 105 KiB |
| Todo | 120 KiB |
| Dashboard | 150 KiB |

Run:

```bash
scripts/size-budget.sh
```

The budget gate also checks gzip and brotli delivery sizes and optional zstd
when the `zstd` tool is installed.

Measured during the foundation audit:

| Example | TinyGo WASM |
|---|---:|
| Counter | 77,890 B |
| Components | 83,159 B |
| Todo | 109,483 B |
| Dashboard pressure test | 146,832 B |

Counter and Components show the approximate runtime cost of MVP 9 lifecycle
hooks. Todo includes example-level localStorage persistence and compact string
encoding, so its increase is not purely runtime overhead. Dashboard is a
larger example-level pressure test with 300 rows and sorting/filtering logic.

## Performance Review

The primary browser smoke target remains `examples/todo`, with
`examples/dashboard` added as a dashboard-sized pressure test. The Todo debug
build verifies:

- typing rerenders `TodoForm` only;
- `App`, `Header`, `TodoList`, and `TodoItem` render deltas remain zero during
  input typing;
- root, Header, TodoList, and TodoForm child-list mutations remain zero during
  input typing;
- `createElement`, `createTextNode`, structural child operations, and event
  listener add/remove operations remain zero during input typing;
- keyed Todo DOM identity is preserved across reorder/removal;
- event handlers do not multiply after repeated updates;
- duplicate key warnings are emitted in debug builds by the dedicated fixture
  smoke test.

Pure benchmark baseline on Linux amd64, Intel i5-10400F:

| Benchmark | Result | Allocations |
|---|---:|---:|
| Dirty queue pruning | 330.4 ns/op | 32 B/op, 1 alloc |
| Keyed child matching | 11,441 ns/op | 8,744 B/op, 6 allocs |
| Unkeyed child matching | 2,314 ns/op | 8,744 B/op, 6 allocs |
| Props splitting | 2,383 ns/op | 848 B/op, 15 allocs |
| Event name normalization | 285.4 ns/op | 24 B/op, 3 allocs |
| State slot access | 221.5 ns/op | 32 B/op, 4 allocs |
| Key unwrapping | 8.516 ns/op | 0 B/op, 0 allocs |

Performance risks that remain:

- The dirty queue has no priority or cancellation.
- Parent rerender always rerenders component descendants it reaches.
- Paint Flashing is still checked indirectly through DOM operations and
  MutationObserver records, not direct browser paint metrics.

## Security/Safety Review

Runtime:

- Text is rendered through text nodes, not `innerHTML`.
- The runtime does not use user-provided HTML injection.
- Attributes are set through DOM attribute/property APIs.
- Event `js.Func` values are released on unmount/replace.

CLI:

- Compiler commands use `exec.Command` with arguments, not shell strings.
- Manifest child paths are validated as relative child paths.
- `clean` removes `.goframe/work`, `.goframe/build`, and `.goframe/package` by
  default; `--generated` also removes `.goframe/gen` and legacy adjacent
  `.gox.go` files.
- `clean --legacy` removes old `build/` and GoFrame-owned `dist/` output while
  preserving user-owned `dist/` directories.
- Package asset logical names are validated before writing into `assets/` so
  package helpers cannot escape the package asset directory.
- `serve` is localhost-only.

Remaining safety risks:

- The development file server follows the behavior of Go's `http.FileServer`.
  Do not expose it as a production server.
- External scripts or DOM mutation outside the mounted root are outside runtime
  ownership assumptions.

## Fixed Issues

- Fixed uncomparable prop panic in `splitProps`.
- Added strict manifest parsing for unknown fields.
- Added staged package publishing.
- Added source regression gates for heavyweight runtime imports and debug build
  tags.
- Added size budget script for packaged TinyGo artifacts.
- Added dirty queue ancestor pruning.
- Added debug-only duplicate key diagnostics.
- Added pure runtime benchmarks and CI-friendly check scripts.
- Added minimal lifecycle/effect hooks and cleanup semantics.
- Added debug-only lifecycle warnings and an effect update-loop guard.

## Remaining Risks

- Duplicate key diagnostics are debug-only and currently warning-only.
- Component identity does not include Go function identity.
- State and lifecycle slots are positional and do not support conditional hook
  ordering.
- Lifecycle/effects are intentionally minimal: no context, error boundaries,
  async effects, or priorities.
- No props memoization exists; this is a size and correctness tradeoff.
- GOX parser remains a focused parser, not a full Go-aware frontend.

## Regression Gates

Recommended local gate:

```bash
scripts/check.sh

# Optional headless Chrome gate.
scripts/browser-smoke.sh
```

Automated checks currently include:

- Go unit tests.
- GOX golden tests.
- Runtime forbidden-import source tests.
- Runtime debug build-tag source tests.
- CLI manifest and package staging tests.
- Browser Todo smoke script.
- Browser duplicate-key smoke script.
- Browser dashboard smoke and dashboard pressure-test trace.
- Size budget script.
- Pure runtime benchmark script.
- Component identity decision document.

## Recommended Next Steps

Recommended next hardening steps before new product features:

1. Use the dashboard pressure test to guide runtime optimization work with
   before/after render, patch, and DOM operation metrics.
2. Keep memoization, context, router, Player, SSR, hydration, and bundle
   splitting out of scope until the current packaging/runtime gates stay green
   through PR review.
3. Decide whether component identity should include function identity or a
   generated stable type token before adding larger component lifecycle features.
4. Add debug duplicate-key source locations only if diagnostics need to become
   developer-facing rather than smoke-test-facing.
