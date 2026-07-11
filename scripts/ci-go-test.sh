#!/usr/bin/env bash
set -euo pipefail

heartbeat_seconds="${CI_GO_TEST_HEARTBEAT_SECONDS:-60}"
if [[ ! "${heartbeat_seconds}" =~ ^[0-9]+$ ]] || [[ "${heartbeat_seconds}" -lt 1 ]]; then
	heartbeat_seconds=60
fi

export GOMAXPROCS="${GOMAXPROCS:-2}"

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"

testlog_renderer_path=""
testlog_renderer_dir=""
testlog_status_files=()
owned_child_roots=()
package_map_temp=""
package_lane_rows=""

cleanup_testlog_renderer() {
	local status_file
	for status_file in "${testlog_status_files[@]}"; do
		rm -f "${status_file}"
	done
	if [[ -n "${testlog_renderer_dir}" ]]; then
		rm -f "${testlog_renderer_dir}/testlog"
		rmdir "${testlog_renderer_dir}" 2>/dev/null || true
	fi
	if [[ -n "${package_map_temp}" ]]; then
		rm -f "${package_map_temp}"
	fi
}
trap cleanup_testlog_renderer EXIT

register_owned_child() {
	owned_child_roots+=("$1")
}

unregister_owned_child() {
	local completed_pid="$1"
	local pid
	local -a remaining=()
	for pid in "${owned_child_roots[@]}"; do
		if [[ "${pid}" != "${completed_pid}" ]]; then
			remaining+=("${pid}")
		fi
	done
	owned_child_roots=("${remaining[@]}")
}

terminate_owned_tree() {
	local root_pid="$1"
	local child_pid
	local children
	children="$(pgrep -P "${root_pid}" 2>/dev/null || true)"
	for child_pid in ${children}; do
		terminate_owned_tree "${child_pid}"
	done
	kill -KILL "${root_pid}" 2>/dev/null || true
}

terminate_owned_children() {
	local root_pid
	for root_pid in "${owned_child_roots[@]}"; do
		terminate_owned_tree "${root_pid}"
	done
	for root_pid in "${owned_child_roots[@]}"; do
		wait "${root_pid}" 2>/dev/null || true
	done
	owned_child_roots=()
}

handle_wrapper_signal() {
	local rc="$1"
	trap - INT TERM
	terminate_owned_children
	exit "${rc}"
}

trap 'handle_wrapper_signal 130' INT
trap 'handle_wrapper_signal 143' TERM

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
	register_owned_child "${pid}"

	(
		elapsed=0
		sleep_pid=""
		trap 'if [[ -n "${sleep_pid}" ]]; then kill "${sleep_pid}" 2>/dev/null || true; fi; exit 0' TERM INT
		while true; do
			sleep "${heartbeat_seconds}" &
			sleep_pid="$!"
			wait "${sleep_pid}" || exit 0
			sleep_pid=""
			if ! kill -0 "${pid}" 2>/dev/null; then
				exit 0
			fi
			elapsed=$((elapsed + heartbeat_seconds))
			printf '[ci] %s still running after %ss\n' "${label}" "${elapsed}"
		done
	) &
	heartbeat_pid="$!"
	register_owned_child "${heartbeat_pid}"

	wait "${pid}" || rc="$?"
	unregister_owned_child "${pid}"
	kill "${heartbeat_pid}" 2>/dev/null || true
	wait "${heartbeat_pid}" 2>/dev/null || true
	unregister_owned_child "${heartbeat_pid}"
	echo "::endgroup::"
	return "${rc}"
}

prepare_testlog_renderer() {
	if [[ -n "${testlog_renderer_path}" ]]; then
		return
	fi
	if [[ -n "${CI_TESTLOG_RENDERER:-}" ]]; then
		testlog_renderer_path="${CI_TESTLOG_RENDERER}"
		return
	fi
	testlog_renderer_dir="$(mktemp -d "${TMPDIR:-/tmp}/glade-ci-testlog-bin.XXXXXX")"
	testlog_renderer_path="${testlog_renderer_dir}/testlog"
	if ! go build -o "${testlog_renderer_path}" ./scripts/internal/testlog; then
		echo "[ci] testlog renderer build failed; live output unavailable" >&2
		testlog_renderer_path="/usr/bin/false"
		return 1
	fi
}

