# Transactional Document-State Handoff

## Status

Result B confirmed: a bounded document-specific application-update handoff is
technically viable.

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

The first failed-replacement fixture retained A as a mounted parent and placed
failing B beneath it, so it proved failed nested-override isolation rather than
replacement of A's lifetime. A keyed replacement probe against the reviewed
head replaced owner A with a distinct failing owner B under one Error Boundary.
B retained ID zero, but outgoing cleanup released A, restored the authored
baseline, and left the coordinator with no active owner. The observed sequence
was `A -> authored baseline`; retry could not be classified as a direct handoff.

The first bounded retention correction derived the failed replacement from a
document participant rollback. That was too narrow. A panic before B observed
document metadata created no participant and dropped A. A sibling panic could
retain A only when B had already created a participant. Ownerless recovery left
A retained, a nested successful boundary associated an owner with the inner
boundary rather than the final outer transaction owner, and publication failure
lost the teardown release before the next retry. These cases established that
protected-boundary outcomes and durable detach intent, rather than document-hook
encounter order, are the required inputs to the coordinator.

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
   the document update batch validates and computes its complete next owner
   state without changing committed coordinator state. It then publishes at
   most one changed title/description pair.
7. Successful publication commits the owner list, IDs, selected pair, and
   statistics, closes the batch, and only then delivers buffered observer
   events. Failed publication attempts to restore the previous pair and
   discards the batch without advancing committed coordinator state.
8. The runtime reports the completed render and flushes pending effects.
9. An effect that marks state dirty schedules a later browser update; it does
   not reopen the completed document batch.

Resource and document work use the existing lifecycle participant boundary.
Their encounter order is not used to choose document ownership: all document
operations remain staged until the update-level finalization after
reconciliation. State retains its existing explicit first-finalization rule.

Protected Error Boundary descendants defer lifecycle completion to the owning
boundary. Every protected attempt now records one explicit outcome:
`committed`, `failed`, or `delegated`. A nested successful boundary delegates
to its outer boundary, so the final outer boundary owns the transaction outcome
even when the metadata owner was rendered beneath an inner boundary. Outcome
reporting occurs independently of whether a document participant was observed.

During a direct keyed A-to-B replacement, outgoing component teardown still
runs. If publication fails, the document coordinator retains A's committed
metadata owner and records B plus A's completed release in one unresolved
ownership plan. A successful B retry resolves that plan and commits B. A
successful ownerless recovery releases A and reveals the next parent owner or
authored baseline. Unmounting the failed boundary abandons the retained owner.
This retention is specific to document ownership and does not preserve A's
component lifetime.

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

The final private coordinator owns:

- the authored baseline pair;
- an ordered list of committed owner records;
- the current published pair and monotonically increasing committed owner IDs;
- one active document-specific application-update batch;
- failed-boundary state and retained committed-owner releases;
- bounded unresolved ownership plans;
- a failure-reporting pair publisher and an optional observer.

Each unresolved ownership plan retains an ordered set of pending owners. Every
pending entry keeps its latest metadata, resolved boundary scope, readiness,
and abandonment state. A plan may additionally own outgoing committed releases
and boundary recovery or abandonment authority. Pending-owner-only plans are
valid: neither an outgoing release nor a finalization is required when failed
publication leaves a mounted owner pending with ID zero.

Publication failure does not change committed owners, IDs, selected metadata,
or committed statistics. It closes the active batch and creates or extends the
causal unresolved plan. An unrelated successful update does not consume that
plan. A pending owner advances only when that owner publishes again or is
abandoned by its real component lifetime. Readiness survives across updates,
latest metadata replaces older pending metadata, and retry encounter order does
not change the retained owner order.

Outgoing releases and boundary finalization are consumed only by the plan that
causally owns the retained committed owner. This authority can cross boundary
scopes when an external or nested successor consumes that owner. A newer
failure from the same boundary supersedes stale recovery authority. Final
outer-boundary delegation, rather than intermediate boundary equality,
determines the effective protected outcome.

