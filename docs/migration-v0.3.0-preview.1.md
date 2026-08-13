# Migrating to v0.3.0-preview.1

## Status / Scope

Most valid applications using the documented `v0.2.0-preview.6` runtime, GOX,
and package contracts require no source migration. `v0.3.0-preview.1` retains
the public `pkg/goframe` export set and the standalone package metadata shape.

Two safety and ownership changes can require action. They are described below
without changing the pre-1.0 experimental compatibility boundary.

## Generated Workspace Compiler Environment

### Affected Surface

`goxc` compiler operations that run inside the generated application workspace,
including development packaging and source-selection discovery.

### Previous Behavior

A parent `go.work`, ambient `GOFLAGS`, or persisted Go environment flags could
influence generated-workspace source selection or dependency resolution.

### Current Behavior

The generated workspace runs with `GOWORK=off`, module mode enabled, and
GoFrame-owned `GOFLAGS`. Parent workspace membership, vendor mode, overlays,
ambient build tags, and other caller flags do not configure those compiler
commands. The remaining process environment, including module proxy, checksum,
private-module authentication, certificate, cache, temporary directory, and Go
toolchain settings, remains available.

### Migration

- Put every application dependency in the app module's authored `require` and
  `replace` directives.
- Rewrite parent-workspace-only local dependencies as explicit app-module
  `replace` entries when they are required by the application.
- Use GoFrame's documented compiler and workspace options instead of ambient
  `GOFLAGS` to select the application compiler or workspace.
- Do not rely on a parent `go.work` as an implicit dependency contract for a
  packaged app.

This isolation does not add a general multi-module workspace contract.

## Repeated Mount Descendant Targets

### Affected Surface

A second `gf.Mount` call that targets a different element contained by the
currently active root.

### Previous Behavior

An application-owned descendant reproduced a detached-target ownership defect.
A host-owned descendant outside the GoFrame-mounted range could remain
connected and host the replacement application.

### Current Behavior

The runtime rejects every different descendant of the current root before
teardown with:

```text
goframe: cannot mount inside current root
```

The whole-root guard intentionally rejects both application-owned and
host-owned descendants. This is a simpler, bounded ownership contract; the
host-owned case is an intentional narrowing rather than the same historical
detached-target defect.

### Migration

- Reuse the current root when replacing the active application in place; or
- mount into a root outside the current root subtree when transferring
  ownership.

GoFrame still owns one active application. This change does not add nested
application adoption, simultaneous multi-root mounting, or a public unmount
handle.

## Newly Rejected GOX Collisions

No migration is expected for valid authored GOX. The compiler now rejects
duplicate or effectively colliding destinations such as `class` plus
`className`, `htmlFor` plus `FOR`, event names differing only by case, duplicate
component fields, or explicit `Children` combined with renderable nested
children. Resolve such a diagnostic by keeping one authored value for each
effective destination.

## Rollback

During preview evaluation, pin the previous published module when a migration
cannot be completed immediately:

```bash
go install github.com/graybuton/goframe/cmd/goxc@v0.2.0-preview.6
```

Keep the CLI and application module on the same intended preview while
comparing behavior. GoFrame remains pre-1.0 and experimental.
