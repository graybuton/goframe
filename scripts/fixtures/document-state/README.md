# Document State Ownership Pressure Fixture

This fixture measures one bounded browser ownership question: whether independent
component owners can coordinate an authored document title and description with
the current GoFrame effect, unmount, router, and Error Boundary APIs. It is
executable evidence, not a public document-head API proposal.

## Baseline

GoFrame exposes committed effects and browser DOM access, but it has no public
document-head ownership abstraction. The existing router example retains its
authored `goframe router` title after route navigation and has no route-driven
description meta element.

A temporary uncommitted save/restore probe used this sequence:

```text
parent User 42
nested Editing User 42
parent update to User 7
nested cleanup
```

The nested cleanup restored the value it captured at mount, producing stale
`User 42` title and description values instead of the latest `User 7` pair.
This demonstrates a failure in that specific independent captured-value
composition; it is not a claim about every possible application design.

## Proven

The host tests and two standard Go/WASM browser runs prove:

- the authored title, description, and unrelated meta element are captured
  before application ownership commits;
- route owners apply deterministic Home, user, search, and not-found pairs;
- same-pattern user updates replace title and description coherently;
- an editor owner overrides its route owner;
- route-owner updates do not change editor priority;
- editor cleanup reveals the latest route state;
- cross-pattern cleanup does not overwrite the new route owner;
- rapid A, B, and C navigation settles on C without exposing A or B metadata;
- Back and Forward align the hash target, rendered target, title, and
  description;
- a speculative owner in a failed render never enters the committed registry
  or document head;
- unmounting the owner scope restores the authored baseline, and remounting
  reapplies the current hash route;
- the authored title and description nodes retain identity;
- exactly one description meta element remains present;
- the unrelated authored meta value remains unchanged;
- every observed title/description mutation delivery contains the expected
  pair.

Title and description remain separate DOM writes. The evidence does not claim
an atomic browser head transaction.

## Ownership Model

The fixture uses one package-local ordered coordinator created by `main` and
passed through component props. Route, editor, and speculative owners are
separate component instances using the same private `useDocumentState` helper.

New owners append to priority order. Updating an existing owner changes its
desired pair without changing priority. Removing the top owner reveals the
latest remaining owner; removing a non-top owner leaves the visible owner
unchanged. Removing the final owner selects the authored baseline.

Owner effects update the pure coordinator after commit and pass its selected
snapshot to the retained application root. A separate committed effect applies
that snapshot after the expected-state markers have been patched. Browser DOM
access remains in the `js && wasm` adapter.

There is no global mutable ownership registry. The fixture publishes one
mutable JavaScript evidence object for browser counters and diagnostics; the
application never reads it as ownership state.

## Measured Coordination

- Fixture application Go/GOX: 704 lines.
- Pure ownership model: 167 lines.
- Ownership model tests: 166 lines.
- Browser adapter: 175 JS/WASM lines plus 37 host-stub lines.
- Browser harness: 948 lines.
- Application-owned state slots: 4 call sites.
- Reducers: 0.
- Effects: 3 call sites, with at most 4 committed effect slots while the editor
  is active.
- `UseUnmount`: 2 call sites, with at most 3 committed registrations while the
  editor is active.
- Owner records: at most 2 committed records (`route` and `editor`).
- Cleanup callbacks: 2 registration sites, with at most 3 mounted callbacks.
- Browser DOM query sites: 3.
- Browser DOM write sites: 2.
- Head elements created: 0.
- Head elements removed: 0.
- Global mutable ownership variables: 0.
- Global mutable evidence variables: 1.
- Pure helper types/functions: 6 types and 9 functions.
- Route/head handoff points: 1 selected-snapshot callback boundary.

The focused browser runs produced identical counters:

| Counter | Run 1 | Run 2 |
|---|---:|---:|
| route metadata commits | 13 | 13 |
| nested owner activations | 2 | 2 |
| nested owner releases | 2 | 2 |
| owner updates | 3 | 3 |
| owner removals | 12 | 12 |
| title mutation batches | 17 | 17 |
| description mutation batches | 17 | 17 |
| head snapshots | 17 | 17 |
| invalid title/description pairs | 0 | 0 |
| duplicate description observations | 0 | 0 |
| stale metadata appearances | 0 | 0 |
| speculative metadata appearances | 0 | 0 |
| baseline restorations | 1 | 1 |
| scope mounts | 2 | 2 |
| scope unmounts | 1 | 1 |
| Error Boundary captures | 1 | 1 |
| application-root identity changes | 0 | 0 |
| unrelated-meta mutations | 0 | 0 |

The fixture was packaged with TinyGo 0.41.1 using Go 1.22.12 on Linux amd64.
Sizes are informational and introduce no threshold:

| Encoding | Bytes |
|---|---:|
| raw | 314205 |
| gzip (`-n -9`) | 133222 |
| Brotli (`-q 11`) | 109017 |
| Zstandard (`-19`) | 115582 |

## Failure Semantics

`SpeculativeDocumentOwner` declares its desired pair through the ordinary
private hook and then panics during the same render. The nearest
`gf.ErrorBoundary` displays fallback UI. The runtime rolls back the failed
render attempt, so the owner's effect does not commit, no owner record is
created, and the previously committed route state remains active.

Removing the failed subtree leaves the route metadata unchanged. Standard
Go/WASM supplies the browser panic-containment evidence. TinyGo compilation is
covered, but TinyGo trap-mode panic containment is not claimed.

## Remaining Limitations

The evidence covers one document, title, description, nested owners,
same-pattern route updates, cross-pattern route changes, Back, Forward, failed
render isolation, scope teardown, and authored baseline restoration.

It does not cover SSR, hydration, arbitrary head elements, canonical links,
Open Graph or other social metadata, JSON-LD, scripts, styles, multiple browser
documents, coordination with external head mutations, or broad browser
compatibility. It does not establish production SEO support.

## Decision

**Result B:** Correct nested and out-of-order ownership requires one small
shared coordinator in this demonstrated fixture. The repeated contract is
ordered ownership with priority-preserving updates and current-state reveal on
release.

No public document-state API is selected in this branch. The evidence records a
possible narrow ownership problem for later comparison; it does not select an
arbitrary head manager, router integration, SSR contract, or metadata framework.
