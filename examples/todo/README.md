# Todo example

The Todo example is the first small application-level integration probe. It
demonstrates:

- controlled input state through `OnInput` and `gf.InputEvent`;
- form submission through `OnSubmit` and `gf.Event.PreventDefault`;
- add, toggle, remove, and keyed reorder actions;
- GOX render expressions: `{condition && <Node />}` and
  `{condition ? <A /> : <B />}`;
- list rendering through `gf.Map` with GOX markup inside callback returns;
- component composition and children;
- keyed component and DOM identity through `Key={...}`;
- todo state scoped to `TodoApp` and input state scoped to `TodoForm`;
- dirty component updates followed by a mounted-tree DOM patch;
- `gf.UseEffect` for localStorage persistence.

Install `goxc`, package with TinyGo, and serve the result:

```bash
go install ./cmd/goxc
goxc generate ./examples/todo
goxc package ./examples/todo --compiler=tinygo
goxc size ./examples/todo
goxc serve ./examples/todo --port=8080
```

Open <http://127.0.0.1:8080>. Add `?sw=1` to opt into the service-worker cache.

Use standard Go compatibility mode when TinyGo is unavailable:

```bash
goxc package ./examples/todo --compiler=go
```

The runtime preserves the root, input, list, and keyed item DOM nodes while
patching text, props, events, and child order. Typing marks only `TodoForm`
dirty. Todo changes mark `TodoApp` dirty. `App` and `Header` remain outside
both update paths.

Todo persistence intentionally lives in the example, not in the core runtime.
It uses a compact string encoding instead of `encoding/json` so the runtime and
TinyGo bundle stay small. `UseEffect(fn)` loads localStorage after the first
DOM mount, and `UseEffect(fn, gf.Deps(encodedTodos))` writes only when the
encoded todo list changes.

Build an instrumented regression bundle, serve it on port `18080`, and run the
dependency-free browser identity probe:

```bash
tinygo build -target=wasm -no-debug -panic=trap -tags=goframe_debug \
  -o ./examples/todo/dist/main.wasm ./examples/todo
goxc serve ./examples/todo --port=18080
node --experimental-websocket scripts/todo-browser-smoke.mjs
```

The smoke test verifies node identity, component render/patch counts,
MutationObserver records, structural DOM operations, and event listener churn.
During typing it expects zero root/Header/TodoList/TodoForm child-list
mutations, zero structural DOM operations, and zero listener add/remove calls.
It also verifies that todos are persisted to localStorage and restored after a
page reload.
