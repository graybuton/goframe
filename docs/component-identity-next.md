# Component Identity Next

## Current Model

GoFrame currently reuses component instances by:

```text
component name + key/position
```

GOX capitalized tags generate a string name:

```go
gf.Component("Header", HeaderProps{}, Header)
```

The runtime preserves state, effects, context subscriptions, and memoization
metadata when the component name and sibling identity match. Keys override
positional matching for reorderable siblings.

## Why It Works Today

The current examples keep all application components in one Go package and use
unique component names. That makes string identity small, readable in debug
output, and TinyGo-friendly.

It also keeps generated code simple. Direct Go function calls remain ordinary
calls, while `gf.Component` marks the runtime boundary.

## Where It Becomes Risky

String names become risky when the project grows toward reusable packages or
multi-package applications:

- different packages can define components with the same name;
- generated GOX code currently does not encode package identity;
- moving a component between packages could accidentally preserve or reset
  state incorrectly depending on the generated name;
- direct function calls and `gf.Component` boundaries remain behaviorally
  different;
- memoization and effects depend on instance reuse being correct;
- keyed lists rely on component identity plus key, not key alone.

These are not current blockers, but they should be resolved before claiming
multi-package app support.

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

This is acceptable for the current single-package examples, but it should not
be the final answer for multi-package apps.

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

GOX could generate stable package-aware tokens:

```go
var _goxHeaderType = gf.ComponentType("main.Header")

gf.ComponentT(_goxHeaderType, HeaderProps{}, Header)
```

Pros:

- separates debug label from identity token;
- can be package-aware without repeated long strings;
- avoids function identity and reflection;
- gives the compiler a future place for metadata.

Cons:

- requires a new runtime entry point;
- changes generated code;
- needs migration and size measurements;
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

The compiler should continue to reject unsupported namespaces/spread props
until those features are designed separately.

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

Do not implement generated identity in MVP 18. Before multi-package app
support, prototype compiler-generated or package-aware identity and compare
runtime size, generated code size, and migration cost.

The current recommendation is:

```text
single-package apps: keep name + key
multi-package apps: require package-aware or compiler-generated identity first
```

## Open Questions

- Should package identity use module path, import path, or a compiler-stable
  package token?
- How should identity behave for local `main` packages in examples/tests?
- Should debug labels remain short while identity tokens are fully qualified?
- Can old and new generated code safely mix in one app?
- How much TinyGo growth is acceptable for identity metadata?