run_testlog_renderer() {
	local renderer_rc=0
	local -a args=(-output /dev/null)
	if [[ "${CI_VERBOSE:-0}" == "1" ]]; then
		args+=(-verbose)
	fi
	"${testlog_renderer_path}" "${args[@]}" || renderer_rc="$?"
	# A renderer that exits early must not close the test pipeline. Drain the
	# remaining stream so tee can finish the raw artifact and go test can exit.
	cat >/dev/null
	return "${renderer_rc}"
}

run_json_with_heartbeat() {
	local label="$1"
	local artifact="$2"
	shift 2
	local status_file
	local pid
	local heartbeat_pid
	local native_rc
	local tee_rc
	local renderer_rc

	mkdir -p "$(dirname "${artifact}")"
	prepare_testlog_renderer || true
	status_file="$(mktemp "${TMPDIR:-/tmp}/glade-ci-testlog.XXXXXX")"
	testlog_status_files+=("${status_file}")
	echo "::group::${label}"
	printf '[ci] GOMAXPROCS=%s\n' "${GOMAXPROCS}"
	printf '+ go test -json -vet=off'
	printf ' %q' "$@"
	printf ' | testlog -output %q\n' "${artifact}"

	(
		set +e
		go test -json -vet=off "$@" | tee "${artifact}" | run_testlog_renderer
		pipeline_status=("${PIPESTATUS[@]}")
		printf '%s %s %s\n' "${pipeline_status[0]}" "${pipeline_status[1]}" "${pipeline_status[2]}" >"${status_file}"
	) &
	pid="$!"
	register_owned_child "${pid}"
	(
		elapsed=0
		sleep_pid=""
		trap 'if [[ -n "${sleep_pid}" ]]; then kill "${sleep_pid}" 2>/dev/null || true; fi; exit 0' TERM INT
		while true; do
			sleep "${heartbeat_seconds}" &
			sleep_pid="$!"
			wait "${sleep_pid}" || exit 0
			sleep_pid=""
			if ! kill -0 "${pid}" 2>/dev/null; then
				exit 0
			fi
			elapsed=$((elapsed + heartbeat_seconds))
			printf '[ci] %s still running after %ss\n' "${label}" "${elapsed}"
		done
	) &
	heartbeat_pid="$!"
	register_owned_child "${heartbeat_pid}"
	wait "${pid}" || true
	unregister_owned_child "${pid}"
	kill "${heartbeat_pid}" 2>/dev/null || true
	wait "${heartbeat_pid}" 2>/dev/null || true
	unregister_owned_child "${heartbeat_pid}"
	echo "::endgroup::"
	if ! read -r native_rc tee_rc renderer_rc <"${status_file}"; then
		rm -f "${status_file}"
		echo "[ci] unable to read go test pipeline status" >&2
		return 1
	fi
	rm -f "${status_file}"
	if [[ "${tee_rc}" -ne 0 ]]; then
		printf '[ci] raw event writer failed with status %s; artifact: %s\n' "${tee_rc}" "${artifact}" >&2
	fi
	if [[ "${renderer_rc}" -ne 0 ]]; then
		printf '[ci] testlog renderer failed with status %s; raw events: %s\n' "${renderer_rc}" "${artifact}" >&2
	fi
	return "${native_rc}"
}

testlog_artifact() {
	local kind="$1"
	local pkg="$2"
	local name="${pkg#./}"
	name="${name//\//-}"
	printf '%s/%s-%s.json\n' "${CI_GO_TEST_ARTIFACT_DIR:-ci-artifacts/go-test}" "${kind}" "${name}"
}

load_package_lanes() {
	if [[ -n "${package_lane_rows}" ]]; then
		return
	fi
	local go_command="${CI_GO_COMMAND:-go}"
	package_map_temp="$(mktemp "${TMPDIR:-/tmp}/glade-ci-packages.XXXXXX")"
	(cd "${repo_root}" && "${go_command}" list ./...) >"${package_map_temp}"
	package_lane_rows="$(cd "${repo_root}" && "${go_command}" run ./scripts/internal/cishard --package-manifest "${script_dir}/ci-package-lanes.json" --packages "${package_map_temp}")"
}

