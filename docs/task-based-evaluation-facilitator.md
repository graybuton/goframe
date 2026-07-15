# GoFrame Task-Based Evaluation Facilitator Protocol

This protocol accompanies the participant-facing
[GoFrame Task-Based Evaluation](task-based-evaluation.md). It standardizes
session setup, observation, and raw-result capture while keeping implementation
help out of the participant task.

## Study Boundary

This protocol measures onboarding and composition friction in the published
browser/WASM preview. It is not a benchmark of developer ability, and one or
more sessions are not adoption evidence by themselves.

A single session does not authorize a new API or select a product direction.
Raw participant work is not committed to the GoFrame repository by default.
Store or share artifacts only with the participant's agreement and remove
credentials or personal information first.

A maintainer-side mechanical pilot may verify that instructions and commands
are feasible. It must be labeled as a wording and feasibility check, not as an
independent participant session or external evidence.

## Recommended Sample

Run the core task with **2–4 independent Go developers** who are not current
GoFrame maintainers or contributors. Prefer participants with ordinary Go
module and package experience; prior TinyGo, WASM, GOX, or JavaScript-framework
experience is not required.

A single session may reveal a deterministic defect. Do not select the next
product direction from one person's preference, architectural suggestion, or
workflow alone.

## Facilitator Rules

The facilitator must:

- provide only the participant brief as the task specification;
- avoid implementation hints, sample code, and solution-oriented links beyond
  the public sources named by the brief;
- answer only questions that clarify task wording or session procedure;
- record every intervention, including environment help;
- never silently fix the participant's machine or working tree;
- let a participant remain blocked long enough to expose and describe the
  problem;
- distinguish a product or documentation failure from a missing machine
  prerequisite;
- avoid leading questions while the task is in progress;
- preserve exact failed commands and errors where practical;
- remind the participant that they may stop at any time.

If safety, credentials, or accidental destructive work are at risk, intervene
immediately and record why. Safety intervention time should not be interpreted
as product-task time.

## Study Material Revisions

Before recruiting or running a study series, record:

```text
Study series ID:
Study-kit revision:
Participant-brief revision:
Published CLI/module version:
Published CLI/module tag target:
Documentation/examples revision:
Documentation delivery method:
```

Use exact Git SHA or tag values. The study-kit revision is the selected commit
that contains `docs/task-based-evaluation.md`,
`docs/task-based-evaluation-facilitator.md`, and `docs/evaluator-guide.md`; do
not assume a future merge SHA. The participant-brief revision records the exact
commit and `docs/task-based-evaluation.md` path actually delivered. The
published CLI/module version identifies the release installed through Go and
used by participant applications, while its tag target identifies that
release's exact source commit. The documentation/examples revision identifies
the separate immutable documentation and examples snapshot available during
the task.

For the first published-preview series, use this material contract:

```text
Published CLI/module version:
v0.2.0-preview.6

Published CLI/module tag target:
9548345776e6398cd70e8fc58435dd5dab687c7d

Documentation/examples revision:
3997797c40f764601df9bf6bbec6a070eaaa0ffb
```

The CLI/module tag target is the published source revision installed by Go.
The documentation/examples revision is the later post-publication snapshot
whose evaluator and deployment guidance is aligned to preview.6. The latter is
a descendant of the tag target; the intervening changes are documentation-only,
and product source and examples are unchanged.

Record one documentation delivery method: pinned GitHub permalinks, a detached
local checkout, a read-only archive, or another explicitly described immutable
snapshot. Do not give participants moving default-branch links.

When the documentation/examples revision differs from the published tag target,
record why the revisions differ, how the documentation was verified against
the tested release, whether relevant examples and product behavior are
unchanged or otherwise explicitly compatible, and the exact comparison
evidence used. Participants do not perform this verification.

For the first series, the recorded comparison establishes that
`3997797c40f764601df9bf6bbec6a070eaaa0ffb` descends from
`9548345776e6398cd70e8fc58435dd5dab687c7d`; only documentation paths changed
between them, examples are unchanged, and `docs/evaluator-guide.md` and
`docs/deployment.md` identify preview.6. Preserve this evidence with the study
series record.

Every participant in one study series must use the same study-kit,
participant-brief, and documentation/examples revisions. Do not silently
update documentation or examples during a series. If any study-material
revision changes, start or clearly label a new series or cohort. Preserve that
revision difference during aggregation, and do not compare sessions as
equivalent unless the difference is explicitly recorded and judged irrelevant
to the repeated observation. A deterministic product defect may be reproduced
immediately, but its originating material revision remains part of the
evidence.

The published tag target and documentation/examples revision are intentionally
different in the first-series material contract. That defined split does not
create separate cohorts; changing either recorded revision does.

## Session Setup

Before starting, record:

- study series ID;
- study-kit revision;
- participant-brief revision;
- published CLI/module version;
- published CLI/module tag target;
- documentation/examples revision;
- documentation delivery method;
- an anonymized participant ID;
- broad Go experience band;
- prior WASM or TinyGo experience as `yes` or `no`;
- operating system;
- editor;
- Go, TinyGo, Node.js, browser, and `goxc` versions;
- the clean session-directory path or an anonymized label for it;
- exact session start timestamp.

