#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 2 ]]; then
	echo "usage: scripts/ci-race-test.sh <package> <slug>" >&2
	exit 2
fi

package="$1"
slug="$2"
go_command="${CI_RACE_GO_COMMAND:-go}"
resource_runner="${CI_RACE_RESOURCE_RUNNER:-scripts/ci-resource-run.sh}"
shard_planner="${CI_RACE_SHARD_PLANNER:-}"
artifact_dir="ci-artifacts/race/${slug}"

if [[ ! "${package}" =~ ^\./[A-Za-z0-9._/-]+$ ]] || [[ "${package}" == *...* ]]; then
	printf '[ci] invalid exact race package %q\n' "${package}" >&2
	exit 2
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
	./internal/apextest|./internal/gladecli|./internal/playground)
		heavy_package=true
		;;
	*)
		set +e
		"${resource_runner}" "${artifact_dir}/resource.json" "race-${slug}" -- \
			"${go_command}" test -race -count=1 -timeout=60m "${package}"
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
	exec "${timeout_command}" --signal=TERM --kill-after=30s 60m \
		"${BASH}" "${BASH_SOURCE[0]}" "${package}" "${slug}"
fi

package_name="github.com/glade-sh/glade/${package#./}"
discovery_raw="${artifact_dir}/discovery-command.txt"
discovery="${artifact_dir}/discovery.txt"
discovery_benchmarks="${artifact_dir}/discovery-benchmarks.txt"
ordinary_discovery="${artifact_dir}/ordinary-discovery.txt"
plan="${artifact_dir}/plan.json"
union_validation="${artifact_dir}/union-validation.json"
union_sentinel="${union_validation}.tmp.$$"
if [[ "${package}" == "./internal/playground" ]]; then
	sentinel_counts='[0,0,0,0,0,0,0,0,0]'
else
	sentinel_counts='[0,0,0,0]'
fi
printf '{"schema_version":1,"package":"%s","discovered_count":0,"shard_counts":%s,"names_sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","valid":false}\n' "${package_name}" "${sentinel_counts}" >"${union_sentinel}"
mv "${union_sentinel}" "${union_validation}"

"${go_command}" test -race -list '.' "${package}" >"${discovery_raw}"
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
        else:
            raise SystemExit(f"invalid race discovery line {number}: {line!r}")
if not names:
    raise SystemExit("race discovery contains no top-level tests")
if len(names) != len(set(names)):
    raise SystemExit("race discovery contains duplicate top-level tests")
if len(benchmarks) != len(set(benchmarks)):
    raise SystemExit("race discovery contains duplicate benchmarks")
if trailers != 1:
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

planner_tests="${discovery}"
planner_shards=4
plan_mode=generic
if [[ "${package}" == "./internal/playground" ]]; then
	planner_tests="${ordinary_discovery}"
	planner_shards=5
	plan_mode=playground
fi
if [[ -n "${shard_planner}" ]]; then
	if [[ "${shard_planner}" == */* ]]; then
		[[ -x "${shard_planner}" ]] || { printf '[ci] shard planner is not executable: %s\n' "${shard_planner}" >&2; exit 2; }
	elif ! command -v "${shard_planner}" >/dev/null 2>&1; then
		printf '[ci] shard planner was not found: %s\n' "${shard_planner}" >&2
		exit 2
	fi
	"${shard_planner}" --package "${package_name}" --shards "${planner_shards}" --tests "${planner_tests}" >"${plan}"
else
	go run ./scripts/internal/cishard --package "${package_name}" --shards "${planner_shards}" --tests "${planner_tests}" >"${plan}"
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

lane_names=()
if [[ "${package}" == "./internal/playground" ]]; then
	lane_names=(group-0 group-1 group-2 group-3 ordinary-0 ordinary-1 ordinary-2 ordinary-3 ordinary-4)
else
	lane_names=(shard-0 shard-1 shard-2 shard-3)
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
		"${go_command}" test -json -race -count=1 -timeout=60m -run "${regex}" "${package}"
	native_rc="$?"
	set -e
	set +e
	validate_resource_evidence "${resource}" "race-${slug}-${lane_name}" "${native_rc}"
	validation_rc="$?"
	set -e
	if [[ "${native_rc}" -ne 0 ]]; then
		exit "${native_rc}"
	fi
	if [[ "${validation_rc}" -ne 0 ]]; then
		exit "${validation_rc}"
	fi
	python3 - "${shard_dir}/selection.json" "${events}" "${package_name}" "${lane_name}" <<'PY'
import json
import sys

selection_path, events_path, package, index = sys.argv[1:]

def reject_duplicates(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError(f"duplicate JSON key {key!r}")
        value[key] = item
    return value

with open(selection_path, encoding="utf-8") as source:
    expected = json.load(source, object_pairs_hook=reject_duplicates)["tests"]
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
    if terminals[name] != ["pass"]:
        raise SystemExit(f"race shard {index} test {name} does not have exactly one passing terminal")
PY
	event_paths+=("${events}")
done

python3 - "${discovery}" "${package_name}" "${union_validation}" "${event_paths[@]}" <<'PY'
import hashlib
import json
import os
import sys
import tempfile

discovery_path, package, output_path, *event_paths = sys.argv[1:]
expected_lanes = 9 if package == "github.com/glade-sh/glade/internal/playground" else 4
if len(event_paths) != expected_lanes:
    raise SystemExit(f"race union validation requires exactly {expected_lanes} lane event files")
with open(discovery_path, encoding="utf-8") as source:
    expected = source.read().splitlines()
passed = []
shard_counts = []
for path in event_paths:
    shard_passed = []
    with open(path, encoding="utf-8") as source:
        for line in source:
            if not line.strip():
                continue
            event = json.loads(line)
            name = event.get("Test")
            if event.get("Package") == package and isinstance(name, str) and "/" not in name and event.get("Action") == "pass":
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
