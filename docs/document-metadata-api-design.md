# Document Metadata API Design

## Status

Corrected - Result D: no implementation candidate selected.

The hook, non-DOM component, and owner-handle prototypes satisfy the focused
coordinator and wrapper-lifecycle contract. After owner creation was moved out
of render, however, all three corrected server-backed projections exposed an
extra committed initialization phase between route-owner lifetimes. That phase
temporarily restored the authored baseline during ordinary route changes.

No public API is added by this branch. The previous Component Candidate result
and its measurements are superseded by this record.

## Context

The document-state fixture and the server-backed example independently
implement the same title and description ownership model. Each application
currently owns an ordered coordinator, stable owner identity, publication and
release lifecycle, selected-snapshot state, browser bindings, and a central
document committer.

The repeated value is one pair:

```go
type DocumentMetadata struct {
	Title       string
	Description string
}
```

The comparison asks which application-facing shape can hide owner keys and
coordinator plumbing while retaining component-scoped ownership. It does not
add a general head manager or change runtime lifecycle semantics.

## Evidence Surfaces

The executable comparison uses
`scripts/fixtures/document-state-api-design`. It contains one ordered
coordinator model, one explicit string-owner control, and private hook,
component, and owner-handle wrappers.

Pure Go tests exercise coordinator ordering, token identity, publication,
release, and error behavior. They do not execute hook, component, or handle
lifecycle wrappers through a host renderer.

The browser harness executes the actual wrappers in one Chrome process. A
pre-boot observer records the authored title and description, their node
identities, managed title and description mutations, coordinator statistics,
owner IDs, and candidate-specific lifetime events before WASM starts. One
unrelated authored metadata node remains outside the managed pair.

Temporary candidate-only copies provide TinyGo size evidence. Temporary copies
of `examples/server-backed` test package-level integration. None of those
measurement copies is committed.

## Previous Allocation Defect

The old wrappers passed side-effectful expressions to `UseState`:

```go
owner, _ := gf.UseState(bindings.coordinator.NewOwner())
```

```go
handle, _ := gf.UseState(&documentMetadataOwnerHandle{
	owner: bindings.coordinator.NewOwner(),
})
```

The handle publication used the same eager pattern. Go evaluates those
arguments on every render even when `UseState` retains the existing slot.
`NewOwner` also assigned an ID immediately. Ordinary rerenders and failed
speculative renders therefore created discarded objects and consumed IDs.

A temporary old-head probe observed the route committed as ID 1 while the same
initial route pass evaluated a discarded ID 2. An ordinary update evaluated
discarded IDs 7 and 8 while the active route remained ID 1. A same-value
rerender evaluated discarded IDs 25 and 26. A failed speculative hook render
evaluated ID 41 without adding an active owner. Component and handle modes
showed the same class of discarded allocation.

## Corrected Identity Contract

An owner token begins inactive:

```text
ID = 0
not in active order
does not reserve the next committed ID
```

`NewOwner` creates only that inactive token. The first successful
`Coordinator.Publish` assigns the next numeric ID, appends the owner, and does
so from committed effect work. Updating the same owner retains its ID and
priority. Releasing an unpublished token is a no-op. IDs are never recycled.

The candidate wrappers use two committed phases:

```text
render with nil private state
-> initialization effect creates and installs token/handle/publication
-> next committed render publishes through the stable token
```

No candidate setter runs during render. A render that fails before commit
creates no token, handle, publication, ID, or active owner.

## Fixed Ownership Contract

The corrected prototypes retain these requirements:

1. Title and description are one value.
2. A newly committed owner receives highest priority.
3. Updating an active owner changes its pair without changing priority.
4. Releasing the selected owner reveals the latest remaining owner.
5. Releasing a non-selected owner does not change the selected pair.
6. Releasing the final owner restores the authored baseline.
7. Remount creates a new ownership lifetime.
8. Failed speculative renders publish no owner.
9. Identical selected metadata causes no document write.
10. Authored title and description node identities remain stable.
11. Unrelated authored head elements remain unchanged.
12. No document mutation occurs during render.

The browser evidence establishes pair consistency at committed application and
MutationObserver boundaries. It does not claim one atomic browser operation for
both DOM writes.

## Non-Goals

This comparison does not define:

- independent title and description ownership;
- arbitrary head elements;
- router-specific metadata;
- a public coordinator or global registry;
- SSR or hydration behavior;
- SEO guarantees;
- cross-document ownership;
- a public API in this branch.

