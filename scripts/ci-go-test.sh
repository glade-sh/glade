#!/usr/bin/env bash
set -euo pipefail

heartbeat_seconds="${CI_GO_TEST_HEARTBEAT_SECONDS:-60}"
if [[ ! "${heartbeat_seconds}" =~ ^[0-9]+$ ]] || [[ "${heartbeat_seconds}" -lt 1 ]]; then
	heartbeat_seconds=60
fi

apextest_shard_size="${CI_APEXTEST_SHARD_SIZE:-25}"
if [[ ! "${apextest_shard_size}" =~ ^[0-9]+$ ]] || [[ "${apextest_shard_size}" -lt 1 ]]; then
	apextest_shard_size=25
fi

export GOMAXPROCS="${GOMAXPROCS:-2}"

heavy_packages=(
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

join_test_pattern() {
	local pattern=""
	local name
	for name in "$@"; do
		if [[ -n "${pattern}" ]]; then
			pattern="${pattern}|${name}"
		else
			pattern="${name}"
		fi
	done
	printf '^(%s)$' "${pattern}"
}

run_apextest_shards() {
	local mode="$1"
	local timeout="$2"
	local tmp_dir
	local binary
	local rc=0
	local compile_args=(-c -o)
	local compile_label="go test -c"
	local run_label="go test"
	local tests=()
	local total
	local shard=1
	local start=0
	local end
	local pattern

	tmp_dir="$(mktemp -d)"
	binary="${tmp_dir}/apextest.test"
	if [[ "${mode}" == "race" ]]; then
		compile_args=(-race -c -o)
		compile_label="go test -race -c"
		run_label="go test -race"
	fi

	run_with_heartbeat "${compile_label} ./internal/apextest" go test "${compile_args[@]}" "${binary}" ./internal/apextest || rc="$?"
	if [[ "${rc}" -eq 0 ]]; then
		mapfile -t tests < <(cd internal/apextest && "${binary}" -test.list '^Test' | grep '^Test' || true)
		total="${#tests[@]}"
		while [[ "${start}" -lt "${total}" ]]; do
			end=$((start + apextest_shard_size))
			if [[ "${end}" -gt "${total}" ]]; then
				end="${total}"
			fi
			pattern="$(join_test_pattern "${tests[@]:start:end-start}")"
			(
				cd internal/apextest
				run_with_heartbeat "${run_label} ./internal/apextest shard ${shard} (${start}-${end}/${total})" \
					"${binary}" -test.v "-test.timeout=${timeout}" -test.run "${pattern}"
			) || {
				rc="$?"
				break
			}
			start="${end}"
			shard=$((shard + 1))
		done
	fi
	rm -rf "${tmp_dir}"
	return "${rc}"
}

run_normal_tests() {
	local pkg
	local remaining
	run_apextest_shards test 30m
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
	run_apextest_shards race 60m
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
