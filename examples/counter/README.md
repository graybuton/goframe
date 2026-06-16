# Counter example

The counter proves the complete `.gox -> Go -> WASM -> browser` pipeline. Its
`goframe.json` selects TinyGo by default.

For typed props, children, and fragments, see `examples/components`.

## Install the toolchain

From the repository root:

```bash
go install ./cmd/goxc
goxc doctor
```

## Recommended workflow

```bash
goxc generate ./examples/counter
goxc package ./examples/counter
goxc size ./examples/counter
goxc serve ./examples/counter --port=8080
```

Open <http://127.0.0.1:8080>. Use `?sw=1` to enable the optional service-worker
cache. Browser console logs show WASM instantiation. Render-duration probes are
compiled only with the `goframe_debug` build tag.

## Raw compiler output

Build only the raw TinyGo artifact:

```bash
goxc build ./examples/counter --compiler=tinygo
```

Output:

```text
examples/counter/build/main.wasm
```

Use `--compiler=go` for standard Go compatibility mode.

## Package output

`goxc package` creates:

```text
dist/
├── index.html
├── main.wasm
├── manifest.json
├── service-worker.js
└── wasm_exec.js
```

The default package is not compressed. Compression normally belongs to the
deployment server or CDN. Explicit precompression is available for experiments:

```bash
goxc package ./examples/counter --compress=br
```

`goxc serve` always serves the raw WASM and does not perform compressed-content
negotiation.
