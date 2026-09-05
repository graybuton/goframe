#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
source "$ROOT_DIR/scripts/security-analysis.sh"

TEST_DIR="$(mktemp -d "${TMPDIR:-/tmp}/goframe-security-runner-tests.XXXXXX")"
trap 'rm -rf "$TEST_DIR"' EXIT

write_fake_go() {
	local path="$1"
	local mode="$2"
	local expected_pattern="$3"
	local expected_goos="$4"
	local expected_goarch="$5"
	local expected_cgo_enabled="$6"
	cat >"$path" <<EOF
#!/usr/bin/env bash
set -euo pipefail
if [[ "\${1:-}" != list || "\${!#}" != '$expected_pattern' ]]; then
  exit 97
fi
if [[ "\${GOOS:-}" != '$expected_goos' || "\${GOARCH:-}" != '$expected_goarch' || "\${CGO_ENABLED:-}" != '$expected_cgo_enabled' ]]; then
  exit 96
fi
case '$mode' in
  failure)
    exit 1
    ;;
  empty)
    exit 0
    ;;
  host)
    printf 'example.test/project/one\\t%s\\n' '$TEST_DIR/repository/one'
    printf 'example.test/project/two\\t%s\\n' '$TEST_DIR/repository/two'
    ;;
  browser)
    printf 'example.test/project/browser\\t%s\\n' '$TEST_DIR/repository/browser'
    ;;
esac
EOF
	chmod +x "$path"
}

write_fake_gosec() {
	local path="$1"
	local mode="$2"
	local expected_goos="$3"
	local expected_goarch="$4"
	local expected_cgo_enabled="$5"
	cat >"$path" <<EOF
#!/usr/bin/env bash
set -euo pipefail
if [[ "\${GOOS:-}" != '$expected_goos' || "\${GOARCH:-}" != '$expected_goarch' || "\${CGO_ENABLED:-}" != '$expected_cgo_enabled' ]]; then
  exit 96
fi
if [[ '$mode' == failure ]]; then
  exit 1
fi
if [[ '$mode' == no-report ]]; then
  exit 0
fi
report=''
for argument in "\$@"; do
  case "\$argument" in
    -out=*) report="\${argument#-out=}" ;;
  esac
done
if [[ -z "\$report" ]]; then
  exit 95
fi
printf '%s\\n' '{"Golang errors":{},"Issues":[],"Stats":{"files":1,"lines":1,"nosec":0,"found":0},"GosecVersion":"dev"}' >"\$report"
EOF
	chmod +x "$path"
}

mkdir -p \
	"$TEST_DIR/repository/one" \
	"$TEST_DIR/repository/two" \
	"$TEST_DIR/repository/browser"

write_fake_go "$TEST_DIR/go-host" host ./... linux amd64 1
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 enumerate_gosec_packages \
	"$TEST_DIR/repository" \
	"example.test/project" \
	"$TEST_DIR/host.txt" \
	"$TEST_DIR/go-host" \
	./...
