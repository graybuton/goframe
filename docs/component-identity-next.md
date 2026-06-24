# Component Identity Next

## Current Model

GoFrame's legacy path reuses component instances by:

```text
component name + key/position
```

Older handwritten code and older generated output can still use a string name:

```go
gf.Component("Header", HeaderProps{}, Header)
```

MVP 19 adds a typed identity path:

```go
var _goxComponent_app_Header = gf.NewComponentType("main.Header", "Header")

gf.ComponentT(_goxComponent_app_Header, HeaderProps{}, Header)
```

The runtime preserves state, effects, context subscriptions, and memoization
metadata when the component identity and sibling identity match. Keys override
positional matching for reorderable siblings. The debug name remains readable,
but it does not define typed identity.

## Why It Works Today

MVP 19 introduced typed component ids using package name plus component name,
for example `main.Header`. MVP 20 extends the `goxc` path so package import
paths are used when the module path is known, for example
`github.com/graybuton/goframe/examples/multipackage/internal/ui.Header`.

Direct Go function calls remain ordinary calls. `gf.Component` and
`gf.ComponentT` mark runtime component boundaries.

## Where It Becomes Risky

String names become risky when the project grows toward reusable packages or
multi-package applications:

- different packages can define components with the same name;
- generated GOX through `goxc` now encodes import path when known, while
  lower-level `GenerateNamed` still falls back to package name;
- moving a component between packages could accidentally preserve or reset
  state incorrectly depending on the generated name;
- direct function calls and `gf.Component` boundaries remain behaviorally
  different;
- memoization and effects depend on instance reuse being correct;
- keyed lists rely on component identity plus key, not key alone.

These are smaller after MVP 20 and MVP 22, but multi-module workspaces,
reusable component package policy, and the final public identity policy remain
open.

## Required Properties

A future identity model should:

- avoid `reflect` and `unsafe`;
- work with TinyGo;
- keep debug names readable;
- preserve existing `gf.Component` compatibility;
- avoid large metadata tables;
- make package-level collisions unlikely or impossible;
- keep key behavior stable;
- provide a migration story for generated `.gox.go` output.

## Option A: Keep Name + Key

This is the current model.

Pros:

- smallest runtime shape;
- readable debug counters;
- no generated-code migration;
- no new public runtime API.

Cons:

- same-name collisions across packages;
- no implementation identity;
- package moves are ambiguous;
- large apps must rely on naming discipline.

This is acceptable as legacy compatibility, but it should not be the final
answer for package-heavy apps.

## Option B: Package-Aware Identity

GOX could emit identity strings that include package information:

```go
gf.Component("github.com/example/app/internal/ui.Header", HeaderProps{}, Header)
```

Pros:

- simple runtime change;
- readable enough for debug output;
- no token registry.

Cons:

- longer strings can increase generated output and bundles;
- local module paths and vendoring may affect identity;
- needs a policy for package renames and generated test packages.

## Option C: Compiler-Generated Component Tokens

GOX now generates prototype component tokens:

```go
var _goxHeaderType = gf.NewComponentType("main.Header", "Header")

gf.ComponentT(_goxHeaderType, HeaderProps{}, Header)
```

Pros:

- separates debug label from identity token;
- can be package-aware without repeated long strings;
- avoids function identity and reflection;
- gives the compiler a future place for metadata.

Remaining cons:

- `goxc` generated ids use import paths when known, but lower-level generation
  helpers can still emit package-name fallback ids;
- token variable names include source-file context and are not public API;
- multi-module support still needs additional package-token decisions;
- token lifetime and initialization must stay simple for TinyGo.

## Option D: Explicit User Tokens

Users could define component identity tokens manually.

Pros:

- no compiler magic;
- explicit migration for package-level components.

Cons:

- poor ergonomics;
- easy to misuse;
- adds boilerplate to normal GOX authoring;
- risks turning a compiler concern into user ceremony.

## Impact On Runtime

The runtime identity check currently compares names and keys. A token model
would likely add an optional identity field while retaining the debug name.

Memoization, dirty descendant tracking, context subscriptions, effects, and
state slots should remain instance-owned. The only intended change is the
reuse/remount decision for component boundaries.

## Impact On GOX

GOX codegen would need to emit either package-aware names or token variables.
Direct function calls would remain direct calls. Capitalized tags would keep
creating component boundaries.

The compiler should continue to reject unsupported XML-style namespace syntax,
selector chains beyond `packageAlias.Component`, and spread props until those
features are designed separately.

## Impact On TinyGo Size

Any identity change must be measured against:

```bash
scripts/size-budget.sh
scripts/perf-report.sh
```

Avoid:

- reflection;
- unsafe function identity;
- large global registries;
- large debug metadata in production builds.

## Migration Strategy

A conservative path:

1. Keep `gf.Component` string identity.
2. Add an optional token-based API behind generated-code usage.
3. Update GOX codegen to emit tokens in a compatibility mode.
4. Measure size and smoke behavior.
5. Keep old generated code working for at least one transition period.
6. Document package-aware identity before enabling multi-package apps.

## Recommendation

MVP 19 prototypes compiler-generated component tokens while preserving legacy
`gf.Component` compatibility. MVP 20 makes the `goxc` path import-path-aware
when a nearest module path is known. MVP 22 uses the same package-directory
identity for child entry packages such as `./cmd/app`. That is enough for the
current single-module app workspace, but not a final answer for multi-module
monorepos or public component package policy.

The current recommendation is:

```text
entry "." and child-entry apps: use generated ComponentT tokens with import-aware ids when known
handwritten/raw Go: gf.Component remains compatible; ComponentT is available
future workspace forms: require multi-module identity design
```

## Open Questions

- Should package identity use module path, import path, or a compiler-stable
  package token?
- How should identity behave for local `main` packages in examples/tests?
- Should debug labels remain short while identity tokens are fully qualified?
- Can old and new generated code safely mix in one app?
- How much TinyGo growth is acceptable for identity metadata?
