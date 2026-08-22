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

Package and export success require a complete validated declared graph. Package
staging is validated before publication. Export validates the current source
before touching its destination, validates a private staged copy, and verifies
the published result before printing success.

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

Browser asset extensions are matched case-insensitively for ASCII letters
only. ASCII variants such as `.WASM`, `.JS`, `.CSS`, and `.HTML` retain their
WASM, JavaScript, CSS, and HTML semantics. Unicode lookalikes such as `.waſm`,
`.jſ`, and `.csſ` do not receive those browser roles; otherwise valid files
remain ordinary assets.

Before packaging, goxc builds an asset namespace plan. Manifest assets cannot
collide with generated names such as `bundle.wasm`, `wasm_exec.js`, generated
metadata, or `.gz`/`.br` sidecars. Duplicate assets after path normalization
are rejected before publication.

The recommended preview manifest uses an asset directory:

```json
{
  "entry": "./cmd/app",
  "compiler": "tinygo",
  "assets": "./assets"
}
```

Files inside that directory are packaged relative to the directory root:
`assets/styles.css` becomes package `assets/styles.css`, and
`assets/data/issues.txt` becomes package `assets/data/issues.txt`. If
`assets/index.html` exists, it is the custom standalone HTML template and is
published as package root `index.html`. It is not copied to
`assets/index.html`.

If no custom HTML template is selected, `goxc package` generates a default
root `index.html` with a `root` mount element, final runtime/WASM paths,
optional preload hints, and stylesheet links for packaged CSS assets. Legacy
explicit lists such as `"assets": ["index.html", "styles.css"]` remain
supported. In that mode, listed `index.html` is a custom template and must
exist as a regular non-symlink file; if it is omitted, generated HTML is used.
Missing non-index listed assets remain optional in the current public-preview
contract: packaging prints a message and skips them.

## HTML Rewriting

Custom `index.html` files may use explicit package blocks:

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

These exact, case-sensitive comments are build-time ownership delimiters.
Packaging validates all three block types before changing the document, then
replaces only the complete managed blocks with final package references. The
comments remain in packaged and development HTML and do not participate in
application rendering. Duplicate, orphaned, reversed, nested, or interleaved
managed markers fail packaging before publication. With `--preload` disabled,
a valid preload block retains its delimiters with an empty interior.

Markerless compatibility is structural rather than textual. It rewrites only:

- the URL value of an executable `<script src>` that names `wasm_exec.js` or
  `./wasm_exec.js`;
- a static single- or double-quoted `bundle.wasm` or `main.wasm` URL used as
  the first argument of a direct `fetch(...)` call in executable inline
  JavaScript;
- the URL value of a stylesheet or style-preload `<link>` that names a
  declared packaged stylesheet.

Those forms retain a query string or fragment. Absolute, protocol-relative,
root-relative, data, blob, dynamic-loader, and similar-name references are not
rewritten. Comments, text, data attributes, inline JSON, import maps,
speculation rules, templates, style text, JavaScript comments, unrelated
strings, template literals, and regular expressions remain authored bytes. A
custom dynamic loader should use the explicit `goframe:bootstrap` block.

The rewriter scans the original source and changes only validated byte ranges;
it does not parse and serialize the whole document. Whitespace, line endings,
doctype spelling, attribute casing and order, quotes, and formatting outside
managed or recognized reference spans are preserved.

## Preload Hints

`--preload` injects preload hints for the WASM bundle, runtime shim, and CSS
assets:

```html
<link rel="preload" href="assets/bundle.a83f19c4.wasm" as="fetch" type="application/wasm" crossorigin>
<link rel="preload" href="assets/wasm_exec.91b2cc10.js" as="script">
<link rel="preload" href="assets/styles.77a1de20.css" as="style">
```

CSS preload is included only when CSS assets are packaged through the manifest
asset directory or explicit asset list. For markerless custom HTML, preload
markup is inserted immediately before the structural closing `</head>` tag.
Text resembling `</head>` inside comments, scripts, styles, attributes, or
examples is ignored. If no unambiguous closing head exists, packaging fails and
recommends an explicit `goframe:preload` block.

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
  "toolchainVersion": "devel",
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

Local checkout builds write `devel`; tagged module installs write the module
version recorded in Go build information, such as `vX.Y.Z-preview.N`.

`goframe-package.json` is the authoritative current package completion marker.
`goxc` publishes it last and removes it first during destructive package
cleanup. The ownership predicate requires this marker, the companion asset
manifest, and the referenced HTML/WASM/runtime files to be regular files inside
the package root. It intentionally remains usable for cleanup when another
declared asset or sidecar has subsequently been damaged.

Publication integrity is stricter than cleanup ownership. Package and export
validate every declared entrypoint, ordinary asset, compressed sidecar, path,
hash, collision, physical identity, and the completion-marker generation.
They remove a failed published marker and print success only after the complete
graph passes.

## Package Graph Inspection

`goxc inspect` reads and validates the declared graph of an existing current
standalone package. It does not build, repair, or otherwise write package or
workspace state.

