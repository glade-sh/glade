# OAER Local Test Performance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make broad `oaer compat local-tests` runs finish as fast as possible on Salesforce-shaped projects.

**Architecture:** Attack the largest repeated costs first: cold process startup, full org clones, per-test setup cloning, unbalanced shards, and cold parse/type/IR work. Keep the runtime in Go. Add storage and runner abstractions only where profiles prove repeated copying or cold work dominates.

**Tech Stack:** Go 1.26, `go test`, `runtime/pprof`, Go execution traces, Go PGO, tree-sitter incremental parsing, existing `internal/*` packages, existing compatibility fixtures, `oaer compat local-tests`.

---

## Current Signals

These recommendations come from the current repo, prior local-test frontier runs, and current Go/tree-sitter/SQLite documentation:

- `compat local-tests` exposes `--parallel`, `--parallel-methods`, `--perf-json`, `--cpu-profile`, `--mem-profile`, `--changed-since`, and `--class-file`.
- The example-project baseline script still shells through `go run ./cmd/oaer`, which pays build/startup cost per project.
- Full-suite `compat local-tests` routes through `localTestParallelism`, class batching, and `apextest.RunCasesContext`.
- `apextest.runCase` clones the base VM and often calls `cloneRuntimeOrg` before every test method.
- `storage.OrgState.CloneRuntime` deep-clones object definitions, records, indexes, ID sequences, and transactions.
- `dml.Engine.WithTransaction` and VM DML paths still use whole-org rollback snapshots in several places.
- Per-class `@TestSetup` gives a natural isolation boundary. Sequential methods in one class can run from a setup snapshot, journal their own mutations, and roll back without a full clone.
- Prior frontier work found that monolithic class-file shards can burn 20+ minutes without useful movement. Duration-balanced shards should replace count-balanced shards.
- Go's standard profile and trace tools are enough to find the next bottleneck.
- PGO is a later polish after representative profiles exist.
- Tree-sitter incremental parsing matters for watch and changed-since workflows, not cold full-corpus runs.
- SQLite savepoints are a useful model for mark/rollback semantics. Moving the runtime to SQLite is not first-order performance work unless profiles prove map storage is the bottleneck.

## Performance North Star

Optimize for wall-clock local test completion on broad Salesforce-shaped projects.

Primary command shape:

```bash
./bin/oaer-perf compat local-tests \
  --project example-projects/src-nmb-nu-develop \
  --json \
  --timeout 60000 \
  --parallel "$(sysctl -n hw.logicalcpu)" \
  --parallel-methods \
  --perf-json /tmp/oaer-local-tests.perf.json \
  --cpu-profile /tmp/oaer-local-tests.cpu.pprof \
  --mem-profile /tmp/oaer-local-tests.mem.pprof
```

Track these numbers before and after every performance lane:

- total wall-clock seconds
- cases discovered
- cases run
- run phase milliseconds
- load/discover/analyze phase milliseconds
- top slow classes by summed test duration
- CPU profile top cumulative functions
- heap profile top allocators
- `CloneRuntime`, `CloneRollbackSnapshot`, `Record.Clone`, `ObjectDefinition.Clone`, map allocation, and GC share

Do not claim a performance win from one focused method. A focused method may prove correctness. A representative class shard or project sentinel proves speed.

## Work Order

Run these lanes in order:

1. Performance baseline harness and budgets.
2. Build-once corpus runner and PGO hook.
3. Parallelism defaults and duration-balanced sharding.
4. Runner isolation journal for sequential class methods.
5. Frozen runtime org template with shared immutable shape.
6. Copy-on-write record/index snapshots for parallel isolation.
7. Warm runner daemon for watch and changed-since loops.

## Lane 1: Performance Baseline Harness

**Purpose:** Put numbers on the slow path before changing it. The profile is the pencil line.

**Files:**

- Modify: `internal/compat/local_tests.go`
- Modify: `internal/compat/local_tests_test.go`
- Modify: `internal/apextest/runner_benchmark_test.go`
- Create: `scripts/local-test-perf.sh`
- Create: `docs/fixtures/perf/local-tests-baseline.example.json`

**Commands to preserve:**