When every remaining pending owner is ready, or all pending owners are
abandoned, the coordinator evaluates the plan against copies of committed
state. It publishes at most one final title/description pair. Only successful
publication commits owner order, metadata, IDs, releases, finalization state,
statistics, and observer events. Resolved plans and their indexes are removed
rather than retained as history.

The publisher validates the complete operation plan before document side
effects. Its browser adapter restores the previous pair when the second write
fails and preserves publication plus restoration errors. Observer delivery
occurs after publication, internal commit, and batch closure. An observer
failure remains visible but does not roll back committed state or reactivate
the batch.

This model is document-specific. It does not make rendering, scheduling, or
arbitrary lifecycle side effects transactional.

## Historical Intermediate Model

One coordinator owns:

- the authored baseline pair;
- an ordered list of committed owner records;
- the current published pair;
- monotonically increasing committed owner IDs;
- one active application-update batch;
- final protected-boundary outcomes and retained owners;
- durable, deduplicated detach intents for completed owner teardown;
- a failure-reporting publication callback;
- an optional failure-reporting observer callback.

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

Component teardown is not replayable, so a staged release caused by real
teardown is retained as a one-shot detach intent until a later successful batch
commits it. A failed publication keeps both the old committed owner and that
intent. The next clean batch injects the intent before evaluating new
operations. The intent set deduplicates repeated observation and is cleared
only after a successful batch incorporates it. Protected-boundary abandonment
is represented by such a batch rather than by a separate out-of-band cleanup.

Batch commit first validates and evaluates the complete operation plan against
copies of the owner and boundary state. The publisher receives the previous and
next complete pair. Its browser adapter writes title and then description; if
description fails after title changes, it attempts to restore both fields and
preserves the publication and restoration errors. A publisher failure assigns
no ID, commits no owner or statistic, closes the batch, and permits a later
clean retry.

Observer events are buffered while the batch is active. They are delivered only
after successful publication, internal commit, and batch closure. An observer
failure remains visible as a runtime panic, but it does not roll back the
already consistent document and coordinator or reactivate the batch.

The implementation rejects foreign owners, publication after release,
duplicate release, release of an inactive owner, operations outside a batch,
nested batches, and coordinator replacement during an active update.

## Final Closeout Evidence

Pure Go tests exercise 34 deterministic unresolved-ownership-plan matrix
cells. The matrix passed 100 repetitions, 20 race-detector repetitions, and 20
`goframe_debug` repetitions. It covers direct replacement, unrelated updates
during a failed handoff, pending-owner abandonment, repeated publication
failure, latest pending metadata, retained owner order, partial readiness, all
six three-owner retry permutations, same-boundary and cross-boundary
finalization, newer-failure supersession, pending-owner-only plans, teardown,
and observer-after-commit ordering.

The standard-Go/WASM browser lane ran 22 scenarios in ten fresh executions.
All ten passed with normalized behavior SHA-256:

```text
e53dbd3fde3533e50d8087770d0de9207a7e6b8b3c7aec5a95a6916a500e52e5
```

The browser artifact was 2,863,166 raw bytes with SHA-256:

```text
364fca2b9b24f27fe26dc2c0b0242577384efa29fb9faf5b21c1b4e61a80fa10
```

The standard-Go scenarios establish exact direct `A -> B` handoff, no
`A -> authored baseline -> B` interval, zero B rerenders and zero document
publication during an unrelated update, deterministic retained priority under
reversed retry order, readiness across separate updates, pending-owner
abandonment and retry, external and cross-boundary finalization absorption,
and newer boundary failure superseding stale recovery authority. Exact
`MutationObserver` and animation-frame sequences contained no mixed
title/description pair. Authored title and description node identities remained
stable, completed batches were inactive, and completed scenarios retained no
pending plan or finalization.

The TinyGo `0.41.1` browser lane ran eight successful-path scenarios in ten
fresh executions. All ten passed with normalized behavior SHA-256:

