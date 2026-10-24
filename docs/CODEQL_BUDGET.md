# CodeQL release budget

The authoritative query profiles are the two inline `config` values in
[codeql.yml](../.github/workflows/codeql.yml), rather than separate CodeQL
configuration files. [codeql-budget.json](../.github/codeql-budget.json) records
these limits:

| Measure | Limit |
| --- | ---: |
| Distinct full-profile waived query IDs | 12 |
| Distinct fast-profile waived query IDs | 7 |
| Full-profile Analyze step wall time | 300 seconds |
| Full-profile query wall time | 240 seconds |

The fast profile runs for every pull request, regardless of changed-file count,
and for other CI events. The full profile runs weekly, on `v*` release tags,
and on manual Security dispatch. The existing query filters are unchanged:
fast retains the former small-PR profile (7 waivers), and full retains 12.
Both profiles retain the default suite and add `security-extended`. The full
profile repeats `go/weak-sensitive-data-hashing`; counts deduplicate it.

The bounded command uses the configuration recorded by CodeQL init in the
database. Finalization, evaluation and SARIF interpretation share 300 seconds;
evaluation also has a 240-second limit. An actual deadline expiry kills the
process group, reports a warning and leaves the job successful with no upload.
An analysis error, malformed SARIF or any unsuppressed alert fails the job.
Setup/build and uploads have a separate 15-minute emergency job limit; failures
or cancellation there still fail. See the [workflow summary](CODEQL_WORKFLOW.md)
for required checks and rollout risks.

## Waiver review: 2026-10-22

Review this exact list at every release and record the date and disposition here.
The release reviewer must also obtain a completed full-profile timing report;
a warning-only timeout is incomplete evidence and cannot satisfy that command.
This review carries the existing waivers forward, with no additional exclusions.

| Query ID | Fast | Full | Disposition |
| --- | --- | --- | --- |
| `go/incomplete-hostname-regexp` | waived | waived | Recheck completion cost. |
| `go/regex/missing-regexp-anchor` | waived | waived | Recheck completion cost. |
| `go/weak-sensitive-data-hashing` | waived | waived | Recheck completion cost. |
| `go/allocation-size-overflow` | waived | waived | Recheck completion cost. |
| `go/incorrect-integer-conversion` | waived | waived | Recheck completion cost. |
| `go/cookie-httponly-not-set` | waived | waived | Apex-controlled sink; assessment below. |
| `go/cookie-secure-not-set` | waived | waived | Apex-controlled sink; assessment below. |
| `go/reflected-xss` | enabled | waived | Recheck whole-branch completion cost. |
| `go/bad-redirect-check` | enabled | waived | Recheck whole-branch completion cost. |
| `go/uncontrolled-allocation-size` | enabled | waived | Recheck whole-branch completion cost. |
| `go/unsafe-quoting` | enabled | waived | Recheck whole-branch completion cost. |
| `go/unvalidated-url-redirection` | enabled | waived | Recheck whole-branch completion cost. |

The maintainer's historical record, supplied on 2026-10-22: 16 alerts ever,
15 fixed in July, none since; 10 of the last 40 Security runs failed and 6 were
cancelled, mostly CodeQL timeouts. These are historical counts, not a fresh
scan of this workflow or evidence that a timeout is a false positive.

## Measurement

Re-measured on 2026-10-22 using `gh run view --json jobs` and each CodeQL job's
`gh run view --log --job` output. These completed v0.3.0 release and follow-up
runs all used the full profile, finished 25 queries, and pass the recorded limits.
These are historical measurements, not validation of a subsequent release.

