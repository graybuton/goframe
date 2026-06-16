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
  package before publishing.

Foundation Hardening II then closed two runtime risks:

- dirty queue ancestor pruning removes redundant child updates when an ancestor
  is dirty in the same flush;
- duplicate sibling keys now produce debug-only browser diagnostics.

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
- There are no unmount hooks, effects, or callback lifecycle APIs.
- Event listener release depends on runtime unmount/replace paths; there is no
  public disposal API for external DOM mutation.

## GOX Compiler Review

GOX remains deliberately small:

- Lowercase tags generate `gf.El`.
- Capitalized tags generate `gf.Component`.
- Props follow `<ComponentName>Props`.
- Children become `Children: []gf.Node{...}`.
- Expressions become `gf.Child(expr)`.
- Fragments generate `gf.Fragment`.

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

- `generate`: `.gox` to `.gox.go`.
- `build`: raw WASM into `build/`.
- `package`: runnable `dist/` bundle.
- `size`: artifact inspection.
- `serve`: localhost static server with `application/wasm`.
- `doctor`: toolchain diagnostics.
- `clean`: build/package artifact cleanup.

Fixed CLI issues:

- Manifest parsing now rejects unknown fields with a useful JSON error.
- `package` builds the bundle in a staging directory first, then publishes
  prepared artifacts into `dist/`.

Remaining CLI risks:

- Package publishing preserves unrelated files in `dist/`; this avoids deleting
  user files but means stale removed assets can remain.
- `serve` is a local development server. It binds to `127.0.0.1` and is not a
  hardened production static server.
- `--out` is intentionally user-directed and may point outside the application
  directory.
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

Run:

```bash
scripts/size-budget.sh
```

Measured during the foundation audit:

| Example | TinyGo WASM |
|---|---:|
| Counter | 76,767 B |
| Components | 82,066 B |
| Todo | 101,752 B |

## Performance Review

The primary browser smoke target is `examples/todo`. The debug build verifies:

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
- `clean` removes only `build/`, manifest output, and generated `.gox.go` when
  explicitly requested.
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

## Remaining Risks

- Duplicate key diagnostics are debug-only and currently warning-only.
- Component identity does not include Go function identity.
- State slots are positional and do not support conditional hook ordering.
- No lifecycle/effects/unmount hooks exist.
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
- Size budget script.
- Pure runtime benchmark script.
- Component identity decision document.

## Recommended Next Steps

Recommended next hardening steps before new product features:

1. Add debug duplicate-key source locations.
2. Add ancestor dirty queue metrics in `goframe_debug`.
3. Add CI wiring for TinyGo size budgets and browser smoke tests.
4. Decide whether component identity should include function identity or a
   generated stable type token.
5. Keep lifecycle/effects/context/router work out of the project until the
   regression gates run reliably in CI.