if ((${#GOSEC_PACKAGE_DIRS[@]} != 2)); then
	echo 'runner test: host package coverage count mismatch' >&2
	exit 1
fi
host_package_dirs=("${GOSEC_PACKAGE_DIRS[@]}")

write_fake_go "$TEST_DIR/go-browser" browser ./pkg/goframe js wasm 0
GOOS=js GOARCH=wasm CGO_ENABLED=0 enumerate_gosec_packages \
	"$TEST_DIR/repository" \
	"example.test/project" \
	"$TEST_DIR/browser.txt" \
	"$TEST_DIR/go-browser" \
	./pkg/goframe
if ((${#GOSEC_PACKAGE_DIRS[@]} != 1)) || [[ "${GOSEC_PACKAGE_IMPORTS[0]}" != example.test/project/browser ]]; then
	echo 'runner test: browser package scope was not isolated' >&2
	exit 1
fi
browser_package_dirs=("${GOSEC_PACKAGE_DIRS[@]}")
if ((${#host_package_dirs[@]} != 2)) || [[ "${host_package_dirs[0]}" == "${GOSEC_PACKAGE_DIRS[0]}" ]]; then
	echo 'runner test: host package coverage was contaminated by browser coverage' >&2
	exit 1
fi

write_fake_go "$TEST_DIR/go-browser-failure" failure ./pkg/goframe js wasm 0
if GOOS=js GOARCH=wasm CGO_ENABLED=0 enumerate_gosec_packages \
	"$TEST_DIR/repository" \
	"example.test/project" \
	"$TEST_DIR/browser-failure.txt" \
	"$TEST_DIR/go-browser-failure" \
	./pkg/goframe; then
	echo 'runner test: failed browser package enumeration was accepted' >&2
	exit 1
fi

write_fake_go "$TEST_DIR/go-browser-empty" empty ./pkg/goframe js wasm 0
if GOOS=js GOARCH=wasm CGO_ENABLED=0 enumerate_gosec_packages \
	"$TEST_DIR/repository" \
	"example.test/project" \
	"$TEST_DIR/browser-empty.txt" \
	"$TEST_DIR/go-browser-empty" \
	./pkg/goframe; then
	echo 'runner test: empty browser package enumeration was accepted' >&2
	exit 1
fi

write_fake_gosec "$TEST_DIR/gosec-host" success linux amd64 1
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 run_gosec_analysis \
	"$TEST_DIR/gosec-host.json" \
	"$TEST_DIR/gosec-host" \
	"${host_package_dirs[@]}"

write_fake_gosec "$TEST_DIR/gosec-browser" success js wasm 0
GOOS=js GOARCH=wasm CGO_ENABLED=0 run_gosec_analysis \
	"$TEST_DIR/gosec-browser.json" \
	"$TEST_DIR/gosec-browser" \
	"${browser_package_dirs[@]}"

write_fake_gosec "$TEST_DIR/gosec-browser-failure" failure js wasm 0
if GOOS=js GOARCH=wasm CGO_ENABLED=0 run_gosec_analysis \
	"$TEST_DIR/gosec-browser-failure.json" \
	"$TEST_DIR/gosec-browser-failure" \
	"${browser_package_dirs[@]}"; then
	echo 'runner test: failed browser gosec execution was accepted' >&2
	exit 1
fi

write_fake_gosec "$TEST_DIR/gosec-browser-no-report" no-report js wasm 0
if GOOS=js GOARCH=wasm CGO_ENABLED=0 run_gosec_analysis \
	"$TEST_DIR/gosec-browser-no-report.json" \
	"$TEST_DIR/gosec-browser-no-report" \
	"${browser_package_dirs[@]}"; then
	echo 'runner test: missing browser gosec report was accepted' >&2
	exit 1
fi

# Exercise main in a fresh shell so errexit and command-local environments are
# tested at the policy call sites, not just on the individual helpers.
mkdir -p "$TEST_DIR/policy-bin" "$TEST_DIR/repository/windows" "$TEST_DIR/repository/three"
cat >"$TEST_DIR/policy-bin/fake-tool" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
tool="$(basename "$0")"
printf '%s|%s|%s|%s|%s\n' "$tool" "${GOOS:-}" "${GOARCH:-}" "${CGO_ENABLED:-}" "$*" >>"$CALL_LOG"

require_host() {
  [[ "${GOOS:-}/${GOARCH:-}/${CGO_ENABLED:-}" == linux/amd64/1 ]]
}

selected_target() {
  case "${GOOS:-}/${GOARCH:-}/${CGO_ENABLED:-}" in
    linux/amd64/1) printf host ;;
    windows/amd64/0) printf windows ;;
    js/wasm/0) printf browser ;;
    *) echo 'fake tool: unexpected target environment' >&2; exit 96 ;;
  esac
}

if [[ "$tool" == go ]]; then
  case "$1" in
    env)
      case "$2" in
        GOVERSION) echo go1.26.6 ;;
        GOHOSTOS) echo linux ;;
        GOHOSTARCH) echo amd64 ;;
        GOMOD) require_host; echo "$FAKE_ROOT/go.mod" ;;
        *) exit 95 ;;
      esac
      ;;
    version|install)
      require_host
      ;;
    list)
      if [[ "$2" == -m ]]; then
        require_host
        echo github.com/graybuton/goframe
        exit
      fi
      target="$(selected_target)"
      if [[ "$target" == browser ]]; then
        [[ "${!#}" == ./pkg/goframe ]]
        names=(browser)
      else
        [[ "${!#}" == ./... ]]
        names=(one two)
        if [[ "$target" == windows ]]; then
          [[ "$FAIL_MODE" != enumeration-failure ]] || exit 41
          [[ "$FAIL_MODE" != enumeration-empty ]] || exit 0
          names=(windows two three)
        fi
      fi
      for name in "${names[@]}"; do
        printf 'github.com/graybuton/goframe/%s\t%s/%s\n' "$name" "$FAKE_ROOT" "$name"
      done
      ;;
    run)
      require_host
      [[ "$2" == ./.github/scripts/gosec-report.go && "$3" == -report && "$5" == -root && "$6" == "$FAKE_ROOT" && "$7" == -packages ]]
      case "$4" in
        */gosec-host.json) target=host; count=2 ;;
        */gosec-windows.json) target=windows; count=3 ;;
        */gosec-wasm-runtime.json) target=browser; count=1 ;;
        *) exit 94 ;;
      esac
      [[ "$8" == "$count" ]]
      grep -Fq '"target":"'"$target"'"' "$4"
      if [[ "$target" == windows && "$FAIL_MODE" == classifier-failure ]]; then
        echo 'fake Windows report validation failed' >&2
        exit 42
      fi
      echo "classified $target"
      ;;
    *) exit 93 ;;
  esac
  exit
