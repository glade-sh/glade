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
