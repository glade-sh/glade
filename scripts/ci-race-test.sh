#!/usr/bin/env bash
set -euo pipefail

race_mode="test"
if [[ "$#" -eq 3 && "$1" == "partition" ]]; then
	race_mode="partition"
	package="$2"
	slug="$3"
elif [[ "$#" -eq 2 ]]; then
	package="$1"
	slug="$2"
else
	echo "usage: scripts/ci-race-test.sh [partition] <package> <slug>" >&2
	exit 2
fi

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
go_command="${CI_RACE_GO_COMMAND:-go}"
resource_runner="${CI_RACE_RESOURCE_RUNNER:-scripts/ci-resource-run.sh}"
shard_planner="${CI_RACE_SHARD_PLANNER:-}"
git_command="${CI_RACE_GIT_COMMAND:-git}"
artifact_dir="${PWD}/ci-artifacts/race/${slug}"
apextest_runner="${CI_RACE_APEXTEST_RUNNER:-}"
apextest_partition_dir="${CI_RACE_PARTITION_DIR:-}"
apextest_head_sha="${CI_RACE_HEAD_SHA:-}"
apextest_assigned=false
batch_timeout="${CI_RACE_BATCH_TIMEOUT_SECONDS:-60m}"
test_timeout_arg="${CI_RACE_TEST_TIMEOUT_SECONDS:-60m}"
if [[ -n "${CI_RACE_APEXTEST_SHARD_INDEXES:-}" ]]; then
	echo "[ci] CI_RACE_APEXTEST_SHARD_INDEXES is obsolete; Apex shard assignments come from the race partition" >&2
	exit 2
fi
browser_shard_index="${CI_RACE_SHARD_INDEX:-}"
browser_shard_count=0
vm_shard_count="${CI_RACE_SHARD_COUNT:-}"
case "${package}" in
	./internal/visualforce) browser_shard_count=4 ;;
	./internal/lwc/compile) browser_shard_count=8 ;;
	./internal/vm)
		# The plan job derives the VM shard count (balanced shards plus one
		# per isolated giant test) from the measured race history; a bare
		# local run without the plan keeps the four balanced shards.
		if [[ -z "${vm_shard_count}" ]]; then
			if [[ -n "${CI_RACE_ISOLATED_TESTS:-}" ]]; then
				printf '[ci] the VM lane needs CI_RACE_SHARD_COUNT when tests are isolated\n' >&2
				exit 2
			fi
			vm_shard_count=4
		fi
		if [[ ! "${vm_shard_count}" =~ ^[0-9]+$ ]] || [[ "${vm_shard_count}" -lt 1 ]] || [[ "${vm_shard_count}" -gt 16 ]]; then
			printf '[ci] CI_RACE_SHARD_COUNT must be a shard count between 1 and 16\n' >&2
			exit 2
		fi
		browser_shard_count="${vm_shard_count}"
		;;
esac

if [[ "${race_mode}" == "partition" ]]; then
	if [[ "${package}" != "./internal/apextest" ]]; then
		printf '[ci] only ./internal/apextest produces a race partition\n' >&2
		exit 2
	fi
	if [[ -n "${apextest_runner}" || -n "${apextest_partition_dir}" || -n "${browser_shard_index}" ]]; then
		printf '[ci] race partition production cannot also consume a runner assignment\n' >&2
		exit 2
	fi
fi

if [[ ! "${package}" =~ ^\./[A-Za-z0-9._/-]+$ ]] || [[ "${package}" == *...* ]]; then
	printf '[ci] invalid exact race package %q\n' "${package}" >&2
	exit 2
fi
if [[ -n "${browser_shard_index}" ]] && { [[ ! "${browser_shard_index}" =~ ^[0-9]+$ ]] || [[ "${browser_shard_index}" -ge "${browser_shard_count}" ]]; }; then
	echo "[ci] race shard index must select one of the lane's ${browser_shard_count} shards" >&2
	exit 2
fi
case "${batch_timeout}" in
	60m) ;;
	*[!0-9]* | "") printf '[ci] CI_RACE_BATCH_TIMEOUT_SECONDS must be plain seconds\n' >&2; exit 2 ;;
	*) [[ "${batch_timeout}" -ge 60 ]] || { printf '[ci] CI_RACE_BATCH_TIMEOUT_SECONDS must be at least 60 seconds\n' >&2; exit 2; } ;;
esac
case "${test_timeout_arg}" in
	60m) ;;
	*[!0-9]* | "") printf '[ci] CI_RACE_TEST_TIMEOUT_SECONDS must be plain seconds\n' >&2; exit 2 ;;
	*) [[ "${test_timeout_arg}" -ge 60 ]] || { printf '[ci] CI_RACE_TEST_TIMEOUT_SECONDS must be at least 60 seconds\n' >&2; exit 2; } ;;
esac
go_test_timeout="${test_timeout_arg}"
if [[ "${go_test_timeout}" != "60m" ]]; then
	go_test_timeout="${go_test_timeout}s"
elif [[ "${batch_timeout}" == "60m" ]]; then
	# With the default flat deadlines the go test timeout would fire together
	# with the outer batch wrapper, so a slow shard dies to the outer kill with
	# no timeout evidence. Keep the go test timeout below the batch: the slow
	# test then ends in its own "test timed out" panic and stack.
	go_test_timeout="55m"
fi
package_body="${package#./}"
if [[ -z "${package_body}" || "${package_body}" == */ ]]; then
	printf '[ci] invalid exact race package %q\n' "${package}" >&2
	exit 2
fi
IFS='/' read -r -a package_segments <<<"${package_body}"
for segment in "${package_segments[@]}"; do
	if [[ -z "${segment}" || "${segment}" == "." || "${segment}" == ".." ]]; then
		printf '[ci] invalid exact race package segment %q in %q\n' "${segment}" "${package}" >&2
		exit 2
	fi
done
expected_slug="${package#./}"
expected_slug="${expected_slug//\//-}"
if [[ ! "${slug}" =~ ^[A-Za-z0-9._-]+$ ]] || [[ "${slug}" != "${expected_slug}" ]]; then
	printf '[ci] invalid race artifact slug %q for package %q\n' "${slug}" "${package}" >&2
	exit 2
fi
if [[ "${go_command}" == */* ]]; then
	[[ -x "${go_command}" ]] || { printf '[ci] Go command is not executable: %s\n' "${go_command}" >&2; exit 2; }
elif ! command -v "${go_command}" >/dev/null 2>&1; then
	printf '[ci] Go command was not found: %s\n' "${go_command}" >&2
	exit 2
fi
if [[ "${resource_runner}" == */* ]]; then
	[[ -x "${resource_runner}" ]] || { printf '[ci] resource runner is not executable: %s\n' "${resource_runner}" >&2; exit 2; }
elif ! command -v "${resource_runner}" >/dev/null 2>&1; then
	printf '[ci] resource runner was not found: %s\n' "${resource_runner}" >&2
	exit 2
fi

mkdir -p "${artifact_dir}"

validate_package_summary() {
	local events="$1" summary="$2"
	local metadata="${artifact_dir}/package-metadata.tsv"
	printf '%s\thas-tests\n' "${package_name}" >"${metadata}"
	bash "${script_dir}/ci-go-test.sh" package-summary race "${slug}" "${events}" "${summary}" "${metadata}" "${package_name}"
}