fi

if [[ "$1" == -version ]]; then
  require_host
  exit
fi
target="$(selected_target)"
case "$tool" in
  staticcheck)
    [[ "$1" == '-checks=SA*' ]]
    if [[ "$target" == browser ]]; then
      [[ "$2" == -tests=false && "${!#}" == ./pkg/goframe ]]
    else
      [[ "$*" != *-tests=false* && "${!#}" == ./... ]]
    fi
    if [[ "$target" == windows ]]; then
      [[ "$FAIL_MODE" != staticcheck-failure ]] || exit 43
      if [[ "$*" == *-tags=goframe_debug* && "$FAIL_MODE" == staticcheck-debug-failure ]]; then
        exit 43
      fi
    fi
    ;;
  govulncheck)
    [[ "$1" == -scan=symbol ]]
    if [[ "$target" == browser ]]; then
      [[ "$2" == ./pkg/goframe ]]
    else
      [[ "$2" == ./... ]]
    fi
    [[ "$target/$FAIL_MODE" != windows/vulnerability-failure ]] || exit 44
    ;;
  gosec)
    [[ "$1" == -no-fail && "$2" == -fmt=json && "$3" == -log=* && "$4" == -out=* ]]
    report="${4#-out=}"
    shift 4
    case "$target" in
      host) [[ "$*" == "$FAKE_ROOT/one $FAKE_ROOT/two" && "$report" == */gosec-host.json ]] ;;
      windows) [[ "$*" == "$FAKE_ROOT/windows $FAKE_ROOT/two $FAKE_ROOT/three" && "$report" == */gosec-windows.json ]] ;;
      browser) [[ "$*" == "$FAKE_ROOT/browser" && "$report" == */gosec-wasm-runtime.json ]] ;;
    esac
    if [[ "$target" == windows ]]; then
      case "$FAIL_MODE" in
        analyzer-failure) exit 45 ;;
        missing-report) exit 0 ;;
        empty-report) : >"$report"; exit 0 ;;
      esac
    fi
    printf '{"target":"%s","Golang errors":{},"Issues":[],"Stats":{"files":1,"lines":1,"nosec":0,"found":0},"GosecVersion":"dev"}\n' "$target" >"$report"
    ;;
  *) exit 92 ;;
