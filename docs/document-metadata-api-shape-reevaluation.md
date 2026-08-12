# Transactional Document Metadata API-Shape Reevaluation

## Status

API-shape Result D - no shape selected.

The merged private lifecycle foundation remains Lifecycle Result B: a bounded,
document-specific application-update handoff is technically viable. This
reevaluation projects the implicit hook, non-DOM component, and explicit owner
handle onto that same foundation. The hook and non-DOM component pass the
current lifecycle hard gates. The handle fails stable reuse across a
zero-publication interval and is disqualified from the current comparison.

No public API is added. The build-tagged identifiers in the comparison fixture
are fixture-only exports, are absent from ordinary builds and package
documentation, and are not compatibility candidates.

## Historical Baseline

PR #122 compared corrected hook, component, and handle prototypes over an
effect-initialized owner model. Every candidate required a committed
initialization render before its first publication. Direct owner replacement
therefore exposed an authored-baseline interval. That shared lifecycle blocker
led to API-shape Result D.

PR #134 merged a private transactional ownership foundation. It combines
incoming publication and outgoing release in one document-specific
application-update batch, retains failed handoffs as bounded unresolved
ownership plans, and commits owner IDs and metadata only after publication
succeeds. That work confirmed Lifecycle Result B without selecting an API
shape.

The merged foundation removes the shared initialization-render blocker, so the
three shapes can now be compared on their own call sites, lifetime expression,
misuse contracts, and cost.

## Shared Foundation

The control and all three candidates use the same:

- authored title and description baseline;
- `documentMetadataCoordinator` and unresolved ownership plans;
- document publisher and failure-atomic pair writer;
- application-update mount and dirty-update wrapper;
- Error Boundary outcome integration;
- browser observer and scenario sequence.

The direct control calls the existing private handoff primitive. It is not an
API candidate and exists only to attribute shared foundation behavior.

Every candidate's first successful render creates one speculative owner,
stages one publication, publishes one pair, and commits one owner ID. No effect
initialization or setter-driven render is involved.

## Executable Projections

### Foundation Control

```go
gf.UseDocumentMetadataHandoffExperiment(value)
```

The control uses one state slot and one unmount callback in the merged private
primitive.

### Candidate A - Implicit Hook

```go
gf.UseDocumentMetadata(gf.DocumentMetadata{
	Title:       "...",
	Description: "...",
})
```

One hook slot owns one stable owner. Two calls in one component create two
ordered owners. Conditional ownership requires a real component boundary so
hook order remains stable. Metadata updates retain owner identity and priority,
and unmount releases each slot once.

### Candidate B - Non-DOM Component

The compiled GOX projection is:

```gox
<gf.DocumentMetadataComponent
	Metadata={gf.DocumentMetadata{
		Title:       "...",
		Description: "...",
	}}
>
	{content}
</gf.DocumentMetadataComponent>
```

Go cannot declare a metadata value type and component function with the same
package-level identifier. The conceptual `<gf.DocumentMetadata>` spelling is
therefore not executable alongside `gf.DocumentMetadata{...}`; the private
fixture uses `DocumentMetadataComponent` so the call site is real rather than
hypothetical.

One mounted component instance owns one stable owner. Conditional mount and
unmount directly express the ownership lifetime. Prop updates retain identity.
The component returns a fragment, emits no wrapper host node, and preserves
child DOM identity.

### Candidate C - Explicit Owner Handle

```go
owner := gf.UseDocumentMetadataOwner()
publish(owner, gf.DocumentMetadata{
	Title:       "...",
	Description: "...",
})
```

```go
func publish(owner *gf.DocumentMetadataOwner, value gf.DocumentMetadata) {
	gf.UseOwnedDocumentMetadata(owner, value)
}
```

One handle represents one logical owner and survives helper forwarding. A
publication has its own hook slot. Identical simultaneous publications
coalesce; conflicting values are rejected before coordinator or document
mutation. Releasing a duplicate keeps the primary active, a sole primary can
update, and final publication release releases the owner once. Primary status
does not transfer to a duplicate publication.

The first browser implementation exposed a candidate-local retry defect: an
identical rerender after publisher failure was treated as already published.
The handle adapter now restages its primary publication while preserving the
coordinator unchanged. Focused host and browser regressions cover that rule.

A later host-lifetime test exposed a separate terminal-lifetime defect. When
the final forwarded publication unmounts, the coordinator releases the
underlying owner while the handle-owning component and stable Go handle remain
mounted. Its next render panics with:

```text
goframe: document metadata owner is already released
```

