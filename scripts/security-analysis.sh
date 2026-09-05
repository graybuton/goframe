#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
EXPECTED_GO_VERSION="go1.26.6"
MAIN_MODULE="github.com/graybuton/goframe"
STATICCHECK_VERSION="v0.8.1"
GOVULNCHECK_VERSION="v1.7.0"
GOSEC_VERSION="v2.29.0"

GOSEC_PACKAGE_DIRS=()
GOSEC_PACKAGE_IMPORTS=()
SECURITY_WORK_DIR=""

security_error() {
	printf 'security analysis: %s\n' "$*" >&2
}

cleanup_security_analysis() {
	if [[ -n "$SECURITY_WORK_DIR" ]]; then
		rm -rf -- "$SECURITY_WORK_DIR"
	fi
}

enumerate_gosec_packages() {
	local repository_root="$1"
	local module_path="$2"
	local output_path="$3"
	local go_command="$4"
	local package_pattern="$5"

	if [[ -z "$package_pattern" ]]; then
		security_error "Go package enumeration requires an explicit package pattern"
		return 1
	fi
	if ! GOWORK=off "$go_command" list -buildvcs=false -f '{{.ImportPath}}{{"\t"}}{{.Dir}}' "$package_pattern" >"$output_path"; then
		security_error "Go package enumeration failed"
		return 1
	fi

	GOSEC_PACKAGE_DIRS=()
	GOSEC_PACKAGE_IMPORTS=()
	local row import_path directory
	local -A seen_imports=()
	local -A seen_directories=()
	while IFS= read -r row; do
		if [[ "$row" != *$'\t'* ]]; then
			security_error "invalid Go package row: $row"
			return 1
		fi
		import_path="${row%%$'\t'*}"
		directory="${row#*$'\t'}"
		if [[ -z "$import_path" || -z "$directory" || "$directory" == *$'\t'* ]]; then
			security_error "invalid Go package row: $row"
			return 1
		fi
		if [[ "$import_path" != "$module_path" && "$import_path" != "$module_path/"* ]]; then
			security_error "package $import_path is outside the main module"
			return 1
		fi
		if [[ ! -d "$directory" ]]; then
			security_error "package directory does not exist: $directory"
			return 1
		fi
		case "$directory/" in
			"$repository_root/"*) ;;
			*)
				security_error "package directory is outside the repository: $directory"
				return 1
				;;
		esac
		if [[ -n "${seen_imports[$import_path]:-}" || -n "${seen_directories[$directory]:-}" ]]; then
			security_error "duplicate Go package coverage row: $row"
			return 1
		fi
		seen_imports["$import_path"]=1
		seen_directories["$directory"]=1
		GOSEC_PACKAGE_IMPORTS+=("$import_path")
		GOSEC_PACKAGE_DIRS+=("$directory")
	done <"$output_path"

	if ((${#GOSEC_PACKAGE_DIRS[@]} == 0)); then
		security_error "Go package enumeration returned no packages"
		return 1
	fi
	printf 'gosec package coverage: %d first-party packages\n' "${#GOSEC_PACKAGE_DIRS[@]}"
	printf '  %s\n' "${GOSEC_PACKAGE_IMPORTS[@]}"
}

run_gosec_analysis() {
	local report_path="$1"
	local gosec_command="$2"
	shift 2

	if (($# == 0)); then
		security_error "gosec package coverage is empty"
		return 1
	fi
	local log_path="$report_path.log"
	rm -f -- "$report_path" "$log_path"
	# gosec v2.29.0 suppresses a clean JSON report under -quiet. Keep the
	# structured report mandatory while sending progress logs to a temporary file.
	if ! "$gosec_command" -no-fail -fmt=json -log="$log_path" -out="$report_path" "$@"; then
		if [[ -s "$log_path" ]]; then
			cat "$log_path" >&2
		fi
		security_error "gosec execution failed"
		return 1
	fi
	if [[ ! -s "$report_path" ]]; then
		if [[ -s "$log_path" ]]; then
			cat "$log_path" >&2
		fi
		security_error "gosec did not produce a non-empty JSON report"
		return 1
	fi
}

verify_root_module_surface() {
	local output_path="$1"
	local -a modules=()
	if ! GOWORK=off go list -m all >"$output_path"; then
		security_error "root Go module enumeration failed"
		return 1
	fi
	mapfile -t modules <"$output_path"
	printf 'root Go module graph:\n'
	printf '  %s\n' "${modules[@]}"
	if ((${#modules[@]} != 1)) || [[ "${modules[0]:-}" != "$MAIN_MODULE" ]]; then
		security_error "root Go module graph must contain only $MAIN_MODULE"
		return 1
	fi
}

main() {
	cd "$ROOT_DIR"
	export GOTOOLCHAIN=local
	export GOWORK=off
	export GOFLAGS=-buildvcs=false

	local actual_go_version
	local host_goos
	local host_goarch
	actual_go_version="$(go env GOVERSION)"
	host_goos="$(go env GOHOSTOS)"
	host_goarch="$(go env GOHOSTARCH)"
	export GOOS="$host_goos"
	export GOARCH="$host_goarch"
	go version
	if [[ "$actual_go_version" != "$EXPECTED_GO_VERSION" ]]; then
		security_error "requires $EXPECTED_GO_VERSION, found $actual_go_version"
		return 1
	fi
	if [[ "$(go env GOMOD)" != "$ROOT_DIR/go.mod" ]]; then
		security_error "repository root module is not selected"
		return 1
	fi

	SECURITY_WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/goframe-security-analysis.XXXXXX")"
	trap cleanup_security_analysis EXIT
	local tool_dir="$SECURITY_WORK_DIR/bin"
	local module_report="$SECURITY_WORK_DIR/modules.txt"
	local host_package_report="$SECURITY_WORK_DIR/packages-host.txt"
	local windows_package_report="$SECURITY_WORK_DIR/packages-windows.txt"
	local browser_package_report="$SECURITY_WORK_DIR/packages-wasm-runtime.txt"
	local host_gosec_report="$SECURITY_WORK_DIR/gosec-host.json"
	local windows_gosec_report="$SECURITY_WORK_DIR/gosec-windows.json"
	local browser_gosec_report="$SECURITY_WORK_DIR/gosec-wasm-runtime.json"
	mkdir -p "$tool_dir"

	echo '== Install pinned analyzers =='
	GOBIN="$tool_dir" go install "honnef.co/go/tools/cmd/staticcheck@$STATICCHECK_VERSION"
	GOBIN="$tool_dir" go install "golang.org/x/vuln/cmd/govulncheck@$GOVULNCHECK_VERSION"
	GOBIN="$tool_dir" go install "github.com/securego/gosec/v2/cmd/gosec@$GOSEC_VERSION"
	export PATH="$tool_dir:$PATH"
	staticcheck -version
	govulncheck -version
	gosec -version
	go version -m "$tool_dir/staticcheck"
	go version -m "$tool_dir/govulncheck"
	go version -m "$tool_dir/gosec"

	echo '== Root Go module dependency surface =='
	verify_root_module_surface "$module_report"

	echo '== Host Staticcheck SA correctness =='
	staticcheck -checks='SA*' ./...
	staticcheck -checks='SA*' -tags='goframe_debug' ./...

	echo '== Windows target Staticcheck SA correctness =='
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 staticcheck -checks='SA*' ./...
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 staticcheck -checks='SA*' -tags='goframe_debug' ./...

	echo '== Browser runtime Staticcheck SA correctness =='
	GOOS=js GOARCH=wasm CGO_ENABLED=0 staticcheck -checks='SA*' -tests=false ./pkg/goframe
	GOOS=js GOARCH=wasm CGO_ENABLED=0 staticcheck -checks='SA*' -tests=false -tags='goframe_debug' ./pkg/goframe

	echo '== Host reachable vulnerability analysis =='
	govulncheck -scan=symbol ./...

	echo '== Windows target reachable vulnerability analysis =='
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 govulncheck -scan=symbol ./...

	echo '== Browser runtime reachable vulnerability analysis =='
	GOOS=js GOARCH=wasm CGO_ENABLED=0 govulncheck -scan=symbol ./pkg/goframe

	echo '== Host gosec advisory analysis =='
	enumerate_gosec_packages "$ROOT_DIR" "$MAIN_MODULE" "$host_package_report" go ./...
	local -a host_gosec_package_dirs=("${GOSEC_PACKAGE_DIRS[@]}")
	run_gosec_analysis "$host_gosec_report" gosec "${host_gosec_package_dirs[@]}"
	go run ./.github/scripts/gosec-report.go \
		-report "$host_gosec_report" \
		-root "$ROOT_DIR" \
		-packages "${#host_gosec_package_dirs[@]}"

	echo '== Windows target gosec advisory analysis =='
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 enumerate_gosec_packages \
		"$ROOT_DIR" \
		"$MAIN_MODULE" \
		"$windows_package_report" \
		go \
		./...
	local -a windows_gosec_package_dirs=("${GOSEC_PACKAGE_DIRS[@]}")
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 run_gosec_analysis \
		"$windows_gosec_report" \
		gosec \
		"${windows_gosec_package_dirs[@]}"
	go run ./.github/scripts/gosec-report.go \
		-report "$windows_gosec_report" \
		-root "$ROOT_DIR" \
		-packages "${#windows_gosec_package_dirs[@]}"

	echo '== Browser runtime gosec advisory analysis =='
	GOOS=js GOARCH=wasm CGO_ENABLED=0 enumerate_gosec_packages \
		"$ROOT_DIR" \
		"$MAIN_MODULE" \
		"$browser_package_report" \
		go \
		./pkg/goframe
	local -a browser_gosec_package_dirs=("${GOSEC_PACKAGE_DIRS[@]}")
	GOOS=js GOARCH=wasm CGO_ENABLED=0 run_gosec_analysis \
		"$browser_gosec_report" \
		gosec \
		"${browser_gosec_package_dirs[@]}"
	go run ./.github/scripts/gosec-report.go \
		-report "$browser_gosec_report" \
		-root "$ROOT_DIR" \
		-packages "${#browser_gosec_package_dirs[@]}"

	echo 'security analysis: ok'
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
	main "$@"
fi