validate_resource_evidence() {
	local path="$1"
	local lane="$2"
	local expected_status="$3"
	python3 - "${path}" "${lane}" "${expected_status}" <<'PY'
import json
import math
import sys

path, lane, status_text = sys.argv[1:]
expected_status = int(status_text)

def reject_duplicates(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError(f"duplicate JSON key {key!r}")
        value[key] = item
    return value

with open(path, encoding="utf-8") as source:
    value = json.load(source, object_pairs_hook=reject_duplicates)
keys = {"schema_version", "lane", "elapsed_seconds", "user_seconds", "system_seconds", "max_rss_kb", "exit_status"}
if not isinstance(value, dict) or set(value) != keys:
    raise SystemExit("race resource evidence has an invalid schema")
if type(value["schema_version"]) is not int or value["schema_version"] != 1:
    raise SystemExit("race resource evidence has an invalid schema version")
if value["lane"] != lane:
    raise SystemExit("race resource evidence has the wrong lane")
if type(value["exit_status"]) is not int or value["exit_status"] != expected_status:
    raise SystemExit("race resource evidence has the wrong exit status")
for field in ("elapsed_seconds", "user_seconds", "system_seconds", "max_rss_kb"):
    number = value[field]
    if isinstance(number, bool) or not isinstance(number, (int, float)) or not math.isfinite(number) or number < 0:
        raise SystemExit(f"race resource evidence has invalid {field}")
PY
}

heavy_package=false
case "${package}" in
	./internal/apextest|./internal/gladecli|./internal/playground|./internal/server|./internal/testdaemon|./internal/vm|./internal/visualforce|./internal/lwc/compile)
		heavy_package=true
		;;
	*)
		set +e
		"${resource_runner}" "${artifact_dir}/resource.json" "race-${slug}" -- \
			"${go_command}" test -race -count=1 -timeout="${go_test_timeout}" "${package}"
		native_rc="$?"
		set -e
		set +e
		validate_resource_evidence "${artifact_dir}/resource.json" "race-${slug}" "${native_rc}"
		validation_rc="$?"
		set -e
		if [[ "${native_rc}" -ne 0 ]]; then
			exit "${native_rc}"
		fi
		if [[ "${validation_rc}" -ne 0 ]]; then
			exit "${validation_rc}"
		fi
		exit "${native_rc}"
		;;
esac

