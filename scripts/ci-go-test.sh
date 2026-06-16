#!/usr/bin/env bash
set -euo pipefail

heartbeat_seconds="${CI_GO_TEST_HEARTBEAT_SECONDS:-60}"
if [[ ! "${heartbeat_seconds}" =~ ^[0-9]+$ ]] || [[ "${heartbeat_seconds}" -lt 1 ]]; then
	heartbeat_seconds=60
fi

heavy_packages=(
	./internal/apextest
	./internal/gladecli
	./internal/playground
	./internal/sema
	./internal/server
)

process_active() {
	local pid="$1"
	local state
	state="$(ps -p "${pid}" -o stat= 2>/dev/null | awk '{print $1}' || true)"
	[[ -n "${state}" && "${state:0:1}" != "Z" ]]
}

run_with_heartbeat() {
	local label="$1"
	shift
	local log
	local pid
	local rc=0
	local elapsed=0

	log="$(mktemp "${TMPDIR:-/tmp}/glade-ci-go-test.XXXXXX")"
	echo "::group::${label}"
	printf '+'
	printf ' %q' "$@"
	printf '\n'

	"$@" >"${log}" 2>&1 &
	pid="$!"

	while process_active "${pid}"; do
		for ((i = 0; i < heartbeat_seconds; i++)); do
			sleep 1
			if ! process_active "${pid}"; then
				break 2
			fi
		done
		elapsed=$((elapsed + heartbeat_seconds))
		printf '[ci] %s still running after %ss\n' "${label}" "${elapsed}"
	done

	wait "${pid}" || rc="$?"
	cat "${log}"
	rm -f "${log}"
	echo "::endgroup::"
	return "${rc}"
}

remaining_packages() {
	go list ./... | grep -Ev '/internal/(apextest|gladecli|playground|sema|server)$' || true
}

run_normal_tests() {
	local pkg
	local remaining
	for pkg in "${heavy_packages[@]}"; do
		run_with_heartbeat "go test ${pkg}" go test -timeout=30m "${pkg}"
	done
	remaining="$(remaining_packages)"
	if [[ -n "${remaining}" ]]; then
		# Intentional word splitting: package import paths do not contain spaces.
		run_with_heartbeat "go test remaining packages" go test -p=2 -timeout=20m ${remaining}
	fi
}

run_race_tests() {
	local pkg
	local remaining
	for pkg in "${heavy_packages[@]}"; do
		run_with_heartbeat "go test -race ${pkg}" go test -race -timeout=60m "${pkg}"
	done
	remaining="$(remaining_packages)"
	if [[ -n "${remaining}" ]]; then
		# Intentional word splitting: package import paths do not contain spaces.
		run_with_heartbeat "go test -race remaining packages" go test -race -p=1 -timeout=30m ${remaining}
	fi
}

case "${1:-test}" in
	test)
		run_normal_tests
		;;
	race)
		run_race_tests
		;;
	*)
		echo "usage: scripts/ci-go-test.sh [test|race]" >&2
		exit 2
		;;
esac
