# Cache-Safe Package Delivery

MVP 13 makes `goxc package` produce a small, cache-safe web bundle without
turning GoFrame into a production web server.

## Default Package

```bash
goxc package ./examples/todo --compiler=tinygo
```

Default output:

```text
examples/todo/.goframe/package/standalone/
├── index.html
├── asset-manifest.json
├── goframe-package.json
└── assets/
    ├── bundle.wasm
    └── wasm_exec.js
```

The logical WASM bundle name is `bundle.wasm`. Existing manifests that
explicitly set `"wasm": "main.wasm"` still package, but new examples and docs
use `bundle.wasm`.

`goxc package` keeps package output inside the hidden app workspace so the
authored application directory stays clean. Use `goxc export` when you want a
visible deployment directory:

```bash
goxc package ./examples/todo --compiler=tinygo --asset-hash --preload --compress=gzip,br
goxc export ./examples/todo --out ./dist
```

Only the export step creates `./dist`.

If you intentionally pass `goxc package --out <dir>`, that directory is also
treated as package output owned by goxc. It must be empty or already contain a
complete current GoFrame package marker; otherwise package fails before
removing any existing `assets/` directory. Empty `{}` marker placeholders,
malformed metadata, standalone `asset-manifest.json`, symlinked markers, and
generic web `manifest.json` files are not treated as GoFrame ownership. The
recommended visible deployment flow remains `goxc package` followed by
`goxc export`.

The export directory is tool-owned. If `--out` already exists, is non-empty,
and does not contain complete `goframe-package.json` metadata plus matching
regular companion files, or a recognized historical GoFrame `manifest.json`
package signature, `goxc export` fails before touching it:

```bash
goxc export ./examples/todo --out ./dist
# fails if ./dist is a non-empty user directory
```

Use `--force` only when you intentionally want goxc to treat that directory as
package output and overwrite package-owned assets:

```bash
goxc export ./examples/todo --out ./dist --force
```

## Hashed Release Package

```bash
goxc package ./examples/todo --compiler=tinygo --asset-hash --preload --compress=gzip,br
```

With `--asset-hash`, emitted assets include an 8-character SHA-256 content hash
based on the original uncompressed bytes:

```text
examples/todo/.goframe/package/standalone/
├── index.html
├── asset-manifest.json
├── goframe-package.json
└── assets/
    ├── bundle.a83f19c4.wasm
    ├── bundle.a83f19c4.wasm.br
    ├── bundle.a83f19c4.wasm.gz
    ├── wasm_exec.91b2cc10.js
    ├── wasm_exec.91b2cc10.js.br
    └── wasm_exec.91b2cc10.js.gz
```

CSS assets are emitted under `assets/` too, for example
`assets/styles.77a1de20.css`.

Before packaging, goxc builds an asset namespace plan. Manifest assets cannot
collide with generated names such as `bundle.wasm`, `wasm_exec.js`, generated
metadata, or `.gz`/`.br` sidecars. Duplicate assets after path normalization
are rejected before publication.

## HTML Rewriting

Example `index.html` files use explicit package blocks:

```html
<!-- goframe:preload -->
<!-- /goframe:preload -->

<!-- goframe:runtime -->
<script src="wasm_exec.js"></script>
<!-- /goframe:runtime -->

<!-- goframe:bootstrap -->
<script>
  const go = new Go();
  WebAssembly.instantiateStreaming(fetch("bundle.wasm"), go.importObject)
      .then((result) => go.run(result.instance));
</script>
<!-- /goframe:bootstrap -->
```

Packaging rewrites those blocks to the final asset paths. If a legacy HTML file
does not have the markers, packaging falls back to simple `wasm_exec.js` and
`main.wasm`/`bundle.wasm` string rewrites.

## Preload Hints

`--preload` injects preload hints for the WASM bundle, runtime shim, and CSS
assets:

```html
<link rel="preload" href="assets/bundle.a83f19c4.wasm" as="fetch" type="application/wasm" crossorigin>
<link rel="preload" href="assets/wasm_exec.91b2cc10.js" as="script">
<link rel="preload" href="assets/styles.77a1de20.css" as="style">
```

CSS preload is included only when CSS assets are declared in `goframe.json`.

## Asset Manifest

`asset-manifest.json` describes final asset paths:

```json
{
  "version": 1,
  "assets": {
    "bundle.wasm": {
      "path": "assets/bundle.a83f19c4.wasm",
      "hash": "a83f19c4",
      "type": "application/wasm",
      "compressed": {
        "br": "assets/bundle.a83f19c4.wasm.br",
        "gzip": "assets/bundle.a83f19c4.wasm.gz"
      }
    }
  },
  "entrypoints": {
    "wasm": "assets/bundle.a83f19c4.wasm",
    "runtime": "assets/wasm_exec.91b2cc10.js"
  }
}
```