## Candidate A - Implicit Hook

The compared shape is:

```go
gf.UseDocumentMetadata(gf.DocumentMetadata{
	Title:       "...",
	Description: "...",
})
```

The private hook retains one nil token state slot. Its initialization effect
creates the token, its publication effect activates or updates it, and its
unmount callback releases it. Two calls in one component create two committed
owners. Failed initial render creates neither.

The hook has the smallest corrected candidate-only TinyGo result and the
shortest ordinary route call site. Ownership is not visible in the returned
node tree. Conditional ownership still needs another component boundary, and
the extra committed initialization phase is visible when one route component
replaces another.

## Candidate B - Non-DOM Component

The compared shape is:

```gox
<gf.DocumentMetadata
	Metadata={gf.DocumentMetadata{
		Title:       "...",
		Description: "...",
	}}
>
	{content}
</gf.DocumentMetadata>
```

One mounted component instance retains one nil token state slot. Conditional
mounting expresses activation and unmount expresses release. Prop updates
retain owner identity and priority. The component returns children as a
fragment and emits no wrapper element.

This shape still makes ownership most visible in GOX and avoids positional-hook
conditions in application components. Its corrected server-backed projection
nevertheless has the same committed initialization gap as the hook.

## Candidate C - Explicit Owner Handle

The compared shape is:

```go
owner := gf.UseDocumentMetadataOwner()
gf.UseOwnedDocumentMetadata(owner, gf.DocumentMetadata{
	Title:       "...",
	Description: "...",
})
```

The private prototype retains a nil handle slot and a nil publication slot.
Committed initialization effects create them. A later publication effect
activates the owner. Passing the handle through a helper preserves identity.
The first publication becomes primary. An identical duplicate coalesces without
another coordinator publication. While multiple publications coexist, their
metadata is immutable: changing either one is rejected before coordinator,
document, handle, or publication state changes. Releasing the duplicate leaves
the primary active, after which that sole primary can update through the same
owner. The prototype does not transfer primary status when the primary itself
is released first.

The handle supports helper composition explicitly, but requires two calls,
three effect sites, and split handle/publication lifetime rules. It also has the
largest corrected source and TinyGo cost, while retaining the same integration
gap.

## Pure Coordinator Conformance

Pure host tests cover the control and opaque-token coordinator models, not the
actual candidate wrappers.

| Scenario | Required result | Corrected result |
|---|---|---|
| inactive token | ID 0 | pass |
| unpublished token before published token | no ID gap | pass |
| first publication | ID 1 | pass |
| existing-owner update | same ID and priority | pass |
| identical publication | no transition | pass |
| unpublished release | no-op | pass |
| release and remount | exactly next ID | pass |
| foreign token | rejected before and after publication | pass |
| title-only and description-only updates | compare the complete pair | pass |

The focused pure suite passed 100 repetitions and 20 race-detector
repetitions.

The handle publication state machine has separate focused coverage:

| Scenario | Corrected result |
|---|---|
| first publication | primary, count one |
| identical duplicate | coalesced, count two, no coordinator write |
| conflicting duplicate activation | rejected before mutation |
| primary update while duplicated | rejected before mutation |
| duplicate release | count one, owner remains active |
| sole-primary update after duplicate release | same owner, one update |
| final release | exactly one coordinator release |
| repeated release | no-op |
| publication-count underflow | rejected |

Those tests also passed 100 repetitions and 20 race-detector repetitions.

## Candidate Wrapper Lifecycle Evidence

Actual hook, component, and handle lifecycle wrappers are exercised in the
browser renderer. Pure host tests cover their shared coordinator model and
identity allocation rules.

The ordinary route lifetime for each candidate produced one token, one
committed ID, and one active addition. Metadata and identical rerenders created
no token or ID. A speculative failed owner created none. Scope remount created
exactly one next lifetime.

Candidate-specific browser evidence showed:

- hook: the two-slot probe created tokens and IDs 5 and 6, then released both
  exactly once;
- component: five successful mounts matched four unmounts with one active
  remount, and no wrapper DOM element appeared;
- handle: seven handle/token/ID lifetimes and nine publication lifetimes were
  committed; one forwarded duplicate and one conflict-probe duplicate
  coalesced through owner IDs 5 and 6;
