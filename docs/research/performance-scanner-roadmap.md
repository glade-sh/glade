# Post-Performance-Scanner Roadmap

> Generated from deep research on Salesforce managed packages, ISV patterns,
> existing static analyzers (PMD 7.25.0, Code Analyzer 5.13.0, apex-ls v6.0.2,
> SonarQube, Checkmarx, CodeScan, Gearset), and the 10 identified tooling gaps.
> 
> **Prerequisite:** The base scanner in `internal/perfscan` must be landed first
> (Tasks 1-10 in `docs/superpowers/plans/2026-06-09-salesforce-performance-scanner.md`).

## Motivation

The base scanner covers static Apex heuristics, metadata scanning, and measured
trace ingestion. But the current Salesforce tooling landscape has 10 material
gaps that no existing tool addresses well. This roadmap closes the highest-value
gaps in priority order.

## Phase 1: Apex AST-Based Scanning (replace regex heuristics)

**Why now:** The base scanner uses regex to find loops, SOQL, DML, and describe
calls. Regex cannot distinguish `for` inside a string literal from a real loop,
cannot track nested loop depth, cannot follow control flow, and produces false
positives on commented-out code. The existing `internal/apexast` parser already
produces a full AST with exact ranges.

### Phase 1a: AST-aware loop detection

- Walk `apexast.Result.Files[].Declarations[]` for `ForStatement`, `WhileStatement`, `DoWhileStatement` nodes.
- Compute exact start/end ranges from `apexast.Range` fields instead of brace matching.
- Track loop nesting depth.
- Replace `loopBlocks()` in `apex_scan.go` with AST walks.

### Phase 1b: AST-aware SOQL/DML detection inside loops

- Walk child nodes inside each loop body.
- Detect `SoqlQueryExpression`, `DmlStatement`, `MethodCallExpression` (for Database.insert/update, System.enqueueJob, etc.).
- Use type resolution from `typesys.Index` where available to distinguish Database methods from unrelated `insert` method calls on custom classes.
- Replace regex-based `soqlInlineRe`, `dmlStatementRe`, `databaseDMLRe` detection with AST walks.

### Phase 1c: AST-aware describe and async detection

- Detect `Schema.getGlobalDescribe()`, `Schema.describeSObjects()`, `.getDescribe()` via AST method call nodes.
- Detect `System.enqueueJob()`, `Database.executeBatch()`, `System.schedule()` via AST method call nodes.
- Use `typesys.Index` to confirm the method call resolves to the platform method, not a custom method with the same name.

### Phase 1d: False-positive elimination

- Skip string literals: `'for (x in y)'` is not a loop.
- Skip comments: AST parser already strips them.
- Skip code inside `if (Test.isRunningTest())` blocks (not a real perf issue).
- Add regression tests with Apex fixtures containing string-literal loops, commented-out SOQL, and method calls that shadow platform methods.

## Phase 2: Selectivity And SOQL Analysis

**Why now:** PMD 7.4.0 has `AvoidNonRestrictiveQueries` (no WHERE or LIMIT)
and PMD 7.0.0 has `OperationWithHighCostInLoop`. But no tool predicts whether
a SOQL query will use an index or perform a full table scan. Salesforce's own
Query Plan tool only works on live orgs with data. A static selectivity
analyzer fills the gap.

### Phase 2a: WHERE clause selectivity scoring

- Parse SOQL via `internal/apexast`'s SOQL parsing.
- Score each WHERE clause predicate on selectivity:
  - **High selectivity** (score +0): Id field, indexed fields (Name, OwnerId, CreatedDate, SystemModStamp, RecordTypeId, lookup/master-detail fields, external ID fields, unique fields).
  - **Medium selectivity** (score +1): Text fields with `=` operator, date fields with `=`.
  - **Low selectivity** (score +3): `LIKE` with leading `%`, `!=`, `NOT`, `NOT LIKE` (prevents index use).
  - **Unknown** (score +2): Formula fields, compound fields (BillingAddress), non-indexed custom fields.
