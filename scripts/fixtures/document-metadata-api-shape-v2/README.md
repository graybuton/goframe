# Transactional Document Metadata API-Shape Fixture

This private fixture compares four projections over the merged document
metadata ownership transaction:

- `control` calls the existing private handoff primitive directly;
- `hook` calls one implicit owner hook per positional slot;
- `component` mounts the fixture-only non-DOM component through generated GOX;
- `handle` creates an explicit owner and forwards publications through a helper.

Every mode runs the same successful ownership scenarios. Standard Go also runs
render, ErrorBoundary, and publisher failure cases. TinyGo runs successful paths
without a recover claim. Candidate-specific scenarios cover two hook slots,
component conditional lifetime and child DOM identity, and handle duplicate,
conflict, update, and release behavior. A focused host characterization also
proves that after the final handle publication releases its underlying owner,
the stable handle cannot publish again: reuse reports `goframe: document
metadata owner is already released` without changing the authored baseline or
coordinator state.

The component failure cases mount the real non-DOM component, stage its
metadata publication, and fail in a descendant inside its subtree. Failed
initial render records a rolled-back publication with no committed token, owner
ID, or document write. Failed replacement retains A and retries with a fresh B
component to publish exact `A -> B` without an authored-baseline interval.

The harness generates `projections.gox` inside a temporary workspace, then
builds with `goframe_document_state_experiment`. Nothing in this directory is a
public API or compatibility promise, and the experiment tag remains absent from
ordinary builds.

The comparison harness is focused reproducible research evidence and is not
invoked by `scripts/browser-smoke.sh`. Run it directly for standard-Go or
TinyGo evidence; no generic retry wrapper is part of the fixture. Its private
CDP client rejects pending and future calls deterministically after WebSocket
close, error, or send failure so disconnects cannot leave the run hanging.

Ten accepted standard-Go runs produced combined behavior SHA-256
`fba6c1d9d3df2c5b6d361be35aaa369f2f01e8ebac49536bcf41a80f7328f6ee`.
Ten accepted TinyGo successful-path runs produced combined behavior SHA-256
`69e153d6abc68edf47f04a5cee9a2d7e4eec7a95d9a74e76adcea27dbd3a9d05`.
The focused artifacts are 2,979,773 raw bytes under standard Go with SHA-256
`33f5e060bdc3088c72557da047272b89c23f56b90862abc5ae384d1a99e19529`
and 283,739 raw bytes under TinyGo with SHA-256
`04f1d83a6f73d532b4d95b4b652ed8df4fe5c69e741630d4aaaa263c4e449451`.
Generated GOX remains SHA-256
`d927def1df878d39030491037f7972dbaff2507ab44e74c9df8e1d92c02def99`.
The comparison retains API-shape Result D. Hook and component pass the current
candidate hard gates. The handle shares the accepted transactional-foundation
evidence but is disqualified by released-owner reuse, while the two eligible
shapes remain materially tied. No public shape is selected. The complete
decision and size record is in the
[transactional API-shape reevaluation](../../../docs/document-metadata-api-shape-reevaluation.md).