| Run | Route | Analyze seconds | Query wall seconds |
| --- | --- | ---: | ---: |
| [39644690618](https://github.com/glade-sh/glade/actions/runs/39644690618) | Large PR | 165 | 119.65 |
| [39652775311](https://github.com/glade-sh/glade/actions/runs/39652775311) | Large PR | 201 | 149.91 |
| [39664590856](https://github.com/glade-sh/glade/actions/runs/39664590856) | Release push | 224 | 160.17 |
| [39702133782](https://github.com/glade-sh/glade/actions/runs/39702133782) | Follow-up push | 213 | 155.85 |

Query wall time spans the `codeql database run-queries` command through the
last `Evaluation done` result. It includes query loading; it does not sum
parallel evaluator times or include database extraction and SARIF upload.
Historical action Analyze time includes extraction and upload. The new bounded
CLI step includes finalization, queries and interpretation; its separate upload
must also succeed. These are different step boundaries, so obtain fresh hosted
timings before comparing performance. The budgets left about 34% and 50%
headroom over the slowest historical Analyze and query times.
These limits now also bound the workflow's analysis subprocesses; the six-minute
step timeout leaves cleanup time after the five-minute process deadline.

Before each release, check out the source used by its completed full-profile
Security run and run:

```sh
# Set RUN_ID to the release candidate's completed Security run.
CODEQL_REPORT_DIR="$(mktemp -d)"
gh run view "$RUN_ID" --json jobs > "$CODEQL_REPORT_DIR/run.json"
CODEQL_JOB_ID="$(gh run view "$RUN_ID" --json jobs --jq '.jobs[] | select(.name == "CodeQL / CodeQL (full)") | .databaseId')"
gh run view "$RUN_ID" --log --job "$CODEQL_JOB_ID" > "$CODEQL_REPORT_DIR/query.log"
go run ./scripts/codeql-budget -run-json "$CODEQL_REPORT_DIR/run.json" -query-log "$CODEQL_REPORT_DIR/query.log"
```

Keep these downloaded files outside the checkout. Record the resulting times
in this table for the release. Review a budget increase explicitly rather than
adding waivers to pass a slow scan. No network request is made by the checker.
It rejects failed jobs, missing full-profile initialization, missing or invalid
times, incomplete query logs, and query timestamps outside the Analyze interval.
Use both exports from the same job. The checker does not authenticate exports.

`go run ./scripts/codeql-budget` checks waiver counts without timing. The scripts
suite's `TestCICodeQLRecordedWaiverBudgetRoutes` enforces both counts in CI.
`TestCodeQLBudgetReleaseCommandRoutes` exercises the release command with
synthetic workflow, run and log inputs, including exact-limit acceptance,
both waiver-growth routes, timing overruns and rejected incomplete evidence.
The completed-run time check remains a release command; the workflow enforces
the same deadlines while analysis runs. New reusable-workflow reports must also
show successful result upload and alert-check steps, so a timeout after query
evaluation cannot pass as a completed release scan. Historical `CodeQL` job
exports remain supported with their original profile and timing checks.

## Cookie sink assessment

`applyVisualforceRequestResponse` in `internal/server/visualforce.go` writes
`http.Cookie.Secure` from `value.Secure` and `http.Cookie.HttpOnly` from
`value.HTTPOnly`. `VM.CurrentPageResponseCookies` carries those values from
Apex Cookie fields. The shared HTTP sink serves HTML, redirects, PDF and AJAX.

The concrete flag-hardening candidate is:

```diff
-            Secure:   value.Secure,
-            HttpOnly: value.HTTPOnly,
+            Secure:   true,
+            HttpOnly: true,
```

Reject this change under the behavior-preserving requirement. It changes the
Set-Cookie header whenever Apex supplies false: Secure changes whether a browser
sends the cookie over HTTP; HttpOnly changes JavaScript access. The current
`TestVisualforceApexCookiesCarryExactSecurityFlags` covers all four Boolean
combinations and expiry/session cookies. The proposed change would fail its
`neither_flag`, `secure_only` and `httponly_only` cases. A literal true followed
by restoring the supplied value, or replacing `http.SetCookie` with raw headers,
would preserve behavior but would not harden the cookie.

If a future change intentionally overrides Apex flags, first capture a synthetic
Cookie controller at Apex API 62 and 67 and Visualforce API 59 and 67. Pin native
constructor/setter values and actual Set-Cookie headers in `org.tsv` and
`ORG-RUN.txt`. Add a data-driven VM test for anonymous and named test routes,
plus project-level server tests for HTML, redirect, PDF and AJAX with all four
flag combinations; the tests must fail before the runtime change. No native
capture was made for this assessment because no runtime behavior was changed.

Both cookie query exclusions remain in both profiles. No behavior-preserving
flag hardening was identified, and no claim is made that the candidate would
make either query finish. Un-waiving requires a fresh hosted full-profile and
fast-profile scan with the queries enabled and complete timing results. The existing
`TestOnlyVisualforceWritesResponseCookies` guard continues to reject additional
Go cookie sinks outside this explicitly reviewed boundary.
