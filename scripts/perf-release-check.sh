#!/usr/bin/env bash
set -euo pipefail

usage() {
	cat >&2 <<'EOF'
usage: scripts/perf-release-check.sh --label <label> [--cache-mode <mode>] --output <directory> -- <command> [args...]

Runs one release/check phase without changing cache state and writes
<directory>/release-check.json.

Cache modes are caller-declared: cold, warm, no-go-cache, or unverified.
EOF
}

label=""
output=""
cache_mode="unverified"
cache_mode_declared=false
while [[ "$#" -gt 0 ]]; do
	case "$1" in
		--label)
			[[ "$#" -ge 2 ]] || { usage; exit 2; }
			label="$2"
			shift 2
			;;
		--output)
			[[ "$#" -ge 2 ]] || { usage; exit 2; }
			output="$2"
			shift 2
			;;
		--cache-mode)
			[[ "$#" -ge 2 ]] || { usage; exit 2; }
			cache_mode="$2"
			cache_mode_declared=true
			shift 2
			;;
		--)
			shift
			break
			;;
		*)
			usage
			exit 2
			;;
	esac
done

if [[ -z "${label}" || -z "${output}" || "$#" -eq 0 ]]; then
	usage
	exit 2
fi
if [[ ! "${label}" =~ ^[A-Za-z0-9._-]+$ ]]; then
	printf '[perf] invalid label %q\n' "${label}" >&2
	exit 2
fi
case "${cache_mode}" in
	cold|warm|no-go-cache|unverified)
		;;
	*)
		printf '[perf] invalid cache mode %q\n' "${cache_mode}" >&2
		exit 2
		;;
esac

command=("$@")
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${root}"

if ! command -v python3 >/dev/null 2>&1; then
	echo '[perf] python3 is required to write measurement JSON' >&2
	exit 2
fi
if [[ ! -x /usr/bin/time ]]; then
	echo '[perf] /usr/bin/time is required to measure release checks' >&2
	exit 2
fi

platform="$(uname -s)"
case "${platform}" in
	Darwin)
		cpu_count="$(sysctl -n hw.ncpu)"
		memory_bytes="$(sysctl -n hw.memsize)"
		;;
	Linux)
		cpu_count="$(getconf _NPROCESSORS_ONLN)"
		memory_bytes="$(( $(getconf PAGESIZE) * $(getconf _PHYS_PAGES) ))"
		;;
	*)
		printf '[perf] unsupported platform: %s\n' "${platform}" >&2
		exit 2
		;;
esac

commit_sha="$(git rev-parse --verify HEAD)"
if [[ -n "$(git status --porcelain=v1 --untracked-files=normal)" ]]; then
	dirty=true
else
	dirty=false
fi

mkdir -p -- "${output}"
raw_time="$(mktemp "${output}/.perf-release-check-time.XXXXXX")"
record_tmp="$(mktemp "${output}/.release-check.json.XXXXXX")"
cleanup() {
	rm -f -- "${raw_time}" "${record_tmp}"
}
trap cleanup EXIT

if [[ "${platform}" == "Darwin" ]]; then
	time_args=(-l -o "${raw_time}")
else
	time_args=(-v -o "${raw_time}")
fi

version_or_empty() {
	"$@" 2>/dev/null || true
}
go_version="$(version_or_empty go version)"
node_version="$(version_or_empty node --version)"
npm_version="$(version_or_empty npm --version)"
gomaxprocs="${GOMAXPROCS:-}"
selected_jobs="${LOCAL_GO_TEST_JOBS:-}"
if [[ "${LC_ALL+x}" == x ]]; then
	child_lc_all_state=set
	child_lc_all_value="${LC_ALL}"
else
	child_lc_all_state=unset
	child_lc_all_value=""
fi
if [[ "${GOCACHE+x}" == x ]]; then
	go_cache_environment_override=true
else
	go_cache_environment_override=false
fi
if [[ "${npm_config_cache+x}" == x || "${NPM_CONFIG_CACHE+x}" == x ]]; then
	npm_cache_environment_override=true
else
	npm_cache_environment_override=false
fi
if [[ "${GLADE_CACHE_DIR+x}" == x ]]; then
	glade_cache_environment_override=true
else
	glade_cache_environment_override=false
fi

set +e
LC_ALL=C /usr/bin/time "${time_args[@]}" -- bash -c '
if [[ "$1" == set ]]; then
  export LC_ALL="$2"
else
  unset LC_ALL
fi
shift 2
exec "$@"
' perf-release-check-child "${child_lc_all_state}" "${child_lc_all_value}" "${command[@]}"
command_rc="$?"
set -e

if ! python3 - "${raw_time}" "${record_tmp}" "${output}/release-check.json" "${platform}" "${label}" "${cache_mode}" "${cache_mode_declared}" "${go_cache_environment_override}" "${npm_cache_environment_override}" "${glade_cache_environment_override}" "${commit_sha}" "${dirty}" "$(uname -m)" "${cpu_count}" "${memory_bytes}" "${go_version}" "${node_version}" "${npm_version}" "${gomaxprocs}" "${selected_jobs}" "${command_rc}" "${command[@]}" <<'PY'
import json
import math
import os
import re
import sys

(
    raw_path,
    temporary_record,
    record_path,
    platform,
    label,
    cache_mode,
    cache_mode_declared,
    go_cache_environment_override,
    npm_cache_environment_override,
    glade_cache_environment_override,
    commit_sha,
    dirty_text,
    architecture,
    cpu_text,
    memory_text,
    go_version,
    node_version,
    npm_version,
    gomaxprocs,
    selected_jobs,
    exit_status_text,
    *command,
) = sys.argv[1:]

