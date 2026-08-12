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

The harness generates `projections.gox` inside a temporary workspace, then
builds with `goframe_document_state_experiment`. Nothing in this directory is a
public API or compatibility promise, and the experiment tag remains absent from
ordinary builds.

The comparison harness is focused reproducible research evidence and is not
invoked by `scripts/browser-smoke.sh`. Run it directly for standard-Go or
TinyGo evidence; no generic retry wrapper is part of the fixture.

Ten accepted standard-Go runs produced combined behavior SHA-256
`c1651f9bbb1a870432dc369eeef1eef948bff42f8ff672356edbe0dbd1b3a4dc`.
Ten accepted TinyGo successful-path runs produced combined behavior SHA-256
`69e153d6abc68edf47f04a5cee9a2d7e4eec7a95d9a74e76adcea27dbd3a9d05`.
The comparison retains API-shape Result D. Hook and component pass the current
candidate hard gates. The handle shares the accepted transactional-foundation
evidence but is disqualified by released-owner reuse, while the two eligible
shapes remain materially tied. No public shape is selected. The complete
decision and size record is in the
[transactional API-shape reevaluation](../../../docs/document-metadata-api-shape-reevaluation.md).
