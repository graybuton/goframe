# GOX Language Support for VS Code

This extension provides the first editor experience for GoFrame `.gox` files.

## Features

- `.gox` language registration and file icons;
- TextMate highlighting for Go code, HTML-like elements, capitalized
  components, attributes, fragments, and embedded Go expressions;
- comment, bracket, surrounding-pair, and indentation configuration;
- snippets for applications, components, state, buttons, cards, and fragments;
- lightweight highlighting for GOX render expressions such as
  `{condition && <Node />}` and `{condition ? <A /> : <B />}`;
- inline GOX source diagnostics backed by `goxc check --format=json` when a
  saved `.gox` file or its workspace is checked;
- terminal-backed command-palette actions for `generate`, `package`, `serve`,
  and `doctor`.

## Commands

- `GOX: Check Current Project` checks the selected workspace folder and updates
  inline source diagnostics
- `GOX: Generate Current Project` runs `goxc generate .`
- `GOX: Package Current Project` runs `goxc package .`
- `GOX: Serve Current Project` runs `goxc serve .`
- `GOX: Run Doctor` runs `goxc doctor`

Project commands use the workspace containing the active editor, falling back
to the first workspace folder. Set `gox.goxcPath` when `goxc` is not available
directly from the integrated terminal's `PATH`.

## Diagnostics

The extension runs `goxc check <workspace-folder> --format=json` without a
shell and consumes the versioned schema-v1 report. This requires a current
`goxc` executable that includes the `check` command. Set `gox.goxcPath` to an
executable path or command name available through `PATH`; the setting is not a
shell command and must not include arguments.

The extension executes the configured `goxc` executable and is disabled in VS
Code Restricted Mode and untrusted workspaces. Automatic saved-file diagnostics
require a trusted workspace.

Saving an authored `.gox` file checks its owning workspace folder. The manual
`GOX: Check Current Project` command checks the workspace containing the active
editor, falling back to the first workspace folder. Multi-root workspaces keep
diagnostics and active checks isolated by folder. A newer check supersedes an
older in-flight check for the same folder, while checks in different folders
can run independently.

Diagnostics use saved source only. The extension does not check every edit or
inspect unsaved buffer content, and `goxc check` does not perform Go or TinyGo
semantic type checking. Operational launch, process, and schema failures leave
the previous diagnostics intact and are recorded in the `GOX Diagnostics`
Output Channel. Compiler diagnostics are normal completed check results.

## Development

Requirements: Node.js 20+, npm, and VS Code.

```bash
cd extensions/vscode-gox
npm install
npm test
```

`pnpm install` and `pnpm run compile` are also supported.

Open this directory in VS Code and press `F5`, or run the
`Run GOX Extension` launch configuration. The Extension Development Host can
open `samples/demo.gox` to inspect highlighting, snippets, and commands.

For continuous TypeScript compilation:

```bash
npm run watch
```

Optional local VSIX packaging:

```bash
npx @vscode/vsce package
code --install-extension gox-0.1.0.vsix
```

Marketplace publishing is intentionally outside the current MVP.

## Snippets

| Prefix | Purpose |
|---|---|
| `goxapp` | Minimal GOX application |
| `component` | Typed function component |
| `state` | tuple-style `gf.UseState` declaration |
| `map` | `gf.Map` list rendering with GOX markup |
| `ifnode` | GOX `condition && <Node />` render expression |
| `ternary` | GOX ternary render expression |
| `button` | Button component call |
| `card` | Card component with children |
| `fragment` | GOX fragment |

## Icons

Language icons live in `icons/gox-light.svg`, `icons/gox-dark.svg`, and
`icons/gox-file.svg`. VS Code uses the light/dark language icons for `.gox`
files; `gox-file.svg` is the neutral source asset for future packaging and icon
theme work.

## Limitations

This MVP does not include an LSP, semantic highlighting, type-aware completion,
formatting, unsaved-buffer diagnostics, Go/TinyGo semantic type checking,
diagnostic watch mode, debugger integration, or marketplace publishing.
TextMate highlighting is intentionally heuristic, and the CLI-backed source
diagnostics do not replace the Go compiler.

## License

Apache-2.0. See the repository root LICENSE file.