- handle conflict probe: changing the primary while its identical duplicate
  remained active produced the precise conflict diagnostic with no coordinator,
  document, handle-count, or publication-state change; after duplicate release,
  the primary published metadata C through the same owner ID;
- all candidates: the failed-render token, ID, and active-owner deltas were
  zero.

## Focused Browser Evidence

| Scenario | Control | Hook | Component | Handle |
|---|---:|---:|---:|---:|
| route activation | pass | pass | pass | pass |
| nested activation | pass | pass | pass | pass |
| parent update beneath nested owner | pass | pass | pass | pass |
| selected release reveals updated parent | pass | pass | pass | pass |
| non-selected release | pass | pass | pass | pass |
| identical selected rerender | pass | pass | pass | pass |
| failed speculative render | pass | pass | pass | pass |
| final release restores baseline | pass | pass | pass | pass |
| scope remount | pass | pass | pass | pass |
| node identity | pass | pass | pass | pass |
| duplicate description prevention | pass | pass | pass | pass |

The harness requires positive observer evidence for every mode. Exact counts
below are deterministic evidence for the pinned toolchain, not compatibility
requirements:

| Mode | Title batches | Description batches | Head snapshots | Snapshot length | Invalid pairs |
|---|---:|---:|---:|---:|---:|
| control | 7 | 7 | 7 | 7 | 0 |
| hook | 9 | 9 | 9 | 9 | 0 |
| component | 7 | 7 | 7 | 7 | 0 |
| handle | 12 | 12 | 12 | 12 | 0 |

Every mode requires title batches, description batches, and head snapshots to
be positive, and requires snapshot length to equal the head-snapshot count. A
temporary fault injection that classified managed title and description nodes
as unrelated failed with `control observer liveness: no title mutation
batches`; the injected copy was removed.

Two focused browser runs produced byte-identical normalized evidence after
excluding elapsed time:

```text
SHA-256:
1bf05e4c9b3f0cb26f0835146d6440f9850eb380d4b8f1eb1b35458b16b2b03b
```

The runs took 7,580 ms and 7,486 ms. Transition counters, coordinator
statistics, committed owner-ID sequences, mutation logs, baseline restoration,
node identity, handle-conflict evidence, full-navigation evidence, and failure
counters were identical.

Final focused statistics were:

| Mode | Tokens | IDs | Adds | Updates | Releases | Active | Last ID |
|---|---:|---:|---:|---:|---:|---:|---:|
| control | 5 | 5 | 5 | 1 | 4 | 1 | 5 |
| hook | 7 | 7 | 7 | 1 | 6 | 1 | 7 |
| component | 5 | 5 | 5 | 1 | 4 | 1 | 5 |
| handle | 7 | 7 | 7 | 2 | 6 | 1 | 7 |

Candidate mode links preserve boot-time mode selection by navigating the full
document to `./?candidate=<mode>#/<mode>`. The browser sequence control -> hook
-> component -> handle -> control increased the document boot count from 1 to
5. Every navigation updated the query and hash, rendered the target mode, and
recaptured the authored baseline. The fixture adds no router or `hashchange`
listener.

Browser command validation is awaited before fixture packaging, temporary
browser roots, or the detached fixture server are created. The actual Chrome
spawn is also awaited. A missing absolute browser command returned its original
`ENOENT` spawn diagnostic, left the selected port closed, created no fixture
package or task-owned temporary root, and settled without a leaked server.

## Candidate Measurements

Counts below cover one ordinary owner. Initialization renders count committed
renders from mount through the first publication. Effect sites count
`UseEffect` calls; unmount callbacks are listed separately. Support lines use
the same source-span rule as the superseded measurements: the complete private
lifecycle wrapper and its private owner/publication types, including embedded
fixture evidence calls.

| Measure | Control | Hook | Component | Handle |
|---|---:|---:|---:|---:|
| private state slots | 0 | 1 | 1 | 2 |
| effect sites | 1 | 2 | 2 | 3 |
| initialization renders | 1 | 2 | 2 | 2 |
| token creations | 1 | 1 | 1 | 1 |
| committed owner IDs | 1 | 1 | 1 | 1 |
| unmount callbacks | 1 | 1 | 1 | 1 |
| ownership support lines | 34 | 44 | 61 | 280 |
| application-visible owner keys | 2 | 0 | 0 | 0 |
| positional-hook restriction at caller | yes | yes | no | yes |

