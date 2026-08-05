# Transactional Document-State Handoff

## Status

Result B: document metadata ownership requires a bounded update-level handoff.

This record establishes a private lifecycle mechanism, not a public document
metadata API. The browser implementation and fixture bridge are available only
with the `goframe_document_state_experiment` build tag. Ordinary JS/WASM
applications do not compile the mechanism.

The earlier [document metadata API design](document-metadata-api-design.md)
remains Result D. Its hook, non-DOM component, and owner-handle spellings are
not selected by this research.

## Question

The experiment asks whether one successful application update can replace
committed owner A with committed owner B while publishing this sequence:

```text
A -> B
```

and never this sequence:

```text
A -> authored baseline -> B
```

Title and description remain one selected value. The observer-level guarantee
does not claim that two browser DOM assignments form one atomic DOM operation.

## Baseline

The corrected Result D candidates created their private owner in a committed
initialization effect and published from a later committed render. During a
route-lifetime replacement, outgoing `UseUnmount` cleanup released A before B
had an active owner to publish.

A temporary probe of the existing document-state fixture recorded:

```text
selected before:       Editor B
MutationObserver:      Editor B -> authored baseline -> Dialog C
animation frames:      Editor B -> authored baseline -> Dialog C
title mutation delta:  2
description delta:     2
baseline restorations: 1
```

The interval was therefore observable both within mutation delivery and at a
browser frame boundary. It was not merely an internal coordinator state.

## Lifecycle Order

The current render and browser update path has this order:

1. A component render begins a lifecycle attempt. New state slots and reducer
   replacements are speculative.
2. A successful component render finalizes the specialized state participant
   first. Generic lifecycle participants then commit their staged state; the
   document participant stages a publish but does not write the document.
3. Lifecycle hook slots are committed and pending effects are queued. Effect
   bodies have not run.
4. The rendered node is reconciled into the mounted tree.
5. Removed component instances run effect and `UseUnmount` cleanup. A document
   owner cleanup stages its release in the same application update.
6. After all dirty roots have reconciled and focus restoration has completed,
   the document update batch applies its ordered publish and release operations,
   assigns IDs to owners that survived commit, selects the final owner, and
   publishes at most one changed pair.
7. The runtime reports the completed render and flushes pending effects.
8. An effect that marks state dirty schedules a later browser update; it does
   not reopen the completed document batch.

Resource and document work use the existing lifecycle participant boundary.
Their encounter order is not used to choose document ownership: all document
operations remain staged until the update-level finalization after
reconciliation. State retains its existing explicit first-finalization rule.

Protected Error Boundary descendants defer lifecycle completion to the owning
boundary. A nested successful boundary delegates to its outer boundary. A
failed protected attempt rolls back its document participant without staging a
publish, while retained teardown leaves the previous committed owner active.

## Candidate A

Candidate A attached owner publication to the existing component render
transaction. It could create an uncommitted owner token in a speculative state
slot, commit publication after a successful render, and roll publication back
with an Error Boundary attempt.

It could not coalesce both sides of replacement. Incoming publication belongs
to the new component's render attempt, while outgoing release occurs later in
the old component's reconciliation cleanup. Finalizing either component-local
attempt cannot know the complete owner set after both events. Delaying release
to an effect would retain an ownership gap in the opposite direction and add
effect ordering as a contract.

Candidate A was rejected. Component-local participation remains useful for
speculative publication, but it is not the handoff boundary.

## Candidate B

Candidate B begins one document-specific batch at the browser application's
mount or dirty-update entry point. Component lifecycle commits stage incoming
publication. Reconciliation cleanup stages outgoing release. The wrapper then
commits the combined operation list after reconciliation and before effects.

The batch is not a scheduler transaction and does not make arbitrary side
effects transactional. It owns only the document metadata coordinator's
publish and release operations.

Candidate B was accepted because it observes both lifetimes and produces one
final selected snapshot without an initialization render.

## Selected Model

One coordinator owns:

- the authored baseline pair;
- an ordered list of committed owner records;
- the current published pair;
- monotonically increasing committed owner IDs;
- one active application-update batch;
- the publication callback.

An owner token starts pending with ID zero. Rendering may allocate that ordinary
Go object and store it in a speculative state slot. It does not reserve an ID,
join the active order, or publish. A successful render stages the owner pair. A
failed render clears the participant state and leaves ID zero.

Batch commit evaluates operations in their recorded order against a copy of the
committed owner list. New surviving owners are appended and receive IDs only
after validation. Updating an active owner retains its position. Releasing an
owner removes it. The last remaining owner is selected; no remaining owner
selects the authored baseline. The browser callback runs only when that final
pair differs from the current pair.

Batch rollback discards every staged operation. It does not change committed
owners, IDs, priority, or document state.

The implementation rejects foreign owners, publication after release,
duplicate release, release of an inactive owner, operations outside a batch,
nested batches, and coordinator replacement during an active update.

## Behavioral Evidence

The focused Chrome harness observes the authored nodes before WASM boot and
uses `MutationObserver` plus deterministic animation-frame sampling.