Record the exact end timestamp when the session stops. Do not collect names,
email addresses, employer names, repository credentials, access tokens, or
other personal identifying information.

Confirm that the participant starts outside the GoFrame repository and that
`goxc version` reports `v0.2.0-preview.6`. If a machine prerequisite is
missing, log it. Do not silently substitute a local checkout or unpublished
binary.

## Stage Timeline

Record the first timestamp at which each stage is demonstrably reached:

| Stage | Timestamp |
|---|---|
| environment ready | |
| first GOX source created | |
| first clean `goxc check` | |
| first browser render | |
| multi-package component working | |
| router/query form working | |
| backend endpoint working | |
| resource UI working | |
| final package running | |
| session ended | |

Use `not reached` rather than estimating an event that did not occur. Derive
time-to-stage values from the timestamps after the session; do not interrupt
the participant to calculate them.

## Friction Log

Create one row for every material blocker, repeated failed attempt, surprising
result, or point where the intended next action was unclear.

| Field | Meaning |
|---|---|
| stage | where it occurred |
| attempted action | what the participant tried |
| command or UI action | exact action when available |
| observed result | error or unexpected behavior |
| source consulted | document, example, or implementation-source path |
| intervention | facilitator help, if any |
| resolution | how it was resolved, or `unresolved` |
| elapsed time | approximate time spent on this friction point |
| classification | environment, docs, CLI, GOX, runtime, router, resource, package, editor, or task wording |

Do not assign a severity score during the session. Classification records the
surface involved, not a root-cause verdict.

## Required Observations

Record the following whether or not the participant completes the task:

- time to first browser render;
- time to first clean diagnostic run;
- number of failed commands;
- number of facilitator interventions;
- every GoFrame implementation-source inspection and why it occurred;
- examples copied or heavily imitated;
- whether generated files caused confusion;
- whether `.goframe` caused confusion;
- manifest confusion;
- package, export, or serve confusion;
- diagnostic clarity, including authored file and location usefulness;
- query-state ownership confusion;
- resource-lifecycle confusion;
- backend or static-serving confusion;
- final raw WASM size when a package exists;
- final completion state.

Prefer observable events and exact outputs over opinion scores. A short
participant explanation may provide context, but it does not replace the
event log.

## Post-Session Questions

Ask these questions after the timed task in a neutral tone:

1. What was the first point where the intended next action was unclear?
2. Which document or example was most useful?
3. Which command behaved differently from your expectation?
4. Which concept required the most inference?
5. What did you try to build that the current surface could not express?
6. Which step would you remove or automate first?
7. Would you be able to repeat the task without repository source inspection?

Keep answers verbatim where practical. Do not reinterpret an answer into a
feature request during the session.

## Optional VS Code Add-On

The GoFrame VS Code extension is not Marketplace-published. Editor diagnostics
are therefore outside the mandatory core session.

A facilitator may run a separate 20-minute add-on only when the extension
development environment is prepared before participant time starts. Record
extension setup time separately from product-task time, and do not combine
CLI-only and VS Code-assisted outcomes without labeling them.

The add-on tests:

- one saved GOX source error;
- inline diagnostics against authored source;
- fixing the error and clearing stale diagnostics;
- VS Code Workspace Trust behavior.

Do not turn the add-on into an extension-installation test unless that is a
separately stated study question.

## Raw Result Template

Copy the following template into a private session record. Leave unavailable
fields as `not reached`, `not observed`, or `not applicable`. Do not commit a
filled participant result to the GoFrame repository by default.

