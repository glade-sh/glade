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

## Phase 3: Managed Package-Aware Rules

**Why now:** Zero existing tools have managed-package-specific rules. PMD's
`AvoidGlobalModifier` exists but is unaware of packaging lifecycle. No tool
analyzes cross-namespace limit consumption, dynamic SOQL namespace injection
overhead, or license-check patterns.

### Phase 3a: Global API surface audit

- Detect `global` classes and `global` methods.
- Compute API surface size (count of global methods × parameters).
- Flag classes with high global surface area (risk of version lock-in).
- Compare global method signatures across git history or package version artifacts
  to detect breaking changes before release.
- Assign severity based on surface size: 1-5 global methods = low, 6-15 = medium, 16+ = high.

### Phase 3b: Namespace injection overhead

- Detect dynamic SOQL/SOSL strings with `String.format`, string concatenation, or variable injection for namespace prefixes.
- Flag methods that call `Schema.getNamespace()` and inject it into dynamic queries
  (runtime overhead per query).
- Suggest static SOQL with namespace-qualified field references where possible
  (the compiler handles namespace resolution at compile time for static SOQL).
- Detect the pattern:
  ```apex
  String ns = Schema.getNamespace();
  String query = 'SELECT ' + ns + '__Field__c FROM ' + ns + '__Object__c';
  // BAD: runtime overhead, SOQL injection risk
  ```
  versus:
  ```apex
  List<MyObject__c> records = [SELECT Field__c FROM MyObject__c];
  // GOOD: compile-time namespace resolution
  ```

### Phase 3c: License/framework overhead

- Detect Custom Metadata Type queries executed in trigger or synchronous transaction contexts.
- Flag license-check patterns that query `License__mdt` or similar metadata types
  without caching (Platform Cache, static variable).
- Detect `FeatureManagement.checkPermission()` calls in hot paths (UI controllers, triggers).

### Phase 3d: Certified vs non-certified package limit modeling

Salesforce docs (`apex_gov_limits.md`) distinguish two categories with
dramatically different limit behavior:

| Aspect | Non-Certified | Certified (AppExchange Security Review) |
|--------|--------------|----------------------------------------|
| Per-transaction SOQL | Shared with org (100 sync / 200 async) | **Separate, independent** limits |
| Per-transaction DML | Shared with org (150) | **Separate, independent** limits |
| Per-transaction SOSL | Shared with org (20) | Separate (220 cross-namespace = 11×20) |
| Per-transaction Callouts | Shared with org (100) | Separate (1,100 cross-namespace = 11×100) |
| Heap size | **Shared** (6 MB / 12 MB) | **Still shared** — heap is never independent |
| CPU time | **Shared** (10s / 60s) | **Still shared** — CPU is never independent |
| Transaction time | **Shared** (10 min) | **Still shared** |
| Max unique namespaces | 10 per transaction | 10 per transaction |
| Platform Cache | No free tier | **3 MB free** for security-reviewed packages |

**Implementation:**
- Detect if a project is a managed package (has `namespace` in sfdx-project.json).
- If no evidence of AppExchange certification, model limits as **shared** with subscriber org.
- If certified (needs a flag or config), model limits as **independent** for SOQL/DML/SOSL/callouts.
- Track per-entry-point governor limit budgets separately for certified vs non-certified paths.
- Flag entry points whose static analysis suggests >50% limit consumption.
- **Always** flag heap and CPU time as shared regardless of certification.

### Phase 3e: Dynamic SOQL namespace injection detection

- Detect dynamic SOQL/SOSL strings that concatenate `Schema.getNamespace()` or namespace prefixes.
- Flag patterns where static SOQL with compile-time namespace resolution would suffice.
- Detect SOQL injection risk from namespace concatenation (complements PMD `ApexSOQLInjection`).
  ```apex
  // BAD: runtime namespace resolution + SOQL injection risk
  String query = 'SELECT ' + Schema.getNamespace() + '__Field__c FROM Account';
  // GOOD: static SOQL with compile-time namespace resolution
  List<Account> records = [SELECT MyField__c FROM Account];
  ```