```bash
goxc inspect ./examples/todo
goxc inspect ./examples/todo --format=json
goxc inspect ./examples/todo/.goframe/package/standalone
goxc inspect --dir ./dist --format=json
```

An application argument resolves through its current workspace, including
`--workspace` and `GOFRAME_WORKSPACE`, to
`.goframe/package/standalone`. A positional current package root, an exported
package root, or `--dir <package-root>` is inspected directly. Direct roots
must be real non-symlink directories. Only the current
`goframe-package.json` plus `asset-manifest.json` package format is supported;
recognized legacy packages receive migration guidance rather than a report.

The package metadata, asset manifest, and the declared files are the graph's
source of truth. The report includes package-relative metadata, HTML, ordinary
assets, entrypoint edges, and compressed-sidecar edges. It verifies contained
regular files, actual byte counts and full SHA-256 values, declared short
hashes, unique paths, and matching entrypoints before emitting output. WASM,
runtime, and style entrypoints must respectively resolve to `.wasm`, `.js`, and
`.css` assets with the current producer media types `application/wasm`,
`text/javascript`, and `text/css`. When `hashAssets` is true, every ordinary
asset path must exactly match the current content-addressed filename produced
from its logical name and declared hash.

Generated logical asset names are namespace keys. The producer normalizes
native separators with `filepath.ToSlash`, and current-package readers require
each generated key to equal that canonical result. Parent components,
slash-rooted names, dot-only names, remaining backslashes, and non-canonical
spellings such as repeated separators, dot components, or trailing separators
are rejected. Canonical nested names, spaces, graphic Unicode, leading-dot
child names such as `.well-known/config.json`, case differences, and colons
remain valid. A drive-looking key such as `C:logo.svg` is data in this namespace,
not an independent package path.

Generated package paths are a separate entity. They use `/` exclusively, must
be canonical and package-relative, and reject absolute or drive-prefixed forms.
An asset with logical name `C:logo.svg` therefore has the contained package path
`assets/C:logo.svg`. Authored `goframe.json` entries remain filesystem paths and
retain their existing absolute and drive-prefix checks; the drive-looking name
is produced when such a filename is discovered below an asset directory on a
filesystem that permits it. A package containing that filename is not promised
to be portable to Windows or another filesystem that reserves the character.
Inspection schema-version-1 path fields and edge endpoints use the same
slash-only package-path representation.

Schema-version-1 string ordering is ascending lexical order of the raw UTF-8
bytes. This applies to style entrypoints, artifact paths and roles, and each
edge field in `from`, `kind`, `encoding`, `to` order.

Inspection captures the regular non-symlink `goframe-package.json` marker's
filesystem identity, byte length, and full SHA-256 before graph construction.
The complete text or JSON report is buffered, then the marker is revalidated
immediately before stdout. If package or development publication removes,
replaces, or changes the marker during inspection, the command reports
`package changed during inspection; retry` and emits no report. Retrying after
package publication completes is supported. This fence relies on GoFrame's
completion-marker publication protocol; external mutation of arbitrary package
artifacts that bypasses that protocol remains outside the static inspection
guarantee.

JSON output uses schema version 1. This complete minimal package example uses
illustrative sizes and hashes:

```json
{
  "schemaVersion": 1,
  "package": {
    "name": "counter",
    "compiler": "tinygo",
    "toolchainVersion": "devel",
    "hashAssets": false,
    "preload": false
  },
  "entrypoints": {
    "html": "index.html",
    "wasm": "assets/bundle.wasm",
    "runtime": "assets/wasm_exec.js",
    "styles": []
  },
  "artifacts": [
    {"path":"asset-manifest.json","logicalName":"","mediaType":"application/json","bytes":300,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","declaredHash":"","encoding":"","roles":["asset-metadata"]},
    {"path":"assets/bundle.wasm","logicalName":"bundle.wasm","mediaType":"application/wasm","bytes":1000,"sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","declaredHash":"","encoding":"","roles":["asset","wasm-entrypoint"]},
    {"path":"assets/wasm_exec.js","logicalName":"wasm_exec.js","mediaType":"text/javascript","bytes":800,"sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","declaredHash":"","encoding":"","roles":["asset","runtime-entrypoint"]},
    {"path":"goframe-package.json","logicalName":"","mediaType":"application/json","bytes":200,"sha256":"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd","declaredHash":"","encoding":"","roles":["package-metadata"]},
    {"path":"index.html","logicalName":"","mediaType":"text/html; charset=utf-8","bytes":500,"sha256":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","declaredHash":"","encoding":"","roles":["html-entrypoint"]}
  ],
  "edges": [
    {"from":"index.html","to":"assets/wasm_exec.js","kind":"runtime-entrypoint","encoding":""},
    {"from":"index.html","to":"assets/bundle.wasm","kind":"wasm-entrypoint","encoding":""}
  ],
  "summary": {
    "artifactCount": 5,
    "edgeCount": 2,
    "totalBytes": 2800
  }
}
```

