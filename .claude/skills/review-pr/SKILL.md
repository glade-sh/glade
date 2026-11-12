---
name: review-pr
description: Use when asked to review a glade pull request or branch and return an APPROVE or CHANGES verdict. Read-only review against the evidence rules - native rows, fail-before, route twins, loosened assertions, CI manifests, public safety.
---

# Review a pull request

Read-only by default: do not edit, merge, comment on GitHub or leave
background jobs. Exception: when the only finding is small (for example a
missing lane or manifest entry), push one fix commit to the branch, say so in
the review, then re-review the new head. Never push twice. Review the whole
diff once and report every finding in one pass.

## Setup

    git fetch origin
    git diff origin/main...<head>
    bash scripts/pr-check.sh --base origin/main --head <head>

## Check in this order

1. Evidence. Each Salesforce-observable change has a `NATIVE` line citing
   `org.tsv:<line>` at a matching route and API. Open the row and read it.
   Each Glade-only change cites a local contract and a regression test.
2. Scope. For each rule in the diff, name the rows that prove it. A rule that
   admits or rejects a shape with no row, or that keys on an identifier or
   literal, is blocking. Try one input the rows do not cover.
3. Fail-before. Check out the test commit in a temporary detached worktree and
   run the test: it fails for the stated reason. Remove the worktree.
4. Route twins. List callers and overloads of each changed function. Each
   reachable route has a test.
5. Existing tests. No assertion, fixture or budget is deleted, loosened or
   skipped without a row or contract. Run focused tests for touched packages
   at the head. Never run the whole `internal/apextest` package. For sema or
   typesys changes run `go test ./internal/apextest -run 'TestCorpusShape'`.
6. Correctness. Wrong results, races, values shared with a cache, stale cache
   keys, unbounded memory.
7. CI manifests. New packages, goroutines, `sync` use or `t.Parallel` tests
   have entries in `scripts/ci-package-lanes.json` and
   `scripts/ci-race-push.json`.
8. Public safety. No third-party names or content, org ids, usernames,
   personal paths or credentials, in the diff or in any commit of the branch.
   No file named for a ticket or review round.
9. CI. If a pull request is open, read `gh pr checks <n>`. A red required
   check is a finding; name the failing tests from the log. A component branch
   with no pull request gets its full CI in the train; missing CI is not a
   finding there. A train or release PR needs its dispatched full run green on
   the exact head.

## Verdict

First line exactly one of:

    APPROVE <head sha>
    CHANGES <head sha>: <one line>

Then findings as `file:line`, each marked:

- `[P1]` blocks: correctness defect, regression, behavior without a row, a
  rule broader than its rows, private material, a failing check.
- `[P2]` fix in this PR: missing fail-before, missing route twin, loosened
  assertion, missing manifest entry.
- `[note]` everything else. Notes do not affect the verdict.

Do not request changes for style, naming taste, extra tests beyond the above,
or work outside the PR's scope. ABI constants are bumped at the release cut; a
PR that names its bump class is correct.

State what you ran and what you did not run.
