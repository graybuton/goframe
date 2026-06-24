# Resource

This example demonstrates the experimental `gf.UseResource` primitive.

It loads small packaged text assets through browser `fetch`, but the runtime
resource API itself is transport-agnostic. The fetch, parsing, delay, and abort
logic live in the example under `internal/data`.

## Run

```bash
goxc package ./examples/resource --compiler=tinygo
goxc serve ./examples/resource --port=8080
```

## Notes

- The first render shows loading before the loader starts.
- `Load Missing` produces explicit failed state, not an Error Boundary.
- `Load Slow Open` followed by `Load Fast All` demonstrates stale completion
  protection.
- `Toggle panel` unmounts the resource panel and runs loader cleanup.
- There is no cache, Suspense, route loader, retry policy, JSON helper, or
  runtime fetch API in this MVP.
