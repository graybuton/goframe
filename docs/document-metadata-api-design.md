# Document Metadata API Design

## Status

Accepted — Component Candidate

This record selects the non-DOM component form as the implementation candidate.
No public API is added by this branch.

## Context

The document-state fixture and the server-backed example independently implement
the same title and description ownership model. Each application currently owns
an ordered coordinator, stable owner identity, publication and release
lifecycle, selected-snapshot state, browser bindings, and a central document
committer.

The repeated value is one pair:

```go
type DocumentMetadata struct {
	Title       string
	Description string
}
```

The design comparison asks which application-facing shape can hide owner keys
and coordinator plumbing while retaining component-scoped ownership. The
comparison does not add a general head manager or change runtime lifecycle
semantics.

## Evidence Surfaces

The executable comparison uses
`scripts/fixtures/document-state-api-design`. It contains one common ordered
model, one explicit string-owner control, and separate hook, component, and
owner-handle implementations.

The browser harness loads control, hook, component, and handle modes in one
Chrome process. A pre-boot observer records the authored title, description,
viewport metadata, node identities, and every managed mutation before WASM
starts.

Temporary copies of `examples/server-backed` project each candidate onto route
metadata and the saved-editor override. Those copies are measurement artifacts;
none is part of this branch.

## Current Application-Local Pattern

The control keeps the current application responsibilities:

- string owner keys;
- coordinator and snapshot callback bindings;
- one publication effect and one unmount callback;
- selected-snapshot state;
- one central browser committer;
- explicit route and nested-editor owner wrappers.

This pattern implements the required behavior, but ownership identity and
coordination remain application code.

## Fixed Ownership Contract

The candidates use the same contract:

1. The authored title and description are captured before the first owner.
2. A newly committed owner receives the highest priority.
3. Updating an active owner changes its pair without changing priority.
4. Releasing the selected owner reveals the latest remaining owner.
5. Releasing a non-selected owner does not change the selected pair.
6. Releasing the final owner restores the authored baseline.
7. Failed speculative renders publish no owner.
8. Title and description are committed as one selected pair.
9. Identical selected values cause no document write.
10. Existing title, description, and unrelated head-node identities remain
    stable.

Owner activation is lifecycle-committed. None of the candidates mutates the
document during render.

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

## Candidate A — Implicit Hook

The proposed shape is:

```go
gf.UseDocumentMetadata(gf.DocumentMetadata{
	Title:       "...",
	Description: "...",
})
```

The private prototype allocates one opaque owner per hook slot. Its effect
publishes after commit, updates retain priority, and its unmount callback
releases the owner. Two calls in one component produce two distinct owners.

The hook has the smallest candidate-specific TinyGo result and the shortest
ordinary route call site. Ownership is not visible in the returned node tree,
however. Conditional ownership requires an additional component boundary or an
unconditional hook call with separately defined inactive semantics. Moving a
call across a condition can change owner-slot identity without a visible tree
change.

## Candidate B — Non-DOM Component

The proposed shape is:

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

The exact props spelling remains open. One mounted component instance owns one
metadata pair. The component returns its children as a fragment and emits no DOM
element. Prop updates retain owner identity and priority. Conditional component
mounting expresses activation and unmount expresses release.

This form makes the ownership boundary visible in GOX and avoids positional-hook
conditions in application components. Its main surprise is that a component
with children owns browser state but adds no layout or DOM node. Documentation
and naming must state that behavior directly.

## Candidate C — Explicit Owner Handle

The proposed shape is:

```go
owner := gf.UseDocumentMetadataOwner()
gf.UseOwnedDocumentMetadata(owner, gf.DocumentMetadata{
	Title:       "...",
	Description: "...",
})
```

The private prototype keeps a stable opaque handle and a separate publication
slot. Passing the handle through a helper preserves owner identity. Identical
duplicate publications coalesce, while conflicting simultaneous publications
are rejected.

The handle supports helper composition explicitly, but requires two calls and
two lifecycle state values for ordinary ownership. Its misuse surface includes
escaping a handle, reusing it across unrelated component lifetimes, and
confusing handle creation with active publication.

## Behavioral Evidence

All candidates pass the same host and browser scenarios:

| Scenario | Control | Hook | Component | Handle |
|---|---:|---:|---:|---:|
| route activation | pass | pass | pass | pass |
| nested activation | pass | pass | pass | pass |
| parent update beneath nested owner | pass | pass | pass | pass |
| selected release reveals updated parent | pass | pass | pass | pass |
| non-selected release | pass | pass | pass | pass |
| identical selected update | pass | pass | pass | pass |
| failed speculative render | pass | pass | pass | pass |
| final release restores baseline | pass | pass | pass | pass |
| scope remount | pass | pass | pass | pass |