All candidates require one more effect site than the eager versions. Hook and
component support grew from 21 and 36 lines to 44 and 61. The corrected
duplicate-publication state machine grows handle support from the earlier 147
lines to 280.

## TinyGo Size Evidence

Measurements use Go 1.22.12, TinyGo 0.41.1, Linux amd64, `gzip -n -9`,
Brotli quality 11, and Zstandard level 19. Each candidate-only copy retains the
same scenario UI, coordinator, and browser adapter, with only one candidate
wrapper and its candidate-specific probe reachable.

| Fixture | Raw | gzip | Brotli | Zstandard | Raw delta from control |
|---|---:|---:|---:|---:|---:|
| control only | 307,855 B | 131,023 B | 107,380 B | 113,305 B | 0 B |
| hook only | 314,220 B | 132,653 B | 108,452 B | 114,642 B | +6,365 B |
| component only | 315,374 B | 133,030 B | 108,848 B | 115,222 B | +7,519 B |
| handle only | 331,049 B | 139,465 B | 112,706 B | 119,471 B | +23,194 B |
| combined comparison fixture | 352,383 B | 146,161 B | 117,522 B | 124,598 B | not comparable |

The corrected component costs 1,154 raw bytes more than the hook. The handle
measurement includes the corrected conflict state machine and its focused
probe. The previous candidate-only and combined values are superseded.

## Candidate Size Reproduction

The pinned pre-review measurement source remains
`7eb2f01804876c317e652e7b48f90dc9c36d90bb`, a direct ancestor of this record.
The current measurements require the corrected handle and browser evidence, so
their source is commit
`2e197aec7fa52e2c3cf8799371af401eeb9bbf41`. A reproducible temporary layout for
the current values is:

```bash
root="$(mktemp -d)"
git archive 2e197aec7fa52e2c3cf8799371af401eeb9bbf41 \
  | tar -x -C "$root"

for mode in control hook component handle combined; do
  cp -a "$root" "$root-$mode"
done
```

For `control`, `hook`, `component`, and `handle`, replace the
`CandidateOwner` mode switch with the corresponding direct wrapper call. Keep
the common failure and render recording after the direct hook/control/handle
call; the component dispatch returns the component directly. In
`candidateProbeNode`, retain only the hook two-slot probe for `hook`, only the
forwarded duplicate probe for `handle`, and return `gf.Empty()` for `control`
and `component`. Retain `handleConflictProbeNode` and its controls only for the
`handle` copy; make them unreachable in the other candidate-only copies. The
`combined` copy is unchanged. Run `gofmt` on `candidates.go`.

Normalized projection patch hashes are:

| Copy | Patch SHA-256 |
|---|---|
| control | `c2d31c8ec7c8cd6595ded28918765460510d604c4ce0ab448712efe512c36066` |
| hook | `41957c7fea1321f6f110e7407ff6c04cdbb6711827edccea3d3d25181bdd513f` |
| component | `da9cb9c8d313c22bccf8808bcb41c525968c7e1b75f2de37418dd5782d978b39` |
| handle | `76ed7dd79004f524ed9d49763ba83b6ff5381d46f443d4161495df17fadb1e05` |
| combined | no patch |

Each patch is a unified diff with repository-relative labels for
`candidates.go` and `app.gox`, created before generation. Build and measure each
copy with:

```bash
goxc build ./scripts/fixtures/document-state-api-design --compiler=tinygo

wasm="./scripts/fixtures/document-state-api-design/.goframe/build/tinygo/dev/bundle.wasm"
wc -c "$wasm"
gzip -n -9 -c "$wasm" | wc -c
brotli -q 11 -c "$wasm" | wc -c
zstd -19 -q -c "$wasm" | wc -c
sha256sum "$wasm"
```

Remove the copies and owned caches with:

```bash
rm -rf "$root" "$root-control" "$root-hook" "$root-component" \
  "$root-handle" "$root-combined"
```

## Integrated Server-Backed Projection

Each temporary projection:

- generated successfully;
- passed `examples/server-backed/...` pure tests;
- completed a TinyGo WASM build;
- produced a standard-Go standalone package;
- removed six caller-supplied owner-key props and the application
  `DocumentOwner`/`useServerBackedDocumentState` adapter;
- added an inactive opaque token path whose ID is assigned on first committed
  publication.