```text
f18688e83005a227759fe6a46077d9ee58cb422865ba05987807a4f1462febb0
```

The TinyGo artifact was 256,348 raw bytes with SHA-256:

```text
becb96acd49ab7a2c1208954d816f79a8693a4032cd67e6a40d774a74ef4eb08
```

TinyGo proves successful ownership paths only. It does not prove recover-based
publication-failure or Error Boundary rollback.

## Historical Intermediate Behavioral Evidence

The following scenarios and measurements describe earlier coordinator
revisions. They remain useful implementation history but are not the final
closeout scenario count, hashes, or artifact sizes.

The focused Chrome harness observes the authored nodes before WASM boot and
uses `MutationObserver` plus deterministic animation-frame sampling.

| Scenario | Result |
| --- | --- |
| initial owner publishes without an initialization render | pass |
| direct A-to-B replacement records exactly `A -> B` | pass |
| failed initial render commits no ID, owner, or publication | pass |
| direct keyed A-to-failing-B replacement retains committed A | pass |
| retry records exactly `A -> B`, assigns B once, and releases A once | pass |
| panic before B observes metadata retains A and retries to B | pass |
| B metadata followed by a failing sibling retains A and retries to B | pass |
| ownerless recovery consumes the retained release | pass |
| ownerless recovery reveals a surviving parent owner | pass |
| nested metadata ownership follows the final outer boundary | pass |
| repeated failure and boundary abandonment clear retained state | pass |
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
| title or description publication failure leaves the previous pair and coordinator intact | pass |
| publication failure retains completed A teardown for a later B-only retry | pass |
| B unmount after publication retry restores the authored baseline | pass |
| restoration failures preserve the original and restoration diagnostics | pass |
| observer failure occurs after commit with the batch inactive | pass |

Ten fresh standard-Go runs produced the same normalized evidence hash:

```text
4ba1ba77f0ca324d6c18095d7c564cc2976fac594c9599845956ed4748ba8486
```

Their focused artifact was 2,682,484 raw bytes with SHA-256:

```text
07513666fbf33a40fb220f4888674b05e9fdac764314e78d02ae8cf1e922d52d
```

Ten fresh TinyGo successful-path runs produced the same normalized evidence
hash:

```text
f139ce79f23b5dc0feb54df7e81e8cec78fff1348109455afee3fb698de6d416
```

Their focused artifact was 209,711 raw bytes with SHA-256:

```text
1f3e66103f5166b2194f27351e16891d5499261008bd9fb119bc39cdb580ba06
```

The decisive direct-replacement scenario used two owner tokens, assigned two
committed IDs, released A once, performed two document publications total
(initial A and replacement B), restored the baseline zero times, and required
one render for each owner lifetime.

The direct failed-replacement scenario kept A selected with one active owner,
left failing B at ID zero, published no baseline or mixed pair, and reported one
render failure. Retry rendered a fresh B lifetime, assigned committed ID 2,
released A exactly once, and produced `A -> B` in both observer and
animation-frame sequences. The final coordinator had one owner and no active
batch, failed-boundary marker, or retained release.

The boundary-outcome matrix also failed before B observed metadata, failed in a
sibling after B observed metadata, recovered without a replacement owner,
revealed a surviving parent owner, delegated nested success to the final outer
boundary, and abandoned repeated failures. Every failed attempt retained A
without publishing the authored baseline. Retrying with B produced `A -> B`;
ownerless recovery or abandonment released A exactly once and left no active
batch, failed boundary, or retained detach intent.

The publication-failure browser case performed A's real cleanup once, rejected
the A-to-B publication, kept A selected, left B at ID zero, and retained one
detach intent. A later ordinary B rerender, without another A cleanup, assigned
B committed ID 2 and released A. B unmount then restored the authored baseline.
The tagged mount wrapper reports the typed publication failure through the
existing runtime error channel after the document batch has closed; unrelated
runtime invariant panics are not converted into reports.

