#!/usr/bin/env bash
set -euo pipefail

heartbeat_seconds="${CI_GO_TEST_HEARTBEAT_SECONDS:-60}"
if [[ ! "${heartbeat_seconds}" =~ ^[0-9]+$ ]] || [[ "${heartbeat_seconds}" -lt 1 ]]; then
	heartbeat_seconds=60
fi

export GOMAXPROCS="${GOMAXPROCS:-2}"

heavy_packages=(
	./internal/apextest
	./internal/gladecli
	./internal/playground
	./internal/sema
	./internal/server
)

run_with_heartbeat() {
	local label="$1"
	shift
	local pid
	local heartbeat_pid
	local rc=0

	echo "::group::${label}"
	printf '[ci] GOMAXPROCS=%s\n' "${GOMAXPROCS}"
	printf '+'
	printf ' %q' "$@"
	printf '\n'

	"$@" &
	pid="$!"

	(
		elapsed=0
		while true; do
			sleep "${heartbeat_seconds}" || exit 0
			if ! kill -0 "${pid}" 2>/dev/null; then
				exit 0
			fi
			elapsed=$((elapsed + heartbeat_seconds))
			printf '[ci] %s still running after %ss\n' "${label}" "${elapsed}"
		done
	) &
	heartbeat_pid="$!"

	wait "${pid}" || rc="$?"
	kill "${heartbeat_pid}" 2>/dev/null || true
	wait "${heartbeat_pid}" 2>/dev/null || true
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
		run_with_heartbeat "go test ${pkg}" go test -v -timeout=30m "${pkg}"
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
		run_with_heartbeat "go test -race ${pkg}" go test -race -v -timeout=60m "${pkg}"
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