## Phase 4: Async Chain Analysis

**Why now:** Queueable chaining, Batch finish-to-start chains, and recursive
`System.enqueueJob()` patterns can create unbounded async work or exhaust daily
async limits. No existing tool traces async call chains statically.

### Phase 4a: Queueable chain detection

- Walk AST for `System.enqueueJob(new SomeQueueable(...))` calls.
- Follow the `SomeQueueable.execute()` method for further `enqueueJob` calls.
- Build a call graph of queueable → queueable transitions.
- Flag chains deeper than 3 levels (risk of stacking limit at 50 jobs/day per chain).
- Flag chains that form cycles (queueable A → queueable B → queueable A).

### Phase 4b: Batch chain detection

- Detect `Database.executeBatch()` in `Batchable.finish()`.
- Detect `Database.executeBatch()` in `Batchable.execute()`.
- Flag unbounded batch chains (no termination condition in source).
- Flag batches that enqueue Queueable jobs from `execute()` (transaction scope risk).

### Phase 4c: Future/schedule detection

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

## Phase 9: Package-Version-Aware Rules

**Why now:** Managed packages have a strict version lifecycle (beta → released
→ patch). Once a version is released, `global` members cannot be removed or
have signatures changed. A scanner that understands this lifecycle can prevent
breaking changes before release.

### Phase 9a: Global API diff

- Accept a baseline JSON artifact from a prior scan (e.g., `glade inspect performance --json` from the previous release).
- Compare current `global` API surface to baseline.
- Flag:
  - **Removed global methods** → BREAKING (severity critical).
  - **Changed global method parameter types** → BREAKING.
  - **Changed global method return type** → BREAKING.
  - **Added mandatory parameters** → BREAKING (existing callers will fail).
  - **Added optional parameters** → OK.
  - **Added new global methods** → advisory (expands API surface).
- Produce a diff report: `glade inspect performance --diff-baseline v1.0.0-scan.json`.

### Phase 9b: Deprecation cycle enforcement

- Detect `@deprecated` annotations on global methods.
- Flag deprecated methods that still have callers inside the package.
- Flag deprecated methods in a version where they should be removed (after N major versions).

### Phase 9c: API version consistency

- Scan sfdx-project.json and metadata for API version (e.g., `sourceApiVersion: "64.0"`).
- Verify no Apex class overrides the package default API version.
- Flag mixed API versions within a single package (behavioral inconsistency risk).

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

3. **Phase 3 (Managed Package)**: Unique value proposition. Zero existing tools
   address managed package patterns. Directly useful for ISV partners and
   AppExchange listing preparation.

4. **Phase 4 (Async Chains)**: Structural analysis of async call graphs. PMD
   flags individual `enqueueJob` calls but cannot trace chains. High value for
   debugging transaction storms.

5. **Phase 5 (Trigger Deepening)**: Extends existing trigger detection with
   re-entry and handler audit. Complements PMD's `AvoidLogicInTrigger` and
   `AvoidDirectAccessTriggerMap`.

6. **Phase 6 (Platform Cache)**: Niche but high-value for ISV packages that
   use cache heavily. No tooling exists.

7. **Phase 7 (LWC/Aura)**: Expands scanner scope beyond Apex. Competitive
   differentiator (PMD/SonarQube are Apex-only).

8. **Phase 8 (SARIF)**: CI/CD integration. Required for adoption in existing
   pipelines alongside Code Analyzer and PMD.

9. **Phase 9 (Version-aware)**: Sophisticated analysis for ISV release
   management. Depends on Phase 3.

10. **Phase 10 (Profiling)**: Bridges static analysis and runtime measurement.
    Complements the base scanner's trace ingestion (Task 7).

11. **Phase 11 (Limit estimation)**: Hardest problem, highest value. Requires
    Phases 1-5 as foundation.

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