package_lane_packages() {
	local lane="$1"
	load_package_lanes
	awk -F '\t' -v lane="${lane}" '$1 == lane { print $2 }' <<<"${package_lane_rows}"
}

run_package_lane() {
	local lane="$1"
	local kind="$2"
	local timeout="$3"
	local parallelism="$4"
	local -a packages=()
	local -a args=()
	load_package_lanes
	while IFS= read -r pkg; do
		packages+=("${pkg}")
	done < <(awk -F '\t' -v lane="${lane}" '$1 == lane { print $2 }' <<<"${package_lane_rows}")
	if [[ "${#packages[@]}" -eq 0 ]]; then
		echo "[ci] package lane ${lane} is empty" >&2
		return 1
	fi
	if [[ "${kind}" == "race" ]]; then
		args+=(-race)
	fi
	if [[ "${parallelism}" != "0" ]]; then
		args+=(-p="${parallelism}")
	fi
	args+=(-timeout="${timeout}")
	run_json_with_heartbeat "go test ${lane}" "$(testlog_artifact "${kind}" "${lane}")" "${args[@]}" "${packages[@]}"
}

run_core_tests() {
	run_named_package_lane "gladecli"
	run_named_package_lane "sema"
	run_named_package_lane "server-and-playground"
	run_named_package_lane "repoguard"
	run_named_package_lane "remaining-go"
}

run_named_package_lane() {
	local lane="$1"
	case "${lane}" in
		gladecli|sema|server-and-playground|repoguard)
			run_package_lane "${lane}" test 30m 0
			;;
		remaining-go)
			run_package_lane "${lane}" test 20m 2
			;;
		apextest)
			echo "[ci] apextest must use dedicated apex-shard routing" >&2
			return 2
			;;
		*)
			echo "[ci] unknown package lane: ${lane}" >&2
			return 2
			;;
	esac
}

run_full_tests() {
	run_package_lane "apextest" test 30m 0
	run_core_tests
}

run_race_tests() {
	run_package_lane "apextest" race 60m 0
	run_package_lane "gladecli" race 60m 0
	run_package_lane "sema" race 60m 0
	run_package_lane "server-and-playground" race 60m 0
	run_package_lane "repoguard" race 60m 0
	run_package_lane "remaining-go" race 30m 1
}

validate_discovery() {
	local raw_path="$1"
	local discovery_path="$2"
	python3 - "${raw_path}" "${discovery_path}" <<'PY'
import re
import sys

raw_path, discovery_path = sys.argv[1:]
names = []
with open(raw_path, encoding="utf-8") as source:
    for raw_line in source:
        line = raw_line.rstrip("\n")
        if re.fullmatch(r"Test[A-Za-z0-9_]*", line):
            names.append(line)
        elif re.match(r"^ok\s+github\.com/glade-sh/glade/internal/apextest(?:\s|$)", line):
            continue
        elif line:
            raise SystemExit(f"invalid Apex discovery output: {line!r}")
if not names:
    raise SystemExit("no Apex tests discovered")
if len(names) != len(set(names)):
    raise SystemExit("duplicate Apex test name")
names.sort()
with open(discovery_path, "w", encoding="utf-8") as target:
    target.write("\n".join(names) + "\n")
PY
}