The rejected render creates no token, committed ID, owner, publication,
pending plan, or document write; the authored baseline remains selected and
the batch is inactive. The experiment intentionally does not renew the
underlying owner, because that would weaken the rule that one handle denotes
one logical owner. It also does not defer final release until handle unmount,
because that would leave stale metadata selected while the publication count
is zero. This is a handle-candidate disqualification, not a transactional
foundation failure.

## Host Evidence

Experiment-tagged host tests execute real render lifecycle helpers rather than
testing only detached data structures. They establish:

- one committed render to first publication for every candidate;
- state-slot counts of one for hook and component and two for handle;
- zero committed state slots, tokens, IDs, owners, or document writes after a
  failed initial render;
- exact direct `A -> B` replacement without a baseline publication;
- stable ID and priority on selected updates;
- no document write for non-selected release;
- final release to baseline and remount with a new ID;
- hook and component conditional-owner remount while their composition hosts
  remain mounted;
- ordered hook slots, component child preservation, the handle duplicate and
  conflict contract, and released-handle reuse characterization.

The focused suite passed 100 repetitions, 20 race-detector repetitions, and 20
`goframe_debug` repetitions under Go 1.26.5. The real GOX projection generated,
parsed, and type-checked as a complete package in 20 fixture-test repetitions.

## Hard-Gate Matrix

| Gate | Hook | Component | Handle | Executable evidence |
| --- | --- | --- | --- | --- |
| merged coordinator and update batch | pass | pass | pass | tagged host lifecycle tests and shared browser fixture |
| first successful render publishes | pass | pass | pass | first-render host test, one render and one publication |
| no initialization render | pass | pass | pass | first-render host counter |
| failed render commits nothing | pass | pass | pass | failed-initial host and standard-Go browser scenarios |
| exact keyed `A -> B` | pass | pass | pass | host replacement test plus observer and frame sequences |
| updates preserve identity and priority | pass | pass | pass | priority/update host test and selected-update browser path |
| selected release reveals survivor or baseline | pass | pass | pass | nested and final-release scenarios |
| conditional ownership can disappear and return while the composition host remains mounted | pass through component-boundary lifetime | pass through ordinary conditional mount | fail; stable handle retains its released underlying owner | `TestDocumentMetadataAPIShapeConditionalOwnershipRemount` |
| non-selected release writes nothing | pass | pass | pass | non-selected browser scenario |
| component and application teardown release once | pass | pass | pass | lifetime, teardown, and repeated-Mount scenarios |
| no generic renderer or scheduler transaction | pass | pass | pass | candidate-only tagged adapter diff |
| merged coordinator unchanged | pass | pass | pass | no foundation path changed by the branch |
| absent from ordinary builds | pass | pass | pass | Go/TinyGo source selection, artifact search, 44-stream comparison |
| explicit executable misuse contract | pass | pass | pass | slot, conditional-boundary, component, and handle conflict tests |
| standard Go failure and recovery | pass | pass | pass | 22 common browser scenarios per candidate |
| TinyGo successful paths only | pass | pass | pass | eight common successful scenarios per candidate |

The hook and component remain eligible. The handle fails the conditional
disappearance-and-return gate and is disqualified. Passing the remaining hard
gates does not restore its eligibility.

## Browser Evidence

Ten accepted standard-Go/WASM executions produced identical per-candidate and
combined behavior hashes:

| Mode | Standard-Go behavior SHA-256 |
| --- | --- |
| control | `e53dbd3fde3533e50d8087770d0de9207a7e6b8b3c7aec5a95a6916a500e52e5` |
| hook | `13301feeea702981b8003df3228d80259a02cd1031f6b1711915a5058243ce6c` |
| component | `6b592bf9c90cf262edda061612d3428ae74d54946b9a0b8708032d6aff80121c` |
| handle | `ddc6db4b999ecb611167ff8eb9fdb31fdfdc18e7e5949e2d8309aad0c78ce00d` |
| combined | `fba6c1d9d3df2c5b6d361be35aaa369f2f01e8ebac49536bcf41a80f7328f6ee` |

The shared standard-Go comparison artifact is 2,979,773 raw bytes with
SHA-256 `33f5e060bdc3088c72557da047272b89c23f56b90862abc5ae384d1a99e19529`.
Generated GOX is stable at SHA-256
`d927def1df878d39030491037f7972dbaff2507ab44e74c9df8e1d92c02def99`.

Every candidate runs the same 22 common scenarios. Direct replacement records
only A and B, assigns IDs 1 and 2, releases A once, and restores the baseline
zero times. Failed initial render assigns zero tokens and IDs and performs zero
document publications. Completed scenarios leave an inactive batch, no
pending ownership plan, no pending finalization, stable authored head nodes,
and no mixed metadata pair.

