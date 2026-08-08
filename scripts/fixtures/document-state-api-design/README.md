# Document Metadata API Design Fixture

This private fixture compares four application-local ownership surfaces against
one ordered title and description coordinator:

- `#/control` keeps explicit string owner keys and coordinator plumbing.
- `#/hook` models one implicit owner per positional hook slot.
- `#/component` models one owner per mounted non-visual component.
- `#/handle` models explicit owner-handle creation plus lifecycle publication.

The fixture is design evidence only. It does not add a public GoFrame API.

The `cmd/handoff` entry point and
`scripts/document-state-transactional-handoff-browser-smoke.mjs` exercise a
separate transactional-lifecycle experiment. They require the private
`goframe_document_state_experiment` build tag and compare standard Go and
TinyGo browser behavior. The build-tagged bridge is absent from ordinary
applications and is not a fifth API candidate. The experiment uses the same
mount, scheduling, dirty-update, focus, reconciliation, and effect-loop core as
ordinary browser builds. Its build-tagged wrapper adds only the private
document batch around that core. Standard Go exercises 22 scenarios, including
direct keyed replacement failure, an unrelated update during unresolved
handoff, reversed multi-owner retry order, partial readiness, additive and
initial pending-owner abandonment and retry, cross-boundary finalization, newer
boundary failure, and final release. Snapshots expose committed owner order and
IDs, pending owner plans, finalization counts, batch state, exact observer and
animation-frame sequences, and authored head-node identity. TinyGo exercises
eight successful-path scenarios without a recover claim.