- Flag queries with cumulative score >= threshold as "non-selective".
- Flag queries with no WHERE clause at all (severity high).

**Note:** Skinny Tables are a Salesforce Support-enabled feature not covered in
public developer docs — they cannot be statically detected. The scanner should
not reference them.

### Phase 2b: SOQL aggregation scan

- Detect queries that return more fields than needed (e.g., `SELECT *` via `SELECT FIELDS(ALL)` or 20+ explicit fields).
- Detect queries with subqueries that amplify row counts (parent-child subqueries without LIMIT).
- Flag queries that reference formula fields in WHERE (not indexable).
- Flag queries with `ORDER BY` on non-indexed fields (full sort required).

### Phase 2c: ORG-backed query plan enrichment (future)

- Accept an optional org connection and run `EXPLAIN` on flagged queries against a sandbox.
- Merge the Query Plan results into findings as `ConfidenceMeasured` evidence.
- Mark as a separate CLI flag (`--org` or `--query-plan`).

## Phase 3: PR Performance Impact Analysis

**Why now:** Every other tool (PMD, Code Analyzer, SonarQube) does full-project
scans. In a PR, the reviewer doesn't need 500 findings from unchanged code — they
need the **3 new findings the PR introduced**. This is the daily code review
use case that no existing Salesforce tool addresses.

**Design:** Run the scanner on the base branch, run on the PR branch, compare
findings. Report only deltas: new findings, removed findings, score changes.
Flag entry points downstream of changed methods.

**Implementation plan:** `docs/superpowers/plans/2026-06-09-pr-performance-impact.md`

### Phase 3a: Baseline comparison engine

- Accept `--baseline <path>` pointing to a prior scanner JSON report.
- Compare findings by key (ID + file + line). Categorize each as new/removed/changed.
- Changed findings surface score deltas: `perf.dml.loop score: 55 → 75 (+20)`.
- Output as JSON (for CI tooling) or Markdown (for PR comments).

### Phase 3b: Entry-point impact propagation

- When a PR changes a method that's called from an entry point (trigger, invocable, batch, queueable, REST, LWC wire), flag the entry point as affected.
- Build a call graph from entry points → files they depend on using the type index.
- Report: "AccountHandler.cls change affects 3 entry points: AccountTrigger, AccountBatch, AccountREST."
- This tells reviewers which parts of the system are impacted by a seemingly-small change.

### Phase 3c: CI integration

- `--fail-on-new` flag: exit code 1 if any new findings are detected (CI gate).
- `--ci` flag: emit GitHub Actions workflow annotations (`::warning file=...::soql in loop`).
- `--sarif` flag: emit SARIF v2.1.0 for GitHub Advanced Security, GitLab SAST, VS Code.
- Baseline committed to repo as a CI artifact (similar to `package-lock.json`).

### Phase 3d: SARIF output

- Map `Finding.Severity` → SARIF `result.level` (high→error, medium→warning, low→note).
- Map `Finding.Location` → SARIF `result.locations[].physicalLocation`.
- Map `Finding.Fix` → SARIF `result.fixes[]` with a `description.text`.
- Use SARIF v2.1.0 schema (`$schema: "https://json.schemastore.org/sarif-2.1.0.json"`).
- Enables native integration with GitHub code scanning alerts, GitLab SAST dashboard, VS Code Problems panel.

## Phase 4: Async Chain Analysis (implemented)

**Why now:** Queueable chaining, Batch finish-to-start chains, and recursive
`System.enqueueJob()` patterns can create unbounded async work or exhaust daily
async limits. No existing tool traces async call chains statically.

### Phase 4a: Queueable chain detection (implemented)

- Walk AST for `System.enqueueJob(new SomeQueueable(...))` calls.
- Follow the `SomeQueueable.execute()` method for further `enqueueJob` calls.
- Build a call graph of queueable → queueable transitions.
- Flag chains deeper than 3 levels (risk of stacking limit at 50 jobs/day per chain).
- Flag chains that form cycles (queueable A → queueable B → queueable A).

