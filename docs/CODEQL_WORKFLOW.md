# CodeQL workflow

CodeQL runs from one reusable workflow, `.github/workflows/codeql.yml`, with two
profiles. Both share the same explicit query exclusions, listed and reviewed in
[CODEQL_BUDGET.md](CODEQL_BUDGET.md#waiver-review-2026-10-22).

| Profile | Runs from | Display name | Merge gate |
| --- | --- | --- | --- |
| fast | `CI`, on every pull request, `main` push, schedule and dispatch | `CodeQL / CodeQL (fast)` | Yes, through `Required CI` |
| full | `Security`, on the Monday 09:17 UTC schedule, `v*` tags and manual dispatch | `CodeQL / CodeQL (full)` | No; visible in Security only |

The fast profile keeps pull-request diff restrictions when init provides them,
including an empty diff. Without diff ranges it analyzes the whole branch.

## Time budget

Analysis finalization, query evaluation and SARIF interpretation share a hard
300-second deadline; evaluation also has a 240-second limit. On expiry,
`scripts/ci-codeql.py` kills the process group, emits an Actions warning and
records incomplete analysis in the job summary. It uploads no incomplete or
stale SARIF, and the job succeeds.

Ordinary nonzero exits, invalid results, setup, build and upload errors, and
unsuppressed alerts fail the job. The 15-minute outer job limit bounds setup and
cleanup; cancellation or expiry of that limit is a failure. There is no blanket
analysis `continue-on-error`.

## Known incomplete scans

Large changes regularly exhaust the budget. Only part of the query set
completes and the job reports incomplete analysis. `Required CI` still passes,
because a timed-out analysis returns success. Treat that result as unknown
coverage, not as a clean scan. Release notes state an incomplete scan under
known issues. The release timing check in [CODEQL_BUDGET.md](CODEQL_BUDGET.md)
rejects incomplete scans as release timing evidence. Small changes complete.

## Required checks

The default-branch ruleset requires only `Required CI`, from GitHub Actions,
with strict up-to-date checks (read 2026-10-22). `Required CI` accepts only
successful dependencies, and the fast CodeQL job is one of them. A completed
scan with alerts, or an analysis error, fails both. Full-scan failures stay
visible in Security but are outside the merge aggregate. Full tag scans run
alongside publication and are not release dependencies.

## Maintenance

The wrapper preserves the pinned init action's diff-range extension protocol
and post-job completion and unknown flags. These are internal hooks: action
upgrades must recheck them against
[the pinned action](https://github.com/github/codeql-action/tree/a recorded revision/src).

The fast profile retains five queries that full scans waive. The existing
generated-file path declaration does not prove that compiled Go extraction
excludes generated files.

The Visualforce cookie sink honors Apex-supplied Secure and HttpOnly flags. The
prior assessment found no behavior-preserving hardening, so its two waivers
remain. Overriding those flags requires native evidence and route tests.
