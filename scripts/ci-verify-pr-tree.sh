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
head_ref="$(jq -r '.head.ref // empty' <<<"$pr")"
[[ "$head_ref" == train/* ]] || fail "pull request is not a train"
jq -e --arg repository "$repository" '.head.repo.full_name == $repository and .base.repo.full_name == $repository' <<<"$pr" >/dev/null || fail "pull request is not from this repository"

push_json="$(gh api --method GET "/repos/$repository/commits/$sha")"
push_tree="$(jq -r '.commit.tree.sha // empty' <<<"$push_json")"
[[ "$push_tree" =~ $hex40 ]] || fail "cannot read the tree of the main commit"
base_sha="$(jq -r 'if (.parents | length) == 2 then .parents[0].sha else empty end' <<<"$push_json")"
[[ "$base_sha" =~ $hex40 ]] || fail "main commit is not a two-parent merge"
jq -e --arg head "$head_sha" '.parents[1].sha == $head' <<<"$push_json" >/dev/null || fail "main merge has a different pull request head"

# The receipt producer and its Required CI policy must be the policy already
# present on the main parent. A pull request cannot define its own authority.
workflow_path=".github/workflows/ci.yml"
base_blob="$(gh api --method GET "/repos/$repository/contents/$workflow_path" -f "ref=$base_sha" --jq .sha)" || fail "cannot read base workflow"
push_blob="$(gh api --method GET "/repos/$repository/contents/$workflow_path" -f "ref=$sha" --jq .sha)" || fail "cannot read merge workflow"
[[ "$base_blob" =~ $hex40 && "$base_blob" == "$push_blob" ]] || fail "workflow differs from the trusted main parent"
gh api --method GET "/repos/$repository/contents/$workflow_path" -f "ref=$base_sha" --jq .content | base64 --decode > "$work/ci.yml" || fail "cannot read coverage policy"
# The required job list is read from the trusted workflow, not maintained as a
# second inventory. Refuse an unfamiliar layout rather than guessing.
awk '
  /^  required-ci:$/ {job=1; next}
  job && /^  [a-zA-Z0-9_-]+:$/ {exit}
  job && /^    needs:$/ {needs=1; next}
  needs && /^      - [a-zA-Z0-9_-]+$/ {print $2; next}
  needs {exit}
' "$work/ci.yml" > "$work/needs"
[[ -s "$work/needs" ]] || fail "cannot read Required CI job inventory"
required_jobs="$(jq -Rsc 'split("\n") | map(select(length > 0)) | sort' < "$work/needs")"

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
    | if length == 1 then .[0] else empty end
  ' <<<"$jobs_json")"
  [[ -n "$authority" ]] || continue
  jq -e '[.[].jobs[]? | select(.name == "Tree receipt" and .status == "completed" and .conclusion == "success")] | length == 1' <<<"$jobs_json" >/dev/null || continue

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
  jq -e --argjson jobs "$required_jobs" '
    (.coverage | type) == "object" and
    (.coverage | keys | sort) == $jobs and
    all(.coverage | to_entries[];
      if (.key == "pr-tests" or .key == "pr-guards")
      then .value.result == "skipped" else .value.result == "success" end)
  ' <<<"$receipt" >/dev/null || continue

  # Independent check of the claimed checkout: a GitHub merge commit whose
  # second parent is the pull request head and whose tree equals the main tree.
  merge_json="$(gh api --method GET "/repos/$repository/commits/$checkout_sha")" || continue
  jq -e --arg head "$head_sha" --arg base "$base_sha" '(.parents | length) == 2 and .parents[0].sha == $base and .parents[1].sha == $head' <<<"$merge_json" >/dev/null || continue
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
done < <(jq -r --arg sha "$head_sha" --arg ref "$head_ref" --argjson pr "$pr_number" '
  .workflow_runs
  | map(select(
      .head_sha == $sha
      and .head_branch == $ref
      and .event == "pull_request"
      and .status == "completed"
      and .conclusion == "success"
      and (.run_attempt | type == "number" and . >= 1)
      and ((.pull_requests // []) | any(.number == $pr))
    ))
  | sort_by(.created_at // "")
  | reverse[]
  | [.id, .run_attempt, .html_url]
  | @tsv
' <<<"$runs_json")

fail "no successful pull request Required CI run with a matching tree receipt"