| Candidate | Patch SHA-256 | Added | Removed | Total changed | Generate/tests/build/package |
|---|---|---:|---:|---:|---|
| hook | `1558632036e4e7e8ea0cf9c0f0fd93495bef10b7dc9a83c44512f1b4add4bb57` | 189 | 98 | 287 | pass |
| component | `0611546ba45c3bfc5ffc36a8db794e1c58d68277ae1c1bf849e6f81f5f88bbb2` | 144 | 70 | 214 | pass |
| handle | `f0c7c1f84e5a2eb77df50caec653b41de49b902512547b2e205e897f3b994ae8` | 349 | 100 | 449 | pass |

Opaque owner identity and evidence labels are stored separately. A token's first
publication labels the first active candidate owner as `route` and a deeper
owner as `saved-editor`; the existing string-key coordinator path retains its
exact labels for pure-test compatibility. Labels are not identity and do not
appear at candidate call sites.

The earlier corrected projection ran the full existing server-backed browser
harness against all three candidates. The refreshed handle projection includes
the corrected duplicate-publication state machine and again reached the final
document assertion with request and safety evidence intact:

```text
ordinary greeting requests: 11
retained-transition requests: 10
saved GET requests: 4
saved POST requests: 4
invalid metadata pairs: 0
stale metadata appearances: 0
duplicate descriptions: 0
saved-editor activations/releases: 5/5
```

The refreshed handle result matches the prior shared lifecycle failure:

| Counter | Reference | Hook | Component | Handle |
|---|---:|---:|---:|---:|
| authored-baseline restorations | 1 | 9 | 9 | 9 |
| title mutation batches | 43 | 51 | 51 | 51 |
| description mutation batches | 43 | 51 | 51 | 51 |
| document snapshots | 139 | 147 | 147 | 147 |

The eight extra restorations occur when an old route owner releases before the
replacement wrapper reaches its second committed publication phase. They are
observable completed states, not invalid title/description pairs.

## Server Projection Reproduction

Use corrected evidence commit
`2e197aec7fa52e2c3cf8799371af401eeb9bbf41` and one temporary copy per
candidate:

```bash
root="$(mktemp -d)"
git archive 2e197aec7fa52e2c3cf8799371af401eeb9bbf41 \
  | tar -x -C "$root"

for mode in hook component handle; do
  cp -a "$root" "$root-$mode"
done
```

Apply these exact transformation rules to each copy:

1. In `document_state.go`, adapt the `Owner`, `NewOwner`, `Publish`, `Release`,
   and foreign-owner validation from
   `scripts/fixtures/document-state-api-design/internal/documentmeta/model.go`
   to `serverBackedDocumentState`. Store `coordinator` and `key` on the token,
   add `nextID uint64`, and set the key to
   `"owner-"+strconv.FormatUint(nextID, 10)` only in first `Publish`. Preserve
   the existing string `Set`, `Remove`, `Snapshot`, and pure tests. Store opaque
   owner identity separately from the evidence label; label the first candidate
   owner `route` and a deeper candidate owner `saved-editor` when its token
   first publishes.
2. In `app.gox`, remove `DocumentOwnerProps`, the two string-owner constants,
   `DocumentOwner`, and `useServerBackedDocumentState`.
3. Add `document_candidate.go` with `//go:build js && wasm`. For hook,
   component, or handle respectively, copy the corrected lifecycle structure
   of `useHookDocumentMetadata`, `ComponentDocumentMetadata`, or the
   `documentMetadataOwnerHandle`/`documentMetadataPublication` block from
   `scripts/fixtures/document-state-api-design/cmd/app/candidates.go`. Remove
   roles and fixture evidence calls, substitute the server-backed types, and
   call `OnSnapshot` only when the coordinator reports a change.
4. Replace the five route ownership sites and one saved-editor site. Hook and
   handle projections add a conditional `SavedGreetingEditor` component;
   component projection directly replaces `DocumentOwner` with
   `DocumentMetadata`.
5. Do not modify tests, the browser harness, backend protocol, document
   adapter, or committer.

Create repository-relative unified diffs in `app.gox`, added
`document_candidate.go`, then `document_state.go` order; their hashes must match
the table above. Run from each temporary root:

```bash
goxc generate ./examples/server-backed
go test -count=1 ./examples/server-backed/...
goxc build ./examples/server-backed --compiler=tinygo
goxc package ./examples/server-backed --compiler=go
```

