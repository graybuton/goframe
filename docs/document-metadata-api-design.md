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
pre-boot observer records the authored title, description, viewport metadata,
node identities, managed mutations, coordinator statistics, owner IDs, and
candidate-specific lifetime events before WASM starts.

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
Identical duplicate publications coalesce, while conflicting simultaneous
publications are rejected.

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
- handle: six handle/token/ID lifetimes and seven publication lifetimes were
  committed; one forwarded duplicate coalesced through owner ID 5;
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

Two focused browser runs produced byte-identical normalized evidence after
excluding elapsed time:

```text
SHA-256:
8a6df8a2eab01b652405c861d66609299c6f67f122d6d3cf1429428f9a5dc058
```

The runs took 5,908 ms and 5,674 ms. Transition counters, coordinator
statistics, committed owner-ID sequences, mutation logs, baseline restoration,
node identity, and failure counters were identical.

Final focused statistics were:

| Mode | Tokens | IDs | Adds | Updates | Releases | Active | Last ID |
|---|---:|---:|---:|---:|---:|---:|---:|
| control | 5 | 5 | 5 | 1 | 4 | 1 | 5 |
| hook | 7 | 7 | 7 | 1 | 6 | 1 | 7 |
| component | 5 | 5 | 5 | 1 | 4 | 1 | 5 |
| handle | 6 | 6 | 6 | 1 | 5 | 1 | 6 |

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
| ownership support lines | 34 | 44 | 61 | 147 |
| application-visible owner keys | 2 | 0 | 0 | 0 |
| positional-hook restriction at caller | yes | yes | no | yes |

All candidates require one more effect site than the eager versions. Hook and
component support grew from 21 and 36 lines to 44 and 61. Handle support grew
from 90 to 147 lines.

## TinyGo Size Evidence

Measurements use Go 1.22.12, TinyGo 0.41.1, Linux amd64, `gzip -n -9`,
Brotli quality 11, and Zstandard level 19. Each candidate-only copy retains the
same scenario UI, coordinator, and browser adapter, with only one candidate
wrapper and its candidate-specific probe reachable.

| Fixture | Raw | gzip | Brotli | Zstandard | Raw delta from control |
|---|---:|---:|---:|---:|---:|
| control only | 307,235 B | 130,835 B | 107,144 B | 113,214 B | 0 B |
| hook only | 313,588 B | 132,352 B | 108,503 B | 114,483 B | +6,353 B |
| component only | 314,758 B | 132,782 B | 108,507 B | 114,976 B | +7,523 B |
| handle only | 319,836 B | 135,242 B | 109,902 B | 116,914 B | +12,601 B |
| combined comparison fixture | 341,382 B | 142,089 B | 114,833 B | 121,807 B | not comparable |

The corrected component costs 1,170 raw bytes more than the hook. The corrected
combined fixture is 9,973 raw bytes, 2,908 gzip bytes, 2,432 Brotli bytes, and
2,599 Zstandard bytes larger than the old eager-allocation head. The previous
candidate-only and combined measurements are superseded.

## Candidate Size Reproduction

The measurement source is commit
`7eb2f01804876c317e652e7b48f90dc9c36d90bb`. A reproducible temporary layout is:

```bash
root="$(mktemp -d)"
git archive 7eb2f01804876c317e652e7b48f90dc9c36d90bb \
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
and `component`. The `combined` copy is unchanged. Run `gofmt` on
`candidates.go`.

Normalized projection patch hashes are:

| Copy | Patch SHA-256 |
|---|---|
| control | `62d6193bbb811d50a1719407a960cf1effa7dfd0ee2548c155461db986ec7d64` |
| hook | `85cc1776b00a6467354a58f8ab1b8a52f4443f2449560054367d3089e0a10bde` |
| component | `afd9195392a7eb70bb5794b877bc30677076d6ef74b909cb82e5564c73799ee6` |
| handle | `436687cc326028c6d951dab38437299f4c6676f7825380424f440b29d51d6614` |
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
| hook | `0f559023f210fda6697722cab125c3f4c8d795e671cbf394e00dfdffe0d79ec7` | 159 | 99 | 258 | pass |
| component | `a9dc2653d80932bd178e4ad28dc235b987006e5c336e1b8c2c6eab00c3673970` | 128 | 69 | 197 | pass |
| handle | `5dc404d16abaf5958660e9121956fe6317c5527abbe485cde691917e90702bb4` | 244 | 99 | 343 | pass |

The evidence-label adapter maps zero active owners to the authored baseline,
one active owner to `route`, and a deeper active owner to `saved-editor`.
Those labels are not identity and do not appear at candidate call sites.

The full existing server-backed browser harness was run against all three
corrected projections. Each reached the final document assertion with the
request and safety evidence intact:

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

Each projection failed the existing lifecycle counters in the same way:

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

Use the same source commit and one temporary copy per candidate:

```bash
root="$(mktemp -d)"
git archive 7eb2f01804876c317e652e7b48f90dc9c36d90bb \
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
   the existing string `Set`, `Remove`, `Snapshot`, and pure tests. Map the
   candidate result label to empty at count 0, `route` at count 1, and
   `saved-editor` above count 1.
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

Create repository-relative unified diffs for `app.gox`,
`document_state.go`, and the added `document_candidate.go`; their hashes must
match the table above. Run from each temporary root:

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
