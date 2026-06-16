# Components example

This example demonstrates the MVP GOX component model:

- capitalized function components;
- typed `<ComponentName>Props` structs;
- component children through `[]gf.Node`;
- fragments;
- expressions and state updates.

```bash
goxc generate ./examples/components
goxc package ./examples/components
goxc size ./examples/components
goxc serve ./examples/components --port=8080
```

The manifest selects TinyGo by default. Use `--compiler=go` for standard Go
compatibility mode.
