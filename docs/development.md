# Local Development Workflow

`goxc dev` packages a browser/WASM application, serves the latest completed
package, and rebuilds it when effective authored inputs change:

```bash
goxc dev <app-directory> [--compiler=go|tinygo] [--port=8080] [--workspace=directory]
```

goxc dev is a local development workflow, not production hosting.

## Start And Serve

The command validates the application and workspace roots, collects an initial
input snapshot, and runs the existing development package path. The HTTP server
starts only after a complete package has been published and verified. It binds
to `127.0.0.1`; pass `--port=0` to select an available local port and read the
actual URL from the command output.

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
served.

If `--compiler` is omitted, every accepted build uses the compiler currently
selected by `goframe.json`. A command-line `--compiler=go` or
`--compiler=tinygo` override remains fixed until the process exits.

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
manifest errors leave the last completed package available. The server remains
on the same listener, the failed snapshot is not retried indefinitely, and a
later authored change starts a new package attempt. Successful recovery replaces
the package through the normal publication path.

This preservation guarantee does not override existing package publication or
filesystem failure semantics. A failure that invalidates package ownership or
completion metadata is reported as such.

The server adds `Cache-Control: no-store` to development responses. Refresh the
browser manually after a successful rebuild to load the latest completed
package.

## Shutdown

`Ctrl-C` stops polling and prevents another build from starting. If a package
attempt is already running, the command waits for that in-process attempt to
return, then shuts down the HTTP server and exits successfully.

## Current Limits

Each accepted change batch performs a full development package. The command
does not provide:

- browser auto-reload or auto-open;
- an error overlay;
- HMR or browser state preservation;
- incremental compilation or an incremental build cache;
- source maps;
- file watching outside the effective app and module inputs;
- remote-network binding, history-routing fallback, or production compression
  negotiation.

Build failures are terminal output only. Release asset hashing, preload hints,
gzip, and Brotli remain explicit `goxc package` concerns.
