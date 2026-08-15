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
	timeout_cmd=(gtimeout "${timeout_seconds}")
elif command -v timeout >/dev/null 2>&1; then
	timeout_cmd=(timeout "${timeout_seconds}")
else
	echo "warning: no timeout command found; ci-go-test's 30m Go test timeout remains active" >&2
fi

pids=()
cleanup() {
	local pid
	for pid in "${pids[@]}"; do
		kill "${pid}" 2>/dev/null || true
	done
	for pid in "${pids[@]}"; do
		wait "${pid}" 2>/dev/null || true
	done
}
trap cleanup INT TERM

for index in 0 1; do
	shard_artifact_dir="${artifact_root}/shard-${index}"
	shard_log="${artifact_root}/shard-${index}.log"
	mkdir -p "${shard_artifact_dir}"
	(
		CI_APEXTEST_ARTIFACT_DIR="${shard_artifact_dir}" \
		CI_APEXTEST_HISTORY_PATH="${history_path}" \
			"${timeout_cmd[@]}" "${repo_root}/scripts/ci-go-test.sh" apex-shard "${index}"
	) >"${shard_log}" 2>&1 &
	pids+=("$!")
done

status=0
for pid in "${pids[@]}"; do
	if ! wait "${pid}"; then
		status=1
	fi
done
pids=()

if [[ "${status}" -ne 0 ]]; then
	echo "apextest shards failed; per-shard logs and evidence remain under ${artifact_root}" >&2
	exit "${status}"
fi

"${repo_root}/scripts/ci-go-test.sh" apex-history-refresh \
	"${artifact_root}/shard-0" \
	"${artifact_root}/shard-1" \
	"${history_path}"

echo "apextest shards passed; evidence and duration history are under ${artifact_root}"