```bash
go test ./internal/storage -bench 'BenchmarkOrgStateClone' -benchmem -run '^$'
go test ./internal/apextest -bench 'BenchmarkRunTestSuite' -benchmem -run '^$'
go run ./cmd/oaer compat local-tests \
  --project example-projects/src-nmb-nu-develop \
  --json \
  --timeout 60000 \
  --parallel "$(sysctl -n hw.logicalcpu)" \
  --parallel-methods \
  --perf-json /tmp/oaer-local-tests.perf.json \
  --cpu-profile /tmp/oaer-local-tests.cpu.pprof \
  --mem-profile /tmp/oaer-local-tests.mem.pprof
```

**Steps:**

- [ ] Add per-phase allocation and clone counters behind the existing `--perf-json` path. Keep the JSON additive.
- [ ] Count calls to `cloneRuntimeOrg`, `OrgState.CloneRuntime`, and `CloneRollbackSnapshot` during a local-test run.
- [ ] Add `topCloneClasses` to the perf JSON by class name, with `setupClones`, `testClones`, and `durationMs`.
- [ ] Expand `BenchmarkRunTestSuite` to include 100, 500, and 1000 methods with and without `@TestSetup`.
- [ ] Add `scripts/local-test-perf.sh` that builds a local binary once, runs the representative project command, stores perf JSON and profiles under `/tmp/oaer-perf/<timestamp>/`, and prints a capped summary.
- [ ] Run the script once against `src-nmb-nutpl-develop` and once against `src-nmb-nu-develop` if both projects are present.
- [ ] Commit the harness and baseline example, not the generated `/tmp` profiles.

**Acceptance:** Every later lane can show whether wall time, clone count, allocation count, or GC share moved.

## Lane 2: Build-Once Corpus Runner And PGO Hook

**Purpose:** Stop paying compile/startup cost for every corpus project. Add PGO as a measured polish, not as a guess.

**Files:**

- Modify: `scripts/baseline-local-tests-example-projects.mjs`
- Create: `scripts/build-oaer-for-perf.sh`
- Modify: `docs/LOCAL_APEX_TEST_EXECUTION_PLAN.md`
- No runtime behavior changes.

**Design:**

- `scripts/build-oaer-for-perf.sh` builds `./bin/oaer-perf` once.
- The baseline script accepts `OAER_BIN`, defaults to `./bin/oaer-perf` when present, and falls back to `go run ./cmd/oaer` only when no binary exists.
- PGO uses a representative CPU profile from `compat local-tests`, passed with `go build -pgo=<profile>`.
- Do not commit `default.pgo` until the team decides the profile is representative enough for reproducible releases.

**Steps:**

- [ ] Add `OAER_BIN` support to `scripts/baseline-local-tests-example-projects.mjs`.
- [ ] Replace the hard-coded `go run ./cmd/oaer compat local-tests` command with the configured binary plus args.
- [ ] Add `scripts/build-oaer-for-perf.sh` with `CGO_ENABLED=0 go build -trimpath -o ./bin/oaer-perf ./cmd/oaer`.
- [ ] Add optional `PGO_PROFILE=/path/to/profile.pprof` support that runs `go build -trimpath -pgo="$PGO_PROFILE" -o ./bin/oaer-perf ./cmd/oaer`.
- [ ] Document the flow in `docs/LOCAL_APEX_TEST_EXECUTION_PLAN.md`.
- [ ] Validate with one small fixture project and one example project.
- [ ] Commit script and docs changes only.

**Acceptance:** Corpus baselines no longer invoke `go run` per project, and PGO can be tested without changing release builds.

## Lane 3: Parallelism Defaults And Duration-Balanced Sharding

**Purpose:** Keep all cores busy and stop one shard from carrying the whole ridge pole.

**Files:**

- Modify: `internal/compat/local_tests.go`
- Modify: `internal/compat/local_tests_test.go`
- Modify: `internal/oaercli/cli.go`
- Modify: `internal/oaercli/cli_test.go`
- Create: `internal/compat/local_test_shards.go`
- Create: `internal/compat/local_test_shards_test.go`
- Create: `docs/fixtures/perf/local-test-durations.example.json`