The corrected component failure path mounts `DocumentMetadataComponent`,
stages its speculative metadata publication, and then fails in a descendant
inside that component subtree. Failed initial render now records the rolled
back publication while committing zero tokens, owner IDs, or document writes.
Failed replacement rolls back the speculative B publication, keeps A selected,
and retries with a fresh B component to commit exact `A -> B`; A releases once
and no authored-baseline interval appears. This makes the component's failure
evidence equivalent to the hook and control foundation boundary rather than
inferring rollback from non-participation.

Ten accepted TinyGo 0.41.1 executions produced identical successful-path
hashes:

| Mode | TinyGo behavior SHA-256 |
| --- | --- |
| control | `f18688e83005a227759fe6a46077d9ee58cb422865ba05987807a4f1462febb0` |
| hook | `2b42db1fe20e25ae992c53bc02fe318e9b03018372a5adf7c4b395096f1b0044` |
| component | `1182d1c9ee783a33e7c19dbe62cf9faf4d9ac8265068ebad823060ee4be1b0ba` |
| handle | `00f26a44f08c7c3fbe9b06a3d88e22601921b8d040428f3b97be42fcad400e68` |
| combined | `69e153d6abc68edf47f04a5cee9a2d7e4eec7a95d9a74e76adcea27dbd3a9d05` |

The shared TinyGo artifact is 283,739 raw bytes with SHA-256
`04f1d83a6f73d532b4d95b4b652ed8df4fe5c69e741630d4aaaa263c4e449451`.
TinyGo executes eight shared successful-path scenarios and the successful
candidate-specific contracts. It does not intentionally panic and does not
establish recover-based publication-failure or Error Boundary rollback.

All ten corrected standard-Go attempts passed. The TinyGo closeout required
eleven attempts: one pre-WASM startup attempt loaded the authored page but did
not initialize the application, changed no application or document state, and
was preserved as a failed attempt; the other ten passed with identical hashes.

The comparison harness remains focused reproducible research evidence. It is
not invoked by `scripts/browser-smoke.sh`; the merged transactional ownership
foundation retains its separate standard-Go and TinyGo aggregate lanes. No
generic retry hides application, runtime, or scenario failures.

The focused harness CDP client also treats the first WebSocket close, error, or
send failure as terminal. It rejects and clears every pending call with one
deterministic harness error, rejects future calls immediately, ignores late
responses, and keeps close idempotent. This is harness-lifecycle reliability;
it does not add a scenario retry or make the focused comparison an aggregate
Browser Smoke gate.

## Candidate-Specific Evidence

| Candidate | Evidence |
| --- | --- |
| hook | two calls create ordered owners; updating the selected slot keeps identity; removing the conditional child boundary releases both slots exactly once |
| component | conditional mount and prop update keep one owner ID; no wrapper element is emitted; the child element retains DOM identity |
| handle | helper forwarding retains identity; identical duplicate coalesces; conflict reports one precise render error without mutation; duplicate release preserves primary; sole-primary update and final release succeed; after final release, the stable handle remains a Go object but reuse reports the exact released-owner panic without state mutation |

## Ergonomics And Implementation Cost

Support lines count nonblank, non-comment lines in each complete private
projection, including shared candidate-only owner machinery used by that
projection. Call-site lines count the executable candidate-only size fixture.

| Measure | Control | Hook | Component | Handle |
| --- | ---: | ---: | ---: | ---: |
| private state slots | 1 | 1 | 1 | 2 |
| committed renders to first publication | 1 | 1 | 1 | 1 |
| token creations | 1 | 1 | 1 | 1 |
| committed IDs | 1 | 1 | 1 | 1 |
| unmount callbacks per ordinary owner | 1 | 1 | 1 | 1 |
| candidate support lines | n/a | 117 | 125 | 341 |
| application ownership call-site lines | 1 | 1 | 3 | 5 |

| Dimension | Hook | Component | Handle |
| --- | --- | --- | --- |
| ownership visibility | implicit hook slot | explicit node-tree lifetime | explicit value plus publication slots |
| conditional ownership | requires component boundary | ordinary conditional mount | publication calls remain positional; boundary required |
| nested composition | concise but implicit | directly visible in GOX | explicit but verbose |
| helper composition | helper call owns caller's hook slot | component composition | explicit handle forwarding |
| GOX ergonomics | ordinary Go statement in component body | strongest structural projection, with naming collision cost | ordinary Go plus helper plumbing |
| misuse surface | conditional hook order | extra mounted owner component | nil/foreign handle, duplicate, primary, and conflict rules |
| testability | render a hook owner | mount a non-DOM owner | coordinate handle and publication lifetimes |

