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
ruby -ryaml - "$here/../.github/workflows/ci.yml" > "$work/names" <<'RUBY'
workflow = YAML.load_file(ARGV.fetch(0))
steps = workflow.fetch("jobs").fetch("history").fetch("steps")
saves = steps.select { |step| step.fetch("uses", "").start_with?("actions/cache/save@") && !step.fetch("if", "").include?("failure()") }
uploads = steps.select { |step| step.fetch("uses", "").start_with?("actions/upload-artifact@") }
abort "missing history caches" if saves.empty?
stems = saves.map do |step|
  path = step.fetch("with").fetch("path")
  match = /\A\/tmp\/glade-([a-z0-9-]+-duration-history)\z/.match(path)
  abort "unrecognized history cache path" unless match
  stem = match[1]
  name = "#{stem}-${{ github.run_id }}-${{ github.run_attempt }}"
  matching = uploads.select { |upload| upload.fetch("with").fetch("name", "") == name && upload.fetch("with").fetch("path", "") == "#{path}/#{stem}.json" }
  abort "missing or ambiguous history evidence" unless matching.length == 1
  stem
end
abort "duplicate history caches" unless stems.uniq.length == stems.length
puts stems.sort
RUBY
[[ -s "$work/names" ]]
mkdir -p "$out"
while IFS= read -r stem; do
  mkdir -p "$out/$stem"
  gh run download "$run" --repo "$repo" -n "$stem-$run-$attempt" -D "$out/$stem" >/dev/null
  jq -e 'type == "object" and .complete == true and (.tests | type == "array" and length > 0)' "$out/$stem/$stem.json" >/dev/null
done < "$work/names"
printf '%s\n' "$receipt" > "$out/receipt.json"
printf '%s\n' "$receipt"