select_and_validate_shard() {
	local discovery_path="$1"
	local plan_path="$2"
	local index="$3"
	local selected_path="$4"
	python3 - "${discovery_path}" "${plan_path}" "${index}" "${selected_path}" <<'PY'
import json
import re
import sys

discovery_path, plan_path, index_text, selected_path = sys.argv[1:]
index = int(index_text)
with open(discovery_path, encoding="utf-8") as source:
    discovered = [line.rstrip("\n") for line in source]
with open(plan_path, encoding="utf-8") as source:
    plan = json.load(source)
if plan.get("version") != 1 or plan.get("package") != "github.com/glade-sh/glade/internal/apextest":
    raise SystemExit("planner returned wrong schema or package")
shards = plan.get("shards")
if not isinstance(shards, list) or len(shards) != 2:
    raise SystemExit("planner did not return exactly two shards")
seen = []
for expected_index, shard in enumerate(shards):
    if not isinstance(shard, dict) or shard.get("index") != expected_index:
        raise SystemExit("planner returned invalid shard index")
    tests = shard.get("tests")
    if not isinstance(tests, list) or not tests or tests != sorted(tests):
        raise SystemExit("planner returned empty or non-canonical shard")
    if any(not isinstance(name, str) or not re.fullmatch(r"Test[A-Za-z0-9_]*", name) for name in tests):
        raise SystemExit("planner returned invalid test name")
    if not isinstance(shard.get("regex"), str) or not shard["regex"]:
        raise SystemExit("planner returned invalid regex")
    seen.extend(tests)
if len(seen) != len(set(seen)) or sorted(seen) != sorted(discovered):
    raise SystemExit("planner two-shard union does not match discovery")
with open(selected_path, "w", encoding="utf-8") as target:
    json.dump(shards[index], target, sort_keys=True, indent=2)
    target.write("\n")
PY
}

render_failure_output() {
	local events_path="$1"
	python3 - "${events_path}" <<'PY' || true
import json
import sys

try:
    with open(sys.argv[1], encoding="utf-8") as source:
        for line in source:
            event = json.loads(line)
            if isinstance(event.get("Output"), str):
                print(event["Output"], end="")
except Exception as error:
    print(f"unable to render Apex JSON failure output: {error}", file=sys.stderr)
PY
}

validate_apextest_results() {
	local selected_path="$1"
	local events_path="$2"
	local summary_path="$3"
	python3 - "${selected_path}" "${events_path}" "${summary_path}" <<'PY'
import json
import sys

selected_path, events_path, summary_path = sys.argv[1:]
summary = {"valid": False, "expected": [], "passed": [], "errors": []}
try:
    with open(selected_path, encoding="utf-8") as source:
        selected = json.load(source)
    expected = selected["tests"]
    summary["expected"] = expected
    terminal = {}
    with open(events_path, encoding="utf-8") as source:
        for line_number, line in enumerate(source, 1):
            if not line.strip():
                continue
            event = json.loads(line)
            if not isinstance(event, dict):
                raise ValueError(f"event {line_number} is not an object")
            package = event.get("Package")
            if package != "github.com/glade-sh/glade/internal/apextest":
                raise ValueError(f"event {line_number} has wrong package {package!r}")
            name = event.get("Test")
            action = event.get("Action")
            if not isinstance(name, str) or "/" in name or action not in ("pass", "fail", "skip"):
                continue
            terminal.setdefault(name, []).append(action)
    passed = sorted(name for name, actions in terminal.items() if actions == ["pass"])
    summary["passed"] = passed
    if passed != sorted(expected):
        summary["errors"].append("top-level passing set does not match selected shard")
    duplicates = sorted(name for name, actions in terminal.items() if len(actions) != 1)
    if duplicates:
        summary["errors"].append("duplicate top-level terminal results: " + ", ".join(duplicates))
    wrong = sorted(name for name, actions in terminal.items() if actions != ["pass"])
    if wrong:
        summary["errors"].append("non-passing top-level results: " + ", ".join(wrong))
    extras = sorted(set(terminal) - set(expected))
    if extras:
        summary["errors"].append("extra top-level results: " + ", ".join(extras))
    missing = sorted(set(expected) - set(terminal))
    if missing:
        summary["errors"].append("missing top-level results: " + ", ".join(missing))
    summary["valid"] = not summary["errors"]
except Exception as error:
    summary["errors"].append(str(error))
with open(summary_path, "w", encoding="utf-8") as target:
    json.dump(summary, target, sort_keys=True, indent=2)
    target.write("\n")
if not summary["valid"]:
    for error in summary["errors"]:
        print(f"Apex result validation: {error}", file=sys.stderr)
    raise SystemExit(1)
PY
}

