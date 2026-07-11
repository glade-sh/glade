#!/usr/bin/env bash
set -euo pipefail

heartbeat_seconds="${CI_GO_TEST_HEARTBEAT_SECONDS:-60}"
if [[ ! "${heartbeat_seconds}" =~ ^[0-9]+$ ]] || [[ "${heartbeat_seconds}" -lt 1 ]]; then
	heartbeat_seconds=60
fi

export GOMAXPROCS="${GOMAXPROCS:-2}"

testlog_renderer_path=""
testlog_renderer_dir=""
testlog_status_files=()
owned_child_roots=()

cleanup_testlog_renderer() {
	local status_file
	for status_file in "${testlog_status_files[@]}"; do
		rm -f "${status_file}"
	done
	if [[ -n "${testlog_renderer_dir}" ]]; then
		rm -f "${testlog_renderer_dir}/testlog"
		rmdir "${testlog_renderer_dir}" 2>/dev/null || true
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
	printf '+ go test -json'
	printf ' %q' "$@"
	printf ' | testlog -output %q\n' "${artifact}"

	(
		set +e
		go test -json "$@" | tee "${artifact}" | run_testlog_renderer
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

remaining_packages() {
	go list ./... | grep -Ev '/internal/(apextest|gladecli|playground|sema|server)$' || true
}

run_core_tests() {
	local pkg
	local remaining
	for pkg in "${heavy_packages[@]}"; do
		run_json_with_heartbeat "go test ${pkg}" "$(testlog_artifact test "${pkg}")" -timeout=30m "${pkg}"
	done
	remaining="$(remaining_packages)"
	if [[ -n "${remaining}" ]]; then
		# Intentional word splitting: package import paths do not contain spaces.
		run_json_with_heartbeat "go test remaining packages" "$(testlog_artifact test remaining-packages)" -p=2 -timeout=20m ${remaining}
	fi
}

run_full_tests() {
	run_json_with_heartbeat "go test ./internal/apextest" "$(testlog_artifact test ./internal/apextest)" -timeout=30m ./internal/apextest
	run_core_tests
}

run_race_tests() {
	local pkg
	local remaining
	run_json_with_heartbeat "go test -race ./internal/apextest" "$(testlog_artifact race ./internal/apextest)" -race -timeout=60m ./internal/apextest
	for pkg in "${heavy_packages[@]}"; do
		run_json_with_heartbeat "go test -race ${pkg}" "$(testlog_artifact race "${pkg}")" -race -timeout=60m "${pkg}"
	done
	remaining="$(remaining_packages)"
	if [[ -n "${remaining}" ]]; then
		# Intentional word splitting: package import paths do not contain spaces.
		run_json_with_heartbeat "go test -race remaining packages" "$(testlog_artifact race remaining-packages)" -race -p=1 -timeout=30m ${remaining}
	fi
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

	set +e
	go test -list '^Test' ./internal/apextest >"${discovery_raw}" 2>"${discovery_stderr}"
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
		"${CI_SHARD_PLANNER}" --shards 2 --tests "${discovery}" >"${plan}"
	else
		go run ./scripts/internal/cishard --shards 2 --tests "${discovery}" >"${plan}"
	fi
	select_and_validate_shard "${discovery}" "${plan}" "${index}" "${selected}"
	regex="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["regex"])' "${selected}")"

	set +e
	run_json_with_heartbeat "go test Apex shard ${index}" "${events}" -timeout=30m -run "${regex}" ./internal/apextest
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
		apex-shard)
			run_apextest_matrix_shard "${2:-}"
			;;
		*)
			echo "usage: scripts/ci-go-test.sh [core|test|race|apex-shard 0|1]" >&2
			return 2
			;;
	esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
	main "$@"
fi