| Scenario | Result |
| --- | --- |
| initial owner publishes without an initialization render | pass |
| direct A-to-B replacement records exactly `A -> B` | pass |
| failed initial render commits no ID, owner, or publication | pass |
| failed replacement retains A until retry commits B | pass |
| Error Boundary retry establishes one new lifetime | pass |
| nested owner overrides its parent | pass |
| parent update beneath the override causes no document write | pass |
| nested release reveals the latest parent value | pass |
| non-selected release causes no document write | pass |
| identical selected value causes no document write | pass |
| release, update, and activation in one update are deterministic | pass |
| final release restores the authored baseline once | pass |
| unmount and remount allocate distinct committed IDs | pass |
| application teardown restores the baseline once | pass |
| repeated `Mount` hands ownership directly to the replacement | pass |
| title, description, and unrelated head-node identities remain stable | pass |
| no mixed title/description pair is observed | pass |

Ten fresh standard-Go runs produced the same normalized evidence hash:

```text
07a6a05bb5d3bf2dd684d31e226c2ad974670ce481a8758c596dbef1c16f0f4a
```

Their focused artifact was 2,544,136 raw bytes with SHA-256:

```text
e5d766c2dd80c22d9eacbbb72bfb537e6ace64dd48899aaf07966ddec9d077f1
```

Ten fresh TinyGo successful-path runs produced the same normalized evidence
hash:

```text
1dd34c9bab3c8a4beac17b331aa42806875dc4748e8df35f214eb3cb9cb7f2ea
```

Their focused artifact was 182,330 raw bytes with SHA-256:

```text
49bf7a637b2a863de132b87ef15882a3aaccea44ebad3444dedcc822454ba092
```

The decisive direct-replacement scenario used two owner tokens, assigned two
committed IDs, released A once, performed two document publications total
(initial A and replacement B), restored the baseline zero times, and required
one render for each owner lifetime.

Standard Go covers recover-based failed initial render, failed replacement,
and retry. TinyGo `0.41.1` uses trap-style panic behavior, so its browser lane
covers the complete successful activation, update, priority, replacement,
release, remount, repeated-mount, and teardown path without claiming
recover-based rollback.

## Size And Reachability

The experiment bridge and telemetry make the selected fixture an upper bound,
not a proposed public API size. The candidate comparison below uses matched
physical output paths and deterministic gzip headers; its standard-Go selected
artifact therefore differs from the separately built browser artifact above.

| Compiler | Candidate | Raw | gzip | Brotli | Zstandard |
| --- | --- | ---: | ---: | ---: | ---: |
| standard Go | control mount path | 2,469,698 | 702,082 | 533,193 | 566,651 |
| standard Go | selected experiment | 2,544,119 | 718,492 | 543,407 | 580,970 |
| TinyGo | control mount path | 157,353 | 66,311 | 55,601 | 59,164 |
| TinyGo | selected experiment | 182,330 | 75,653 | 63,227 | 67,077 |

The selected-minus-control deltas are:

| Compiler | Raw | gzip | Brotli | Zstandard |
| --- | ---: | ---: | ---: | ---: |
| standard Go | +74,421 | +16,410 | +10,214 | +14,319 |
| TinyGo | +24,977 | +9,342 | +7,626 | +7,913 |

These deltas include the build-tagged exported bridge, browser event stream,
statistics, diagnostics, and all experiment scenarios. They are not attributed
solely to the coordinator.

In the matched candidate comparison, standard Go added 44,857 code-section
bytes, 27,154 data-section bytes, and 2,317 name-section bytes. TinyGo added
18,165 code-section bytes, 5,708 data-section bytes, and 1,064 name-section
bytes. The remaining raw delta is section framing and other fixture metadata.

A nullable ordinary-build bridge retained 323-433 raw bytes in every measured
TinyGo application. A no-op wrapper still retained 39 bytes. Keeping the
participant in ordinary builds while tagging only its mount integration
retained 41 bytes in the virtualized fixture through the generic lifecycle
interface. The final source selection therefore excludes the complete
coordinator, participant, bridge, and update wrapper from ordinary JS/WASM
builds.

All eleven budgeted applications were rebuilt at the same physical output path
from the base and selected code. Raw bytes, compressed bytes, and SHA-256 were
identical for every application. All existing absolute and ratio budgets pass;
no budget changed.

## Limitations

- The accepted mechanism is experiment-tagged. This stage establishes the
  lifecycle boundary but does not ship a document metadata feature.
- The fixture bridge is exported only so a separate `main` package can exercise
  private runtime behavior in a real browser. Those names are absent from
  ordinary builds and are not compatibility candidates.
- The update batch is tied to the current single-active-application browser
  mount path. It is not a generic transaction facility.
- Pair consistency is established at committed framework and recorded observer
  boundaries, not as one browser DOM instruction.
- External scripts that concurrently mutate the authored nodes are not covered.
- Arbitrary head elements, SSR, hydration, SEO behavior, multiple documents,
  portals, and concurrent applications are outside this result.

## Decision Boundary

Result B supersedes only the lifecycle blocker identified by Result D. A later
API-shape comparison may reevaluate the implicit hook, non-DOM component, and
explicit owner-handle shapes against this foundation. It must still measure
call-site semantics, misuse resistance, generated GOX ergonomics, ordinary
build reachability, and browser behavior before selecting any public API.

This research is independent of Issue #117. It uses neither the evaluator
workspace nor unpublished evaluator submissions and makes no conclusion about
that task.