run_apextest_matrix_shard() {
	local index="${1:-}"
	local apex_package="./internal/apextest"
	local -a apex_packages=()
	local artifact_suffix="invalid"
	if [[ "${index}" =~ ^[01]$ ]]; then
		artifact_suffix="${index}"
	fi
	local artifact_dir="${CI_APEXTEST_ARTIFACT_DIR:-ci-artifacts/apextest-${artifact_suffix}}"
	local discovery_raw="${artifact_dir}/discovery-command.txt"
	local discovery_stderr="${artifact_dir}/discovery-stderr.txt"
	local discovery="${artifact_dir}/discovery.txt"
	local plan="${artifact_dir}/plan.json"
	local selected="${artifact_dir}/selected-shard.json"
	local events="${artifact_dir}/events.json"
	local summary="${artifact_dir}/validation-summary.json"
	local regex
	local discovery_rc
	local native_rc
	local validation_rc=0

	mkdir -p "${artifact_dir}"
	: >"${discovery_raw}"
	: >"${discovery}"
	: >"${discovery_stderr}"
	: >"${plan}"
	: >"${selected}"
	: >"${events}"
	printf '{"valid": false, "errors": ["shard did not reach result validation"]}\n' >"${summary}"
	if [[ ! "${index}" =~ ^[01]$ ]]; then
		echo "shard index must be 0 or 1" >&2
		return 2
	fi
	if [[ -z "${CI_SHARD_PLANNER:-}" ]]; then
		while IFS= read -r pkg; do
			apex_packages+=("${pkg}")
		done < <(package_lane_packages "apextest")
		if [[ "${#apex_packages[@]}" -ne 1 ]]; then
			echo "[ci] apextest package lane must contain exactly one package" >&2
			return 1
		fi
		apex_package="${apex_packages[0]}"
	fi

	set +e
	GOFLAGS="${GOFLAGS:+${GOFLAGS} }-vet=off" go test -list '^Test' "${apex_package}" >"${discovery_raw}" 2>"${discovery_stderr}"
	discovery_rc="$?"
	set -e
	if [[ -s "${discovery_stderr}" ]]; then
		cat "${discovery_stderr}" >&2
	fi
	if [[ "${discovery_rc}" -ne 0 ]]; then
		cat "${discovery_raw}" >&2
		return "${discovery_rc}"
	fi
	validate_discovery "${discovery_raw}" "${discovery}"

	if [[ -n "${CI_SHARD_PLANNER:-}" ]]; then
		planner=("${CI_SHARD_PLANNER}" --shards 2 --tests "${discovery}")
	else
		planner=(go run ./scripts/internal/cishard --shards 2 --tests "${discovery}")
	fi
	if [[ -s "${CI_APEXTEST_HISTORY_PATH:-}" ]]; then
		planner+=(--history "${CI_APEXTEST_HISTORY_PATH}")
	fi
	"${planner[@]}" >"${plan}"
	select_and_validate_shard "${discovery}" "${plan}" "${index}" "${selected}"
	regex="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["regex"])' "${selected}")"

	set +e
	run_json_with_heartbeat "go test Apex shard ${index}" "${events}" -timeout=30m -run "${regex}" "${apex_package}"
	native_rc="$?"
	set -e
	validate_apextest_results "${selected}" "${events}" "${summary}" || validation_rc="$?"
	if [[ "${native_rc}" -ne 0 || "${validation_rc}" -ne 0 ]]; then
		render_failure_output "${events}"
	fi
	if [[ "${native_rc}" -ne 0 ]]; then
		return "${native_rc}"
	fi
	return "${validation_rc}"
}

