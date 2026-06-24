# Public Surface Audit V

## Summary

Foundation Audit V polished the public repository surface after MVP 22. The
pass focused on README, documentation maps, example READMEs, command wording,
local link/example checks, CI documentation, and the changelog.

No runtime, GOX language, or `goxc` product feature was added.

## Scope

Reviewed:

- `README.md`;
- `CHANGELOG.md`;
- docs under `docs/`;
- example READMEs under `examples/`;
- public commands in docs and examples;
- local Markdown links;
- example inventory coverage;
- CI and local check descriptions.

Out of scope:

- router, SSR, hydration, Player/Engine implementation, `.gfapp`;
- dynamic virtualization or infinite loading;
- LSP, formatter, XML-style namespace tags, spread props;
- production deployment server;
- runtime/compiler/toolchain behavior changes.

## README Review

The README was reshaped into a newcomer-facing front door:

- status and non-goals are explicit;
- examples are listed in one table;
- Go-first `cmd/app + internal/...` layout is documented;
- GOX, runtime primitives, and toolchain workflow are summarized;
- documentation links are grouped by topic;
- limitations are stated without promising production readiness.

## Examples Review

All example directories with `goframe.json` have a README:

- `examples/counter`
- `examples/components`
- `examples/todo`
- `examples/dashboard`
- `examples/context`
- `examples/virtualized`
- `examples/multipackage`
- `examples/cmdapp`

Updated example documentation:

- corrected the Todo debug build workspace path;
- replaced stale dashboard byte-budget prose with a pointer to the
  authoritative `scripts/size-budget.sh`;
- aligned Counter compression and `serve` wording;
- clarified that Components uses the current GOX component model.

## Documentation Review

Updated documentation to reflect the current project surface:

- current MVP 22 size and DOM pressure baseline in
  `docs/performance-baseline.md`;
- current TinyGo example sizes and roadmap framing in `docs/architecture.md`;
- typed component identity wording in `docs/component-identity.md`;
- docs consistency checks in `docs/ci.md`.

## Command / Link Review

Added `scripts/docs-check.mjs`.

The check verifies:

- local Markdown links in README, docs, and example READMEs;
- every `examples/<name>` directory with `goframe.json` has a README;
- every such example is mentioned from the root README.

The check is wired into `scripts/check.sh` and the core GitHub Actions
workflow.

## Updated Files

Primary public-surface files updated:

- `README.md`
- `CHANGELOG.md`
- `docs/architecture.md`
- `docs/ci.md`
- `docs/component-identity.md`
- `docs/performance-baseline.md`
- `examples/components/README.md`
- `examples/counter/README.md`
- `examples/dashboard/README.md`
- `examples/todo/README.md`
- `scripts/docs-check.mjs`
- `.github/workflows/ci-core.yml`

## Remaining Risks

- Link checking is intentionally local-only. External URLs are not fetched.
- Docs still describe an experimental project; the API is not stable 1.0.
- The README remains a curated map rather than exhaustive API reference.
- `docs/foundation-audit-iv.md` remains historical by design and should not be
  read as the latest complete audit after MVP 22.

## Recommended Next Steps

- Keep docs checks lightweight and local-only unless a dedicated docs pipeline
  becomes necessary.
- Before public preview, run a separate API terminology review and decide which
  public-candidate APIs are ready for stronger compatibility promises.
- Keep updating `docs/performance-baseline.md` whenever size budgets, smoke
  invariants, or example scope materially change.
