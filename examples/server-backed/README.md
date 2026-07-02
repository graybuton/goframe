# Server-Backed Reference

This example shows a narrow integration pattern:

- a GoFrame browser/WASM app packaged by `goxc`;
- a plain Go `net/http` backend;
- static serving of the packaged standalone app;
- a same-origin `/api/greeting` endpoint;
- browser-side data loading through an example-local fetch bridge and
  `gf.UseResource`;
- a controlled backend failure and recovery path through the same resource/form
  state;
- delayed stale backend response handling without overwriting newer rendered
  data.

It is a reference fixture, not a GoFrame server framework.

## Run

Package the browser app:

```bash
goxc package ./examples/server-backed --compiler=go
```

Run the backend against the packaged output:

```bash
go run ./examples/server-backed/cmd/server \
  --package=./examples/server-backed/.goframe/package/standalone \
  --addr=127.0.0.1:8080
```

Open <http://127.0.0.1:8080>.

## What It Demonstrates

- `goxc package` can produce a browser/WASM bundle that a Go backend serves as
  static files.
- The backend can expose a same-origin API endpoint beside the packaged app.
- The app can use existing GoFrame resource/form patterns to load backend data
  and update rendered UI after form submission.
- The app renders the existing `gf.UseResource` failed state for a controlled
  backend error and recovers after a later valid submission.
- A delayed backend response can be superseded by a newer valid submission
  without replacing the newer rendered greeting.
- The browser fetch bridge lives in this example; it is not a runtime API.

## Project Structure

```text
examples/server-backed/
├── goframe.json
├── assets/
│   ├── index.html
│   └── styles.css
└── cmd/
    ├── app/     # browser/WASM GoFrame app
    └── server/  # plain Go net/http backend
```

## Tests

Focused checks:

```bash
goxc package ./examples/server-backed --compiler=go
go test ./examples/server-backed/...
node --experimental-websocket scripts/server-backed-browser-smoke.mjs
```

The browser smoke packages the example, starts the Go backend on a dynamic
localhost port, opens the app through Chrome/CDP, and verifies initial backend
data, updated backend data, delayed stale response no-overwrite behavior,
controlled backend failure UI, and recovery after a later valid form submission.

## Non-goals

This example intentionally does not provide:

- a GoFrame server framework;
- production server behavior;
- fullstack/server APIs;
- server functions;
- SSR or hydration;
- route loaders;
- auth/session helpers;
- a global resource cache.
