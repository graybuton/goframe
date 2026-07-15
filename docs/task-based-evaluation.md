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
- use only the supplied participant brief and product-material bundle, plus
  ordinary external Go documentation;
- do not browse or request other GoFrame repository paths during the timed
  task;
- record any omitted document or implementation source that appears necessary
  as friction rather than expanding the supplied materials;
- do not copy an existing example wholesale.

### Study Materials

Use only this participant brief and the closed product-material bundle supplied
by the facilitator. The facilitator records its exact revision, allowlist, and
delivery method before the session; you do not need to determine those values
yourself.

Do not switch to another branch, tag, commit, archive, or repository snapshot during the session.

These are the only GoFrame product documentation files supplied for the core
session:

- `docs/gox-language.md`;
- `docs/router.md`;
- `docs/resources.md`;
- `docs/deployment.md`.

The supplied examples are filtered source references, not runnable repository
copies:

- `examples/counter` supplies authored app source, static assets, and its
  TinyGo manifest;
- `examples/multipackage` supplies authored app and internal-package source,
  static assets, and its TinyGo manifest;
- `examples/router` supplies authored routed-app and internal-package source,
  static assets, and its TinyGo manifest;
- `examples/server-backed` supplies browser-app source, plain Go backend
  source, and static assets only. Its standard-Go manifest is not supplied.

Example README files are not supplied, and repository-root commands from the
original examples are not participant instructions. Create your own
`goframe.json` for the participant application. The exact CLI, module,
manifest, and TinyGo requirements in this brief are authoritative.

Do not browse or request other repository paths during the timed task. If an
omitted document appears necessary, record that need as friction rather than
silently expanding the material set. Ordinary external Go documentation
remains allowed.

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

The version command must print:

```text
goxc version v0.2.0-preview.6
```

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
- any non-allowlisted document or implementation source you believed was
  necessary, with the reason;
- one action you expected GoFrame to provide but could not find.

Do not include names, email addresses, employer information, repository
credentials, tokens, or other personal identifying information.
