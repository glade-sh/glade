#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 2 && "$1" =~ ^[0-9a-f]{40}$ && "$2" =~ ^[0-9a-f]{40}$ ]] || {
  echo "usage: $0 <lowercase-40-hex-glade-sha> <lowercase-40-hex-tools-sha>" >&2
  exit 2
}
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-glade-sh/glade}"
failed=0
if ! ci="$(bash "$root/scripts/verify-required-ci.sh" "$GITHUB_REPOSITORY" "$1")"; then
  echo "Do not tag: require a successful main-push Required CI run for $1. PR and manual runs do not qualify." >&2
  failed=1
fi
if ! salesforce="$(bash "$root/scripts/verify-salesforce-check.sh" "$1" "$2")"; then
  echo "Do not tag: run Salesforce correctness for this exact product/tools pair. Do not create a trigger commit." >&2
  failed=1
fi
[[ "$failed" == 0 ]] || exit 1
jq -n --arg gladeSHA "$1" --arg toolsSHA "$2" \
  --argjson requiredCI "$ci" --argjson salesforce "$salesforce" \
  '{schemaVersion: 1, gladeSHA: $gladeSHA, toolsSHA: $toolsSHA, requiredCI: $requiredCI, salesforce: $salesforce}'
