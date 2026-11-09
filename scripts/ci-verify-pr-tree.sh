#!/usr/bin/env bash
# Fail-closed tree attestation: accept a main commit as tested when a merged
# pull request's successful Required CI run checked out a merge commit whose
# git tree equals the tree of the main commit. See docs/RELEASE_POLICY.md.
set -euo pipefail

repository="${1:-}"
sha="${2:-}"
usage="usage: $0 OWNER/REPOSITORY 40_HEX_SHA"
hex40='^[0-9A-Fa-f]{40}$'

if [[ ! "$repository" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || [[ ! "$sha" =~ $hex40 ]]; then
  echo "$usage" >&2
  exit 2
fi

fail() {
  echo "tree attestation refused for $repository@$sha: $*" >&2
  exit 1
}

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

pulls_json="$(gh api --method GET "/repos/$repository/commits/$sha/pulls" -f per_page=100)"
pr="$(jq -c --arg sha "$sha" '
  map(select(.merged_at != null and .merge_commit_sha == $sha and .base.ref == "main"))
  | if length == 1 then .[0] else empty end
' <<<"$pulls_json")"
[[ -n "$pr" ]] || fail "no single merged pull request to main with this merge commit"
pr_number="$(jq -r '.number' <<<"$pr")"
head_sha="$(jq -r '.head.sha' <<<"$pr")"
[[ "$head_sha" =~ $hex40 ]] || fail "pull request head is not a full SHA"

commit_tree() {
  local json
  json="$(gh api --method GET "/repos/$repository/commits/$1")" || return 1
  jq -r '.commit.tree.sha // empty' <<<"$json"
}
push_tree="$(commit_tree "$sha")"
[[ "$push_tree" =~ $hex40 ]] || fail "cannot read the tree of the main commit"

runs_json="$(gh api --method GET \
  "/repos/$repository/actions/workflows/ci.yml/runs" \
  -f "head_sha=$head_sha" \
  -f event=pull_request \
  -f status=completed \
  -f per_page=100)"

while IFS=$'\t' read -r run_id attempt run_url; do
  [[ -n "$run_id" ]] || continue
  jobs_json="$(gh api --method GET --paginate --slurp \
    "/repos/$repository/actions/runs/$run_id/attempts/$attempt/jobs" -f per_page=100)"
  authority="$(jq -c '
    [.[].jobs[]? | select(.name == "Required CI" and .status == "completed" and .conclusion == "success")]
    | first // empty
  ' <<<"$jobs_json")"
  [[ -n "$authority" ]] || continue

  rm -rf "$work/receipt"
  mkdir "$work/receipt"
  gh run download "$run_id" --repo "$repository" -n ci-tree -D "$work/receipt" >/dev/null 2>&1 || continue
  [[ -f "$work/receipt/ci-tree.json" ]] || continue
  receipt="$(jq -c . "$work/receipt/ci-tree.json")" || continue
  # The receipt must name this exact run, attempt and pull request.
  jq -e --argjson id "$run_id" --argjson attempt "$attempt" --argjson pr "$pr_number" '
    .run_id == $id and .run_attempt == $attempt and .pr_number == $pr
  ' <<<"$receipt" >/dev/null || continue
  checkout_sha="$(jq -r '.checkout_sha // empty' <<<"$receipt")"
  [[ "$checkout_sha" =~ $hex40 ]] || continue

  # Independent check of the claimed checkout: a GitHub merge commit whose
  # second parent is the pull request head and whose tree equals the main tree.
  merge_json="$(gh api --method GET "/repos/$repository/commits/$checkout_sha")" || continue
  jq -e --arg head "$head_sha" '(.parents | length) == 2 and .parents[1].sha == $head' <<<"$merge_json" >/dev/null || continue
  merge_tree="$(jq -r '.commit.tree.sha // empty' <<<"$merge_json")"
  [[ "$merge_tree" == "$push_tree" ]] || continue

  jq -n \
    --arg repository "$repository" \
    --arg sha "$sha" \
    --arg tree "$push_tree" \
    --arg checkout_sha "$checkout_sha" \
    --arg head_sha "$head_sha" \
    --arg run_url "$run_url" \
    --argjson pr_number "$pr_number" \
    --argjson run_id "$run_id" \
    --argjson authority "$authority" \
    '{
      schema_version: 1,
      repository: $repository,
      sha: $sha,
      conclusion: "success",
      event: "pull_request_tree",
      tree: $tree,
      pr_number: $pr_number,
      pr_head_sha: $head_sha,
      checkout_sha: $checkout_sha,
      run_id: $run_id,
      run_url: $run_url,
      required_ci_job_id: $authority.id,
      required_ci_job_url: $authority.html_url
    }'
  exit 0
done < <(jq -r --arg sha "$head_sha" --argjson pr "$pr_number" '
  .workflow_runs
  | map(select(
      .head_sha == $sha
      and .event == "pull_request"
      and .conclusion == "success"
      and ((.pull_requests // []) | any(.number == $pr))
    ))
  | sort_by(.created_at // "")
  | reverse[]
  | [.id, (.run_attempt // 1), .html_url]
  | @tsv
' <<<"$runs_json")

fail "no successful pull request Required CI run with a matching tree receipt"
