# Contributing

Thanks for your interest in contributing to goframe.

By contributing to this repository, you agree that your contributions are
licensed under the Apache License, Version 2.0, the same license as the
project.

## Development Checks

Run the local checks before opening a pull request:

```bash
scripts/check.sh
```

Optional browser regression checks:

```bash
scripts/browser-smoke.sh
```

## Branches

Use focused feature branches:

- `feature/runtime/*`
- `feature/gox/*`
- `feature/goxc/*`
- `feature/vscode-extension/*`
- `docs/*`
- `chore/*`

The VS Code extension lives in `extensions/vscode-gox`.
