#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -lt 4 || "$3" != "--" ]]; then
	echo "usage: scripts/ci-resource-run.sh <output.json> <lane> -- <command> [args...]" >&2
	exit 2
fi

output="$1"
lane="$2"
shift 3

if [[ ! "${lane}" =~ ^[A-Za-z0-9._-]+$ ]]; then
	printf '[ci] invalid resource lane %q\n' "${lane}" >&2
	exit 2
fi

time_command="${CI_RESOURCE_TIME_COMMAND:-/usr/bin/time}"
if [[ "${time_command}" == */* ]]; then
	if [[ ! -x "${time_command}" ]]; then
		printf '[ci] resource timer is not executable: %s\n' "${time_command}" >&2
		exit 2
	fi
elif ! command -v "${time_command}" >/dev/null 2>&1; then
	printf '[ci] resource timer was not found: %s\n' "${time_command}" >&2
	exit 2
fi

mkdir -p "$(dirname "${output}")"
rm -f "${output}"
export CI_RESOURCE_LABEL="${lane}"
format="{\"schema_version\":1,\"lane\":\"${lane}\",\"elapsed_seconds\":%e,\"user_seconds\":%U,\"system_seconds\":%S,\"max_rss_kb\":%M}"

set +e
"${time_command}" -o "${output}" -f "${format}" --quiet -- "$@"
command_rc="$?"
set -e

if ! python3 - "${output}" "${lane}" "${command_rc}" <<'PY'
import json
import math
import os
import sys

path, expected_lane, expected_status = sys.argv[1], sys.argv[2], int(sys.argv[3])
try:
    with open(path, encoding="utf-8") as source:
        value = json.load(source)
    expected_keys = {
        "schema_version", "lane", "elapsed_seconds", "user_seconds",
        "system_seconds", "max_rss_kb",
    }
    if not isinstance(value, dict) or set(value) != expected_keys:
        raise ValueError("unexpected field set")
    schema_version = value["schema_version"]
    if isinstance(schema_version, bool) or not isinstance(schema_version, int) or schema_version != 1:
        raise ValueError("schema version mismatch")
    if value["lane"] != expected_lane:
        raise ValueError("lane mismatch")
    for field in ("elapsed_seconds", "user_seconds", "system_seconds", "max_rss_kb"):
        number = value[field]
        if isinstance(number, bool) or not isinstance(number, (int, float)) or not math.isfinite(number) or number < 0:
            raise ValueError(f"invalid {field}")
    value["exit_status"] = expected_status
    validated_path = path + ".validated"
    with open(validated_path, "w", encoding="utf-8") as target:
        json.dump(value, target, sort_keys=True, indent=2)
        target.write("\n")
    os.replace(validated_path, path)
except Exception as error:
    print(f"[ci] resource telemetry rejected: {error}", file=sys.stderr)
    raise SystemExit(1)
PY
then
	if [[ "${command_rc}" -ne 0 ]]; then
		exit "${command_rc}"
	fi
	exit 1
fi

printf '[ci] resource usage %s: ' "${lane}"
cat "${output}"
exit "${command_rc}"
