# CI and Regression Gates

This repository keeps local scripts and GitHub Actions aligned. The goal is to
make runtime, GOX, WASM size, browser smoke, and VS Code extension regressions
visible before merge.

## Workflows

### Core

`.github/workflows/ci-core.yml` runs on pull requests and pushes to `main`.

It checks:

- tracked artifact gate;
- canonical module path gate;
- `go fmt ./...` plus a clean diff check;
- `go test ./...`;
- `go test -race ./pkg/... ./cmd/...`;
- `go vet ./...`;
- `go test -tags=goframe_debug ./...`;
- GOX golden tests.

### WASM Size

`.github/workflows/ci-wasm-size.yml` runs on pull requests, pushes to `main`,
and manually through `workflow_dispatch`.

It installs Go, TinyGo `0.41.1`, brotli, and zstd. Then it packages the
counter, components, todo, and dashboard examples with TinyGo. It also runs a
release-style package pass with `--asset-hash --preload --compress=gzip,br`
before checking:

```bash
scripts/size-budget.sh
```

The workflow uploads the printed size report as an artifact.

The size gate measures the packaged WASM entrypoint under
`.goframe/package/standalone/assets/bundle*.wasm`, with a legacy fallback for
older `main.wasm` packages.

### Browser Smoke

`.github/workflows/ci-browser-smoke.yml` runs on pull requests, pushes to
`main`, and manually through `workflow_dispatch`.

It installs Go, TinyGo `0.41.1`, Node.js 20, Chrome, and compression tools,
then runs:

```bash
scripts/browser-smoke.sh
```

The smoke script chooses dynamic ports, verifies the expected app origin before
storage cleanup, checks WASM MIME type, and separates harness failures from app
failures. It currently covers Todo reconciliation, duplicate-key debug
diagnostics, and dashboard-sized filtering/sorting/selection behavior.

### VS Code Extension

`.github/workflows/ci-vscode.yml` runs on pull requests and pushes to `main`.

It validates extension JSON files, installs dependencies with `npm ci`, and
runs:

```bash
npm run compile
```

## Dependabot

Dependabot version updates are configured in `.github/dependabot.yml`.

The project tracks:

- GitHub Actions updates;
- Go module updates;
- VS Code extension npm dependency updates.

Dependabot runs weekly on Monday in the Europe/Helsinki timezone. Dependabot
PRs are not auto-merged; they must pass CI, including size budgets and browser
smoke when relevant.

Security alerts and security updates should be enabled from the GitHub
repository security settings. Recommended labels are `dependencies`,
`github-actions`, `go`, `vscode`, and `npm`.

## Local Checks

Core local verification:

```bash
go fmt ./...
go test ./...
go test -race ./pkg/... ./cmd/...
go vet ./...
go test -tags=goframe_debug ./...
go test ./pkg/gox -run 'TestGolden|TestErrorGolden'
```

Full local gate, including TinyGo packaging and benchmarks:

```bash
scripts/check.sh
scripts/perf-report.sh
```

Size budget only:

```bash
scripts/size-budget.sh
```

Package delivery details, including content-hashed assets and preload hints,
are documented in `docs/deployment.md`.

Browser smoke:

```bash
scripts/browser-smoke.sh
```

VS Code extension:

```bash
cd extensions/vscode-gox
npm ci
npm run compile
```

## Required Tools

Local checks use:

- Go 1.22 or newer;
- TinyGo 0.41.1 for WASM size and browser smoke gates;
- gzip;
- brotli;
- zstd, optional locally but installed in CI;
- Node.js 20 or newer for browser smoke and extension checks;
- Chrome or Chromium for browser smoke;
- curl for smoke server readiness checks.

Set `CHROME=/path/to/chrome` when Chrome is not available as `google-chrome`.

## Size Budget Failures

Do not loosen budgets by default. First identify whether the growth comes from:

- production runtime imports;
- debug code accidentally compiled into production;
- example-level code;
- TinyGo version changes;
- compression tool changes.

Run:

```bash
scripts/size-budget.sh
scripts/perf-report.sh
```

Update budgets only when the increase is intentional and documented.

## Browser Smoke Failures

Treat smoke failures as either harness failures or app failures.

Harness failures include server bind errors, wrong CDP target, wrong origin,
missing Chrome, unavailable storage, or stale server state. App failures include
DOM identity loss, render counter regressions, localStorage persistence issues,
duplicate key diagnostics failures, dashboard filter/sort regressions, or
listener churn regressions.

The smoke script must not continue against an unknown server or `about:blank`.

## Artifact and Module Gates

`scripts/artifact-check.sh` fails if tracked files include generated bundles,
WASM files, `node_modules`, VSIX packages, or test binaries.

`scripts/module-path-check.sh` enforces:

- `module github.com/graybuton/goframe`;
- no legacy repository path references;
- README documents `go install github.com/graybuton/goframe/cmd/goxc@latest`.