if [[ "${heavy_package}" == true && "${CI_RACE_DEADLINE_ACTIVE:-0}" != "1" ]]; then
	timeout_command="${CI_RACE_TIMEOUT_COMMAND:-timeout}"
	if [[ "${timeout_command}" == */* ]]; then
		[[ -x "${timeout_command}" ]] || { printf '[ci] timeout command is not executable: %s\n' "${timeout_command}" >&2; exit 2; }
	elif ! command -v "${timeout_command}" >/dev/null 2>&1; then
		printf '[ci] timeout command was not found: %s\n' "${timeout_command}" >&2
		exit 2
	fi
	export CI_RACE_DEADLINE_ACTIVE=1
	wrapper_args=()
	if [[ "${race_mode}" == "partition" ]]; then
		wrapper_args=("partition")
	fi
	exec "${timeout_command}" --signal=TERM --kill-after=30s "${batch_timeout}" \
		"${BASH}" "${BASH_SOURCE[0]}" ${wrapper_args[@]+"${wrapper_args[@]}"} "${package}" "${slug}"
fi

package_name="github.com/glade-sh/glade/${package#./}"
if [[ -n "${browser_shard_index}" ]]; then
	if [[ ! "${apextest_head_sha}" =~ ^[0-9a-f]{40}$ ]] || [[ "$("${git_command}" rev-parse HEAD)" != "${apextest_head_sha}" ]]; then
		echo "[ci] browser race runner requires the exact checkout head SHA" >&2
		exit 2
	fi
fi
go_test_package_args=()
if [[ "${package}" == "./internal/server" ]]; then
	go_test_package_args=(-p=1)
fi
discovery_raw="${artifact_dir}/discovery-command.txt"
discovery="${artifact_dir}/discovery.txt"
discovery_benchmarks="${artifact_dir}/discovery-benchmarks.txt"
ordinary_discovery="${artifact_dir}/ordinary-discovery.txt"
plan="${artifact_dir}/plan.json"
union_validation="${artifact_dir}/union-validation.json"
union_sentinel="${union_validation}.tmp.$$"
if [[ "${package}" == "./internal/playground" ]]; then
	sentinel_counts='[0,0,0,0,0,0,0,0,0]'
elif [[ "${package}" == "./internal/apextest" || "${package}" == "./internal/gladecli" ]]; then
	sentinel_counts='[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]'
elif [[ "${package}" == "./internal/server" || "${package}" == "./internal/lwc/compile" ]]; then
	sentinel_counts='[0,0,0,0,0,0,0,0]'
else
	sentinel_counts='[0,0,0,0]'
fi
printf '{"schema_version":1,"package":"%s","discovered_count":0,"shard_counts":%s,"names_sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","valid":false}\n' "${package_name}" "${sentinel_counts}" >"${union_sentinel}"
mv "${union_sentinel}" "${union_validation}"

if [[ "${package}" == "./internal/apextest" ]]; then
	assigned_apextest_indexes=(0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15)
	apextest_assigned=false
	partition_shard_count=""
	partition_batch_seconds=""
	partition_discovery_sha256=""
	partition_plan_sha256=""
	partition_digest=""
	partition_binary_sha256=""
	partition_binary_size=""
	declare -A partition_shard_timeouts=()
	if [[ -n "${apextest_runner}" || -n "${apextest_partition_dir}" ]]; then
		apextest_assigned=true
		if [[ -z "${apextest_runner}" || -z "${apextest_partition_dir}" ]]; then
			printf '[ci] an assigned Apex runner needs both CI_RACE_APEXTEST_RUNNER and CI_RACE_PARTITION_DIR\n' >&2
			exit 2
		fi
		if [[ ! -d "${apextest_partition_dir}" ]]; then
			printf '[ci] the Apex race partition directory does not exist: %q\n' "${apextest_partition_dir}" >&2
			exit 2
		fi
		python3 "${script_dir}/ci-race-plan.py" verify "${apextest_partition_dir}"
	fi
	if [[ "${race_mode}" == "partition" || "${apextest_assigned}" == true ]]; then
		if [[ ! "${apextest_head_sha}" =~ ^[0-9a-f]{40}$ ]]; then
			printf '[ci] the Apex race partition route requires a 40-character lowercase head SHA\n' >&2
			exit 2
		fi
		if [[ "${git_command}" == */* ]]; then
			[[ -x "${git_command}" ]] || { printf '[ci] git command is not executable: %s\n' "${git_command}" >&2; exit 2; }
		elif ! command -v "${git_command}" >/dev/null 2>&1; then
			printf '[ci] git command was not found: %s\n' "${git_command}" >&2
			exit 2
		fi
		if ! actual_apextest_head="$("${git_command}" rev-parse HEAD)"; then
			printf '[ci] could not resolve apextest runner checkout HEAD\n' >&2
			exit 2
		fi
		if [[ ! "${actual_apextest_head}" =~ ^[0-9a-f]{40}$ ]]; then
			printf '[ci] apextest runner checkout HEAD is not a 40-character lowercase SHA\n' >&2
			exit 2
		fi
		if [[ "${actual_apextest_head}" != "${apextest_head_sha}" ]]; then
			printf '[ci] apextest runner checkout HEAD does not match expected head\n' >&2
			exit 2
		fi
	fi
	if [[ "${apextest_assigned}" == true ]]; then
		assigned_apextest_indexes=()
		partition_rows=()
		mapfile -t partition_rows < <(python3 - "${apextest_partition_dir}/partition.json" "${apextest_runner}" <<'PY'
import hashlib
import json
import sys

with open(sys.argv[1], encoding="utf-8") as source:
    partition = json.load(source)
with open(sys.argv[1], "rb") as source:
    digest = hashlib.sha256(source.read()).hexdigest()
runner = next((entry for entry in partition["runners"] if entry["runner"] == sys.argv[2]), None)
if runner is None:
    raise SystemExit(f"race partition has no assignment for runner {sys.argv[2]}")
print(f"shard-count {partition['shard_count']}")
print(f"batch {runner['batch_timeout_seconds']}")
print(f"discovery {partition['discovery_sha256']}")
print(f"plan {partition['plan_sha256']}")
print(f"partition {digest}")
print(f"binary {partition['binary_sha256']}")
print(f"binary-size {partition['binary_size_bytes']}")
for shard in partition["shards"]:
    print(f"timeout {shard['index']} {shard['test_timeout_seconds']}")
for index in sorted(runner["shard_indexes"]):
    print(f"assigned {index}")
PY
		)
		for row in "${partition_rows[@]}"; do
			kind="${row%% *}"
			rest="${row#* }"
			case "${kind}" in
				shard-count) partition_shard_count="${rest}" ;;
				batch) partition_batch_seconds="${rest}" ;;
				discovery) partition_discovery_sha256="${rest}" ;;
				plan) partition_plan_sha256="${rest}" ;;
				partition) partition_digest="${rest}" ;;
				binary) partition_binary_sha256="${rest}" ;;
				binary-size) partition_binary_size="${rest}" ;;
				timeout)
					index="${rest%% *}"
					seconds="${rest#* }"
					if [[ ! "${index}" =~ ^[0-9]+$ ]] || [[ ! "${seconds}" =~ ^[0-9]+$ ]] || [[ "${seconds}" -lt 60 ]]; then
						printf '[ci] partition returned an invalid shard deadline: %q\n' "${row}" >&2
						exit 2
					fi
					partition_shard_timeouts["${index}"]="${seconds}"
					;;
				assigned)
					if [[ ! "${rest}" =~ ^[0-9]+$ ]]; then
						printf '[ci] partition returned an invalid shard index: %q\n' "${row}" >&2
						exit 2
					fi
					assigned_apextest_indexes+=("${rest}")
					;;
				*)
					printf '[ci] partition returned an unrecognized row: %q\n' "${row}" >&2
					exit 2
					;;
			esac
		done
		if [[ "${#assigned_apextest_indexes[@]}" -eq 0 ]]; then
			printf '[ci] partition assigned no shards to runner %q\n' "${apextest_runner}" >&2
			exit 2
		fi
		if [[ ! "${partition_shard_count}" =~ ^[0-9]+$ ]] || [[ "${partition_shard_count}" -lt 16 ]] || [[ "${partition_shard_count}" -gt 96 ]]; then
			printf '[ci] partition shard count must stay between 16 and 96 shards, got %q\n' "${partition_shard_count}" >&2
			exit 2
		fi
		for index in "${assigned_apextest_indexes[@]}"; do
			if [[ -z "${partition_shard_timeouts["${index}"]}" ]]; then
				printf '[ci] partition has no deadline for assigned shard %s\n' "${index}" >&2
				exit 2
			fi
		done
		if [[ "${CI_RACE_BATCH_TIMEOUT_SECONDS:-}" != "${partition_batch_seconds}" ]]; then
			printf '[ci] the workflow batch deadline does not match the partition: %q vs %q\n' \
				"${CI_RACE_BATCH_TIMEOUT_SECONDS:-}" "${partition_batch_seconds}" >&2
			exit 2
		fi
	fi
	binary_metadata="${artifact_dir}/binary.json"
	binary_path="$(mktemp "${TMPDIR:-/tmp}/glade-race-apextest.test.XXXXXX")"
	rm -f "${binary_path}"
	cleanup_apextest_binary() {
		rm -f -- "${binary_path}"
		if [[ -f "${binary_metadata}" ]]; then
			python3 - "${binary_metadata}" <<'PY'
import json, os, sys, tempfile
path = sys.argv[1]
try:
    with open(path, encoding="utf-8") as source:
        value = json.load(source)
    if isinstance(value, dict) and set(value) == {"schema_version", "package", "sha256", "size_bytes", "removed"}:
        value["removed"] = True
        descriptor, temporary = tempfile.mkstemp(prefix=".binary.", dir=os.path.dirname(path), text=True)
        with os.fdopen(descriptor, "w", encoding="utf-8") as target:
            json.dump(value, target, sort_keys=True, separators=(",", ":"))
            target.write("\n")
        os.replace(temporary, path)
except (OSError, ValueError):
    pass
PY
		fi
	}
	active_resource_pid=""
	run_apextest_command() {
		python3 - "$@" <<'PY' &
import os, sys
os.setsid()
os.execvp(sys.argv[1], sys.argv[1:])
PY
		active_resource_pid="$!"
		wait "${active_resource_pid}"
		local wait_rc="$?"
		active_resource_pid=""
		return "${wait_rc}"
	}
	handle_apextest_signal() {
		local signal_name="$1" signal_status="$2"
		trap - HUP INT TERM
		if [[ -n "${active_resource_pid}" ]] && kill -0 "${active_resource_pid}" 2>/dev/null; then
			kill "-${signal_name}" -- "-${active_resource_pid}" 2>/dev/null || kill "-${signal_name}" "${active_resource_pid}" 2>/dev/null || true
			set +e
			wait "${active_resource_pid}" 2>/dev/null
			set -e
			active_resource_pid=""
		fi
		trap - EXIT
		cleanup_apextest_binary
		exit "${signal_status}"
	}
	trap cleanup_apextest_binary EXIT
	trap 'handle_apextest_signal HUP 129' HUP
	trap 'handle_apextest_signal INT 130' INT
	trap 'handle_apextest_signal TERM 143' TERM

	build_resource="${artifact_dir}/build-resource.json"
	set +e
	run_apextest_command "${resource_runner}" "${build_resource}" "race-${slug}-build" -- \
		bash -c 'if [[ -n "${CI_TEST_BIN_DIR:-}" ]]; then
			python3 "$1/ci-test-binary.py" copy-race "$2" "$3"
		else
			"$4" test -trimpath -race -c -o "$3" "$2"
		fi' bash "${script_dir}" "${package}" "${binary_path}" "${go_command}"
	build_native_rc="$?"
	set -e
	set +e
	validate_resource_evidence "${build_resource}" "race-${slug}-build" "${build_native_rc}"
	build_validation_rc="$?"
	set -e
	if [[ "${build_native_rc}" -ne 0 ]]; then exit "${build_native_rc}"; fi
	if [[ "${build_validation_rc}" -ne 0 ]]; then exit "${build_validation_rc}"; fi

	python3 - "${binary_path}" "${binary_metadata}" "${package_name}" <<'PY'