In dev packages, hash fields are omitted.

`asset-manifest.json` is companion metadata only. It is not an ownership or
completion marker without complete `goframe-package.json` metadata.

See [Manifest Compatibility](manifest-compatibility.md) for the input/generated
metadata compatibility policy.

## Package Metadata

`goframe-package.json` records package-level metadata:

```json
{
  "version": 1,
  "name": "todo",
  "compiler": "tinygo",
  "toolchainVersion": "0.1.0",
  "assetsDir": "assets",
  "hashAssets": true,
  "preload": true,
  "entrypoints": {
    "html": "index.html",
    "wasm": "assets/bundle.a83f19c4.wasm",
    "runtime": "assets/wasm_exec.91b2cc10.js"
  },
  "generatedAt": "2026-06-17T00:00:00Z"
}
```

`goframe-package.json` is the authoritative current package completion marker.
`goxc` publishes it last and removes it first during destructive package
cleanup. A directory is considered a complete current GoFrame package only
when this marker, the companion asset manifest, and the referenced
HTML/WASM/runtime files are all regular files inside the package root.

## Clean App Workspace

GoFrame toolchain internals live in an app-local hidden workspace:

```text
<app>/.goframe/
├── gen/
├── work/
├── build/
├── package/
├── cache/
└── logs/
```

Default command outputs:

- `goxc generate <app>` writes generated `.gox.go` files to `.goframe/gen`;
- `goxc build <app>` writes raw WASM to `.goframe/build/<compiler>/dev`;
- `goxc package <app>` writes standalone output to
  `.goframe/package/standalone`;
- `goxc serve <app>` serves `.goframe/package/standalone`;
- `goxc size <app>` reads `.goframe/package/standalone`;
- `goxc export <app> --out <dir>` copies the standalone package to an explicit
  deployment directory.

`GOFRAME_WORKSPACE=/work/goframe` or `--workspace /work/goframe` moves this
workspace outside the source tree. With an external workspace, goxc creates a
safe app-specific subdirectory to avoid collisions between apps. External
workspaces must not overlap the app source tree, including through symlink
aliases.

`goxc generate --in-place` is available only for debugging or legacy workflows.
It writes adjacent `.gox.go` files and prints a warning. Normal source trees
should not commit generated `.gox.go`.

`goxc clean <app>` removes `.goframe/work`, `.goframe/build`, and
`.goframe/package`. `goxc clean <app> --generated` also removes `.goframe/gen`
and adjacent legacy `.gox.go` files. `goxc clean <app> --legacy` helps migrate
old workspaces by removing legacy `build/` and adjacent generated `.gox.go`
files. Legacy `dist/` is removed only if it looks like a GoFrame export; user
directories are skipped instead of silently deleted.

The toolchain rejects symlinked app roots, entry directories, authored source
files, package assets, package/export roots, and symlinked package sources at
safety-sensitive boundaries. Cleanup removes final tool-owned symlinks as
links and rejects intermediate workspace symlinks instead of traversing
external targets. Explicit build/generate/package/export output roots are
also compared against app/package roots using physical path resolution so a
symlink alias cannot point output back into authored source. This is
best-effort protection for static repository trees; hostile concurrent
filesystem mutation is outside the threat model.

The materialized hidden workspace supports `"entry": "."` apps and child entry
packages such as `"./cmd/app"`, `"cmd/app"`, `"./src/app"`, and `"app"` when
they point to package directories inside the app root. GOX discovery remains
app-root-wide so imported internal packages get generated files too.

## Cache Policy

Recommended deployment headers:

- `index.html`: short cache or revalidate;
- `asset-manifest.json`: short cache or revalidate;
- `goframe-package.json`: short cache or revalidate;
- `assets/*.<hash>.*`: `Cache-Control: public, max-age=31536000, immutable`.

When serving precompressed files, configure the web server or CDN to return the
matching `Content-Encoding` for `.gz` and `.br` variants. `goxc serve` is
development-only and does not implement production compression negotiation.

## Hash Router Deployment

The MVP 24 router is hash-based. Routes such as `#/issues/42` are handled by
the browser after `index.html` loads, so static hosting can serve the same
package without server-side route rewrites.

Route query state also lives in the hash, for example
`#/issues?status=open&q=auth`. The server still receives only the original
`index.html` request.

Path/history-mode routing is not implemented. If a future app wants clean URLs
such as `/issues/42`, the server or CDN would need a fallback that serves
`index.html` for application routes. `goxc serve` remains development-only and
does not configure a production fallback policy.

## Not In MVP 13

Bundle splitting is intentionally not part of this stage. It needs an app graph,
route/loading model, loader design, and probably path/history routing,
SSR/hydration, or Player decisions first.