Two focused browser runs produced identical normalized transition, candidate,
and mutation evidence. Every mode reported:

- zero invalid title/description pairs;
- zero duplicate descriptions;
- zero speculative metadata appearances;
- zero unrelated metadata mutations;
- zero title-node replacements;
- zero description-node replacements;
- one authored-baseline restoration;
- one captured speculative render failure.

Candidate-specific evidence also showed:

- hook: two calls in one component had distinct owner identities and added no
  DOM element;
- component: conditional mount and release matched owner lifetime and added no
  wrapper element;
- handle: forwarding retained one owner, and one identical duplicate
  publication coalesced.

## Call-Site Measurements

The common hidden support is one coordinator, one selected-snapshot state, one
context binding, and one central committer. Candidate numbers below count
ownership-specific application shape, not browser evidence.

| Measure | Control | Hook | Component | Handle |
|---|---:|---:|---:|---:|
| metadata arguments per owner | 2 fields plus key | 1 value | 1 value | handle plus 1 value |
| application-visible owner keys | 2 | 0 | 0 | 0 |
| owner state slots | 0 | 1 | 1 | 2 |
| owner publication effects | 1 | 1 | 1 | 1 |
| owner unmount callbacks | 1 | 1 | 1 | 1 |
| owner helper types | 1 adapter | 0 | 1 props type | 2 |
| owner helper functions | 1 | 1 | 1 | 3 |
| coordinator methods visible to caller | 2 | 0 | 0 | 0 |
| explicit caller cleanup callbacks | 1 | 0 | 0 | 0 |
| positional-hook restriction at caller | yes | yes | no | yes |

The temporary server-backed projections contained six ownership sites: five
route owners and one saved-editor override.

| Measure | Current control | Hook | Component | Handle |
|---|---:|---:|---:|---:|
| ownership expression lines | 18 | 8 | 14 | 13 |
| route-owner lines | 14 | 5 | 11 | 10 |
| nested-override lines | 4 | 3 | 3 | 3 |
| owner-key props | 6 | 0 | 0 | 0 |
| candidate-specific route helper needed | no | no | no | no |
| conditional editor helper needed | existing wrapper | yes | no | yes |

The central committer and selected-snapshot plumbing remain common hidden costs;
they are not evidence for one candidate over another.

## Misuse Analysis

| Risk | Hook | Component | Handle |
|---|---|---|---|
| accidental conditional use | positional slot can change | ordinary conditional mount is valid | either handle or publication call can become conditional |
| duplicate owner creation | one owner per accidental extra hook call | one owner per extra mounted component | extra handle creates an owner; repeated publication needs validation |
| stale cleanup | hidden by hook implementation | tied to component unmount | split handle/publication lifetimes require checks |
| token escape or reuse | no token exposed | no token exposed | handle can escape or be reused |
| invisible ownership | high | ownership appears in tree | handle calls are visible but not in tree |
| non-DOM surprise | none | component adds no DOM node | none |
| repeated publication | another hook call means another owner | another component means another owner | same handle needs duplicate/conflict rules |
| missing root binding | runtime diagnostic required | runtime diagnostic required | runtime diagnostic required |
| multiple mounted apps | binding must remain mount-local | binding must remain mount-local | handle must not cross mounts |
| non-browser use | should be an inert lifecycle contract or diagnosed | same | same |
| testing ergonomics | render a hook owner | mount a visible ownership boundary | create and publish a handle |

Concrete invalid forms include calling the hook only inside `if open`, mounting
two metadata components for one logical owner, and retaining a handle after its
publishing component unmounts. A future implementation must diagnose missing
root bindings and reject a handle from another mounted application.

The decision rubric scores 0 as weakest and 3 as strongest:

| Category | Hook | Component | Handle |
|---|---:|---:|---:|
| semantic fit | 3 | 3 | 3 |
| call-site clarity | 2 | 3 | 2 |
| nested-owner ergonomics | 2 | 3 | 2 |
| update ergonomics | 3 | 3 | 3 |
| lifecycle safety | 2 | 3 | 2 |
| positional-hook burden | 1 | 3 | 2 |
| GOX ergonomics | 2 | 3 | 1 |
| helper composition | 3 | 2 | 3 |
| testability | 2 | 3 | 2 |
| misuse resistance | 2 | 2 | 1 |
| implementation narrowness | 3 | 3 | 2 |
| DCE feasibility | 3 | 3 | 3 |
| TinyGo cost | 3 | 2 | 1 |
| integrated projection | 2 | 3 | 1 |
| **Total** | **33** | **39** | **28** |

The score does not override hard rejection conditions. No candidate triggered a
hard rejection; the component lead is supported by the integrated projection,
not aesthetics alone.

## TinyGo Size Evidence

