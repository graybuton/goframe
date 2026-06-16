# GoFrame Player vision

The player is a future architectural direction, not a current implementation:

```text
.gox source
   |
goxc compiler/toolchain
   |
.gfapp bundle
   |
GoFrame Player / Engine
```

The current separation prepares for that possibility:

- GOX describes declarative application UI;
- `goframe` provides the application runtime API;
- `goxc` owns compilation and packaging workflows;
- a future player could host a versioned bundle contract.

A `.gfapp` bundle could eventually describe application code, assets,
permissions, runtime requirements, and platform API versions. Open questions
include sandboxing, updates, native capabilities, rendering, debugging, bundle
signing, and portability.

The project should first validate the browser runtime, manifest, toolchain, and
application model. No custom browser, desktop shell, or player is implemented
in the current MVP.