## Candidate-Only Size

Measurements use Go 1.26.5, TinyGo 0.41.1, deterministic gzip headers,
Brotli quality 11, and Zstandard level 19. Each artifact uses the same physical
path and scenario UI; deltas are relative to the direct foundation control.

| Standard Go | Raw | gzip | Brotli | Zstandard | Code | Data | Name | Raw delta |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| control | 2,420,520 | 701,077 | 532,864 | 570,990 | 1,474,726 | 889,272 | 51,011 | 0 |
| hook | 2,424,600 | 701,714 | 534,396 | 572,275 | 1,476,810 | 890,999 | 51,271 | +4,080 |
| component | 2,436,825 | 703,920 | 538,083 | 574,152 | 1,481,150 | 897,562 | 52,572 | +16,305 |
| handle | 2,444,205 | 706,717 | 538,906 | 575,719 | 1,488,523 | 898,330 | 51,814 | +23,685 |

| TinyGo | Raw | gzip | Brotli | Zstandard | Code | Data | Name | Raw delta |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| control | 154,105 | 68,161 | 57,530 | 61,057 | 116,533 | 28,603 | 7,441 | 0 |
| hook | 155,760 | 68,496 | 57,831 | 61,407 | 117,748 | 28,763 | 7,717 | +1,655 |
| component | 157,534 | 69,075 | 58,275 | 62,002 | 118,911 | 29,095 | 7,988 | +3,429 |
| handle | 161,534 | 70,407 | 59,453 | 63,186 | 122,751 | 29,203 | 8,042 | +7,429 |

Size favors the hook, but the differences do not resolve the ownership and
composition tradeoffs by themselves. The corrected nested component failure
increased only the shared focused fixture: standard Go grew by 2,166 raw bytes
and TinyGo by 365 raw bytes. Rerunning the candidate-only measurement produced
the byte-identical matrix above, so no candidate-only size comparison changed.

## Ordinary-Build Isolation

The base `a93ce243ba0e6f76d4c04160a14314781d16375d` and implementation head
`2227a8357ed493ba1eff6f70f5ad61a06491cf3d` were rebuilt at one matched
physical path. All eleven ordinary applications are byte-identical across raw,
gzip, Brotli, and Zstandard output. Both 44-stream manifests have SHA-256:

```text
9de950d4205b295d9107780e88128e1829882d9284761c8b4cda80653a195bc3
```

All current absolute ceilings and compression-ratio limits pass without a
budget change. Ordinary Go and TinyGo source selection lists the projection
file as ignored. Binary searches of ordinary standard-Go and TinyGo artifacts
found no candidate API, coordinator, or experiment-tag symbol.

## Decision

API-shape Result D - no shape selected.

The hook is the narrowest call site and smallest artifact, but its ownership is
implicit and conditional use inherits positional-hook constraints. The
component gives the clearest conditional and nested lifetime in GOX, but costs
more and cannot use the conceptual `DocumentMetadata` component name alongside
the metadata value type. The handle makes helper forwarding explicit, but a
stable handle cannot publish again after its last forwarded publication
releases the underlying owner. It is therefore disqualified independently of
its two-slot lifecycle, primary/duplicate rules, retry rule, support code, and
size.

The hook and component remain a tradeoff between implicit minimalism and
visible structural ownership. Choosing between those two eligible shapes would
depend mainly on API taste rather than a decisive behavior, safety,
integration, or size result. That is insufficient evidence for a public
surface.

Lifecycle Result B remains confirmed. API-shape Result B is not selected.

## Public Surface Boundary

This branch does not add an ordinary-build public API. It does not select the
eligible hook or component for later compatibility treatment; the handle is
disqualified by its zero-publication reuse contract. The branch does not
authorize a release or assign a stability tier.

The private fixture exports candidate spellings only under
`goframe_document_state_experiment` so a separate browser `main` package can
compile real call sites. Those names are excluded from ordinary source
selection and documentation.

## Limitations And Non-Goals

- The evidence covers one browser document and one mounted application.
- Standard Go establishes failure and recovery behavior; TinyGo establishes
  successful paths only.
- Pair consistency is observed at committed framework, MutationObserver, and
  animation-frame boundaries, not as one physical DOM instruction.
- The component naming conflict is a Go namespace fact, but alternative public
  naming was not compared in this stage.
- No arbitrary head elements, SSR, hydration, SEO, portals, multiple documents,
  or concurrent application contract is established.
- No route loader, transition, cache, action, mutation, or router API is added.
- Issue #117 remains methodologically separate and supplies no evidence to this
  decision.
