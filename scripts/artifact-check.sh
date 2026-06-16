#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT_DIR"

pattern='(^|/)(dist|build)/|\.wasm(\.gz|\.br|\.zst)?$|node_modules|\.vsix$|\.test$'
tracked="$(git ls-files | grep -E "$pattern" || true)"

if [[ -n "$tracked" ]]; then
	echo "tracked build artifacts found:"
	echo "$tracked"
	exit 1
fi

echo "artifact check: ok"