**Design:**

- Default full-project `compat local-tests` parallelism should use a conservative CPU count when no `--parallel` is supplied.
- Keep `--parallel 1` as the explicit serial mode.
- Enable method-level parallelism only when tests do not require shared class mutation, or when the user passes `--parallel-methods`.
- Write perf JSON with per-class duration history.
- Add a shard planner that accepts class durations and emits `N` balanced class files.

**Steps:**

- [ ] Add tests for `localTestParallelism` covering full project, focused class, focused method, and explicit serial mode.
- [ ] Change the default full-project value to `min(runtime.GOMAXPROCS(0), 8)` unless profiles prove higher is better.
- [ ] Add `--shard-count <n>` and `--shard-index <i>` to `compat local-tests` for direct CI splitting.
- [ ] Add `--write-class-shards <dir>` and `--duration-history <path>` to produce class-file shards without running tests.
- [ ] Implement longest-processing-time shard assignment: sort classes by prior duration descending, then place each class onto the shard with the lowest current total.
- [ ] Fall back to method count when no duration exists.
- [ ] Add tests proving one slow class does not land in the same shard as the next slowest class when alternatives exist.
- [ ] Run focused compat and CLI tests.
- [ ] Run the existing monolithic class-file sentinel as balanced shards and record wall-clock improvement in the plan notes.

**Acceptance:** A broad project can be split into balanced shards from prior timings, and the default full-project run uses more than one core unless serial mode is explicit.

## Lane 4: Runner Isolation Journal For Sequential Class Methods

**Purpose:** Avoid full org clone per test method when methods run sequentially in one class.

**Files:**

- Create: `internal/apextest/isolation_journal.go`
- Create: `internal/apextest/isolation_journal_test.go`
- Modify: `internal/apextest/runner.go`
- Modify: `internal/apextest/runner_test.go`
- Modify: `internal/storage/model.go`
- Modify: `internal/storage/model_test.go`

**Design:**

For a class with `@TestSetup`, run setup once into `setupOrg`. For sequential methods in that class, use a journal mark before each method and roll back mutations after each method. Only clone when method-level parallelism is active or when a test uses unsupported mutation paths the journal cannot track.

The first implementation should track storage-level mutations visible to tests:

```go
type IsolationJournal struct {
    inserted []storage.ID
    updated  []recordBefore
    deleted  []recordBefore
    sequences map[string]uint64
}
```

Use explicit fallback:

```go
type IsolationMode string

const (
    IsolationJournaled IsolationMode = "journaled"
    IsolationCloned    IsolationMode = "cloned"
)
```

**Steps:**

- [ ] Write `TestIsolationJournalRestoresInsertUpdateDeleteAndSequences`.
- [ ] Write `TestRunSequentialMethodsReuseSetupOrgWithJournal`.
- [ ] Add perf counters: `isolationMode`, `journalRollbacks`, and `cloneFallbacks`.
- [ ] Implement the journal for inserts, updates, deletes, undeletes, and ID sequence changes.
- [ ] In `runTestPlansWithSetups`, use journal mode only when `opts.ParallelMethods` is false for that class.
- [ ] Fall back to clone mode when the journal sees a mutation kind it cannot reverse.
- [ ] Run `go test ./internal/apextest`.
- [ ] Profile a class with many methods and one `@TestSetup` before and after.

**Acceptance:** Sequential test methods with shared setup stop cloning the whole setup org per method, while isolation behavior remains identical.

## Lane 5: Frozen Runtime Org Template

**Purpose:** Stop deep-copying immutable org shape on every runtime clone.

**Files:**

- Create: `internal/storage/runtime_template.go`
- Create: `internal/storage/runtime_template_test.go`
- Modify: `internal/storage/model.go`
- Modify: `internal/storage/model_benchmark_test.go`
- Modify: `internal/apextest/runner.go`
- Modify: `internal/apextest/runner_test.go`
- Modify: `internal/vm/data_test.go`

**Design:**

Separate cold immutable shape from hot mutable data:

- Object definitions are immutable after project load for normal test execution.
- Records, indexes, ID sequences, and transactions remain mutable.
- Runtime code that must mutate definitions calls a storage helper that clones that one definition first.