refresh_apextest_history() {
	local shard_zero="${1:-}"
	local shard_one="${2:-}"
	local output="${3:-}"
	if [[ -z "${shard_zero}" || -z "${shard_one}" || -z "${output}" ]]; then
		echo "usage: scripts/ci-go-test.sh apex-history-refresh SHARD_0_DIR SHARD_1_DIR OUTPUT" >&2
		return 2
	fi
	python3 - "${shard_zero}" "${shard_one}" "${output}" <<'PY'
import json
import math
import os
import re
import sys
import tempfile

PACKAGE = "github.com/glade-sh/glade/internal/apextest"
TEST_COUNT = 279
TEST_NAME = re.compile(r"Test[A-Za-z0-9_]*\Z")
MAX_MILLIS = (1 << 63) - 1

shard_dirs = sys.argv[1:3]
output_path = sys.argv[3]
try:
    os.unlink(output_path)
except FileNotFoundError:
    pass

def reject(message):
    raise ValueError(message)

def no_duplicate_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            reject(f"duplicate JSON key {key!r}")
        result[key] = value
    return result

def load_json(path):
    with open(path, encoding="utf-8") as source:
        return json.load(source, object_pairs_hook=no_duplicate_object)

def require_exact_keys(value, keys, label):
    if not isinstance(value, dict) or set(value) != set(keys):
        reject(f"{label} does not have the exact schema")

def is_integer(value):
    return isinstance(value, int) and not isinstance(value, bool)

def load_discovery(path):
    with open(path, encoding="utf-8") as source:
        raw = source.read()
    names = raw.splitlines()
    if len(names) != TEST_COUNT:
        reject(f"discovery contains {len(names)} tests, want {TEST_COUNT}")
    if any(not TEST_NAME.fullmatch(name) for name in names):
        reject("discovery contains an invalid top-level test name")
    if len(set(names)) != len(names):
        reject("discovery contains duplicate tests")
    if names != sorted(names) or raw != "\n".join(names) + "\n":
        reject("discovery is not in exact canonical order and format")
    return names

def validate_shard(shard, index, label):
    require_exact_keys(shard, ("index", "tests", "estimatedDurationMillis", "regex"), label)
    if not is_integer(shard["index"]) or shard["index"] != index:
        reject(f"{label} has an invalid shard index")
    tests = shard["tests"]
    if not isinstance(tests, list) or not tests or tests != sorted(tests):
        reject(f"{label} has an empty or non-canonical test list")
    if any(not isinstance(name, str) or not TEST_NAME.fullmatch(name) for name in tests):
        reject(f"{label} has an invalid top-level test name")
    estimate = shard["estimatedDurationMillis"]
    if not is_integer(estimate) or estimate < 0:
        reject(f"{label} has an invalid estimated duration")
    canonical_regex = "^(?:" + "|".join(re.escape(name) for name in tests) + ")$"
    if not isinstance(shard["regex"], str) or shard["regex"] != canonical_regex:
        reject(f"{label} does not have the canonical exact-test regex")
    return tests

try:
    discoveries = [load_discovery(os.path.join(path, "discovery.txt")) for path in shard_dirs]
    if discoveries[0] != discoveries[1]:
        reject("shard discoveries do not match")
    discovery = discoveries[0]

    plans = [load_json(os.path.join(path, "plan.json")) for path in shard_dirs]
    if plans[0] != plans[1]:
        reject("shard plans do not match")
    plan = plans[0]
    require_exact_keys(plan, ("version", "package", "historyUsed", "shards"), "plan")
    if not is_integer(plan["version"]) or plan["version"] != 1 or not isinstance(plan["package"], str) or plan["package"] != PACKAGE:
        reject("plan has wrong schema or package")
    if not isinstance(plan["historyUsed"], bool):
        reject("plan historyUsed is not boolean")
    shards = plan["shards"]
    if not isinstance(shards, list) or len(shards) != 2:
        reject("plan does not contain exactly two shards")
    planned = []
    for index, shard in enumerate(shards):
        planned.extend(validate_shard(shard, index, f"plan shard {index}"))
    if len(planned) != len(set(planned)) or sorted(planned) != discovery:
        reject("plan union does not exactly match discovery")

    durations = {}
    total_duration_millis = 0
    shard_elapsed = []
    for index, shard_dir in enumerate(shard_dirs):
        selected = load_json(os.path.join(shard_dir, "selected-shard.json"))
        validate_shard(selected, index, f"selected shard {index}")
        if selected != shards[index]:
            reject(f"selected shard {index} does not match the canonical plan")
        selected_tests = selected["tests"]
        summary = load_json(os.path.join(shard_dir, "validation-summary.json"))
        require_exact_keys(summary, ("valid", "expected", "passed", "errors"), f"shard {index} validation summary")
        if summary["valid"] is not True:
            reject(f"shard {index} validation summary is not valid")
        if (not isinstance(summary["expected"], list) or
                any(not isinstance(name, str) for name in summary["expected"]) or
                not isinstance(summary["passed"], list) or
                any(not isinstance(name, str) for name in summary["passed"]) or
                not isinstance(summary["errors"], list) or
                any(not isinstance(error, str) for error in summary["errors"]) or
                summary["errors"] != [] or
                summary["expected"] != selected_tests or
                summary["passed"] != selected_tests):
            reject(f"shard {index} validation summary does not exactly match its selection")

        terminal = {}
        with open(os.path.join(shard_dir, "events.json"), encoding="utf-8") as source:
            for line_number, line in enumerate(source, 1):
                if not line.strip():
                    continue
                event = json.loads(line, object_pairs_hook=no_duplicate_object)
                if not isinstance(event, dict):
                    reject(f"shard {index} event {line_number} is not an object")
                if event.get("Package") != PACKAGE:
                    reject(f"shard {index} event {line_number} has the wrong package")
                name = event.get("Test")
                action = event.get("Action")
                if not isinstance(name, str) or "/" in name or action not in ("pass", "fail", "skip"):
                    continue
                terminal.setdefault(name, []).append(event)

        if set(terminal) != set(selected_tests):
            reject(f"shard {index} terminal test set does not match its selection")
        elapsed_total = 0.0
        for name in selected_tests:
            events = terminal[name]
            if len(events) != 1 or events[0].get("Action") != "pass":
                reject(f"shard {index} test {name} does not have one passing terminal result")
            elapsed = events[0].get("Elapsed")
            if isinstance(elapsed, bool) or not isinstance(elapsed, (int, float)) or not math.isfinite(elapsed) or elapsed < 0:
                reject(f"shard {index} test {name} has an invalid duration")
            millis = math.floor(elapsed * 1000 + 0.5)
            if millis > MAX_MILLIS:
                reject(f"shard {index} test {name} duration overflows schema v1")
            if millis > MAX_MILLIS - total_duration_millis:
                reject("duration total overflows schema v1 int64")
            total_duration_millis += millis
            durations[name] = millis
            elapsed_total += elapsed
        if not math.isfinite(elapsed_total):
            reject(f"shard {index} elapsed total is not finite")
        shard_elapsed.append(elapsed_total)

    if len(durations) != TEST_COUNT or sorted(durations) != discovery:
        reject("passing terminal union does not exactly match all discovered tests")
    # With two shards, median is their arithmetic mean. The exact 1.5x
    # boundary is accepted; only a strict excess fails the refresh.
    median = (shard_elapsed[0] + shard_elapsed[1]) / 2.0
    limit = 1.5 * median
    for index, elapsed in enumerate(shard_elapsed):
        if elapsed > limit:
            reject(f"shard {index} elapsed {elapsed:.6f}s exceeds 1.5x two-shard median {median:.6f}s")

    history = {
        "version": 1,
        "package": PACKAGE,
        "complete": True,
        "tests": [{"name": name, "durationMillis": durations[name]} for name in discovery],
    }
    output_dir = os.path.dirname(output_path) or "."
    os.makedirs(output_dir, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=".apextest-duration-history.", dir=output_dir, text=True)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as target:
            json.dump(history, target, sort_keys=True, indent=2, allow_nan=False)
            target.write("\n")
        os.replace(temporary, output_path)
    except BaseException:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise
    print(f"[ci] Apex duration history refreshed: tests={TEST_COUNT} shard_elapsed={shard_elapsed} median={median:.6f}s")
except Exception as error:
    print(f"[ci] Apex duration history refresh rejected: {error}", file=sys.stderr)
    raise SystemExit(1)
PY
}

main() {
	case "${1:-test}" in
		test)
			run_full_tests
			;;
		core)
			run_core_tests
			;;
		race)
			run_race_tests
			;;
		lane)
			if [[ "$#" -ne 2 ]]; then
				echo "usage: scripts/ci-go-test.sh lane {gladecli|sema|server-and-playground|repoguard|remaining-go}" >&2
				return 2
			fi
			run_named_package_lane "$2"
			;;
		apex-shard)
			run_apextest_matrix_shard "${2:-}"
			;;
		apex-history-refresh)
			refresh_apextest_history "${2:-}" "${3:-}" "${4:-}"
			;;
		*)
			echo "usage: scripts/ci-go-test.sh [core|test|race|lane NAME|apex-shard 0|1|apex-history-refresh SHARD_0_DIR SHARD_1_DIR OUTPUT]" >&2
			return 2
			;;
	esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
	main "$@"
fi