Text is the default human-readable format. Both formats use deterministic
ordering and package-relative paths; JSON excludes `generatedAt`, roots,
timestamps, modes, and other location-specific state. Undeclared extra files
are excluded rather than promoted into the graph. Inspection does not parse
custom HTML or CSS, infer Go or GOX source dependencies, or imply asset
splitting, multi-entry WASM, route-lazy delivery, or shared-runtime code
splitting.

Text reports preserve package-controlled values exactly when every rune is
graphic. If a value contains a control, format character, or line/paragraph
separator, the complete value is emitted as a quoted Go string with graphic
characters preserved and non-graphic runes escaped. JSON output continues to
use `encoding/json`; its schema and decoded string values are unchanged by this
human-readable presentation rule.

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
- `goxc inspect <app>` reads and validates the declared current standalone
  package graph without writing;
- `goxc serve <app>` serves `.goframe/package/standalone`;
- `goxc dev <app>` rebuilds and serves the latest completed development package
  from `.goframe/package/standalone`;
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

For one coordinated `goxc generate` request, active generated-source writes and
managed inactive-output removals use one recoverable publication plan. goxc
validates and snapshots the planned destinations and stages every active output
before visible mutation. If a publication operation returns an error, completed
mutations are rolled back to the prior generated-source set; a rollback error is
reported together with the original failure. Successful generated bytes,
filenames, permissions, and CLI output are unchanged. A successful inactive
cleanup still remains committed when the caller subsequently reports the
existing no-active-source result. In single-file generation, unpublished sibling
outputs remain read-only package-allocation verification inputs.

This boundary is not a filesystem-wide atomic transaction. It does not provide
crash recovery, persistent journaling, cross-process locking, or guarantees for
process termination, power loss, hostile concurrent filesystem mutation, or
arbitrary operating-system failure during rollback. It does not apply to
`goxc package`, `goxc export`, or development-generation activation.

`goxc clean <app>` removes `.goframe/work`, `.goframe/build`, and
`.goframe/package`. `goxc clean <app> --generated` also removes `.goframe/gen`
and adjacent legacy `.gox.go` files that retain the goxc generated-file header.
`goxc clean <app> --legacy` helps migrate old workspaces by removing legacy
`build/` and adjacent generated `.gox.go` files. An adjacent output without the
generated-file header is treated as user-owned and stops cleanup before any
requested output is removed. For each planned adjacent file, cleanup retains
its filesystem identity and SHA-256 content fingerprint. It revalidates the
complete adjacent set before removing any member, then repeats containment,
regular-file, header, identity, and fingerprint checks immediately before each
unlink. A planned file that has disappeared remains a no-op. This is not an
atomic filesystem transaction: a concurrent mutation after the final check is
outside the portable guarantee. Legacy `dist/` is removed only if it looks like
a GoFrame export; user directories are skipped instead of silently deleted.

The toolchain rejects symlinked app roots, entry directories, authored source
files, package assets, package/export roots, and symlinked package sources at
safety-sensitive boundaries. Cleanup removes final tool-owned symlinks as
links and rejects intermediate workspace symlinks instead of traversing
external targets. Explicit build/generate/package/export output roots are
also compared against app/package roots using physical path resolution so a
symlink alias cannot point output back into authored source. This is
best-effort protection for static repository trees; hostile concurrent
filesystem mutation is outside the threat model.

During package or export replacement, managed standalone artifacts include
`index.html`, metadata files, generated WASM/runtime assets, compressed
sidecars, and the `assets/` directory. Stale `index.html` from a previous
package is removed before the new package is published, after the old
completion marker has already been invalidated.

The materialized hidden workspace supports `"entry": "."` apps and child entry
packages such as `"./cmd/app"`, `"cmd/app"`, `"./src/app"`, and `"app"` when
they point to package directories inside the app root. GOX discovery remains
app-root-wide so imported internal packages get generated files too.

## Watched Local Development

`goxc dev <app>` combines the existing development package path with a watched
local server. It runs a full package after effective authored inputs change,
keeps the last completed package available through ordinary source or compiler
failures, and serves only the standalone package on `127.0.0.1`. Development
responses use `Cache-Control: no-store`; a manual browser refresh loads the
latest successful package.

This differs from the explicit one-shot workflow:

```bash
goxc package <app> --compiler=tinygo
goxc serve <app> --port=8080
```

`goxc dev` does not enable asset hashing, preload injection, gzip, or Brotli.
Those release-oriented options remain explicit `goxc package` concerns, followed
by `goxc export` when a visible deployment directory is needed. The development
server is loopback-only and is not production hosting.

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

Path/history-mode routing is not implemented. Clean URLs such as `/issues/42`
are outside the current preview router contract; deployments that implement
them independently need a server or CDN fallback that serves `index.html` for
application routes. `goxc serve` remains development-only and does not
configure a production fallback policy.

## Not In The Current Preview Package Contract

The current package contract does not include bundle splitting, asset imports
from Go/GOX, glob asset patterns, route loaders, SSR/hydration, production
server fallback automation. The package surface is limited to explicit app
manifests, generated or custom `index.html`, static assets under package
`assets/`, versioned generated metadata, and exportable static browser/WASM
artifacts.
