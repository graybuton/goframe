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
conflict, update, and release behavior.

The harness generates `projections.gox` inside a temporary workspace, then
builds with `goframe_document_state_experiment`. Nothing in this directory is a
public API or compatibility promise, and the experiment tag remains absent from
ordinary builds.