Add helpers with narrow names:

```go
type RuntimeTemplate struct {
    Org storage.OrgState
}

func NewRuntimeTemplate(org OrgState) RuntimeTemplate
func (t RuntimeTemplate) CloneRuntimeOrg() OrgState
func EnsureMutableObjectDefinition(org *OrgState, objectName string) (*ObjectDefinition, bool)
```

The `bool` return reports whether a clone happened.

**Steps:**

- [ ] Write a benchmark showing current `CloneRuntime` cost on 60 objects and 450 records per object.
- [ ] Write `TestRuntimeTemplateSharesFrozenDefinitionsAndIsolatesRecords`.
- [ ] Write `TestEnsureMutableObjectDefinitionClonesOnlyOneObject`.
- [ ] Implement `RuntimeTemplate` using shared definition maps for frozen objects.
- [ ] Audit writes to `ObjectState.Definition` and route them through `EnsureMutableObjectDefinition`.
- [ ] Change `apextest.runtimeCacheEntry` to store a `RuntimeTemplate` next to the base org.
- [ ] Route test setup and test method org creation through `RuntimeTemplate.CloneRuntimeOrg`.
- [ ] Run storage, apextest, and VM focused tests.
- [ ] Compare `BenchmarkOrgStateCloneRuntime` before and after.

**Acceptance:** Runtime org clone cost drops without sharing mutable records, indexes, sequences, or transaction frames between tests.

## Lane 6: Copy-On-Write Record And Index Snapshots

**Purpose:** Make parallel test isolation cheap enough that parallelism does not drown in allocation.

**Files:**

- Create: `internal/storage/snapshot.go`
- Create: `internal/storage/snapshot_test.go`
- Modify: `internal/storage/model.go`
- Modify: `internal/storage/model_benchmark_test.go`
- Modify: `internal/dml/dml.go`
- Modify: `internal/soql/soql.go`
- Modify: `internal/apextest/runner.go`

**Design:**

This is the largest storage change. Do it after the journal and frozen-template lanes prove where clone cost remains.

Add an internal copy-on-write snapshot representation that preserves the public `OrgState` behavior at package boundaries:

- Reads fall through to the base record map when no overlay exists.
- First write to an object creates an overlay record map for that object.
- First write to an index creates an overlay index set for that object.
- Commit is not needed for tests; rollback drops overlays.
- DML and SOQL helpers should use storage accessors instead of direct map walking where required.

**Steps:**

- [ ] Write snapshot tests for read-through, insert, update, delete, index lookup, ID sequence, and rollback.
- [ ] Add storage accessors for record iteration, record lookup, record put, record delete, and index lookup.
- [ ] Convert DML hot paths to accessors first.
- [ ] Convert SOQL hot paths to accessors next.
- [ ] Keep direct map access in tests until behavior is proven, then clean tests.
- [ ] Wire `apextest` parallel method/class isolation to snapshot mode.
- [ ] Run `go test ./internal/storage ./internal/soql ./internal/dml ./internal/apextest`.
- [ ] Run broad local-test profiles with and without `--parallel-methods`.

**Acceptance:** Parallel broad runs scale with cores without allocating a full deep org copy per test method.

## Lane 7: Warm Runner Daemon For Watch And Changed-Since

**Purpose:** Keep parse, type index, IR, runtime template, and duration history warm across repeated local runs.

**Files:**

- Create: `internal/testdaemon/daemon.go`
- Create: `internal/testdaemon/daemon_test.go`
- Modify: `internal/oaercli/cli.go`
- Modify: `internal/oaercli/cli_test.go`
- Modify: `internal/watch/watch.go`
- Modify: `internal/apexast` files only if incremental tree reuse requires adapter changes.
- Modify: `docs/EDITOR.md`
- Modify: `docs/LOCAL_APEX_TEST_EXECUTION_PLAN.md`

**Design:**

Keep this out of the cold one-shot path. The daemon helps editor/watch loops:

