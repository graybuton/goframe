# Context Selectors Example

This example demonstrates the scoped context API added for MVP 16.

It is intentionally small. The dashboard remains the larger reducer and
memoization pressure test; this app focuses on context semantics:

- `gf.CreateContext` for a typed default value;
- `gf.ProvideContext` as a render-time provider hook;
- `gf.UseContextSelector` for comparable selected values;
- `gf.UseContext` as a broad consumer;
- nested providers;
- memoized leaf components so clean selector consumers can bail out.

## Run

```bash
goxc package ./examples/context --compiler=tinygo
goxc serve ./examples/context --port=8080
```

For a cache-safe package:

```bash
goxc package ./examples/context --compiler=tinygo --asset-hash --preload --compress=gzip,br
```

## What To Check

Changing density should rerender the density selector consumer, not the accent
or counter consumers. Changing accent should rerender the accent selector
consumer. Incrementing the counter should rerender the counter selector
consumer. The broad `UseContext` consumer rerenders on provider updates by
design.

This is not a global store. Context is scoped by the component tree and nearest
provider wins.