def required_int(value, name):
    try:
        parsed = int(value)
    except ValueError as error:
        raise ValueError(f"invalid {name}") from error
    if parsed < 1:
        raise ValueError(f"invalid {name}")
    return parsed

def optional_integer(value):
    if not value or value == "auto":
        # scripts/ci-go-test.sh keeps automatic local release checks serial.
        return 1
    try:
        parsed = int(value)
    except ValueError:
        return None
    return parsed if parsed > 0 else None

def parse_decimal(value, name):
    try:
        parsed = float(value)
    except ValueError as error:
        raise ValueError(f"invalid {name}") from error
    if not math.isfinite(parsed) or parsed < 0:
        raise ValueError(f"invalid {name}")
    return parsed

def match_number(patterns, text, name, required=True):
    for pattern in patterns:
        match = re.search(pattern, text, flags=re.MULTILINE | re.IGNORECASE)
        if match:
            return parse_decimal(match.group(1), name)
    if required:
        raise ValueError(f"missing {name}")
    return None

with open(raw_path, encoding="utf-8", errors="replace") as source:
    raw = source.read()

if platform == "Linux":
    wall_text = None
    match = re.search(r"^\s*Elapsed \(wall clock\) time \(h:mm:ss or m:ss\):\s*([^\s]+)\s*$", raw, flags=re.MULTILINE)
    if match:
        wall_text = match.group(1)
    if not wall_text:
        raise ValueError("missing wall_seconds")
    parts = wall_text.split(":")
    try:
        if len(parts) == 3:
            wall = int(parts[0]) * 3600 + int(parts[1]) * 60 + float(parts[2])
        elif len(parts) == 2:
            wall = int(parts[0]) * 60 + float(parts[1])
        else:
            wall = float(parts[0])
    except ValueError as error:
        raise ValueError("invalid wall_seconds") from error
    if not math.isfinite(wall) or wall < 0:
        raise ValueError("invalid wall_seconds")
    user = match_number([r"^\s*User time \(seconds\):\s*([0-9.]+)\s*$"], raw, "user_seconds")
    system = match_number([r"^\s*System time \(seconds\):\s*([0-9.]+)\s*$"], raw, "system_seconds")
    max_rss = int(match_number([r"^\s*Maximum resident set size \(kbytes\):\s*([0-9.]+)\s*$"], raw, "max_rss_kb")) * 1024
    file_inputs = match_number([r"^\s*File system inputs:\s*([0-9.]+)\s*$"], raw, "file_inputs", required=False)
    file_outputs = match_number([r"^\s*File system outputs:\s*([0-9.]+)\s*$"], raw, "file_outputs", required=False)
else:
    wall = match_number([r"^\s*([0-9.]+)\s+real(?:\s|$)"], raw, "wall_seconds")
    user = match_number([r"\s([0-9.]+)\s+user(?:\s|$)"], raw, "user_seconds")
    system = match_number([r"\s([0-9.]+)\s+sys(?:\s|$)"], raw, "system_seconds")
    max_rss = int(match_number([r"^\s*([0-9.]+)\s+maximum resident set size\s*$"], raw, "max_rss_bytes"))
    file_inputs = match_number([r"^\s*([0-9.]+)\s+(?:block input operations|file system inputs)\s*$"], raw, "file_inputs", required=False)
    file_outputs = match_number([r"^\s*([0-9.]+)\s+(?:block output operations|file system outputs)\s*$"], raw, "file_outputs", required=False)

if max_rss < 1:
    raise ValueError("invalid max_rss_bytes")

record = {
    "schema_version": 1,
    "label": label,
    "command": command,
    "cache": {
        "mode": cache_mode,
        "caller_verified": cache_mode_declared == "true" and cache_mode != "unverified",
        "go_cache_environment_override": go_cache_environment_override == "true",
        "npm_cache_environment_override": npm_cache_environment_override == "true",
        "glade_cache_environment_override": glade_cache_environment_override == "true",
    },
    "commit": {"sha": commit_sha, "dirty": dirty_text == "true"},
    "host": {
        "os": platform,
        "architecture": architecture,
        "cpus": required_int(cpu_text, "cpus"),
        "memory_bytes": required_int(memory_text, "memory_bytes"),
    },
    "toolchain": {
        "go_version": go_version,
        "node_version": node_version,
        "npm_version": npm_version,
        "gomaxprocs": gomaxprocs,
        "selected_job_count": optional_integer(selected_jobs),
    },
    "phases": [{
        "name": "release-check",
        "exit_status": int(exit_status_text),
        "resources": {
            "wall_seconds": wall,
            "user_seconds": user,
            "system_seconds": system,
            "max_rss_bytes": max_rss,
            "file_inputs": None if file_inputs is None else int(file_inputs),
            "file_outputs": None if file_outputs is None else int(file_outputs),
        },
    }],
}
with open(temporary_record, "w", encoding="utf-8") as target:
    json.dump(record, target, sort_keys=True, indent=2)
    target.write("\n")
os.replace(temporary_record, record_path)
PY
then
	if [[ "${command_rc}" -ne 0 ]]; then
		exit "${command_rc}"
	fi
	exit 1
fi

printf '[perf] release-check %s: ' "${label}"
cat "${output}/release-check.json"
exit "${command_rc}"