import hashlib, json, os, stat, sys, tempfile
binary_path, output_path, package = sys.argv[1:]
try:
    info = os.stat(binary_path, follow_symlinks=False)
except FileNotFoundError:
    raise SystemExit("race apextest binary is missing after build")
if not stat.S_ISREG(info.st_mode) or info.st_size <= 0 or not os.access(binary_path, os.X_OK):
    raise SystemExit("race apextest binary is not a nonempty executable regular file")
digest = hashlib.sha256()
with open(binary_path, "rb") as source:
    for chunk in iter(lambda: source.read(1024 * 1024), b""):
        digest.update(chunk)
value = {"schema_version": 1, "package": package, "sha256": digest.hexdigest(), "size_bytes": info.st_size, "removed": False}
descriptor, temporary = tempfile.mkstemp(prefix=".binary.", dir=os.path.dirname(output_path), text=True)
with os.fdopen(descriptor, "w", encoding="utf-8") as target:
    json.dump(value, target, sort_keys=True, separators=(",", ":"))
    target.write("\n")
os.replace(temporary, output_path)
PY
	read -r binary_sha256 binary_size < <(python3 - "${binary_metadata}" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as source:
    value = json.load(source)
if not isinstance(value, dict) or set(value) != {"schema_version", "package", "sha256", "size_bytes", "removed"}:
    raise SystemExit("race apextest binary metadata has an invalid schema")
if value["schema_version"] != 1 or value["removed"] is not False:
    raise SystemExit("race apextest binary metadata has invalid state")
print(value["sha256"], value["size_bytes"])
PY
	)
	if [[ "${apextest_assigned}" == true ]]; then
		if [[ "${binary_sha256}" != "${partition_binary_sha256}" || "${binary_size}" != "${partition_binary_size}" ]]; then
			printf '[ci] the Apex race binary does not match the partition identity\n' >&2
			exit 2
		fi
	fi
	verify_apextest_binary() {
		python3 - "${binary_path}" "${binary_sha256}" "${binary_size}" <<'PY'
import hashlib, os, stat, sys
path, expected_sha, expected_size = sys.argv[1:]
try:
    info = os.stat(path, follow_symlinks=False)
except FileNotFoundError:
    raise SystemExit("race apextest binary is missing")
if not stat.S_ISREG(info.st_mode) or not os.access(path, os.X_OK) or str(info.st_size) != expected_size:
    raise SystemExit("race apextest binary has invalid type, mode, or size")
digest = hashlib.sha256()
with open(path, "rb") as source:
    for chunk in iter(lambda: source.read(1024 * 1024), b""):
        digest.update(chunk)
if digest.hexdigest() != expected_sha:
    raise SystemExit("race apextest binary checksum changed")
PY
	}
	if [[ "${apextest_assigned}" == true ]]; then
		cp "${apextest_partition_dir}/discovery.txt" "${discovery}"
		cp "${apextest_partition_dir}/plan.json" "${plan}"
		python3 - "${apextest_partition_dir}/discovery.txt" "${partition_discovery_sha256}" "${apextest_partition_dir}/plan.json" "${partition_plan_sha256}" <<'PY'
import hashlib
import sys

for path, expected in ((sys.argv[1], sys.argv[2]), (sys.argv[3], sys.argv[4])):
    with open(path, "rb") as source:
        digest = hashlib.sha256(source.read()).hexdigest()
    if digest != expected:
        raise SystemExit("the Apex race partition files do not match their recorded digests")
PY
	else
		set +e
		run_apextest_command bash -c 'cd "$1"; shift; exec "$@"' bash "${PWD}/${package#./}" "${binary_path}" -test.list '.' >"${discovery_raw}"
		discovery_native_rc="$?"
		set -e
		if [[ "${discovery_native_rc}" -ne 0 ]]; then exit "${discovery_native_rc}"; fi
		verify_apextest_binary
	fi
else
	CI_TEST_GO_COMMAND="${go_command}" python3 "${script_dir}/ci-test-binary.py" test "${go_test_package_args[@]}" -race -list '.' "${package}" >"${discovery_raw}"
fi
if [[ "${apextest_assigned}" != true ]]; then
	python3 - "${discovery_raw}" "${discovery}" "${discovery_benchmarks}" "${ordinary_discovery}" "${package_name}" <<'PY'
import re
import sys

raw_path, output_path, benchmark_path, ordinary_path, package = sys.argv[1:]
test_pattern = re.compile(r"Test[A-Za-z0-9_]*\Z")
benchmark_pattern = re.compile(r"Benchmark[A-Za-z0-9_]*\Z")
trailer_pattern = re.compile(r"ok\s+" + re.escape(package) + r"(?:\s+[^\s]+)?\Z")
names = []
benchmarks = []
trailers = 0
direct_passes = 0
with open(raw_path, encoding="utf-8") as source:
    for number, raw in enumerate(source, 1):
        line = raw.rstrip("\n")
        if test_pattern.fullmatch(line):
            names.append(line)
        elif benchmark_pattern.fullmatch(line):
            benchmarks.append(line)
        elif line.startswith("Fuzz"):
            raise SystemExit(f"race discovery contains unsupported fuzz target: {line!r}")
        elif line.startswith("Example"):
            raise SystemExit(f"race discovery contains unsupported example: {line!r}")
        elif trailer_pattern.fullmatch(line):
            trailers += 1
        elif package == "github.com/glade-sh/glade/internal/apextest" and line == "PASS":
            direct_passes += 1
        else:
            raise SystemExit(f"invalid race discovery line {number}: {line!r}")
if not names:
    raise SystemExit("race discovery contains no top-level tests")
if len(names) != len(set(names)):
    raise SystemExit("race discovery contains duplicate top-level tests")
if len(benchmarks) != len(set(benchmarks)):
    raise SystemExit("race discovery contains duplicate benchmarks")
if package == "github.com/glade-sh/glade/internal/apextest":
    if trailers != 0 or direct_passes not in (0, 1):
        raise SystemExit(f"race apextest binary discovery trailer counts are ok={trailers} PASS={direct_passes}")
elif trailers != 1:
    raise SystemExit(f"race discovery package trailer count is {trailers}, want 1")
names.sort()
with open(output_path, "w", encoding="utf-8") as target:
    target.write("\n".join(names) + "\n")
with open(benchmark_path, "w", encoding="utf-8") as target:
    if benchmarks:
        target.write("\n".join(sorted(benchmarks)) + "\n")
ordinary = names
if package == "github.com/glade-sh/glade/internal/playground":
    groups = {
        "TestExampleProjectsRunAnonymousGroupOne",
        "TestExampleProjectsRunAnonymousGroupTwo",
        "TestExampleProjectsRunAnonymousGroupThree",
        "TestExampleProjectsRunAnonymousGroupFour",
    }
    discovered_groups = {name for name in names if name.startswith("TestExampleProjectsRunAnonymous")}
    if discovered_groups != groups:
        raise SystemExit("playground discovery does not contain exactly the four example execution groups")
    ordinary = [name for name in names if name not in groups]
    if not ordinary:
        raise SystemExit("playground discovery contains no ordinary tests")
with open(ordinary_path, "w", encoding="utf-8") as target:
    target.write("\n".join(ordinary) + "\n")
PY
fi

planner_tests="${discovery}"
planner_shards=4
plan_mode=generic
if [[ "${package}" == "./internal/playground" ]]; then
	planner_tests="${ordinary_discovery}"
	planner_shards=5
	plan_mode=playground