Browser evidence uses:

```bash
node --experimental-websocket scripts/server-backed-browser-smoke.mjs
```

The expected corrected-projection result is the final lifecycle-counter
failure shown above. Cleanup is:

```bash
rm -rf "$root" "$root-hook" "$root-component" "$root-handle"
```

## Misuse Analysis

| Risk | Hook | Component | Handle |
|---|---|---|---|
| conditional use | positional slot can change | ordinary conditional mount is valid | handle or publication call can become conditional |
| initialization | hidden two-phase activation | visible owner still activates in two phases | handle and publication initialize separately |
| route replacement | baseline gap before publication | baseline gap before publication | baseline gap before publication |
| duplicate owner creation | extra hook call creates owner | extra mounted component creates owner | extra handle creates owner |
| token escape or reuse | no token exposed | no token exposed | handle can escape or be reused |
| helper composition | implicit slot survives helper only through caller shape | requires component composition | explicit handle forwarding |
| publication conflict | another hook means another owner | another component means another owner | duplicate/conflicting publication rules required |
| missing root binding | diagnostic required | diagnostic required | diagnostic required |
| multiple mounted apps | binding must remain mount-local | binding must remain mount-local | handle must not cross mounts |
| testing ergonomics | render hook owner | mount non-DOM owner component | create handle and publication |

## Decision Matrix

The score uses 0 as weakest and 3 as strongest. Integration receives zero for
all candidates because the corrected projections do not preserve the reference
owner-handoff counters.

| Category | Hook | Component | Handle |
|---|---:|---:|---:|
| semantic fit | 2 | 2 | 2 |
| call-site clarity | 2 | 3 | 2 |
| nested-owner ergonomics | 2 | 3 | 2 |
| update ergonomics | 3 | 3 | 3 |
| lifecycle safety | 1 | 1 | 1 |
| initialization complexity | 2 | 2 | 1 |
| positional-hook burden | 1 | 3 | 2 |
| GOX ergonomics | 2 | 3 | 1 |
| helper composition | 3 | 2 | 3 |
| testability | 2 | 3 | 2 |
| misuse resistance | 2 | 2 | 1 |
| implementation narrowness | 2 | 2 | 1 |
| DCE feasibility | 3 | 3 | 3 |
| TinyGo cost | 3 | 2 | 1 |
| integrated projection | 0 | 0 | 0 |
| **Total** | **30** | **34** | **25** |

## Decision

Result D is selected: no implementation candidate.

The component remains the clearest call-site shape and the narrowest integrated
patch, but that relative lead does not override the shared lifecycle
regression. A component, hook, or handle API should not be selected until
committed owner activation can hand off between component lifetimes without an
observable authored-baseline interval and without moving owner creation back
into render.

No proposed public spelling or stability tier is assigned. If a later design
produces a viable candidate, it remains an Experimental Frontier question, not
a Public-Candidate or compatibility promise.

## Rejected Alternatives

- The explicit string-owner control remains application-local and exposes the
  machinery this design intends to hide.
- Moving `NewOwner` back into a `UseState` argument recreates discarded
  per-render allocations and failed-render ID gaps.
- Render-time setters or direct DOM writes violate lifecycle commit semantics.
- Accepting the eight extra baseline restorations weakens existing
  server-backed ownership evidence.
- Independent title and description APIs can create mixed pairs and are outside
  the fixed contract.
- A public coordinator, global registry, router integration, transactional
  lifecycle redesign, or general head manager broadens this stage.

## Public Surface Boundary

This branch does not add:

```text
gf.DocumentMetadata
gf.UseDocumentMetadata
gf.UseDocumentMetadataOwner
gf.UseOwnedDocumentMetadata
```

No production implementation is authorized by this record.

## Remaining Limits

The evidence covers one browser document, one authored title, one authored
description element, nested component owners, route ownership, editor
overrides, failed-render isolation, baseline restoration, and one real
server-backed route workflow.

It does not establish:

- arbitrary head-element ownership;
- SSR or hydration behavior;
- multiple browser documents or portals;
- concurrent independently mounted apps targeting one document;
- interaction with external scripts that mutate the same head nodes;
- one atomic browser operation for title and description;
- a stable diagnostic or testing API;
- production SEO behavior;
- TinyGo panic/recover containment.

Those limits must not be inferred from the private prototype.