esac
EOF
chmod +x "$TEST_DIR/policy-bin/fake-tool"
for tool in go staticcheck govulncheck gosec; do
	ln -s fake-tool "$TEST_DIR/policy-bin/$tool"
done

run_policy_control() {
	local mode="$1"
	local status=0
	if PATH="$TEST_DIR/policy-bin:$PATH" \
		GOOS=freebsd GOARCH=arm64 CGO_ENABLED=1 \
		RUNNER_SOURCE="$ROOT_DIR/scripts/security-analysis.sh" \
		FAKE_ROOT="$TEST_DIR/repository" FAIL_MODE="$mode" \
		CALL_LOG="$TEST_DIR/$mode.calls" \
		bash -c 'source "$RUNNER_SOURCE"; ROOT_DIR="$FAKE_ROOT"; main' >"$TEST_DIR/$mode.output" 2>&1; then
		status=0
	else
		status=$?
	fi
	if [[ "$mode" == success ]]; then
		if ((status != 0)); then
			cat "$TEST_DIR/$mode.output" >&2
			echo "runner test: successful target policy failed ($status)" >&2
			exit 1
		fi
	else
		if ((status == 0)) || grep -Fq 'security analysis: ok' "$TEST_DIR/$mode.output"; then
			echo "runner test: Windows $mode did not block the runner" >&2
			exit 1
		fi
	fi
}

require_policy_call() {
	if ! grep -Fxq "$1" "$TEST_DIR/success.calls"; then
		echo "runner test: missing policy call: $1" >&2
		exit 1
	fi
}

run_policy_control success
for target in 'linux|amd64|1' 'windows|amd64|0' 'js|wasm|0'; do
	pattern=./...
	test_flag=''
	if [[ "$target" == 'js|wasm|0' ]]; then
		pattern=./pkg/goframe
		test_flag='-tests=false '
	fi
	require_policy_call "staticcheck|$target|-checks=SA* $test_flag$pattern"
	require_policy_call "staticcheck|$target|-checks=SA* ${test_flag}-tags=goframe_debug $pattern"
	require_policy_call "govulncheck|$target|-scan=symbol $pattern"
	require_policy_call "go|$target|list -buildvcs=false -f {{.ImportPath}}{{\"\t\"}}{{.Dir}} $pattern"
done
for target in host windows browser; do
	if ! grep -Fxq "classified $target" "$TEST_DIR/success.output"; then
		echo "runner test: $target report was not independently classified" >&2
		exit 1
	fi
done
grep -Fxq 'security analysis: ok' "$TEST_DIR/success.output"

for mode in enumeration-failure enumeration-empty analyzer-failure missing-report empty-report classifier-failure staticcheck-failure staticcheck-debug-failure vulnerability-failure; do
	run_policy_control "$mode"
	case "$mode" in
		enumeration-failure|enumeration-empty|analyzer-failure|missing-report|empty-report|classifier-failure)
			grep -Fxq 'classified host' "$TEST_DIR/$mode.output"
			;;
	esac
	case "$mode" in
		enumeration-failure) expected='Go package enumeration failed' ;;
		enumeration-empty) expected='Go package enumeration returned no packages' ;;
		analyzer-failure) expected='gosec execution failed' ;;
		missing-report|empty-report) expected='gosec did not produce a non-empty JSON report' ;;
		classifier-failure) expected='fake Windows report validation failed' ;;
		*) expected='' ;;
	esac
	if [[ -n "$expected" ]]; then
		grep -Fq "$expected" "$TEST_DIR/$mode.output"
	fi
done

echo 'security analysis runner tests: ok'