Measurements use Go 1.22.12, TinyGo 0.41.1, Linux amd64, `gzip -n -9`,
Brotli quality 11, and Zstandard level 19. Each candidate-only copy retains the
same coordinator, adapter, and scenario UI, with only one ownership candidate
reachable.

| Fixture | Ownership support lines | Raw | gzip | Brotli | Zstandard | Raw delta from control |
|---|---:|---:|---:|---:|---:|---:|
| control only | 22 | 302,625 B | 129,291 B | 105,829 B | 112,125 B | 0 B |
| hook only | 21 | 307,577 B | 130,542 B | 106,997 B | 112,916 B | +4,952 B |
| component only | 36 | 308,669 B | 130,780 B | 107,106 B | 113,326 B | +6,044 B |
| handle only | 90 | 311,210 B | 131,896 B | 107,898 B | 114,266 B | +8,585 B |
| combined comparison fixture | not applicable | 331,409 B | 139,181 B | 112,401 B | 119,208 B | not comparable |

The component costs 1,092 raw bytes more than the hook in this fixture. This
difference does not outweigh its explicit lifecycle boundary and simpler
conditional ownership in the integrated projection.

Ownership support lines count each candidate's lifecycle wrapper and private
owner/publication types. They exclude the shared coordinator, adapter, scenario,
browser evidence, and dispatch used only to host all candidates in one fixture.

## Integrated Server-Backed Projection

Every temporary projection:

- generated successfully;
- passed `examples/server-backed/...` pure tests;
- completed a Go/WASM build;
- produced a standard-Go standalone package;
- retained route metadata, nested saved-editor ownership, metadata updates
  beneath the editor, validation, pending and confirmed states, editor release,
  and route-scope unmount code paths.

| Candidate | Added | Removed | Total changed lines | Result |
|---|---:|---:|---:|---|
| hook | 29 | 44 | 73 | compile, tests, build, package pass |
| component | 21 | 27 | 48 | compile, tests, build, package pass |
| handle | 53 | 44 | 97 | compile, tests, build, package pass |

The selected component projection also passed the existing focused
server-backed browser harness. It retained 11 ordinary greeting requests, 10
transition requests, 4 saved GETs, 4 saved POSTs, 43 title mutation batches, 43
description mutation batches, 139 document snapshots, one baseline restoration,
and five saved-editor activations and releases. Invalid pairs, stale metadata,
duplicate descriptions, node replacements, false confirmations, and ownership
mismatches remained zero.

The evidence-label adapter in the temporary projection derived the existing
`route` and `saved-editor` labels from owner depth. Those labels were not owner
identity and were not supplied at component call sites.

## Decision

The component candidate is selected for a possible later implementation stage.
It satisfies the fixed contract, makes owner lifetime visible in GOX, expresses
conditional overrides through ordinary component mounting, removes string owner
keys and coordinator methods from call sites, and creates the smallest
server-backed projection.

The hook is smaller, but its invisible ownership and positional conditional-use
burden are material for route and editor composition. The handle preserves
explicit identity through helpers, but its extra ceremony and token-lifetime
rules are not justified by the current evidence.

Selection here means only that the component form is the implementation
candidate. No production API, compatibility promise, or implementation schedule
is established.

## Rejected Alternatives

- The explicit string-owner control remains application-local and exposes the
  machinery this design intends to hide.
- Independent title and description APIs can create mixed document pairs and
  are outside the fixed contract.
- Render-time setters or direct DOM writes violate lifecycle commit semantics.
- A public coordinator, global registry, router integration, or general head
  manager broadens the current evidence boundary.
- The hook and handle remain documented comparison candidates, not alternate
  public spellings to ship together.

## Proposed Stability Tier

If implemented later, the candidate belongs in **Experimental Frontier**.

The exact exported name, props shape, root binding, diagnostics, and non-browser
behavior require implementation evidence. The candidate is not stable and is
not classified as Public-Candidate by this record.

## Implementation Boundary

A later implementation would need a private mount-local coordinator and browser
adapter, stable opaque owner identity per component instance, lifecycle-committed
publication, release on unmount, selected-pair state, baseline capture, and one
document committer.

The application-facing component must not accept a coordinator, callback,
browser adapter, string owner key, owner index, or global registry. Generated
DOM must contain only its children.

## Remaining Limits

The evidence covers one browser document, one authored title, one authored
description element, nested component owners, route ownership, editor
overrides, failed-render isolation, and baseline restoration.

It does not establish:

- SSR, hydration, or server metadata behavior;
- multiple browser documents or portals;
- arbitrary head-element ownership;
- concurrent independently mounted apps targeting one document;
- interaction with authored scripts that mutate the same nodes;
- a stable diagnostic or testing API;
- production SEO behavior.

Those limits must not be inferred from the private prototype.
