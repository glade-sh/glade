#!/usr/bin/env bash
# Stage one Apex scheduling input. Neither this profile nor a cache hit is
# evidence that the current run passed its test or balance gates.
set -euo pipefail
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
: "${CI_APEXTEST_HISTORY_PATH:?}" "${CI_DURATION_HISTORY_CACHE_REF:?}" "${GITHUB_RUN_ID:?}" "${GITHUB_RUN_ATTEMPT:?}" "${GITHUB_REPOSITORY:?}"
stage=${CI_APEXTEST_HISTORY_STAGE:-ci-artifacts/apextest-history-input}
pair=${CI_APEXTEST_BENCHMARK_PAIR:-0}
tmp=$(mktemp -d "${RUNNER_TEMP:-/tmp}/apex-history-input.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

# Validate scheduling-only inputs regardless of whether they came from cache
# or an artifact. The planner separately checks the current test discovery.
valid_profile() {
  python3 - "$1" <<'PY'
import json, re, sys

def unique(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError('duplicate key')
        value[key] = item
    return value
def require(condition):
    if not condition:
        raise ValueError('invalid profile')

try:
    with open(sys.argv[1], encoding='utf-8') as source:
        profile = json.load(source, object_pairs_hook=unique)
    require(type(profile) is dict and set(profile) == {'version', 'package', 'complete', 'tests'})
    require(type(profile['version']) is int and profile['version'] == 1)
    require(profile['package'] == 'github.com/glade-sh/glade/internal/apextest' and profile['complete'] is True)
    require(type(profile['tests']) is list and bool(profile['tests']))
    names, total = set(), 0
    for row in profile['tests']:
        require(type(row) is dict and set(row) == {'name', 'durationMillis'})
        name, duration = row['name'], row['durationMillis']
        require(type(name) is str and re.fullmatch(r'Test[A-Za-z0-9_]*', name) is not None and name not in names)
        require(type(duration) is int and 0 <= duration <= (1 << 63) - 1)
        total += duration
        require(total <= (1 << 63) - 1)
        names.add(name)
except (OSError, ValueError, TypeError, KeyError):
    sys.exit(1)
PY
}

# A rerun of all jobs can reuse the profile from the previous attempt. The
# attempt-scoped cache is independent of the old artifact's availability.
if [[ "$pair" == 0 && "$GITHUB_RUN_ATTEMPT" =~ ^[0-9]+$ && "$GITHUB_RUN_ATTEMPT" -gt 1 && "$GITHUB_RUN_ID" =~ ^[0-9]+$ ]]; then
  previous=$((GITHUB_RUN_ATTEMPT - 1))
  retry_profile=${CI_APEXTEST_RETRY_PROFILE_PATH:-/tmp/apextest-scheduling-profile/profile.json}
  if valid_profile "$retry_profile"; then
    echo '[ci] using cached prior-attempt Apex scheduling profile'
  elif gh run download "$GITHUB_RUN_ID" --name "apextest-scheduling-profile-$GITHUB_RUN_ID-$previous" --dir "$tmp/retry" && valid_profile "$tmp/retry/profile.json"; then
    retry_profile="$tmp/retry/profile.json"
    echo '[ci] using artifact prior-attempt Apex scheduling profile'
  else
    retry_profile=""
    echo '::warning::Prior-attempt Apex scheduling profile unavailable or invalid; trying predecessor history'
  fi
  if [[ -n "$retry_profile" ]]; then
    mkdir -p "$(dirname "$CI_APEXTEST_HISTORY_PATH")"
    cp "$retry_profile" "$CI_APEXTEST_HISTORY_PATH"
  fi
fi

# Benchmark-paired runs are isolated from predecessor artifact lookups.
# Try a bounded, newest-first set of completed runs for the predecessor SHA. An
# unusable run (failed before every shard uploaded, expired artifacts, invalid
# evidence) must not hide an older run with a complete validated set.
if [[ "$pair" == 0 && ! -s "$CI_APEXTEST_HISTORY_PATH" ]]; then
  run_ids=$(gh api --method GET "repos/$GITHUB_REPOSITORY/actions/workflows/ci.yml/runs" -f head_sha="$CI_DURATION_HISTORY_CACHE_REF" -f status=completed -f per_page=100 --jq '[.workflow_runs[] | select(.conclusion == "success" or .conclusion == "failure")][0:5][] | .id' 2>/dev/null) || run_ids=""
  for run_id in $run_ids; do
    [[ "$run_id" =~ ^[0-9]+$ ]] || continue
    shards="$tmp/shards-$run_id"
    gh run download "$run_id" --pattern 'apex-shard-*' --dir "$shards" || continue
    profile="$tmp/predecessor-profile-$run_id.json"
    if GITHUB_EVENT_NAME=push CI_APEXTEST_HISTORY_COMPLETE=1 CI_APEXTEST_SCHEDULING_PROFILE="$profile" bash "$script_dir/ci-go-test.sh" apex-history-refresh "$CI_APEXTEST_HISTORY_PATH" "$shards"/apex-shard-{0..11}; then
      break
    fi
    rm -f "$CI_APEXTEST_HISTORY_PATH"
    if [[ -s "$profile" ]]; then
      mkdir -p "$(dirname "$CI_APEXTEST_HISTORY_PATH")"
      cp "$profile" "$CI_APEXTEST_HISTORY_PATH"
      break
    fi
  done
fi
mkdir -p "$stage"
if [[ -s "$CI_APEXTEST_HISTORY_PATH" ]]; then
  cp "$CI_APEXTEST_HISTORY_PATH" "$stage/apextest-duration-history.json"
else
  echo '::warning::Apex history unavailable or invalid; using deterministic scheduling fallback'
fi
: >"$stage/ready"
