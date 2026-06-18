# Virtualized Collections Example

This example demonstrates the reusable `gf.VirtualList` and `gf.VirtualTable`
primitives. It creates 10,000 deterministic logical items while keeping the
mounted DOM bounded to the visible window plus overscan.

It is intentionally small. The dashboard remains the larger pressure test;
this app is the focused proof that virtualization is framework-level API, not
an example-local workaround.

## What It Tests

- `gf.VirtualList` fixed-height item windows.
- `gf.VirtualTable` fixed-row-height table windows.
- Stable item keys based on logical IDs.
- Scroll-driven range updates.
- Selection and toggle actions after scrolling.
- Bounded DOM and listener counts despite a large logical collection.

## Run

```bash
goxc generate ./examples/virtualized
goxc package ./examples/virtualized --compiler=tinygo
goxc serve ./examples/virtualized --port=8080
```

Then open <http://127.0.0.1:8080>.

For cache-safe packaging:

```bash
goxc package ./examples/virtualized --compiler=tinygo --asset-hash --preload --compress=gzip,br
goxc size ./examples/virtualized
```

## Limitations

- Row and item heights are fixed.
- There is no dynamic measurement engine yet.
- Infinite loading, keyboard navigation, and advanced accessibility behavior are
  future work.
- Hidden rows are not used. Only the visible window plus overscan exists in the
  DOM.
