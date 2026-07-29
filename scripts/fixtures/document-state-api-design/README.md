# Document Metadata API Design Fixture

This private fixture compares four application-local ownership surfaces against
one ordered title and description coordinator:

- `#/control` keeps explicit string owner keys and coordinator plumbing.
- `#/hook` models one implicit owner per positional hook slot.
- `#/component` models one owner per mounted non-visual component.
- `#/handle` models explicit owner-handle creation plus lifecycle publication.

The fixture is design evidence only. It does not add a public GoFrame API.
