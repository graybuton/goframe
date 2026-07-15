# GoFrame Task-Based Evaluation

This brief describes a task-based evaluation of GoFrame's published
browser/WASM preview. The session evaluates the product and its public
documentation, not the participant. Getting blocked, stopping early, or
finishing only part of the application still produces useful evidence.

This document is the participant brief. A facilitator records the session
separately without supplying implementation answers.

## Participant Profile

The primary participant is:

- comfortable with ordinary Go modules and packages;
- able to read basic HTML;
- not required to know TinyGo, WebAssembly, GOX, or GoFrame;
- not a current GoFrame maintainer or contributor.

Prior JavaScript-framework experience is not required.

## Session Conditions

- Core timebox: **90 minutes**.
- Optional editor add-on: **20 minutes**.
- You may stop at any time.

For the core session:

- work in a clean directory outside the GoFrame repository;
- use the exact published `v0.2.0-preview.6` CLI and module dependency;
- do not use AI or code-generation assistance;
- use the supplied participant brief and immutable product repository snapshot,
  plus ordinary external Go documentation;
- inspect GoFrame implementation source only when the public documentation and
  examples are insufficient, and record the path and reason;
- do not copy an existing example wholesale.

### Study Materials

The facilitator supplies this participant brief separately from one full,
immutable GoFrame repository snapshot at:

```text
3997797c40f764601df9bf6bbec6a070eaaa0ffb
```

You may use the public documentation and examples in that snapshot. Do not
switch to another branch, commit, tag, repository snapshot, or moving default
branch during the timed session.

The exact CLI and module commands in this brief are authoritative. Generic
`@latest` or local-checkout installation commands found elsewhere in the
snapshot must not replace the exact study commands.

If the public documentation and examples are insufficient, you may inspect
implementation source in the snapshot. Record every inspected source path and
why the public material was insufficient. Source inspection is study evidence,
not a protocol violation. This rule also applies to source reached through the
Go module cache. Ordinary external Go documentation remains allowed.

Choose what to consult and record what you use.

## Environment Preparation

Install and verify the exact published CLI:

```bash
go install \
  github.com/graybuton/goframe/cmd/goxc@v0.2.0-preview.6

goxc version
goxc doctor
```

Use the exact CLI and module version stated in this participant brief.
Generic unpinned installation guidance is outside this study.

The participant application must use this module dependency:

```text
github.com/graybuton/goframe v0.2.0-preview.6
```

The first output line must be:

```text
goxc version v0.2.0-preview.6
```

Later lines describing the installed Go and TinyGo environments are expected.
Their exact wording is not part of the version assertion.

If installation, version verification, or a required tool reported by
`goxc doctor` blocks you, record the command and result instead of silently
changing the requested GoFrame version.

## Core Task

Create a new Go module outside the GoFrame repository. Build a small
browser/WASM application using the published `v0.2.0-preview.6` module and
toolchain. The finished result must satisfy the outcomes below, but the task
does not prescribe an implementation sequence.

### Application Layout

The application must contain:

- a child entry package under `cmd/app`;
- at least one authored `.gox` file outside the entry package;
- at least one package-qualified GOX component from an internal package;
- a `goframe.json` manifest;
- static assets sufficient to run the application.

### Browser Application

The browser UI must contain:

- a stable outer shell;
- at least two hash routes;
- one route that reads a query parameter;
- one controlled form that changes query state;
- the current query value rendered in the UI;
- a not-found route or an equivalent explicit fallback.

### Same-Origin Backend Data

Add a plain Go `net/http` backend that:

- serves the packaged browser application;
- exposes one same-origin GET endpoint.

One route must own a `gf.UseResource` load through `gf.FetchText`. Its UI must
show explicit loading, ready, and failed states. Provide a controlled backend
failure and demonstrate recovery from that failure.

The task does not require route loaders, actions, caching, SSR, or GoFrame
server APIs.

### Diagnostics

1. Introduce one deliberate GOX source error.
2. Run:

   ```text
   goxc check <application> --format=json
   ```

3. Record whether the diagnostic identifies the authored file and location.
4. Fix the source error.
5. Run the same check and obtain a clean report.

Do not rely on a generated-file diagnostic as the intended result. Record the
actual JSON report or its relevant fields.

### Packaging

Produce and run a TinyGo standalone package for the application. The final
outcome must include:

- a successful standalone package;
- a browser-visible application;
- working hash routing and query-driven form behavior;
- working same-origin backend data;
- package metadata whose `toolchainVersion` identifies
  `v0.2.0-preview.6`.

Use the published packaging and deployment documentation to determine the
commands and output locations. The Go backend may serve the package directly;
`goxc serve` is not required and is not a production-server substitute.

## Completion Check

Before ending, record which outcomes are observable:

- the app renders in a browser;
- an internal package's GOX component is visible;
- both hash routes and the fallback are reachable;
- the controlled form changes the query and the UI displays it;
- the route-owned resource visibly loads, succeeds, fails, and recovers;
- the clean schema-v1 check reports no diagnostics;
- the TinyGo standalone package runs behind the Go backend;
- package metadata reports the tagged toolchain version.

## Completion States

Choose exactly one final state:

- `COMPLETE`: every required outcome was demonstrated.
- `PARTIAL`: at least one application stage worked, but one or more required
  outcomes were not completed before the session ended.
- `BLOCKED`: progress stopped at a specific unresolved product,
  documentation, toolchain, or environment obstacle.
- `ABANDONED`: you chose to stop the session without pursuing the remaining
  task.

These states describe the task outcome, not participant ability.

## Participant Handoff

At the end, provide the facilitator with:

- with your consent, a working artifact or archive, excluding credentials and
  other secrets;
- an anonymized artifact or session label for the written study record; your
  raw local absolute path is not recorded;
- commands you remember failing and their output when available;
- the last completed stage;
- the first unresolved blocker;
- documents and examples you consulted;
- every implementation-source path you inspected and why the public
  documentation or examples were insufficient;
- one action you expected GoFrame to provide but could not find.

Do not include names, email addresses, employer information, repository
credentials, tokens, or other personal identifying information.