elif [[ "${package}" == "./internal/apextest" ]]; then
	planner_shards=16
	plan_mode=apextest
elif [[ "${package}" == "./internal/gladecli" ]]; then
	planner_shards=16
elif [[ "${package}" == "./internal/vm" ]]; then
	planner_shards="${browser_shard_count}"
elif [[ "${package}" == "./internal/server" || "${package}" == "./internal/lwc/compile" ]]; then
	planner_shards=8
fi
planner_args=(--package "${package_name}" --shards "${planner_shards}" --tests "${planner_tests}")
if [[ "${package}" == "./internal/lwc/compile" && -s "${CI_LWC_COMPILE_HISTORY_PATH:-}" ]]; then
	planner_args+=(--history "${CI_LWC_COMPILE_HISTORY_PATH}")
fi
if [[ "${package}" == "./internal/vm" ]]; then
	# The VM shards balance by the raced durations: the measured history
	# first, the checked-in normal table scaled by the multiplier for tests
	# the recorded run never measured. The plan also derives the shard
	# count and the isolated giant tests from that history; the isolated
	# names pin one test per trailing shard.
	vm_race_history="${artifact_dir}/vm-race-history.json"
	python3 "${script_dir}/ci-race-plan.py" vm-history "${discovery}" >"${vm_race_history}"
	planner_args+=(--history "${vm_race_history}")
	if [[ -n "${CI_RACE_ISOLATED_TESTS:-}" ]]; then
		planner_args+=(--isolate "${CI_RACE_ISOLATED_TESTS}")
	fi
fi
case "${package}" in
	./internal/gladecli|./internal/server|./internal/testdaemon|./internal/playground|./internal/visualforce)
		# These lanes have no checked-in normal duration table, so their
		# plans balance by the measured race rows alone; the planner weighs
		# tests the recorded run never measured at the measured median.
		lane_race_history="${artifact_dir}/lane-race-history.json"
		python3 "${script_dir}/ci-race-plan.py" lane-history "${slug}" "${discovery}" >"${lane_race_history}"
		planner_args+=(--history "${lane_race_history}")
		;;
esac
if [[ "${package}" == "./internal/apextest" && "${race_mode}" == "partition" ]]; then
	race_history="${artifact_dir}/race-history.json"
	python3 "${script_dir}/ci-race-plan.py" history "${discovery}" >"${race_history}"
	# Enough shards that the longest-processing-time bound fits the per-shard
	# budget cap: a package total never lands in one shard, and a single test
	# over the cap fails the derivation with its name.
	planner_shards="$(python3 "${script_dir}/ci-race-plan.py" shards "${discovery}" "${race_history}" | python3 -c 'import json,sys
print(json.load(sys.stdin)["shard_count"])')"
	if ! python3 - "${planner_shards}" <<'PY'
import sys
count = int(sys.argv[1])
if not 16 <= count <= 96:
    raise SystemExit(f"planned Apex shard count {count} is outside the 16..96 lane range")
PY
	then
		printf '[ci] the plan returned an invalid Apex shard count: %q\n' "${planner_shards}" >&2
		exit 2
	fi
	planner_args=(--package "${package_name}" --shards "${planner_shards}" --tests "${planner_tests}" --history "${race_history}")
fi
if [[ "${apextest_assigned}" == true ]]; then
	: # The partition's frozen plan already binds discovery to shard regexes.
