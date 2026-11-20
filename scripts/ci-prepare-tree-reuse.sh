#!/usr/bin/env bash
# Prepare a reuse decision only after the receipt and every history artifact
# from that exact attempt are available. Any error leaves normal CI in charge.
set -euo pipefail
repo="${1:-}"
sha="${2:-}"
before="${3:-}"
out="${4:-}"
hex='^[0-9a-fA-F]{40}$'
[[ "$repo" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ && "$sha" =~ $hex && "$before" =~ $hex && -n "$out" ]] || exit 1
here="$(cd "$(dirname "$0")" && pwd)"
commit="$(gh api --method GET "/repos/$repo/commits/$sha")"
jq -e --arg before "$before" '.parents[0].sha == $before' <<<"$commit" >/dev/null
receipt="$(bash "$here/ci-verify-pr-tree.sh" "$repo" "$sha")"
jq -e '(.run_id | type == "number" and . >= 1 and floor == .) and (.run_attempt | type == "number" and . >= 1 and floor == .)' <<<"$receipt" >/dev/null
run="$(jq -r .run_id <<<"$receipt")"
attempt="$(jq -r .run_attempt <<<"$receipt")"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
# History artifact names come from the trusted parent workflow, not a second
# hand-maintained inventory. An unfamiliar or empty inventory refuses reuse.
awk '
  /^  history:$/ {job=1; next}
  job && /^  [a-zA-Z0-9_-]+:$/ {exit}
  job && /^          name: / {
    suffix="-${{ github.run_id }}-${{ github.run_attempt }}"
    name=$0
    sub(/^          name: /, "", name)
    if (index(name, suffix) && index(name, "-duration-history-")) {
      stem=substr(name, 1, index(name, suffix)-1)
      if (stem !~ /^[a-z0-9-]+-duration-history$/) exit 1
      print stem
    }
  }
' "$here/../.github/workflows/ci.yml" | sort -u > "$work/names"
[[ -s "$work/names" ]]
mkdir -p "$out"
while IFS= read -r stem; do
  mkdir -p "$out/$stem"
  gh run download "$run" --repo "$repo" -n "$stem-$run-$attempt" -D "$out/$stem" >/dev/null
  jq -e 'type == "object" and .complete == true and (.tests | type == "array" and length > 0)' "$out/$stem/$stem.json" >/dev/null
done < "$work/names"
printf '%s\n' "$receipt" > "$out/receipt.json"
printf '%s\n' "$receipt"
