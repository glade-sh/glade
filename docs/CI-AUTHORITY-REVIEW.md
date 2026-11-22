# CI authority and test-value review

This review describes the workflow and scripts in this branch. It is a source
review and targeted fixture review, not a claim that a hosted run has complete
coverage.

## Authority routes

| Stage | Evidence and checks | Remaining gap |
| --- | --- | --- |
| PR package selection | `ci-pr-select.py` selects changed packages and reverse consumers; `ci-pr-validate-plan.py` checks selected packages against the matrix routes and rejects missing, duplicate or misrouted assignments. `required-ci` checks the validated plan and worker result. | Assignment is not terminal execution evidence. |
| PR sharded packages | `ci-go-test.sh` validates each worker's discovery, full plan against its own discovery, selected shard and terminal results. All planned workers must succeed. | The PR lane does not compare discovery, plan and terminal unions across workers. Shared history reduces plan drift but does not prove identical discovery or plans. See `TestCIPRLaneConsumesOneSharedHistoryArtifact`. |
| PR ordinary pool and service packages | The executor invokes `go test` for assigned packages; browser-selected work also emits evidence checked by the browser validator. | The ordinary `go test` routes have no equivalent complete terminal/skip audit. A Go test can skip and the command can still succeed; an unexpected skip can therefore be credited by the job result. |
| Full sharded suite | The history refresh validators compare shard discoveries and plans, assignments and terminal results. The history job is a Required CI dependency. | Listed deferrals are recorded separately and are not proof of execution. Review each configured exception against its owner and native or local contract. |
| Node and browser authorities | The Node validator rejects missing, duplicate, skipped and failed selected results; browser evidence validates selected Go and Node output. | This does not cover non-browser pool packages or prove the whole PR shard union. |
| Reused push tree | Required CI requires successful tree reuse, leak scan, history and test jobs for the reuse route. | It relies on the tree attestation's selection and validity; this review did not independently verify a hosted attestation. |

`ci-go-test.sh` records named native boundaries and prerequisite-dependent
DataWeave checks as deferred rather than executed. Some package summaries can
still be valid with documented deferrals. That is a qualified result, not
proof that every selected test passed. No inventory in this review establishes
that every exception still has an active owner.

## Deferral and rerouting inventory

The following are source inventories, not claims that a corresponding hosted
job ran. The listed files remain the source of individual test identities and
reasons; duplicating those names here would let this inventory drift.

| Category | Source of identities and condition | Authority or remaining owner decision |
| --- | --- | --- |
| Node and browser rerouting | `node_integration_run_regex` and `run_ci_package_lane` in `scripts/ci-go-test.sh` exclude provisioned tests from ordinary full-suite lanes. | The Node integration validator and Browser workflow are their execution authorities. Verify their selection and terminal evidence together when changing either list. |
| DataWeave prerequisites | `deferred_dataweave` in `scripts/ci-go-test.sh` names the tests, required inputs and accepted reasons. When all required inputs are configured, a skip is rejected. | The feature remains outside the stated offline surface; a future owner must provision and run these tests before claiming their coverage. |
| Captured Apex gaps | `scripts/ci-apex-native-gaps.json` names exact subtests; the summary validator accepts the matching recorded boundary reason only. | Each gap needs a product owner and a removal condition. Its captured observation is evidence, not an executed replay. |
| SOQL bind boundary | The `native_boundaries` entries in `scripts/ci-go-test.sh` name the API-specific leaf and exact skip reason. | The native case table identifies the unreplayed boundary; a matching implementation and replay remove the deferral. |
| Platform-specific leaves | The package summary validator permits its explicitly checked filesystem case-variant boundary in the `gladecli-ci` lane when its exact filesystem reason matches. | Other skipped leaves still fail that validator. This does not repair the ordinary PR pool's missing skip audit. |

No individual person or team ownership has been established for the listed
product gaps. Assigning names without an authoritative source would disguise
that remaining decision.

## Simplification measurement

The cache-pair experiment removal in this branch deletes the manual dispatch
input, isolated cache prefixes, alternate npm setup, history bypass and their
experiment-specific tests. Against its parent before the test-first commit,
`git diff --shortstat` reports 18 files, 192 insertions and 706 deletions: a
net reduction of 514 lines. The main CI test file falls from 5,317 to 4,987
lines. These are source-size measurements, not measured runner time or proof
of unchanged hosted results. Normal cache key dimensions and history routes
remain covered by the existing focused tests.

The current sizes of `scripts/internal/cicache/main.go` and
`scripts/ci-go-test.sh` are 1,612 and 2,125 lines respectively. Their remaining
checks protect cache and terminal evidence; this review does not justify
removing them without a measured, outcome-equivalent replacement. The six
history families order distinct package shards, and consolidating them needs
an evidence-preserving design and measurement.

## Required follow-up

1. Add an aggregate PR authority that receives every selected shard's discovery,
   plan, selection and terminal evidence, rejects missing or inconsistent
   artifacts, and compares exact unions before Required CI succeeds. Test an
   omitted worker, divergent discoveries and plans, missing/duplicate terminals,
   unexpected skips and documented deferrals.
2. Give ordinary pool and service package routes explicit package and test
   terminal evidence, with a documented policy for justified deferrals. Test
   skipped and missing selected authorities and preserve browser and Node routes.
3. Inventory each existing deferral with an owner, reason, source and expiry or
   removal condition. Distinguish deferred from passing in reports and resolve
   gaps instead of treating successful process exit as execution evidence.
4. Measure candidate reductions in the cache linter, test wrapper and history
   implementations against identical fixtures and terminal outcomes before
   removing safeguards. Collect hosted timing separately.

Until these are complete, the zero-unexplained-skips and complete-union goals
are not established.
