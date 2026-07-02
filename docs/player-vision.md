# GoFrame Player/Engine inactive direction note

Player/Engine is inactive. It is not an active roadmap item, shipped
implementation, current preview promise, or required path for GoFrame's
web-first direction.

```text
.gox source
   |
goxc compiler/toolchain
   |
.gfapp bundle
   |
GoFrame Player / Engine
```

This document is kept as historical context for older references. The sketch
above described a possible portable host/runtime direction, but the accepted
project direction is now web-first Go development:

- GOX describes declarative application UI;
- `goframe` provides the browser/WASM application runtime API;
- `goxc` owns compilation and packaging workflows;
- browser/WASM package/export remains the validated delivery surface.

Player/Engine, `.gfapp`, a portable host/runtime, desktop or mobile shells, and
a custom application engine are not implemented and are not part of the current
preview contract.

GoFrame may still use lessons from this separation: authored GOX, runtime APIs,
toolchain packaging, and deployment contracts remain distinct concerns. That
does not make Player/Engine an active product direction, a preview feature, or
a compatibility promise.
