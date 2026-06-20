# Components example

This example demonstrates the current GOX component model:

- capitalized function components;
- typed `<ComponentName>Props` structs;
- component children through `[]gf.Node`;
- fragments;
- expressions and state updates.

## Run

```bash
goxc package ./examples/components --compiler=tinygo
goxc size ./examples/components
goxc serve ./examples/components --port=8080
```

The manifest selects TinyGo by default. Use `--compiler=go` for standard Go
compatibility mode. Generated files stay under `.goframe` and should not be
committed next to authored `.gox` files.