elif [[ -n "${shard_planner}" ]]; then
	if [[ "${shard_planner}" == */* ]]; then
		[[ -x "${shard_planner}" ]] || { printf '[ci] shard planner is not executable: %s\n' "${shard_planner}" >&2; exit 2; }
	elif ! command -v "${shard_planner}" >/dev/null 2>&1; then
		printf '[ci] shard planner was not found: %s\n' "${shard_planner}" >&2
		exit 2
	fi
	"${shard_planner}" "${planner_args[@]}" >"${plan}"
else
	go run ./scripts/internal/cishard "${planner_args[@]}" >"${plan}"
fi

python3 - "${planner_tests}" "${plan}" "${artifact_dir}" "${package_name}" "${plan_mode}" "${planner_shards}" <<'PY'
import json
import re
import sys

discovery_path, plan_path, artifact_dir, package, mode, shard_count_text = sys.argv[1:]
shard_count = int(shard_count_text)
test_pattern = re.compile(r"Test[A-Za-z0-9_]*\Z")

def reject_duplicates(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError(f"duplicate JSON key {key!r}")
        value[key] = item
    return value

with open(discovery_path, encoding="utf-8") as source:
    discovered = source.read().splitlines()
with open(plan_path, encoding="utf-8") as source:
    plan = json.load(source, object_pairs_hook=reject_duplicates)
if not isinstance(plan, dict) or set(plan) != {"version", "package", "historyUsed", "shards"}:
    raise SystemExit("race planner returned an invalid plan schema")
if plan["version"] != 1 or isinstance(plan["version"], bool) or plan["package"] != package or not isinstance(plan["historyUsed"], bool):
    raise SystemExit("race planner returned wrong schema or package")
shards = plan["shards"]
if not isinstance(shards, list) or len(shards) != shard_count:
    raise SystemExit(f"race planner did not return exactly {shard_count} shards")
union = []
for index, shard in enumerate(shards):
    keys = {"index", "tests", "estimatedDurationMillis", "regex"}
    if not isinstance(shard, dict) or set(shard) != keys or shard["index"] != index or isinstance(shard["index"], bool):
        raise SystemExit(f"race planner returned invalid shard {index}")
    tests = shard["tests"]
    if not isinstance(tests, list) or not tests or tests != sorted(tests) or len(tests) != len(set(tests)):
        raise SystemExit(f"race planner returned empty, duplicate, or non-canonical shard {index}")
    if any(not isinstance(name, str) or not test_pattern.fullmatch(name) for name in tests):
        raise SystemExit(f"race planner returned invalid test in shard {index}")
    estimate = shard["estimatedDurationMillis"]
    if isinstance(estimate, bool) or not isinstance(estimate, int) or estimate < 0:
        raise SystemExit(f"race planner returned invalid estimate in shard {index}")
    canonical_regex = "^(?:" + "|".join(re.escape(name) for name in tests) + ")$"
    if shard["regex"] != canonical_regex:
        raise SystemExit(f"race planner returned non-canonical regex in shard {index}")
    union.extend(tests)
    lane_prefix = "ordinary" if mode == "playground" else "shard"
    shard_dir = f"{artifact_dir}/{lane_prefix}-{index}"
    import os
    os.makedirs(shard_dir, exist_ok=True)
    with open(f"{shard_dir}/selection.json", "w", encoding="utf-8") as target:
        json.dump(shard, target, sort_keys=True, indent=2)
        target.write("\n")
    with open(f"{shard_dir}/regex.txt", "w", encoding="utf-8") as target:
        target.write(canonical_regex + "\n")
if len(union) != len(set(union)) or sorted(union) != discovered:
    raise SystemExit("race planner shard union does not exactly match planner discovery")
if mode == "playground":
    groups = [
        "TestExampleProjectsRunAnonymousGroupOne",
        "TestExampleProjectsRunAnonymousGroupTwo",
        "TestExampleProjectsRunAnonymousGroupThree",
        "TestExampleProjectsRunAnonymousGroupFour",
    ]
    for index, name in enumerate(groups):
        lane_dir = f"{artifact_dir}/group-{index}"
        os.makedirs(lane_dir, exist_ok=True)
        selection = {"index": index, "tests": [name], "estimatedDurationMillis": 0, "regex": "^(?:" + re.escape(name) + ")$"}
        with open(f"{lane_dir}/selection.json", "w", encoding="utf-8") as target:
            json.dump(selection, target, sort_keys=True, indent=2)
            target.write("\n")
        with open(f"{lane_dir}/regex.txt", "w", encoding="utf-8") as target:
            target.write(selection["regex"] + "\n")
PY

# The per-runner validation record is written at three points: an incomplete
# record before the lanes run (so a killed runner leaves evidence naming its
# shard), a fail record when the lane's go test fails or times out, and the
# success record after the union validates. The aggregates read the outcome
# and report a failed or killed shard instead of dying on a missing file.
race_lane_failure_summary() {
	python3 - "${1:-}" <<'PY'
import json
import sys

failing, running = [], []
if sys.argv[1]:
    terminals, started = {}, set()
    try:
        with open(sys.argv[1], encoding="utf-8") as source:
            for line in source:
                if not line.strip():
                    continue
                try:
                    event = json.loads(line)
                except ValueError:
                    continue
                if not isinstance(event, dict):
                    continue
                name, action = event.get("Test"), event.get("Action")
                if not isinstance(name, str) or "/" in name:
                    continue
                if action in ("pass", "fail", "skip"):
                    terminals.setdefault(name, []).append(action)
                elif action == "run":
                    started.add(name)
        for name in sorted(terminals):
            if "fail" in terminals[name]:
                failing.append(name)
        for name in sorted(started):
            if name not in terminals:
                running.append(name)
    except OSError:
        pass
summary = {"failing_tests": failing, "running_tests": running}
print(json.dumps(summary, sort_keys=True, separators=(",", ":")))
PY
}

write_browser_shard_validation() {
	local outcome="$1" exit_status="$2" events="${3:-}"
	local failure_summary
	failure_summary="$(race_lane_failure_summary "${events}")"
	python3 - "${artifact_dir}" "${browser_shard_index}" "${apextest_head_sha}" "${package_name}" "${outcome}" "${exit_status}" "${failure_summary}" <<'PY'
import hashlib
import json
import os
import sys
import tempfile

root, index_text, head, package, outcome, exit_status_text, failure_summary = sys.argv[1:]
index, exit_status = int(index_text), int(exit_status_text)
result = {"schema_version": 1, "package": package, "head_sha": head, "shard_index": index, "outcome": outcome}
for name, filename in (("discovery", "discovery.txt"), ("plan", "plan.json")):
    with open(os.path.join(root, filename), "rb") as source:
        result[name + "_sha256"] = hashlib.sha256(source.read()).hexdigest()
result["failure"] = None
if outcome == "fail":
    result["failure"] = json.loads(failure_summary)
    result["failure"]["exit_status"] = exit_status
descriptor, temporary = tempfile.mkstemp(prefix=".runner-validation.", dir=root, text=True)
try:
    with os.fdopen(descriptor, "w", encoding="utf-8") as target:
        json.dump(result, target, sort_keys=True, separators=(",", ":"))
        target.write("\n")
    os.replace(temporary, os.path.join(root, "runner-validation.json"))
except BaseException:
    try:
        os.unlink(temporary)
    except FileNotFoundError:
        pass
    raise
if outcome == "fail":
    detail = f"go test exit {exit_status}"
    if result["failure"]["failing_tests"]:
        detail += "; failing tests: " + ", ".join(result["failure"]["failing_tests"])
    if result["failure"]["running_tests"]:
        detail += "; still running: " + ", ".join(result["failure"]["running_tests"])
    print(f"[ci] race shard {index} failed: {detail}", file=sys.stderr)
PY
}

write_apextest_runner_validation() {
	local outcome="$1" exit_status="$2" shard_index="$3" events="${4:-}"
	local failure_summary
	failure_summary="$(race_lane_failure_summary "${events}")"
	python3 - "${discovery}" "${plan}" "${binary_metadata}" "${artifact_dir}/runner-validation.json" "${package_name}" "${apextest_runner}" "${actual_apextest_head}" "${partition_digest}" "${partition_shard_count}" "${outcome}" "${exit_status}" "${shard_index}" "${failure_summary}" "${assigned_apextest_indexes[@]}" <<'PY'
import hashlib
import json
import os
import sys
import tempfile

discovery_path, plan_path, binary_path, output_path, package, runner, head_sha, partition_sha256, shard_count_text, outcome, exit_status_text, shard_index_text, failure_summary, *indexes = sys.argv[1:]
indexes = [int(index) for index in indexes]
shard_count, exit_status, shard_index = int(shard_count_text), int(exit_status_text), int(shard_index_text)
if len(indexes) != len(set(indexes)) or any(index < 0 or index >= shard_count for index in indexes):
    raise SystemExit("race apextest runner indexes must be unique shard indexes")
with open(discovery_path, "rb") as source:
    discovery = source.read()
with open(plan_path, "rb") as source:
    plan = source.read()
with open(binary_path, encoding="utf-8") as source:
    binary = json.load(source)
if not isinstance(binary, dict) or set(binary) != {"schema_version", "package", "sha256", "size_bytes", "removed"}:
    raise SystemExit("race apextest binary metadata has an invalid schema after cleanup")
if binary["schema_version"] != 1 or binary["package"] != package:
    raise SystemExit("race apextest binary metadata has invalid state")
if outcome == "success" and binary["removed"] is not True:
    raise SystemExit("race apextest binary metadata has invalid cleanup state")
value = {
    "schema_version": 1,
    "runner": runner,
    "package": package,
    "head_sha": head_sha,
    "assigned_indexes": indexes,
    "discovered_count": len(discovery.splitlines()),
    "discovery_sha256": hashlib.sha256(discovery).hexdigest(),
    "plan_sha256": hashlib.sha256(plan).hexdigest(),
    "partition_sha256": partition_sha256,
    "binary_sha256": binary["sha256"],
    "binary_size_bytes": binary["size_bytes"],
    "binary_removed": binary["removed"],
    "outcome": outcome,
    "failure": None,
}
if outcome == "fail":
    value["failure"] = json.loads(failure_summary)
    value["failure"]["exit_status"] = exit_status
    value["failure"]["shard"] = shard_index
descriptor, temporary = tempfile.mkstemp(prefix=".runner-validation.", dir=os.path.dirname(output_path), text=True)
try:
    with os.fdopen(descriptor, "w", encoding="utf-8") as target:
        json.dump(value, target, sort_keys=True, separators=(",", ":"))
        target.write("\n")
    os.replace(temporary, output_path)
except BaseException:
    try:
        os.unlink(temporary)
    except FileNotFoundError:
        pass
    raise
if outcome == "fail":
    detail = f"assigned shard {shard_index} go test exit {exit_status}"
    if value["failure"]["failing_tests"]:
        detail += "; failing tests: " + ", ".join(value["failure"]["failing_tests"])
    if value["failure"]["running_tests"]:
        detail += "; still running: " + ", ".join(value["failure"]["running_tests"])
    print(f"[ci] race apextest runner {runner} failed: {detail}", file=sys.stderr)
PY
}

if [[ -n "${browser_shard_index}" ]]; then
	write_browser_shard_validation incomplete 0 ""
fi
if [[ -n "${apextest_runner}" ]]; then
	write_apextest_runner_validation incomplete 0 0 ""
fi

if [[ "${race_mode}" == "partition" ]]; then
	partition_path="${artifact_dir}/partition.json"
	python3 "${script_dir}/ci-race-plan.py" schedule "${discovery}" "${plan}" "${binary_sha256}" "${binary_size}" >"${partition_path}"
	python3 "${script_dir}/ci-race-plan.py" verify "${artifact_dir}"
	if ! python3 - "${partition_path}" "${planner_shards}" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as source:
    partition = json.load(source)
if partition["shard_count"] != int(sys.argv[2]):
    raise SystemExit(f"scheduled shard count {partition['shard_count']} does not match the planned {sys.argv[2]}")
PY
	then
		printf '[ci] the scheduled partition does not match the planned shard count\n' >&2
		exit 2
	fi
	cleanup_apextest_binary
	trap - EXIT HUP INT TERM
	exit 0
fi

if [[ "${package}" == "./internal/apextest" ]]; then
	lane_names=()
	for index in "${assigned_apextest_indexes[@]}"; do
		lane_names+=("shard-${index}")
	done
	event_paths=()
	for lane_name in "${lane_names[@]}"; do
		shard_index="${lane_name#shard-}"
		if [[ "${apextest_assigned}" == true ]]; then
			shard_go_timeout="${partition_shard_timeouts["${shard_index}"]}s"
		else
			shard_go_timeout="${go_test_timeout}"
		fi
		shard_dir="${artifact_dir}/${lane_name}"
		events="${shard_dir}/events.json"
		resource="${shard_dir}/resource.json"
		regex="$(<"${shard_dir}/regex.txt")"
		: >"${events}"
		set +e
		run_apextest_command "${resource_runner}" "${resource}" "race-${slug}-${lane_name}" -- \
			bash -c '
set -euo pipefail
binary="$1"; expected_size="$2"; events="$3"; go_command="$4"; package="$5"; regex="$6"; package_dir="$7"; go_timeout="$8"
actual_size="$(python3 - "$binary" <<'"'"'PY'"'"'
import os
import stat
import sys
path = sys.argv[1]
try:
    info = os.stat(path, follow_symlinks=False)
except FileNotFoundError:
    raise SystemExit("race apextest binary is missing before direct shard")
if not stat.S_ISREG(info.st_mode) or info.st_size <= 0 or not os.access(path, os.X_OK):
    raise SystemExit("race apextest binary is not executable before direct shard")
print(info.st_size)
PY
)"
if [[ "$actual_size" != "$expected_size" ]]; then
    echo "race apextest binary changed before direct shard" >&2
    exit 125
fi
set +e
(cd "$package_dir" && "$binary" -test.v=test2json -test.count=1 -test.timeout="$go_timeout" -test.run="$regex") | \
    "$go_command" tool test2json -t -p "$package" >"$events"
statuses=("${PIPESTATUS[@]}")
set -e
if [[ "${statuses[0]}" -ne 0 ]]; then exit "${statuses[0]}"; fi
exit "${statuses[1]}"
' bash "${binary_path}" "${binary_size}" "${events}" "${go_command}" "${package_name}" "${regex}" "${PWD}/${package#./}" "${shard_go_timeout}"
		native_rc="$?"
		set -e
		set +e
		validate_resource_evidence "${resource}" "race-${slug}-${lane_name}" "${native_rc}"
		validation_rc="$?"
		set -e
		if [[ "${native_rc}" -ne 0 || "${validation_rc}" -ne 0 ]]; then
			if [[ -n "${apextest_runner}" ]]; then
				if [[ "${native_rc}" -ne 0 ]]; then
					write_apextest_runner_validation fail "${native_rc}" "${shard_index}" "${events}"
				else
					write_apextest_runner_validation fail "${validation_rc}" "${shard_index}" "${events}"
				fi
			fi
			if [[ "${native_rc}" -ne 0 ]]; then
				exit "${native_rc}"
			fi
			exit "${validation_rc}"
		fi
		validate_package_summary "${events}" "${shard_dir}/package-summary.json"
		python3 - "${shard_dir}/selection.json" "${events}" "${package_name}" "${lane_name}" "${shard_dir}/package-summary.json" <<'PY'
import json
import sys

selection_path, events_path, package, index, summary_path = sys.argv[1:]

def reject_duplicates(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError(f"duplicate JSON key {key!r}")
        value[key] = item
    return value

with open(selection_path, encoding="utf-8") as source:
    selection = json.load(source, object_pairs_hook=reject_duplicates)
if not isinstance(selection, dict) or set(selection) != {"index", "tests", "estimatedDurationMillis", "regex"}:
    raise SystemExit(f"race shard {index} selection has an invalid schema")
expected = selection["tests"]
with open(summary_path, encoding="utf-8") as source:
    deferred = {row["test"] for row in json.load(source)["deferred"]}
terminals = {}
with open(events_path, encoding="utf-8") as source:
    for number, line in enumerate(source, 1):
        if not line.strip():
            continue
        event = json.loads(line, object_pairs_hook=reject_duplicates)
        if not isinstance(event, dict):
            raise ValueError(f"shard {index} event {number} is not an object")
        if event.get("Package") != package:
            raise ValueError(f"shard {index} event {number} has wrong package")
        name = event.get("Test")
        action = event.get("Action")
        if isinstance(name, str) and "/" not in name and action in ("pass", "fail", "skip"):
            terminals.setdefault(name, []).append(action)
if set(terminals) != set(expected):
    raise SystemExit(f"race shard {index} terminal set does not match selection")
for name in expected:
    if terminals[name] != ["pass"] and not (name in deferred and terminals[name] == ["skip"]):
        raise SystemExit(f"race shard {index} test {name} does not have exactly one passing terminal")
PY
		event_paths+=("${events}")
	done

	if [[ -n "${apextest_runner}" ]]; then
		cleanup_apextest_binary
		trap - EXIT HUP INT TERM
		write_apextest_runner_validation success 0 0 ""
		exit 0
	fi

	python3 - "${discovery}" "${package_name}" "${union_validation}" "${event_paths[@]}" <<'PY'
import hashlib
import json
import os
import sys
import tempfile

discovery_path, package, output_path, *event_paths = sys.argv[1:]
if len(event_paths) != 16:
    raise SystemExit("race apextest union validation requires exactly 16 shard event files")
with open(discovery_path, encoding="utf-8") as source:
    expected = source.read().splitlines()
passed = []
shard_counts = []
deferred = []
for path in event_paths:
    shard_passed = []
    with open(os.path.join(os.path.dirname(path), "package-summary.json"), encoding="utf-8") as source:
        deferred.extend(json.load(source)["deferred"])
    with open(path, encoding="utf-8") as source:
        for line in source:
            if not line.strip():
                continue
            event = json.loads(line)
            name = event.get("Test")
            if event.get("Package") == package and isinstance(name, str) and "/" not in name and event.get("Action") in ("pass", "skip"):
                passed.append(name)
                shard_passed.append(name)
    shard_counts.append(len(shard_passed))
if len(passed) != len(set(passed)) or sorted(passed) != expected:
    raise SystemExit("race passing apextest shard union does not exactly match discovery")
canonical = "".join(name + "\n" for name in expected).encode("utf-8")
result = {"schema_version": 1, "package": package, "discovered_count": len(expected), "shard_counts": shard_counts, "names_sha256": hashlib.sha256(canonical).hexdigest(), "deferred": deferred, "valid": True}
descriptor, temporary = tempfile.mkstemp(prefix=".union-validation.", dir=os.path.dirname(output_path), text=True)
try:
    with os.fdopen(descriptor, "w", encoding="utf-8") as target:
        json.dump(result, target, sort_keys=True, separators=(",", ":"))
        target.write("\n")
    os.replace(temporary, output_path)
except BaseException:
    try:
        os.unlink(temporary)
    except FileNotFoundError:
        pass
    raise
PY
	cleanup_apextest_binary
	trap - EXIT HUP INT TERM
	exit 0
fi

lane_names=()
if [[ "${package}" == "./internal/playground" ]]; then
	lane_names=(group-0 group-1 group-2 group-3 ordinary-0 ordinary-1 ordinary-2 ordinary-3 ordinary-4)
elif [[ "${package}" == "./internal/gladecli" ]]; then
	lane_names=(shard-0 shard-1 shard-2 shard-3 shard-4 shard-5 shard-6 shard-7 shard-8 shard-9 shard-10 shard-11 shard-12 shard-13 shard-14 shard-15)
elif [[ "${package}" == "./internal/server" || "${package}" == "./internal/lwc/compile" ]]; then
	lane_names=(shard-0 shard-1 shard-2 shard-3 shard-4 shard-5 shard-6 shard-7)
else
	lane_names=(shard-0 shard-1 shard-2 shard-3)
fi
if [[ -n "${browser_shard_index}" ]]; then
	lane_names=("shard-${browser_shard_index}")
fi
event_paths=()
for lane_name in "${lane_names[@]}"; do
	shard_dir="${artifact_dir}/${lane_name}"
	events="${shard_dir}/events.json"
	resource="${shard_dir}/resource.json"
	regex="$(<"${shard_dir}/regex.txt")"
	: >"${events}"
	set +e
	"${resource_runner}" "${resource}" "race-${slug}-${lane_name}" -- \
		bash -c 'events="$1"; shift; "$@" >"${events}"' bash "${events}" \
		env CI_TEST_GO_COMMAND="${go_command}" python3 "${script_dir}/ci-test-binary.py" test "${go_test_package_args[@]}" -json -race -count=1 -timeout="${go_test_timeout}" -run "${regex}" "${package}"
	native_rc="$?"
	set -e
	set +e
	validate_resource_evidence "${resource}" "race-${slug}-${lane_name}" "${native_rc}"
	validation_rc="$?"
	set -e
	if [[ "${native_rc}" -ne 0 || "${validation_rc}" -ne 0 ]]; then
		if [[ -n "${browser_shard_index}" ]]; then
			if [[ "${native_rc}" -ne 0 ]]; then
				write_browser_shard_validation fail "${native_rc}" "${events}"
			else
				write_browser_shard_validation fail "${validation_rc}" "${events}"
			fi
		fi
		if [[ "${native_rc}" -ne 0 ]]; then
			exit "${native_rc}"
		fi
		exit "${validation_rc}"
	fi
	validate_package_summary "${events}" "${shard_dir}/package-summary.json"
	python3 - "${shard_dir}/selection.json" "${events}" "${package_name}" "${lane_name}" "${shard_dir}/package-summary.json" <<'PY'
import json
import sys

selection_path, events_path, package, index, summary_path = sys.argv[1:]

def reject_duplicates(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError(f"duplicate JSON key {key!r}")
        value[key] = item
    return value

with open(selection_path, encoding="utf-8") as source:
    expected = json.load(source, object_pairs_hook=reject_duplicates)["tests"]
with open(summary_path, encoding="utf-8") as source:
    deferred = {row["test"] for row in json.load(source)["deferred"]}
terminals = {}
with open(events_path, encoding="utf-8") as source:
    for number, line in enumerate(source, 1):
        if not line.strip():
            continue
        event = json.loads(line, object_pairs_hook=reject_duplicates)
        if not isinstance(event, dict):
            raise ValueError(f"shard {index} event {number} is not an object")
        if event.get("Package") != package:
            raise ValueError(f"shard {index} event {number} has wrong package")
        name = event.get("Test")
        action = event.get("Action")
        if isinstance(name, str) and "/" not in name and action in ("pass", "fail", "skip"):
            terminals.setdefault(name, []).append(action)
if set(terminals) != set(expected):
    raise SystemExit(f"race shard {index} terminal set does not match selection")
for name in expected:
    if terminals[name] != ["pass"] and not (name in deferred and terminals[name] == ["skip"]):
        raise SystemExit(f"race shard {index} test {name} does not have exactly one passing terminal")
PY
	event_paths+=("${events}")
done

if [[ -n "${browser_shard_index}" ]]; then
	write_browser_shard_validation success 0 ""
	exit 0
fi

python3 - "${discovery}" "${package_name}" "${union_validation}" "${event_paths[@]}" <<'PY'
import hashlib
import json
import os
import sys
import tempfile

discovery_path, package, output_path, *event_paths = sys.argv[1:]
if package == "github.com/glade-sh/glade/internal/playground":
    expected_lanes = 9
elif package == "github.com/glade-sh/glade/internal/gladecli":
    expected_lanes = 16
elif package in ("github.com/glade-sh/glade/internal/server", "github.com/glade-sh/glade/internal/lwc/compile"):
    expected_lanes = 8
else:
    expected_lanes = 4
if len(event_paths) != expected_lanes:
    raise SystemExit(f"race union validation requires exactly {expected_lanes} lane event files")
with open(discovery_path, encoding="utf-8") as source:
    expected = source.read().splitlines()
passed = []
shard_counts = []
deferred = []
for path in event_paths:
    shard_passed = []
    with open(os.path.join(os.path.dirname(path), "package-summary.json"), encoding="utf-8") as source:
        deferred.extend(json.load(source)["deferred"])
    with open(path, encoding="utf-8") as source:
        for line in source:
            if not line.strip():
                continue
            event = json.loads(line)
            name = event.get("Test")
            if event.get("Package") == package and isinstance(name, str) and "/" not in name and event.get("Action") in ("pass", "skip"):
                passed.append(name)
                shard_passed.append(name)
    shard_counts.append(len(shard_passed))
if len(passed) != len(set(passed)) or sorted(passed) != expected:
    raise SystemExit("race passing lane union does not exactly match discovery")
canonical = "".join(name + "\n" for name in expected).encode("utf-8")
result = {
    "schema_version": 1,
    "package": package,
    "discovered_count": len(expected),
    "shard_counts": shard_counts,
    "names_sha256": hashlib.sha256(canonical).hexdigest(),
    "deferred": deferred,
    "valid": True,
}
output_dir = os.path.dirname(output_path)
descriptor, temporary = tempfile.mkstemp(prefix=".union-validation.", dir=output_dir, text=True)
try:
    with os.fdopen(descriptor, "w", encoding="utf-8") as target:
        json.dump(result, target, sort_keys=True, separators=(",", ":"))
        target.write("\n")
    os.replace(temporary, output_path)
except BaseException:
    try:
        os.unlink(temporary)
    except FileNotFoundError:
        pass
    raise
PY
