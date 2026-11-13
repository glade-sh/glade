#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
artifact_root="${1:-${TMPDIR:-/tmp}/glade-apextest-local}"
history_path="${CI_APEXTEST_HISTORY_PATH:-${artifact_root}/duration-history.json}"
mkdir -p "${artifact_root}"

timeout_seconds="${GLADE_APEXTEST_SHARD_TIMEOUT_SECONDS:-2100}"
if [[ ! "${timeout_seconds}" =~ ^[0-9]+$ ]] || [[ "${timeout_seconds}" -lt 1 ]]; then
	echo "GLADE_APEXTEST_SHARD_TIMEOUT_SECONDS must be a positive integer" >&2
	exit 2
fi
timeout_cmd=()
if command -v gtimeout >/dev/null 2>&1; then
	timeout_cmd=(gtimeout --kill-after=1s "${timeout_seconds}")
elif command -v timeout >/dev/null 2>&1; then
	timeout_cmd=(timeout --kill-after=1s "${timeout_seconds}")
else
	echo "warning: no timeout command found; ci-go-test's 30m Go test timeout remains active" >&2
fi

pids=()
shard_dirs=()
set -m
cleanup() {
	local pid
	for pid in ${pids[@]+"${pids[@]}"}; do
		kill -TERM -- "-${pid}" 2>/dev/null || true
	done
	# A leader may exit on TERM while a member ignores it.
	sleep 1
	for pid in ${pids[@]+"${pids[@]}"}; do
		kill -KILL -- "-${pid}" 2>/dev/null || true
	done
	for pid in ${pids[@]+"${pids[@]}"}; do
		wait "${pid}" 2>/dev/null || true
	done
}
trap 'trap - INT TERM; cleanup; exit 130' INT
trap 'trap - INT TERM; cleanup; exit 143' TERM

shard_count="${CI_APEXTEST_SHARDS:-2}"
case "${shard_count}" in 2|6|12|16) ;; *) echo "CI_APEXTEST_SHARDS must be 2, 6, 12, or 16" >&2; exit 2;; esac
jobs="${GLADE_APEXTEST_SHARD_JOBS:-2}"
if [[ ! "${jobs}" =~ ^[0-9]+$ ]] || [[ "${jobs}" -lt 1 ]]; then
    echo "GLADE_APEXTEST_SHARD_JOBS must be positive" >&2
    exit 2
fi
status=0
wait_batch() {
    local pid rc index=0
    for pid in ${pids[@]+"${pids[@]}"}; do
        if wait "${pid}"; then rc=0; else rc=$?; status=1; fi
        # timeout can exit after its leader dies while descendants survive.
        if [[ "${rc}" -eq 124 || "${rc}" -eq 137 ]]; then
            kill -KILL -- "-${pid}" 2>/dev/null || true
        fi
        printf '%s\n' "${rc}" >"${shard_dirs[${index}]}/exit-status.txt"
        index=$((index + 1))
    done
    pids=()
    shard_dirs=()
}
for ((index=0; index<shard_count; index++)); do
    shard_artifact_dir="${artifact_root}/shard-${index}"
    shard_log="${artifact_root}/shard-${index}.log"
    mkdir -p "${shard_artifact_dir}"
    (
        set +m
        if [[ "${shard_count}" -ne 2 ]]; then export GLADE_HOME="${shard_artifact_dir}/glade-home"; fi
        CI_APEXTEST_ARTIFACT_DIR="${shard_artifact_dir}" \
        CI_APEXTEST_HISTORY_PATH="${history_path}" \
            exec ${timeout_cmd[@]+"${timeout_cmd[@]}"} "${repo_root}/scripts/ci-go-test.sh" apex-shard "${index}"
    ) >"${shard_log}" 2>&1 &
    pids+=("$!")
    shard_dirs+=("${shard_artifact_dir}")
    if [[ "${#pids[@]}" -ge "${jobs}" ]]; then wait_batch; fi
done
wait_batch
if [[ "${status}" -ne 0 ]]; then
    echo "apextest shards failed; per-shard logs and evidence remain under ${artifact_root}" >&2
    exit "${status}"
fi
shards=()
for ((index=0; index<shard_count; index++)); do shards+=("${artifact_root}/shard-${index}"); done
if [[ "${shard_count}" -eq 2 ]]; then
    "${repo_root}/scripts/ci-go-test.sh" apex-history-refresh "${shards[@]}" "${history_path}"
else
    "${repo_root}/scripts/ci-go-test.sh" apex-history-refresh "${history_path}" "${shards[@]}"
fi
echo "apextest shards passed; evidence and duration history are under ${artifact_root}"