```markdown
# GoFrame Task-Based Evaluation Raw Result

## Session Metadata

- Study series ID:
- Study-kit revision:
- Participant-brief revision:
- Published CLI/module version:
- Published CLI/module tag target:
- Documentation/examples revision:
- Documentation delivery method:
- Participant ID:
- Go experience band:
- Prior WASM/TinyGo experience: yes/no
- Operating system:
- Editor:
- Go version:
- TinyGo version:
- Node.js version:
- Browser and version:
- goxc version:
- Session directory label:
- Core start timestamp:
- Core end timestamp:
- Optional editor add-on used: yes/no
- Editor add-on start/end timestamps:
- Record status: raw / reviewed for factual errors

## Stage Timestamps

| Stage | Timestamp | Elapsed from core start | Evidence |
|---|---|---:|---|
| environment ready | | | |
| first GOX source created | | | |
| first clean goxc check | | | |
| first browser render | | | |
| multi-package component working | | | |
| router/query form working | | | |
| backend endpoint working | | | |
| resource UI working | | | |
| final package running | | | |
| session ended | | | |

## Completion State

- Final state: COMPLETE / PARTIAL / BLOCKED / ABANDONED
- Last completed stage:
- First unresolved blocker:
- Reason session ended:

## Friction Log

| # | Stage | Attempted action | Command or UI action | Observed result | Source consulted | Intervention | Resolution | Elapsed time | Classification |
|---:|---|---|---|---|---|---|---|---:|---|
| 1 | | | | | | | | | |

## Commands Attempted

| # | Command | Exit/result | Failed? | Notes |
|---:|---|---|---|---|
| 1 | | | yes/no | |

- Total failed commands:

## Documents And Examples Consulted

- Revision confirmed against supplied snapshot: yes/no

| Source | Reason consulted | Useful outcome | Copied or heavily imitated? |
|---|---|---|---|
| | | | yes/no |

## Implementation Source Inspections

| Path | Reason public docs/examples were insufficient | Outcome |
|---|---|---|
| none / path | | |

## Facilitator Interventions

| # | Timestamp | Trigger | Exact intervention | Classification | Effect on progress |
|---:|---|---|---|---|---|
| 1 | | | | | |

- Total interventions:

## Final Artifact Status

- New module outside GoFrame repository: yes/no
- Child entry package under cmd/app: yes/no
- Authored GOX outside entry package: yes/no
- Package-qualified internal component visible: yes/no
- goframe.json present: yes/no
- Stable outer shell: yes/no
- Two hash routes: yes/no
- Query parameter read and visible: yes/no
- Controlled form changes query: yes/no
- Explicit not-found/fallback: yes/no
- Plain net/http backend running: yes/no
- Same-origin GET endpoint working: yes/no
- Route-owned UseResource through FetchText: yes/no
- Loading/ready/failed UI observed: yes/no
- Controlled failure recovered: yes/no
- TinyGo standalone package produced: yes/no
- Browser app running from backend: yes/no
- Working directory/archive retained with consent: yes/no

## Diagnostics Outcome

- Deliberate GOX error introduced: yes/no
- Check command:
- Diagnostic report exit/result:
- Authored file identified: yes/no
- Authored line identified: yes/no/unknown location
- Authored column identified: yes/no/unknown location
- Diagnostic message useful for correction: factual observation
- Source fixed: yes/no
- Final clean schemaVersion:
- Final clean ok:
- Final clean diagnostics count:
- Generated/authored source confusion observed:

## Package And Size Outcome

- Package command:
- Package output:
- Compiler metadata:
- toolchainVersion metadata:
- Raw WASM path:
- Raw WASM bytes:
- Package/export/serve confusion observed:
- `.goframe` confusion observed:
- Manifest confusion observed:

## Product-Concept Observations

- Query-state ownership confusion:
- Resource-lifecycle confusion:
- Backend/static-serving confusion:
- Generated-file confusion:
- Unsupported behavior attempted:
- Action expected from GoFrame but not found:

## Post-Session Answers

1. First unclear next action:
2. Most useful document or example:
3. Command that differed from expectation:
4. Concept requiring the most inference:
5. Desired behavior the current surface could not express:
6. Step to remove or automate first:
7. Able to repeat without implementation-source inspection:

## Deterministic Defects

List only behavior reproduced as a correctness defect, or `none observed`.

- (none recorded)

## Recurring Gaps

Leave this section unclassified for a single session. During aggregation,
record links to independently similar observations or `none established`.

- (none recorded)

## Participant-Requested Capabilities

Record requests verbatim where practical. These are requests, not selected
roadmap work.

- (none recorded)

## Facilitator Notes

- Task-wording ambiguity observed:
- Environment-only blockers:
- Product/documentation blockers:
- Evidence files retained and storage location:
- Personal or credential data removed: yes/no
```

## Aggregation Rules

Aggregate recurring evidence only when the study-kit, participant-brief, and
documentation/examples revisions are identical, or when each revision
difference is explicitly labeled and judged irrelevant to the repeated
observation. Do not hide a documentation or example correction made between
sessions inside one unqualified aggregate.

Select an engineering follow-up only after at least one of these conditions is
met:

1. The same material friction appears in at least two independent sessions.
2. One session reveals a deterministic correctness defect that a maintainer
   reproduces.
3. Multiple sessions fail the same task stage for independently similar
   reasons.

Do not select a feature because of:

- one preference;
- one unfamiliar tool;
- one unsupported operating system;
- one participant's architectural suggestion;
- raw completion time alone.

Aggregate events and outcomes before interpretations. Preserve distinctions
between machine prerequisites, task wording, public documentation, product
behavior, and unsupported requests. A maintainer reproduction should be linked
to the originating anonymized observation without exposing participant
identity.

## Decision Mapping

Recurring evidence may suggest the following bounded next stages:

| Repeated evidence | Candidate next stage |
|---|---|
| installation/edit/build/browser loop friction | `feat/v03-dev-loop-evidence` |
| external package/import/GOX composition friction | `feat/v03-external-package-evidence` |
| diagnostics or authored/generated source confusion | `feat/v03-source-mapping-evidence` |
| repeated write/pending/error/retry coordination | `feat/v03-mutation-evidence` |
| packaging/export/deployment friction | narrow `goxc` workflow task |
| no repeated material gap | do not add an abstraction; evaluate another workflow |

This table is a decision aid, not a roadmap commitment. Confirm scope,
reproduction, behavior, migration cost, and size implications in a separate
engineering task before changing public APIs.