Standard Go covers recover-based failed initial render, failed replacement,
and retry. TinyGo `0.41.1` uses trap-style panic behavior, so its browser lane
covers successful activation, update, priority, replacement,
release, remount, repeated-mount, and teardown path without claiming
recover-based rollback.

## Final Size And Reachability

The evidence closeout compared
`b465fc7e27dc14041608ca0624a9831d911f6b6e` with
`912e51ac672751a3e7e560b9513bea7b6cc00589` at matched physical output paths.
All eleven ordinary applications were byte-identical across raw, gzip,
Brotli, and Zstandard streams. Both 44-stream manifests have SHA-256:

```text
e58e11d27bc46534243c36be823de21c9c60c9344dff0268847663568e3b340e
```

Across the full research branch, all 44 current absolute ceilings and every
compression-ratio limit passed. No budget or ratio changed.

Ordinary standard-Go and TinyGo source selection excludes the coordinator,
experiment mount wrapper, and fixture bridge. Source-selection and artifact
inspection found none of these experiment-only identifiers or classes:

```text
documentMetadataCoordinator
DocumentMetadataHandoffExperiment
goframe_document_state_experiment
unresolved ownership-plan symbols
fixture controls
experiment telemetry
```

The focused fixture intentionally grew because the evidence closeout added
seven browser scenarios and private telemetry. Standard Go grew by 118,323 raw
bytes: 81,456 code-section bytes, 33,731 data-section bytes, and 2,977
name-section bytes. TinyGo grew by 25,066 raw bytes: 20,673 code-section bytes,
2,558 data-section bytes, and 1,684 name-section bytes. These focused-fixture
deltas do not appear in ordinary application artifacts.

## Historical Intermediate Size And Reachability

The measurements below describe candidate selection and intermediate review
closeouts. They are retained as historical evidence and are not the final
closeout artifact sizes or hashes above.

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

The first review closeout compared `702132c37a1b` with `a0a36a82a13c` at one
matched physical path. Those deltas include the direct failed-owner fixture,
the first bounded retention model, the shared mount core, pair restoration,
observer buffering, fault diagnostics, and their experiment bridge evidence:

| Compiler | Raw | gzip | Brotli | Zstandard |
| --- | ---: | ---: | ---: | ---: |
| standard Go | 2,544,337 -> 2,601,686 (+57,349) | +14,465 | +9,127 | +10,674 |
| TinyGo | 182,330 -> 194,890 (+12,560) | +4,444 | +3,799 | +4,043 |

Standard Go added 35,599 code-section bytes, 19,859 data-section bytes, and
1,810 name-section bytes in that closeout. TinyGo added 10,781 code-section
bytes, 1,152 data-section bytes, and 591 name-section bytes.

The boundary-outcome closeout compared `a0a36a82a13c` with `e73351815664` at
the same physical path. It adds the participant-independent boundary outcomes,
outer-boundary ownership, ownerless recovery, durable detach intents, and the
new browser scenarios:

| Compiler | Raw | gzip | Brotli | Zstandard |
| --- | ---: | ---: | ---: | ---: |
| standard Go | 2,601,644 -> 2,679,032 (+77,388) | +14,009 | +10,033 | +10,998 |
| TinyGo | 194,890 -> 209,711 (+14,821) | +4,386 | +3,010 | +3,279 |

Standard Go added 51,756 code-section bytes, 23,450 data-section bytes, and
2,065 name-section bytes. TinyGo added 12,157 code-section bytes, 1,556
data-section bytes, and 993 name-section bytes. The intermediate raw SHA-256
values were `ccfabc4ffd964c97797c49fb834cbae8aa7e7185438efb1628787696e999eb1c`
for standard Go and `1f3e66103f5166b2194f27351e16891d5499261008bd9fb119bc39cdb580ba06`
for TinyGo.

