# CI execution cost

This branch shares compiled CGO test binaries, batches the push race workload by
observed duration, and keeps the exhaustive race matrix on scheduled and manual
runs. Normal CI still executes all its tests. This is a change in race cadence:
full race coverage is weekly rather than after every push. It is not identical
per-push race coverage.

## Observations and projections

Read-only GitHub collection on 2026-10-19 used the latest 20 runs per workflow.
[jobs.tsv](jobs.tsv) records independent step medians across historical matrix
shapes. The two complete successful reference runs are
[CI](https://github.com/glade-sh/glade/actions/runs/38187076680) and
[Race](https://github.com/glade-sh/glade/actions/runs/38301225330).
No workflow was triggered, rerun, cancelled or pushed for this work.

| Per push | Measured reference jobs | Measured reference job-minutes | Projected jobs | Projected job-minutes |
| --- | ---: | ---: | ---: | ---: |
| CI | 33 | 201.27 | 34 | 189–196 |
| Race | 82 | 521.53 | 9 | 80–130 |
| Combined | 115 | 722.80 | 43 | 269–326 |

The per-push Race projection is the bounded concurrency-focused matrix. The
weekly scheduled and manual runs continue to execute the full race matrix:

| Full Race run | Measured reference jobs | Measured reference job-minutes | Projected jobs | Projected job-minutes |
| --- | ---: | ---: | ---: | ---: |
| Scheduled or manual full Race | 82 | 521.53 | 83 | 497–507 |

The full-run job count includes the shared-binary producer. Its projected
15–25 job-minute reduction comes from compiling each heavy package once and
distributing the verified binary to its shards. The separate per-push reduction
comes mainly from running the concurrency-focused matrix; it does not replace or
weaken the scheduled/manual full-race run.

The projection is 55–63% fewer combined job-minutes. It is not a measured hosted
saving. Browser and Security remain unchanged (13.38 reference job-minutes);
including them gives a projected 54–62% reduction. The 40% target is exceeded by
this model, but has not been established by a hosted run.

Summing explicit setup/cache/install steps in the two complete runs gives 18.32
CI setup minutes and 26.37 Race setup minutes. The median per job is 29.5 seconds
and 14 seconds respectively. Removing 73 race jobs saves about 17 minutes at the
median; most savings come from changing which tests run under race on pushes.
Compilation occurs inside validation steps and is additional to those figures.
Medians in jobs.tsv are independent and should not be summed into a run total.

The push matrix uses 22 packages in eight longest-first batches. Its checked
weights sum to 75.27 minutes, with a maximum batch estimate of 13.88 minutes.
Full-package weights are reference job wall times, so summing them conservatively
repeats setup/build costs that a batch can share. Focused-package weights use
historical selected-test times plus a 120-second build allowance. Two new CLI
tests and the two LWC helper tests lack historical durations. The 80–130 minute
range adds uncertainty for their cost, Linux variability, setup and browser
installation. It is a planning range, not a confidence interval.

## Local comparison

Current builds pin Go 1.26.10. The measurements below retain their recorded toolchain.

[measurements.tsv](measurements.tsv) records bounded local comparisons of the
integration tip before schema/lazy-index changes and the current integration
base. Go 1.26.9,
CGO, GOMAXPROCS=2, GOMEMLIMIT=2GiB, nice 10 and shared warm build caches were used.
Node was 24.21.0; both browser samples used the same cached Chromium executable
through Playwright 1.60.0. Hosted CI uses Node 22, so browser timing does not
establish Linux performance. Only one owned benchmark ran at a time. The host had other workers, so these are
illustrative samples rather than a controlled performance acceptance result.
No train was observed by `pgrep -fl lane_parts` at admission.

The matched Apex API 59 slice passed the same 380 terminal results on each tree:
71.14 → 59.61 seconds (16.2% faster). The unfiltered current parent exceeded the
170-second test limit. A timeout is censored, not a completed measurement.
Neither this sample nor the schema optimization justifies applying that speedup
to LWC browser work. Full V04 and L11 parents hit the 170-second limit on both
trees. They are censored and establish no speedup. The first L11 attempt failed
because its default Chromium build was absent; that environment failure is
excluded from performance estimates. Both bounded retries used the same cached
browser executable.

A matched first-nine-row Apex race slice passed the same 11 terminal results:
16.97 → 17.26 seconds. This small slice provides no evidence of a race
speedup. An initial selector matched no rows; its timings were discarded.

The reference [heavy selections](hosted-selections.tsv) show that CI's
L10/L11/L19 parents took 1166.17/1178.38/1077.80 seconds. More
shards cannot shorten an indivisible parent. Keep normal CI's eight LWC, four VF,
two Apex and two sema shards: fewer would worsen its long tail. **The goal that
every job finishes in about 15 minutes remains unmet for normal CI and full
race.** A separate change must expose the long LWC API passes as independently
selectable work, with native row/result union validation. Existing Apex/VF API
manifests alone do not make the current planner schedule those subtests.

## Apex duration balance

Apex and sema history refresh enforce the existing 1.5× median duration limit
at every shard count. PRs report timing imbalance as an advisory and retain the
observations for the next plan. Push, scheduled, manual and local refreshes reject
imbalance. Discovery, selection and terminal union validation remain mandatory
before either outcome; invalid evidence never publishes history. LWC and
Visualforce keep their existing history policy for indivisible long tests.

The existing longest-first planner uses refreshed per-test durations to rebalance
the next assignment. Both source and shared-binary synthetic project routes pin
the full 12-test union while reducing a deliberately skewed 8-second shard to at
most 4 seconds. The timing observations in this regression are fixed synthetic
values, independent of runner speed.

A read-only scheduling projection used the six shard artifacts from
[the audited CI run](https://github.com/glade-sh/glade/actions/runs/39598388070).
Its 1,552 discovered top-level tests had 1,534 passes, three failures and 15
deferrals. Replanning the same exact union with their recorded elapsed values
produced these aggregate estimates:

| Assignment | Shortest shard | Median shard | Longest shard |
| --- | ---: | ---: | ---: |
| Observed assignment | 1138.56 s | 1528.32 s | 1762.15 s |
| Measured-duration replan | 1475.42 s | 1475.42 s | 1475.43 s |

The projected longest shard is 16.3% shorter. These are summed test durations,
excluding setup and compilation. Failed and deferred observations were included
only for this scheduling analysis; they do not form an accepted CI history or
establish passing coverage. No history cache was published. The current tree has
additional tests, and actual hosted timing remains unverified.

## Coverage and execution

Normal CI's discovery, selection regexes, shard counts, native assertions and
union/result validators are preserved. Required CI additionally requires its
binary producer. Required Scheduled CI and all its dependencies remain.
CI and full Race build each heavy package once and distribute package-specific
artifacts. Manifests bind binaries to revision, CGO/native toolchain settings,
race mode and checksum, including producer-attempt handling on reruns. Corrupt or
incompatible artifacts fail closed. Local runs still use ordinary go test.

The push race scope is in `scripts/ci-race-push.json`. It retains full Test*
execution for 18 packages with concurrency or shared mutable state, including VM,
storage, sema, typesys, caches, server, debugger, test daemon and watcher paths.
Apex and CLI retain all tests that start goroutines, run parallel subtests or
reach local concurrent fixture helpers. VF retains its concurrent render-environment
test. LWC retains the generic browser-pass scheduling and ordering tests; JavaScript execution is
not instrumented by Go's race detector. Full native browser observations still
run in normal CI and in scheduled/manual Race.

The scope guard scans Go ASTs for new production goroutines/sync imports and
checks the focused concurrent-test sets. This is a maintenance guard, not a
proof that all possible races are exercised. The runner discovers tests before
execution, rejects stale selectors, and requires the exact selected top-level
terminal union, successful package completion. The existing package validator rejects skips
except its exact, prerequisite-bound deferred DataWeave cases. Every
batch runs one package at a time, sharing its local Go build cache. CLI retains
its existing GOMAXPROCS=1 budget; the other packages use 2. A failed
package preserves raw events and never writes a successful summary. Push artifacts
include the run attempt so a failed-job rerun retains the earlier evidence.

## Regression evidence and validation

Tests were committed before implementation. A serial repeat on a clean source
snapshot of the unfixed base produced this excerpt (tool paths omitted):

```text
=== RUN   TestCIRacePushRetainsConcurrentTests
    ci_race_push_test.go:52: open ci-race-push.json: no such file or directory
--- FAIL: TestCIRacePushRetainsConcurrentTests (0.00s)
=== RUN   TestCIRacePushPlanUnionAndWorkflow
    ci_race_push_test.go:63: plan: exit status 2
--- FAIL: TestCIRacePushPlanUnionAndWorkflow (0.28s)
=== RUN   TestCISharedBinaryPreservesExecution
    ci_test_binary_test.go:52: build shared binary: exit status 2
--- FAIL: TestCISharedBinaryPreservesExecution (0.32s)
=== RUN   TestCISharedBinaryWorkflowWiring
    ci_test_binary_test.go:132: CI must build shared test binaries once
--- FAIL: TestCISharedBinaryWorkflowWiring (0.00s)
FAIL
FAIL	github.com/glade-sh/glade/scripts	1.133s
FAIL
```

The shared-binary fixture compares all six parent/row/package terminal results
through go test and shared execution, including deliberate failure, race binaries,
corruption, revision changes, copying and the real shell wrapper. Push fixtures
reject missing, extra and skipped results. Planner tests enforce a complete,
disjoint package union and the modeled 15-minute batch budget.

The merged workflow, binary, race-plan, parser and required-check selection
passed in 16.264 seconds:

```sh
go test ./scripts -run '^(TestCIRacePush.*|TestCISharedBinary.*|TestCIRaceWorkflowContract|TestCIRaceWorkflowSchedulesBrowserPackagesSeparately|TestCIVisualforceWorkflowSharesInputAndRequiresUnion|TestCILWCHistoryWorkflowSharesInputAndRequiresUnion|TestCICacheCheckScriptRunsWorkflowParser|TestCIRequiredAggregateContract|TestCIRequiredScheduledAggregateIsFailClosed)$' -count=1 -timeout=3m -v
```

Five adjacent discovery and union guards passed in 100.435 seconds:
`TestLWCCompileShardsPartitionDiscoveryAndNodeAuthority`,
`TestCIPackageLanesRouteThroughCheckedManifest`,
`TestCIRaceLWCHistoryBalancesAllDiscoveredTests`,
`TestCIRaceBrowserRunnersAndAggregateCoverDiscovery` and
`TestCIVisualforceShardsPreserveFullDiscoveryWithRealPlanner`.

The native comparison commands used the same resource environment:

```sh
go test ./internal/apextest -run '^TestRunAsDefaultSecurityConformance$/^59[.]0$' -count=1 -timeout=170s -json
go test ./internal/apextest -race -run '^TestRunAsDefaultSecurityConformance$/^59[.]0$/^C137_01[0-8]$' -count=1 -timeout=170s -json
go test ./internal/visualforce -run '^TestV04SalesforceConformance$' -count=1 -timeout=170s -json
go test ./internal/lwc/compile -run '^TestL11SalesforceConformance$' -count=1 -timeout=170s -json
```

The table records passes and censored limits separately. Browser process groups
were cleaned up after bounded runs. All Go checks used nice 10, CGO=1,
GOTOOLCHAIN=local, GOMEMLIMIT=2GiB and GOMAXPROCS=2; the final binary build uses 1.

Repository hygiene: seven selected `internal/repoguard` checks passed in 4.160s
(corpus references, personal paths, org aliases, planning docs, tracked build
outputs, project scope and plugin boundaries).
`gofmt`, Python syntax checks and `git diff --check` passed.
`CGO_ENABLED=1 GOMAXPROCS=1 go build -o bin/glade ./cmd/glade` passed.

No Salesforce behavior changed or needed clarification; no org was created and
no native rows were captured. Existing native exports supply the runtime checks.

## Advisor gates and remaining risks

Use `glade-sh/glade` on Linux/CGO: normal CI and Required CI, scheduled CI and
Required Scheduled CI, both Race tiers, full Apex/browser race union aggregates,
Browser and Security. Test both cache states, artifact transfer, failed-job reruns
and exact discovered/per-row terminal unions. Compare
`TestL10SalesforceConformance`, `TestL11SalesforceConformance`,
`TestL19SalesforceConformance`, `TestL21SalesforceConformance`,
`TestV04SalesforceConformance`, `TestV09SalesforceConformance`,
`TestV11SalesforceConformance`, `TestV13SalesforceConformance`,
`TestV14SalesforceConformance`, `TestRunAsDefaultSecurityConformance` and
`TestSOQLBindParenOrgConformance`. No `glade-sh/glade-tools` source changed and
no corpus gate was executed.

The reference scheduled CI run rejected sema fuzz-seed parents absent from
Test*-only discovery. This branch does not repair that mismatch or rerun the full
oracle; its current status remains unverified. Hosted savings, the full push race
matrix, the normal CI 15-minute target and all advisor gates remain unverified.
