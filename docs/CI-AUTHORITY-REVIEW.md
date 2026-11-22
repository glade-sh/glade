# CI authority and test-value review

This review describes the workflow and scripts in this branch. It is a source
and fixture review; a hosted run is needed to observe its actual selection and
terminal evidence.

## Authority routes

| Stage | Evidence and checks | Qualification |
| --- | --- | --- |
| PR selection and assignment | `ci-pr-select.py` selects packages; `ci-pr-validate-plan.py` rejects missing, duplicate and misrouted assignments. | Guard packages have their own required jobs. |
| PR sharded packages | Each planned worker uploads discovery, plan, selected shard and raw events. `ci-pr-authority-union.py` requires every worker and invokes the existing history validator to compare discoveries, plans, assignments and terminal unions. It also runs the package summary validator on every shard, including nested skips. | Exact documented deferrals are reported separately. |
| PR pool and service packages | Each worker records per-package test discovery and raw events. The worker and union validators compare the terminal union with discovery, validate package terminals and reject unexpected skips. The union records packages without test files separately. | A permitted deferred leaf is not evidence that it ran. |
| PR CLI doctor route | The union derives the selected test names from the worker command, checks one passing terminal per name and validates nested skips. | The CLI shard discovery separately excludes the tests assigned to this route. |
| Full sharded suite | The history refresh validators compare shard discoveries and plans, assignments and terminal results. The history job is a Required CI dependency. | Listed deferrals are recorded separately and are not proof of execution. |
| Node and browser authorities | The Node validator rejects missing, duplicate, skipped and failed selected results; browser evidence validates selected Go and Node output. | Browser evidence has its own required job. |
| Reused push tree | Required CI requires successful tree reuse, leak scan, history and test jobs for the reuse route. | It relies on the tree attestation's selection and validity. |

The PR union publishes `summary.json` with planned workers, deferred tests and
packages without test files, plus the individual package summaries. It selects
the latest available artifact per worker when jobs are partially rerun, then
checks the resulting evidence together. Artifacts are retained for seven days;
repository policy may impose a shorter limit. Missing or incompatible artifacts
fail the union and Required CI. Cancellation classifications appear in the
Required CI job summary and its `classification` output; they do not change
its fail-closed conclusion.

## Deferral and rerouting ownership

The identities and exact reasons remain in their source files. The owner below
is the responsible maintenance role, not a claim that a particular individual
has accepted an assignment. Until the removal condition is met, report the
entry as deferred rather than as an executed test.

| Category | Source and responsible owner | Removal condition |
| --- | --- | --- |
| Node and browser rerouting | CI workflow maintainers: `node_integration_run_regex`, `run_ci_package_lane` and the browser evidence validator. | Remove a reroute only after its replacement authority discovers and executes the same tests and rejects missing or skipped terminals. |
| DataWeave prerequisites | DataWeave and runtime maintainers: `deferred_dataweave` in `scripts/ci-go-test.sh`. | Provision each entry's explicit engine and Java inputs in its authority and execute the test; configured inputs already turn a skip into a failure. |
| Captured Apex gaps | Apex runtime maintainers: `scripts/ci-apex-native-gaps.json` and the corresponding fixture case tables. | Implement the captured behavior and replay each exact leaf; remove that identity when its test passes. |
| SOQL bind boundary | SOQL and VM maintainers: `native_boundaries` in `scripts/ci-go-test.sh` and `internal/soql/testdata/soql_safe_navigation_bind/case-table.tsv`. | Implement and replay each captured excluded bind leaf, then remove its exact boundary. |
| Case-variant filesystem leaves | CLI and toolchain maintainers: the exact `gladecli-ci` case patterns and reasons in `scripts/ci-go-test.sh`. | Execute those leaves on a case-insensitive filesystem with an authoritative terminal result, or replace them with an equivalent portable test. |

## Simplification measurement

The cache-pair experiment removal in this branch deletes the manual dispatch
input, isolated cache prefixes, alternate npm setup, history bypass and their
experiment-specific tests. Against its parent before the test-first commit,
`git diff --shortstat` reports 18 files, 192 insertions and 706 deletions: a
net reduction of 514 lines. The main CI test file fell from 5,317 to 4,987
lines at that point. These are source-size measurements, not measured runner
time or proof of unchanged hosted results. Normal cache key dimensions and
history routes remain covered by focused tests.

The earlier review measured `scripts/internal/cicache/main.go` at 1,612 lines
and `scripts/ci-go-test.sh` at 2,125 lines. Their remaining checks protect
cache and terminal evidence; this review does not justify removing them without
an outcome-equivalent replacement. The six history families order distinct
package shards. Consolidation needs an evidence-preserving design and measured
savings. Hosted timing and the effect of the additional PR evidence collection
remain to be measured.