A nullable ordinary-build bridge retained 323-433 raw bytes in every measured
TinyGo application. A no-op wrapper still retained 39 bytes. Keeping the
participant in ordinary builds while tagging only its mount integration
retained 41 bytes in the virtualized fixture through the generic lifecycle
interface. That intermediate source-selection correction therefore excludes
the coordinator, participant, bridge, and experiment update wrapper from
ordinary JS/WASM builds. Ordinary and experiment builds now share
`mount_common_js.go`,
which contains mounted application state, scheduling, dirty flushing, focus
handling, effect-loop handling, reconciliation, and repeated-`Mount`
validation. Their build-tag-specific files contain only transaction-free or
document-batch wrappers around that common core.

The preceding review closeout rebuilt all eleven budgeted applications from the
frozen base to `a0a36a82a13c` at the same physical output path. The shared-core
extraction reduced every raw artifact by 27 or 32 bytes and all 44 streams
remained within their current ceilings and ratios.

The boundary-outcome closeout then compared `a0a36a82a13c` with
`e73351815664`. Nine applications were byte-identical across raw, gzip, Brotli,
and Zstandard output. `router-dashboard` and `resource`, the two ordinary
fixtures that compile the Error Boundary path, each added seven raw code-section
bytes with unchanged data and name sections. Their compressed deltas were
respectively +7/-7/-19 and +3/+40/+3 bytes for gzip/Brotli/Zstandard. All 44
absolute cells and every ratio limit passed at that intermediate closeout; no
budget or ratio changed.
Ordinary source selection and binary searches found no coordinator, experiment
bridge, document batch wrapper, or experiment build-tag symbol.

## Validation

The final research head passed the Go `1.22.12`, `1.25.12`, and `1.26.5`
ordinary and `goframe_debug` test lanes plus vet. The supported Go lanes passed
the package and command race matrix. Artifact, module-path, documentation,
actionlint, extension, size, and aggregate Browser Smoke gates passed. Two
fresh aggregate Browser Smoke executions passed under Go `1.26.5`, Node
`24.18.1`, TinyGo `0.41.1`, and Chrome.

Windows host evidence belongs to the existing Core `windows-latest` lane,
which runs formatting, ordinary tests, vet, `goframe_debug` tests, and selected
GOX golden tests. Browser-only `js && wasm` main packages are not a local
Windows all-package build contract.

## Limitations

- The accepted mechanism is experiment-tagged. This stage establishes the
  lifecycle boundary but does not ship a document metadata feature.
- Result B does not select the hook, non-DOM component, or owner-handle API
  shape and does not authorize a release or compatibility guarantee.
- TinyGo evidence covers successful ownership paths only. It does not establish
  recover-based publication failure or Error Boundary rollback.
- The fixture bridge is exported only so a separate `main` package can exercise
  private runtime behavior in a real browser. Those names are absent from
  ordinary builds and are not compatibility candidates.
- The update batch is tied to the current single-active-application browser
  mount path. It is not a generic transaction facility.
- Pair consistency is established at committed framework and recorded observer
  boundaries, not as one browser DOM instruction.
- Pair restoration is best-effort when the browser rejects both publication and
  restoration. All failures remain available in the propagated diagnostic; no
  private mechanism can make two independent DOM assignments physically atomic.
- Observer failure is reported after commit. It cannot request transaction
  rollback and the experiment does not provide a public observer policy.
- External scripts that concurrently mutate the authored nodes are not covered.
- Arbitrary head elements, SSR, hydration, SEO behavior, multiple documents,
  portals, and concurrent applications are outside this result.

## Decision Boundary

Result B confirms only the private lifecycle foundation identified as missing
by Result D. Result D remains the current API-shape decision for the implicit
hook, non-DOM component, and explicit owner-handle candidates: none is
selected. This evidence assigns no public stability tier, compatibility
guarantee, or release authorization.

This research is independent of Issue #117. It uses neither the evaluator
workspace nor unpublished evaluator submissions and makes no conclusion about
that task.