- Load project once.
- Cache source file text, tree-sitter trees, type index, compiled methods, compiled test invocations, runtime template, and duration history.
- On file changes, use tree-sitter incremental parsing where the adapter can safely preserve old trees.
- Rebuild only affected symbol and test slices when possible.
- Fall back to full rebuild when dependency impact is unclear.

**Steps:**

- [ ] Add daemon interface tests with an in-memory project.
- [ ] Add an internal service that can run `RunChangedSince(ref)` and `RunFilter(filter)` against a warm project.
- [ ] Move existing watch command logic to call the daemon service.
- [ ] Add tree-sitter tree reuse in `internal/apexast` only after adapter tests prove identical diagnostics for edited files.
- [ ] Add `oaer test --daemon` or reuse watch mode only if CLI design stays simple.
- [ ] Document editor use in `docs/EDITOR.md`.
- [ ] Validate with `oaer test --watch-once --changed-since main --json` if the final CLI supports both flags.

**Acceptance:** Repeated watch/changed-since runs avoid cold project load and full parse when a small edit changes one file.

## Final Integration Gate

After all lanes land:

```bash
scripts/build-oaer-for-perf.sh
scripts/local-test-perf.sh
go test ./internal/storage -bench 'BenchmarkOrgStateClone' -benchmem -run '^$'
go test ./internal/apextest -bench 'BenchmarkRunTestSuite' -benchmem -run '^$'
go test ./internal/storage ./internal/soql ./internal/dml ./internal/apextest ./internal/compat
./bin/oaer-perf compat local-tests \
  --project example-projects/src-nmb-nutpl-develop \
  --json \
  --timeout 60000 \
  --parallel "$(sysctl -n hw.logicalcpu)" \
  --parallel-methods \
  --perf-json /tmp/oaer-nutpl.perf.json \
  --cpu-profile /tmp/oaer-nutpl.cpu.pprof \
  --mem-profile /tmp/oaer-nutpl.mem.pprof
```

Run `src-nmb-nu-develop` after `src-nmb-nutpl-develop` moves cleanly:

```bash
./bin/oaer-perf compat local-tests \
  --project example-projects/src-nmb-nu-develop \
  --json \
  --timeout 60000 \
  --parallel "$(sysctl -n hw.logicalcpu)" \
  --parallel-methods \
  --perf-json /tmp/oaer-nu.perf.json \
  --cpu-profile /tmp/oaer-nu.cpu.pprof \
  --mem-profile /tmp/oaer-nu.mem.pprof
```

## Research References

- Go diagnostics and profiling: https://go.dev/doc/diagnostics
- Go garbage collector model: https://go.dev/doc/gc-guide
- Go profile-guided optimization: https://go.dev/doc/pgo
- Go `sync.Pool` guidance: https://go.dev/pkg/sync/
- Go benchmark loop guidance: https://go.dev/blog/testing-b-loop
- Tree-sitter advanced parsing and concurrency: https://tree-sitter.github.io/tree-sitter/using-parsers/3-advanced-parsing.html
- SQLite in-memory database behavior: https://www.sqlite.org/inmemorydb.html
- SQLite savepoint rollback model: https://sqlite.org/lang_savepoint.html

## Risks

- Default parallelism can expose shared-state bugs that serial execution hides. Add serial-mode tests and keep `--parallel 1`.
- Journal rollback can miss side effects at first. Count clone fallbacks and keep fallback explicit until profiles prove coverage.
- Frozen definitions can leak mutation across tests if any writer bypasses `EnsureMutableObjectDefinition`.
- Copy-on-write storage can make simple reads slower if every read goes through heavy interfaces. Benchmark read-heavy SOQL before and after.
- A warm daemon can hide stale-index bugs. Every incremental path must have a full-rebuild fallback and parity test.
- PGO can polish hot code, but it will not fix an algorithm that copies the world per test.

## Completion Criteria

- Broad sentinel local-test wall time drops from the recorded baseline.
- Clone counters and heap allocation counters drop on broad local-test runs.
- Slow Cart/CartService local tests show reduced VM execution time or reduced rollback/profile cost.
- Balanced shards finish with no one shard taking more than 125% of the median shard time on the same machine.
- `--parallel 1` remains a correct serial escape hatch.
- No runtime behavior is implemented only to satisfy one example project.
