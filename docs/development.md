# Local Development Workflow

`goxc dev` packages a browser/WASM application, serves the latest completed
development generation, rebuilds it when effective authored inputs change, and
reloads connected browser pages after successful rebuilds:

```bash
goxc dev <app-directory> [--compiler=go|tinygo] [--port=8080] [--workspace=directory]
```

goxc dev is a local development workflow, not production hosting.

## Start And Serve

The command validates the application and workspace roots, collects an initial
input snapshot, and runs the existing development package path. After verifying
the canonical package, it copies and verifies a process-private completed
generation. The HTTP server starts only after that generation is active. It
binds to `127.0.0.1`; pass `--port=0` to select an available local port and read
the actual URL from the command output.

An initial authored-source, manifest, generation, or compilation failure is
reported in the terminal without ending the process. The command continues
watching and starts the server after a later correction produces the first
complete package. Invalid command options, unsafe app or workspace roots, and a
listener bind failure remain command errors.

Development output uses the normal standalone package location:

```text
<app>/.goframe/package/standalone
```

With `--workspace`, the package is placed in the app-specific external
workspace defined by the existing workspace contract. The source tree is not
served. The development server also does not serve the mutable canonical
package directly. Each request uses one verified generation for its full
response, and retired generations remain until their active requests finish.

If `--compiler` is omitted, every accepted build uses the compiler currently
selected by `goframe.json`. A command-line `--compiler=go` or
`--compiler=tinygo` override remains fixed until the process exits.

## Compiler Environment

Each development package attempt and its embed discovery run inside the hidden
generated module with `GOWORK=off` and module mode enabled. `goxc` supplies its
own `GOFLAGS`; ambient `GOFLAGS` values do not configure these commands. This
keeps a parent `go.work`, vendor mode, overlays, build tags, and other caller
flags from changing the generated workspace.

The rest of the process environment remains available, including module proxy,
checksum, private-module authentication, certificate, cache, temporary
directory, and Go toolchain settings. Dependencies must be represented by the
application module's authored `require` and `replace` directives. Dependencies
available only through a parent `go.work` are not supported, and this isolation
does not establish a general multi-module workspace contract.

## Watched Inputs

The development loop observes the effective inputs used by packaging:

- authored `.gox` and `.go` files below the app root;
- `goframe.json`;
- the asset directory or asset files selected by the manifest;
- file creation, modification, deletion, and rename inside a selected asset
  directory;
- the nearest effective `go.mod` and its corresponding `go.sum`.

Snapshots use content fingerprints. Polling and quiet-period debounce are
internal implementation details, not timing compatibility contracts. Accepted
change batches run as serialized full package attempts. A change detected
during a build queues one follow-up attempt, and an unchanged snapshot does not
start another build.

The watcher does not traverse tool-owned, dependency, VCS, or legacy output
directories:

```text
.goframe/**
build/**
dist/**
node_modules/**
.git/**
.goxc-tmp/**
```

Adjacent generated `*.gox.go` files are also ignored. Watched symlinks are not
followed. An unsafe or unreadable watched input is reported once while it
remains unchanged; scanning reports recovery after the path becomes valid
again. A malformed manifest keeps the last resolved asset set under observation
until the manifest can be parsed and the effective assets recomputed.

## Failures And Recovery

After a successful package, ordinary GOX diagnostics, Go compiler errors, and
manifest errors leave the last completed generation available. The server
remains on the same listener, the failed snapshot is not retried indefinitely,
and a later authored change starts a new package attempt. Successful recovery
publishes and verifies the canonical package, activates a new completed
generation, and reloads connected pages.

This preservation guarantee does not override existing package publication or
filesystem failure semantics. A failure that invalidates package ownership,
generation copying, or completion metadata is reported as such and does not
replace the active generation.

## Browser Reload

The server adds `Cache-Control: no-store` to development responses and injects
one same-origin development script into `/` and `/index.html` responses. The
script connects to a development-only Server-Sent Events endpoint. The initial
page connects without reloading itself. Every later successful accepted build
activates one completed generation before publishing one reload event, and the
browser performs an ordinary full-document reload.

Failed builds publish no event, so the current page and last completed
generation remain available. A successful recovery activates a new generation
and reloads connected pages. A page reconnecting with the current generation
waits for a later build; a page reconnecting with an older generation receives
one catch-up reload.

Reload injection changes only development HTTP responses. It does not modify
the canonical `index.html`, asset manifest, package metadata, exported packages,
or ordinary `goxc serve` behavior. A strict Content Security Policy that blocks
the injected same-origin script is a current development limitation; the
server does not rewrite CSP headers.

## Shutdown

`Ctrl-C` stops polling and prevents another build from starting. If a package
attempt is already running, the command waits for that in-process attempt to
return, then closes reload connections, shuts down the HTTP server, removes its
process-private generation root, and exits successfully.

## Current Limits

Each accepted change batch performs a full development package. The command
does not provide:

- browser auto-open;
- an error overlay;
- HMR or browser state preservation; a full-page reload resets browser state;
- incremental compilation or an incremental build cache;
- source maps;
- file watching outside the effective app and module inputs;
- remote-network binding, history-routing fallback, or production compression
  negotiation.

Build failures are terminal output only. Release asset hashing, preload hints,
gzip, and Brotli remain explicit `goxc package` concerns.
