# Resource

This focused example demonstrates the experimental `gf.UseResource` primitive.

It loads small packaged text assets through experimental `gf.FetchText`, while
the runtime resource API itself remains transport-light. The example keeps
issue parsing, `slow:` key handling, delayed completion, and lifecycle cleanup
logic local under `internal/data`.

For an integrated app that combines resources with router/query/forms/Error
Boundary patterns, start with `examples/router-dashboard`.

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
- Loader panic semantics are covered by runtime tests; this TinyGo browser
  smoke focuses on ordinary resource lifecycle behavior.
- There is no cache, Suspense, route loader, retry policy, JSON helper, or
  higher-level data framework in this example.
