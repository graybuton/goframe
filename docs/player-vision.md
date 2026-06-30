# GoFrame Player vision

Player/Engine is a long-term architectural direction, not a current
implementation or first-preview promise:

```text
.gox source
   |
goxc compiler/toolchain
   |
.gfapp bundle
   |
GoFrame Player / Engine
```

The current separation keeps that project direction visible:

- GOX describes declarative application UI;
- `goframe` provides the application runtime API;
- `goxc` owns compilation and packaging workflows;
- Player/Engine remains outside the current preview contract.

A `.gfapp` bundle is not implemented in the current preview. Open design
questions include application code, assets, permissions, runtime requirements,
platform API versions, sandboxing, updates, native capabilities, rendering,
debugging, bundle signing, and portability.

The project should first validate the browser runtime, manifest, toolchain, and
application model. Keeping Player/Engine visible as project vision does not mean
the first public preview promises a custom browser, desktop shell, `.gfapp`
runtime, or player implementation.
