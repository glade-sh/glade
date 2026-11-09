# CodeQL workflow

CodeQL runs from one reusable workflow, `.github/workflows/codeql.yml`, with two
profiles. Both share the same explicit query exclusions, listed and reviewed in
[CODEQL_BUDGET.md](CODEQL_BUDGET.md#waiver-review-2026-10-22).

| Profile | Runs from | Display name | Merge gate |
| --- | --- | --- | --- |
| fast | `Security`, on every pull request and `main` push | `CodeQL / CodeQL (fast)` | No |
| full | `Security`, on the Monday 09:17 UTC schedule, `v*` tags and manual dispatch | `CodeQL / CodeQL (full)` | Release runs the full profile again as a tag publication dependency |

The fast profile keeps pull-request diff restrictions when init provides them,
including an empty diff. Without diff ranges it analyzes the whole branch.

## Time budget

The fast profile declares test paths excluded; the full profile, which gates
releases, declares only generated files. For manual Go builds, CodeQL does not
honor analysis `paths-ignore`; the Go extractor excludes `_test.go` by default,
and the explicit product build does not build testdata. The fast PR diff range
filter also removes `_test.go` and `testdata` paths; product paths remain. See
[GitHub Go extractor documentation](https://docs.github.com/en/code-security/reference/code-scanning/codeql/build-options-for-compiled-languages).

Analysis finalization, query evaluation and SARIF interpretation share a hard
300-second deadline; evaluation also has a 240-second limit. On expiry,
`scripts/ci-codeql.py` kills the process group, emits an Actions warning and
records incomplete analysis in the job summary. It uploads no incomplete or
stale SARIF. Both the fast and the full job succeed with a warning; a timeout is a known issue, not a failure.

Ordinary nonzero exits, invalid results, setup, build and upload errors, and
unsuppressed alerts fail the job. The 15-minute outer job limit bounds setup and
cleanup; cancellation or expiry of that limit is a failure. There is no blanket
analysis `continue-on-error`.

## Known incomplete scans

Large changes regularly exhaust the budget. Only part of the query set
completes and the job reports incomplete analysis. Full scans of the v0.3.2 to v0.3.4 tags all stopped at 23 of 25 queries. `Required CI` does not depend on CodeQL. Treat that result as unknown
coverage, not as a clean scan. Release notes state an incomplete scan under
known issues. The release timing check in [CODEQL_BUDGET.md](CODEQL_BUDGET.md)
rejects incomplete scans as release timing evidence. Small changes complete.

## Required checks

The default-branch ruleset requires only `Required CI`, from GitHub Actions,
with strict up-to-date checks (read 2026-10-22). `Required CI` accepts only
successful dependencies; CodeQL is outside the merge aggregate. A completed
scan with alerts, or an analysis error, fails its job. Release independently
runs the full profile on the tagged SHA and requires success before publication.
A full-scan timeout is a warning and stays a known issue, so the tag gate fails
only on completed findings or errors. The full profile skips five queries the
fast PR scan runs (ReflectedXss, BadRedirectCheck, OpenUrlRedirect,
UncontrolledAllocationSize, StringBreak), so alerts from those queries gate
nothing now that CodeQL is outside Required CI.

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
