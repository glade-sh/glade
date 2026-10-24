# CodeQL workflow change

Prepared for maintainer review, 2026-10-22. This describes the proposed branch
behavior. Hosted execution has not yet validated it.

Pull requests of every size run only the existing fast query profile. The full
profile runs in Security each Monday at 09:17 UTC, on `v*` release tags, and on
manual Security dispatch. Main pushes and other CI events also run fast scans.
The two profiles share a reusable workflow and keep their existing explicit
query exclusions. Fast preserves PR diff restrictions when init provides them,
including an empty diff; absent ranges use whole-branch analysis. The dated [waiver review](CODEQL_BUDGET.md#waiver-review-2026-10-22)
lists every query and must be renewed for each release.

Analysis finalization, query evaluation and SARIF interpretation share a hard
300-second deadline; evaluation also has a 240-second limit. On expiry, the
wrapper kills the process group, emits an Actions warning and records incomplete
analysis in the job summary. It uploads no incomplete or stale SARIF. Ordinary
nonzero exits, invalid results, setup/build/upload errors and unsuppressed alerts
fail. The 15-minute outer job limit bounds setup and cleanup; cancellation or
expiry of that outer limit remains a failure. There is no blanket analysis
`continue-on-error`.

| Gate | Before | After |
| --- | --- | --- |
| Ruleset-required check | `Required CI` | `Required CI` |
| Required CI dependencies | Existing 16 CI jobs; no Security dependency | Existing 16 plus `codeql`, calling the fast profile |
| Fast scan display name | `CodeQL` in Security on small PRs | `CodeQL / CodeQL (fast)` in CI for every PR |
| Full scan display name | `CodeQL` in Security, including large PRs/main pushes | `CodeQL / CodeQL (full)` in Security on weekly/tag/manual runs |

The active default-branch ruleset was read on 2026-10-22: it requires only
`Required CI`, from GitHub Actions, with strict up-to-date checks. No ruleset
mutation is needed. The aggregate still accepts only successful dependencies.
A timed-out analysis returns success with a warning, so the aggregate can pass;
a completed scan with alerts or an analysis error fails its dependency and the
aggregate. Full-scan failures remain visible in Security but are outside the
merge aggregate. The release timing checker still rejects incomplete scans.

The wrapper preserves the pinned init action's diff-range extension protocol
and post-job completion/unknown flags. These are internal hooks: action upgrades
must recheck them against [the pinned action](https://github.com/github/codeql-action/tree/a recorded revision/src).

Risks: a timeout leaves coverage unknown. The fast profile retains five queries
waived in full scans; large PRs may exhaust its budget. Hosted timing, extraction,
upload permissions (including fork PRs), nested check names and cancellation
must be checked on the first public workflow run. The existing generated-file
path declaration does not prove compiled Go extraction excludes generated files.
Full tag scans run alongside publication, without becoming release dependencies.

The Visualforce cookie sink still honors Apex-supplied Secure and HttpOnly
flags. The prior assessment found no behavior-preserving hardening. Its two
waivers remain; overriding those flags requires native evidence and route tests.
No runtime code or Salesforce behavior changes in this branch.

Before public rollout, the coordinator brings this summary and the branch diff
to the maintainer. Verify clean, alert, timeout and cancellation outcomes in
GitHub Actions, and confirm `Required CI` remains the sole required context.
