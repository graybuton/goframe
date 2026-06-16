#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT_DIR"

canonical_module="github.com/graybuton/goframe"
legacy_module="github.com/jin-wu"
legacy_module="$legacy_module/goframe"

if ! grep -qx "module $canonical_module" go.mod; then
	echo "go.mod must declare: module $canonical_module"
	exit 1
fi

legacy_matches="$(git grep -n "$legacy_module" -- . || true)"
if [[ -n "$legacy_matches" ]]; then
	echo "legacy module path references found:"
	echo "$legacy_matches"
	exit 1
fi

if ! grep -q "go install $canonical_module/cmd/goxc@latest" README.md; then
	echo "README.md must document the canonical goxc install command"
	exit 1
fi

echo "module path check: ok"