### Phase 4b: Batch chain detection (implemented)

- Detect `Database.executeBatch()` in `Batchable.finish()`.
- Detect `Database.executeBatch()` in `Batchable.execute()`.
- Flag unbounded batch chains (no termination condition in source).
- Flag batches that enqueue Queueable jobs from `execute()` (transaction scope risk).

### Phase 4c: Future/schedule detection (implemented)

- Detect `@Future` methods (flagged by PMD but useful to surface in a unified report).
- Detect `System.schedule()` with recursive rescheduling patterns.
- Flag `@Future(callout=true)` mixed with DML in the same method (callout+DML = runtime error).

## Phase 5: Trigger Analysis Deepening

**Why now:** The base scanner detects triggers as entry points and flags
SOQL/DML in loops inside triggers. But it does not analyze trigger execution
order, re-entry prevention, or handler dispatch patterns.

### Phase 5a: Re-entry prevention audit

- Detect `static Boolean` flags used for re-entry prevention.
- Flag triggers that lack any re-entry guard.
- Detect the common pattern:
  ```apex
  public static Boolean isRunning = false;
  // in handler:
  if (isRunning) return;
  isRunning = true;
  ```
- Flag incorrect patterns: missing `isRunning = true` set, or missing `return` check.
- Flag re-entry guards that use instance variables instead of static (won't work).

### Phase 5b: Trigger handler dispatch audit

- Detect trigger → handler class dispatch.
- Verify the handler class follows the one-trigger-per-object pattern.
- Flag triggers that contain logic directly (violation of PMD `AvoidLogicInTrigger`).
- Flag trigger handler classes that use `Trigger.old[0]` / `Trigger.new[0]` (non-bulkified, PMD `AvoidDirectAccessTriggerMap`).

### Phase 5c: Cross-namespace trigger ordering

- In multi-package projects, model trigger execution order based on package install order.
- Flag scenarios where Package A trigger + Package B trigger + subscriber trigger
  combined SOQL budget may exceed shared limits.
- This is a static estimation; actual behavior depends on runtime data and install order.

## Phase 6: Platform Cache Analysis

**Why now:** Platform Cache is a critical performance optimization for ISV
packages but no static analyzer validates cache patterns. Misconfigured cache
keys, missing miss handlers, or unbounded cache growth can waste limits.
Salesforce docs provide concrete limits for the scanner to validate against.

**Platform Cache limits (from `apex_platform_cache_limits.md`):**
- Enterprise Edition: **10 MB** total org cache
- Unlimited/Performance Edition: **30 MB** total org cache
- Max single cached item: **100 KB**
- Session cache TTL: 300s min – **28,800s (8 hours)** max
- Org cache TTL: 300s min – **172,800s (48 hours)** max (default 24 hours)
- Local cache per partition per request: **500 KB** (session), **1,000 KB** (org)
- Security-reviewed managed packages get **3 MB free** cache (Provider Free capacity)

### Phase 6a: Cache miss handler detection

- Detect `Cache.Org.get()` and `Cache.Session.get()` calls.
- Verify the call site implements `Cache.CacheBuilder` or has a `doLoad()` miss handler.
- Flag `get()` calls without a miss handler (will return null on miss, causing NPE downstream).

### Phase 6b: Cache key design

- Detect cache keys built from string concatenation of runtime values.
- Flag keys that don't include a namespace or partition prefix (risk of collision).
- Flag cache keys that may exceed the 255-character limit (concatenation of long strings).

### Phase 6c: Cache data volume

- Detect `Cache.Org.put()` and `Cache.Session.put()` calls.
- Flag put calls with large objects (detected by serialization hints, e.g., full SObject records).
- Flag unbounded cache growth patterns (put in loops, accumulates without eviction).

## Phase 7: LWC And Aura Performance Rules

**Why now:** PMD and existing tools focus on Apex. LWC and Aura performance
(JavaScript-side) has no static analysis coverage. Wire adapter misuse, excessive
imperative calls, and DOM re-render storms are common performance issues.

### Phase 7a: LWC wire adapter analysis

- Parse LWC JavaScript files for `@wire` decorator usage.
- Detect `@wire` parameters that use `$propertyName` (reactive) vs hardcoded (call-once).
- Flag `@wire` that depends on multiple reactive properties (re-fires on any change).
- Flag `@wire` at the top level with no config parameter → hardcoded, call-once.
- Flag `@wire(apexMethod, {})` (no params → queries every time).

### Phase 7b: Imperative vs wire detection

- Parse for `import apexMethod from '@salesforce/apex/...'` + `apexMethod()` calls.
- Compare imperative call patterns to `@wire` patterns on the same Apex method.
- Flag the same Apex method being called both via `@wire` and imperatively (duplicate work).

### Phase 7c: Excessive reactivity

- Parse for `@track` decorator on properties that don't change after initialization.
- Flag `@track` properties in loops or on large objects (re-render cost).
- Detect `@api` properties used as inputs from parent + `@track` on internal state (correct pattern).

### Phase 7d: Aura server action analysis

- Parse Aura component markup and controllers.
- Detect `aura:handler` with `init` event + server action dispatch (load-time server call).
- Flag multiple server actions dispatched in a single init handler.
- Compare Aura server calls to LWC patterns for migration recommendations.

## Phase 8: SARIF Output And CI Integration

**Why now:** The base scanner outputs JSON and Markdown. For CI integration,
SARIF (Static Analysis Results Interchange Format) is the industry standard.
Code Analyzer 5.x, PMD, ESLint, and SonarQube all emit SARIF. GitHub Advanced
Security, GitLab SAST, and VS Code consume SARIF natively.

### Phase 8a: SARIF writer

- Add `internal/perfscan/sarif.go` with `WriteSARIF()`.
- Map `Finding.Severity` → SARIF `result.level` (high→error, medium→warning, low→note).
- Map `Finding.Location` → SARIF `result.locations[].physicalLocation`.
- Map `Finding.Fix` → SARIF `result.fixes[]` with a `description.text`.
- Map `Finding.Evidence` → SARIF `result.properties.evidence`.
- Use the SARIF v2.1.0 schema (`$schema: "https://json.schemastore.org/sarif-2.1.0.json"`).

### Phase 8b: SARIF test

- Write `internal/perfscan/sarif_test.go`.
- Marshal a `Report` to SARIF, validate against the JSON schema.
- Verify `$schema`, `version`, `runs[0].tool.driver.name == "glade"`.
- Verify `runs[0].results` count matches report findings.

### Phase 8c: SARIF in CLI

- Add `--sarif` flag to `glade inspect performance`.
- Default stays Markdown for human-readable output.
- SARIF is for CI pipelines: `glade inspect performance --project . --sarif > glade-perf.sarif`.

## Phase 9: Trigger Recursion And Re-Entry Detection

**Why now:** Trigger recursion is the most common production performance outage
in Salesforce. A record update fires a trigger, which updates another record,
which fires another trigger, which updates the first record — and the
transaction either hits the 16-deep recursion limit or consumes all governor
limits before anyone notices. PMD requires developers to hand-write re-entry
guards; no tool validates they actually work.

### Phase 9a: Re-entry guard audit

- Detect `static Boolean` flags used for re-entry prevention.
- Flag classes referenced by triggers that lack any re-entry guard.
- Validate the guard pattern is complete:
  ```apex
  // Correct pattern
  public static Boolean isRunning = false;
  if (isRunning) return;
  isRunning = true;
  try { /* work */ } finally { isRunning = false; }
  ```
- Flag incorrect patterns: missing the `return` check, missing the `set true`,
  missing the `finally` reset, or instance variables instead of static.

### Phase 9b: Cross-object recursion simulation

- Build a trigger dependency graph: "Account trigger updates Contact → Contact trigger updates Case → Case trigger updates Account."
- Detect cycles in the trigger dependency graph (potential recursion).
- Flag trigger chains deeper than 3 objects (risk of recursion depth limit at 16).
- Flag trigger chains that could update the originating object (Account → Contact → Account).

### Phase 9c: Handler dispatch best practices

- Detect trigger → handler class dispatch patterns.
- Flag triggers that contain logic directly (PMD `AvoidLogicInTrigger` but with context about handler structure).
- Flag handler classes using `Trigger.new[0]` (non-bulk, PMD `AvoidDirectAccessTriggerMap`).
- Verify the handler follows the one-trigger-per-object pattern for consistent execution order.

## Phase 10: Profiling Integration With Apex Debug Logs

**Why now:** Salesforce debug logs include profiling events with microsecond
timestamps. The `internal/profile` package can already parse Chrome trace format.
Extending it to parse Apex debug log profiling output would let the scanner merge
real profiling data with static findings.

**Relevant debug log events (from `apex_debugging_system_log_console.md`):**
- `LIMIT_USAGE_FOR_NS` — Per-namespace governor limit consumption (FINEST level)
- `SOQL_EXECUTE_EXPLAIN` — Query plan details showing index/table-scan decisions
- `SOQL_EXECUTE_BEGIN` / `SOQL_EXECUTE_END` — Query execution with duration in ms
- `HEAP_ALLOCATE` / `HEAP_DEALLOCATE` — Heap allocation tracking
- `SYSTEM_METHOD_ENTRY` / `SYSTEM_METHOD_EXIT` — Method-level enter/exit with source line and argument info
- Debug log max size: **20 MB**; system logs retained **24 hours**; monitoring logs **7 days**; 1,000 MB/15-min logging cap

### Phase 10a: Apex profiling log parser

- Add `internal/profile/debug_log.go` with a parser for Apex debug log profiling sections.
- Parse `SYSTEM_METHOD_ENTRY` and `SYSTEM_METHOD_EXIT` lines to extract:
  ```text
  09:15:23.1 (1234567)|SYSTEM_METHOD_ENTRY|[123]|MyClass.myMethod(List<Account>)
  09:15:23.1 (1235067)|SYSTEM_METHOD_EXIT|[123]|MyClass.myMethod
  ```
- Compute duration = exit_timestamp - entry_timestamp.
- Map to `trace.Duration` events.

### Phase 10b: Merge profiling data with static findings

- Accept `--profiling-log` flag in `glade inspect performance`.
- Parse the debug log, extract method-level timing.
- For each `perf.soql.loop` finding, check if the profile log shows method durations to confirm.
- Upgrade `Confidence` from `static` to `combined` when profiling data corroborates static findings.
- Add profile-sourced measurements to the report.

## Phase 11: Run-Time Limit Estimation

**Why now:** No existing tool estimates how many governor limit units a code
path will consume. This is the hardest problem on the roadmap but the
highest-value for ISV partners. Static limit estimation requires data flow
analysis, loop iteration bounds, and query row estimation.

### Phase 11a: Loop iteration bound inference

- Detect `for (SObject item : collection)` loops.
- Trace `collection` back to its source: SOQL result, `Trigger.new`, method parameter.
- Bound the loop iteration count:
  - `Trigger.new` → 200 (standard trigger chunk per Salesforce docs).
  - SOQL for-loop → 200 (default batch size).
  - Batch `execute()` scope → configurable (default 200, max 2,000).
  - SOQL result → no static bound (use configurable default, e.g., 200).
  - Method parameter → no static bound (use configurable default).

### Phase 11b: Per-entry-point limit estimation

- For each entry point (trigger, batch, queueable, etc.), walk the call graph.
- Count:
  - SOQL queries (static + dynamic).
  - DML statements (insert, update, delete, upsert, undel, merge).
  - Async enqueue calls (max 50 `System.enqueueJob` per transaction).
  - Schedule calls.
  - Future annotations (max 50 per transaction).
  - Email invocations (max 10 `sendEmail` per transaction).
  - Callouts (max 100 per transaction, 120s cumulative timeout).
- Multiply by loop iteration bounds where applicable.
- Produce an estimated limits consumption summary.
- Contextualize against certified (independent limits) vs non-certified (shared limits):
  - **Non-certified packages**: Flag >50% of sync SOQL (100), sync DML (150).
  - **Certified packages**: Flag >50% of independent limits, but always flag
    heap (6 MB sync / 12 MB async) and CPU time (10s sync / 60s async) as shared.
- Cross-namespace ceiling for certified packages: **11×** per-namespace limits
  for SOQL (1,100), DML (1,650), SOSL (220), callouts (1,100).

### Phase 11c: Heap estimation (basic)

- Detect `List<SObject>` and `Map<Id, SObject>` type allocations.
- Estimate heap based on field count per SObject × estimated row count.
- Flag allocations that may approach the shared **6 MB** (sync) / **12 MB** (async) heap limit.
- Method bytecode limit: **65,535 instructions** per method (flag methods approaching this).
- This is approximate; precise heap estimation requires object size modeling beyond static analysis scope.

## Phase Ordering Rationale

1. **Phase 1 (AST-based)**: Foundation. Removes false positives from regex
   scanner, enables all subsequent AST-based phases. Highest immediate ROI.

2. **Phase 2 (Selectivity)**: Complements the existing `perf.soql.loop` finding
   with *why* a query is risky. High value for customers migrating from large
   orgs. No equivalent in any existing tool.

3. **Phase 3 (PR Impact)**: Daily code review use case. Shows reviewers exactly
   what performance impact a PR introduces — not a 500-finding full-project scan,
   but the 3 new findings from changed code. GitHub annotations, CI gating,
   SARIF output. No existing Salesforce tool does PR-diff analysis.

4. **Phase 4 (Async Chains)**: Structural analysis of async call graphs. PMD
   flags individual `enqueueJob` calls but cannot trace chains. High value for
   debugging transaction storms. Implemented.

5. **Phase 5 (Trigger Deepening)**: Extends existing trigger detection with
   handler dispatch audit. Complements PMD's `AvoidLogicInTrigger` and
   `AvoidDirectAccessTriggerMap`.

6. **Phase 6 (Platform Cache)**: Validates cache patterns (miss handlers, key
   design, volume) for teams using Platform Cache heavily.

7. **Phase 7 (LWC/Aura)**: Expands scanner scope beyond Apex. Competitive
   differentiator (PMD/SonarQube are Apex-only).

8. **Phase 8 (VM Profiling)**: `glade test --max-queries 20` — per-test governor
   limit gating. Unique to glade because no other tool has a VM that can execute
   Apex and track limits.

9. **Phase 9 (Trigger Recursion)**: Detects trigger dependency cycles and
   validates re-entry guards automatically. Most common production outage pattern
   in Salesforce.

10. **Phase 10 (Profiling)**: Bridges static analysis and runtime measurement.
    Merges Apex debug log profiling with static findings.

11. **Phase 11 (Limit estimation)**: Hardest problem, highest value. Requires
    Phases 1-5 as foundation. Unique to glade because of the VM.

## What This Roadmap Does NOT Cover

- **Security scanning**: Security rules (SOQL injection, CRUD/FLS, sharing,
  XSS) are covered by PMD, Code Analyzer 5.x, and commercial tools. glade
  should not duplicate this; integrate with SARIF output for unified reporting
  instead.

- **Code style enforcement**: Naming conventions, formatting, documentation
  requirements are covered by PMD Code Style and Documentation categories.

- **Deployment validation**: Code Analyzer is required for AppExchange
  Security Review. glade's scanner is advisory, not a gate.

- **Real-time profiling in production**: The scanner is a static analysis tool
  with optional trace/profile input. Production monitoring belongs in a
  separate observability product.

## Preconditions For Any Phase

- The base scanner (Tasks 1-10) is landed and tested.
- `internal/apexast` produces a complete AST with accurate source ranges.
- `internal/typesys` provides type resolution for method calls and variable types.
- `internal/project` loads SFDX project structure and provides file lists for
  all metadata types (Apex, LWC, Aura, Flow, Workflow, Visualforce).
