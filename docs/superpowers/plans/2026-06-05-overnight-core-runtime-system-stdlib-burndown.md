# Overnight Core Runtime System Stdlib Burndown Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run an 8+ hour unattended push that materially reduces `Core.Runtime.SystemAndStdlib` top missing-shape rows without touching HTTP/REST, ConnectApi breadth, or corpus-specific runtime code.

**Architecture:** Start from a fresh surface ledger, split the work into parallel subagent squads, and integrate only after each squad reports exact rows, files changed, fixtures, and validation. Prefer ledger identity fixes for false rows, exact stdlib shape/evidence for already-modeled behavior, and explicit unsupported diagnostics for Salesforce services that cannot be modeled locally yet.

**Tech Stack:** Go 1.26, `go test`, `go run ./cmd/glade compat surface refresh`, `internal/surfaceledger`, `internal/typesys`, `internal/vm`, `internal/capability`, `docs/fixtures`.

---

## Copy-Paste Goal

Use this for `/goal`:

```text
Run docs/superpowers/plans/2026-06-05-overnight-core-runtime-system-stdlib-burndown.md to completion. Use parallel subagent squads. Do not run full large example-project local-test gates. Keep performance and Salesforce behavior front and center. Objective: move the Core.Runtime.SystemAndStdlib top missing-shape frontier by closing ledger false rows, exact result/exception/stdlib shape rows, DMLOptions shape/runtime basics, and explicit unsupported rows for Approval/BusinessHours or other service surfaces that cannot be modeled locally. Finish only after focused tests, generated doc checks, repo guard, and a fresh surface refresh report before/after counts and no surface failures.
```

## Starting State

Reviewed refresh from the current worktree:

```text
implemented=55122 partial=31 passive=46986 explicitUnsupported=692
gaps: missingShape=12239 missingBehavior=0 missingEvidence=7635
failures: parser=0 docsOrgMismatch=0 staleGlade=0 passiveServiceRisk=0
```

Current top `Core.Runtime.SystemAndStdlib` rows include:

```text
apex:System.Answers.findSimilar(Question)
apex:System.Apex
apex:System.Apex.Delete
apex:System.Apex.Insert
apex:System.Apex.Merge
apex:System.Apex.Undelete
apex:System.Apex.Update
apex:System.Apex.Upsert
apex:System.Approval.process(Approval.ProcessRequest)
apex:System.Approval.process(Approval.ProcessRequest,Boolean)
apex:System.BusinessHours.add(String,Datetime,Long)
apex:System.BusinessHours.addGmt(String,Datetime,Long)
apex:System.BusinessHours.nextStartDate(String,Datetime)
apex:System.Comparator.compare(T,T)
apex:System.Custom.*
apex:System.DMLOptions.*
apex:System.DeleteResult.*
apex:System.EmptyRecycleBinResult.*
apex:System.Error.*
apex:System.Exception.Exception(...)
```

The overnight run is successful when these named families no longer appear as unreviewed top missing-shape rows. They may become `implemented`, `partial`, or `explicitUnsupported`, but not silent wrong behavior.

## Hard Rules

- Do not touch `ConnectApi` except to confirm it stays out of this goal.
- Do not work on HTTP, REST, Tooling server, GraphQL, Pub/Sub, Metadata API, SOAP, Bulk, or external product surfaces.
- Do not add corpus-specific class names, package names, object names, or exception names to runtime code or tests.
- Do not fake org-backed behavior. If a Salesforce surface requires metadata or services not loaded locally, add exact shape plus stable `UnsupportedFeature` behavior.
- Performance is a first-class gate. Avoid per-record scans, repeated metadata merges, broad string allocation in hot paths, and shared mutable caches across test isolation.
- Use `strings.EqualFold` for case-insensitive equality checks. Avoid `strings.ToLower` in hot paths.
- Parallel tests may share compiled code and immutable schema-derived caches only.

## File Map

Likely files by responsibility:

- `internal/surfaceledger/docs_snapshot.go`: docs-row parsing and false-row suppression.
- `internal/surfaceledger/ids.go`: namespace and parameter identity normalization.
- `internal/surfaceledger/*_test.go`: ledger identity, docs parsing, and classification tests.
- `internal/typesys/standard_symbols.go`: exact Apex stdlib shape.
- `internal/typesys/standard_symbols_test.go`: shape assertions for new rows.
- `internal/vm/dispatch.go`: static runtime dispatch and unsupported diagnostics.
- `internal/vm/dispatch_static.go`: canonical static call routing.
- `internal/vm/platform_test.go`: focused VM behavior and unsupported diagnostics.
- `internal/vm/database_member_runtime.go`: existing Database result accessor behavior.
- `internal/dml/**`: only if `DMLOptions` behavior must change.
- `internal/capability/stdlib.go`: exact stdlib capability rows.
- `docs/fixtures/core-runtime-*.json`: exact behavior evidence.
- `docs/STDLIB_COVERAGE.md`, `docs/COMPATIBILITY_DASHBOARD.md`, `docs/KNOWN_GAPS.md`: generated docs after capability changes.
- `docs/plans/2026-06-04-salesforce-agent-surface-todo.md`: final closeout note.
- `docs/plans/2026-06-04-salesforce-vertical-priority-overlay.md`: final closeout note.

## Parallel Squad Rules

Use parallel subagent squads in isolated worktrees if the environment supports them. If not, run the subagents as research-and-patch drafts and integrate serially in this workspace.

No squad should commit. The lead agent owns integration, validation, and final status.

Shared-file rule:

- Squads may propose changes to `internal/typesys/standard_symbols.go`, `internal/capability/stdlib.go`, and generated docs.
- Only the lead agent edits those shared files in the main worktree after all squad summaries return.
- Squads may edit isolated test files and new fixture files in their worktrees.

Required squad return format:

```text
Squad:
Rows reviewed:
Rows closed:
Files changed:
Tests added:
Commands run:
Performance risk:
Salesforce behavior risk:
Remaining rows:
Patch summary:
```

## Phase 0: Baseline And Packet

- [ ] **Step 0.1: Capture current status**

Run:

```bash
git status --short
git diff --stat
```

Expected: dirty worktree is allowed. Do not revert unrelated changes.

- [ ] **Step 0.2: Build a fresh surface baseline**

Run:

```bash
tmp="$(mktemp -d /tmp/glade-overnight-system.XXXXXX)"
go run ./cmd/glade compat surface refresh \
  --docs "example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run" \
  --tooling-completions testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp"
go run ./cmd/glade compat surface packet \
  --ledger "$tmp/SURFACE_LEDGER.json" \
  --area Core.Runtime.SystemAndStdlib \
  --out "$tmp/Core.Runtime.SystemAndStdlib.md"
sed -n '50,130p' "$tmp/Core.Runtime.SystemAndStdlib.md"
```

Expected: refresh succeeds with no parser, stale Glade, docs/org mismatch, or passive service failures.

- [ ] **Step 0.3: Save the before counts**

Run:

```bash
jq -r '.summary' "$tmp/SURFACE_LEDGER.json"
```

Expected: record `implemented`, `missingShape`, `missingEvidence`, and `failures` in the final closeout.

## Phase 1: Dispatch Parallel Subagent Squads

- [ ] **Step 1.1: Dispatch Squad A: ledger false rows and namespace cleanup**

Prompt:

```text
You are Squad A for docs/superpowers/plans/2026-06-05-overnight-core-runtime-system-stdlib-burndown.md.

Goal: reduce false Core.Runtime.SystemAndStdlib missing-shape rows caused by docs parsing or namespace identity, without touching VM behavior.

Start from the fresh Core.Runtime.SystemAndStdlib packet. Investigate these rows first:
- apex:System.Apex
- apex:System.Apex.Delete
- apex:System.Apex.Insert
- apex:System.Apex.Merge
- apex:System.Apex.Undelete
- apex:System.Apex.Update
- apex:System.Apex.Upsert
- apex:System.Appendices
- apex:System.Custom.*
- apex:System.Documentation

Do:
1. Inspect the docs source rows for each surface.
2. Decide whether each row is a real Apex surface, docs navigation noise, a namespace inference bug, or a real unsupported runtime surface.
3. Add focused tests in internal/surfaceledger for any parser or identity fix.
4. If fixing code, keep changes to internal/surfaceledger only.
5. Do not edit VM, DML, storage, server, or generated docs.

Validate:
go test ./internal/surfaceledger -count=1

Return the required squad summary with exact row IDs and files.
```

- [ ] **Step 1.2: Dispatch Squad B: result DTO, Error, and Exception shape**

Prompt:

```text
You are Squad B for docs/superpowers/plans/2026-06-05-overnight-core-runtime-system-stdlib-burndown.md.

Goal: close exact shape/evidence rows for result DTOs and exception/error surfaces already modeled locally.

Investigate these rows first:
- apex:System.DeleteResult
- apex:System.DeleteResult.getErrors()
- apex:System.DeleteResult.getId()
- apex:System.DeleteResult.isSuccess()
- apex:System.EmptyRecycleBinResult
- apex:System.EmptyRecycleBinResult.getErrors()
- apex:System.EmptyRecycleBinResult.getId()
- apex:System.EmptyRecycleBinResult.isSuccess()
- apex:System.Error
- apex:System.Error.getFields()
- apex:System.Error.getMessage()
- apex:System.Error.getStatusCode()
- apex:System.Exception.Exception(Exception)
- apex:System.Exception.Exception(String)
- apex:System.Exception.Exception(String,Exception)

Do:
1. Inspect existing Database result fixtures, VM result accessors, and generated ledger rows.
2. Determine whether these docs rows should normalize to Database.* or need System aliases.
3. Add exact type shape and fixture evidence only where local runtime behavior already exists.
4. Add stable unsupported diagnostics only if a row is real but not locally modeled.
5. Do not invent behavior.

Likely files:
- internal/surfaceledger/ids.go
- internal/typesys/standard_symbols.go
- internal/typesys/standard_symbols_test.go
- docs/fixtures/core-runtime-result-error-exception-evidence.json
- internal/capability/stdlib.go

Validate:
go test ./internal/typesys ./internal/surfaceledger ./internal/compat -run 'TestRunDocumentedFixtures|TestCanonical|TestStandard' -count=1

Return the required squad summary with exact row IDs and files.
```

- [ ] **Step 1.3: Dispatch Squad C: DMLOptions shape and local behavior**

Prompt:

```text
You are Squad C for docs/superpowers/plans/2026-06-05-overnight-core-runtime-system-stdlib-burndown.md.

Goal: close DMLOptions shape and the narrow locally modelable behavior used by Database DML options.

Rows:
- apex:System.DMLOptions
- apex:System.DMLOptions.allowFieldTruncation
- apex:System.DMLOptions.assignmentRuleHeader
- apex:System.DMLOptions.emailHeader
- apex:System.DMLOptions.localeOptions
- apex:System.DMLOptions.optAllOrNone

Do:
1. Inspect public docs in the local docs scraper output for DMLOptions.
2. Inspect existing Database.insert/update overloads that accept Database.DMLOptions.
3. Add type shape for DMLOptions and nested header objects only as needed.
4. Implement only safe local behavior:
   - optAllOrNone influences local all-or-none DML behavior if a Database method accepts options.
   - allowFieldTruncation, assignmentRuleHeader, emailHeader, and localeOptions can be stored as properties without side effects unless existing local DML already supports them.
5. If a side effect requires org assignment rules, email delivery, or locale-specific validation not modeled locally, keep it as stored shape or explicit unsupported. Do not fake side effects.

Likely files:
- internal/typesys/standard_symbols.go
- internal/vm/construct_runtime.go
- internal/vm/dispatch.go
- internal/vm/database_member_runtime.go
- internal/dml/dml.go
- internal/vm/platform_test.go
- docs/fixtures/core-runtime-dml-options.json
- internal/capability/stdlib.go

Validate:
go test ./internal/vm ./internal/dml ./internal/compat -run 'DMLOptions|TestRunDocumentedFixtures' -count=1

Return the required squad summary with exact row IDs and files.
```

- [ ] **Step 1.4: Dispatch Squad D: Approval and BusinessHours service boundary**

Prompt:

```text
You are Squad D for docs/superpowers/plans/2026-06-05-overnight-core-runtime-system-stdlib-burndown.md.

Goal: close Approval.process and BusinessHours top rows with Salesforce-shaped local decisions.

Rows:
- apex:System.Approval.process(Approval.ProcessRequest)
- apex:System.Approval.process(Approval.ProcessRequest,Boolean)
- apex:System.BusinessHours.add(String,Datetime,Long)
- apex:System.BusinessHours.addGmt(String,Datetime,Long)
- apex:System.BusinessHours.nextStartDate(String,Datetime)
- apex:System.Answers.findSimilar(Question)

Do:
1. Inspect public docs from the local docs source for signatures and behavior.
2. Inspect existing Approval.lock/Unlock unsupported fixture and runtime.
3. Decide whether any behavior can be modeled without org workflow/business-hours metadata.
4. For service behavior not modelable locally, add exact type shape plus stable UnsupportedFeature diagnostics.
5. For BusinessHours, do not assume 24x7 default hours unless public docs and existing metadata loader justify it. Prefer explicit unsupported over silent wrong datetime math.

Likely files:
- internal/typesys/standard_symbols.go
- internal/vm/dispatch.go
- internal/vm/dispatch_static.go
- internal/vm/platform_test.go
- docs/fixtures/core-runtime-approval-businesshours-unsupported.json
- internal/capability/stdlib.go

Validate:
go test ./internal/vm ./internal/compat -run 'Approval|BusinessHours|TestRunDocumentedFixtures' -count=1

Return the required squad summary with exact row IDs and files.
```

- [ ] **Step 1.5: Dispatch Squad E: performance and validation scout**

Prompt:

```text
You are Squad E for docs/superpowers/plans/2026-06-05-overnight-core-runtime-system-stdlib-burndown.md.

Goal: keep the overnight work fast and safe. This is a read-only scout unless the lead asks for a patch.

Do:
1. Inspect proposed touch points from Squads B/C/D for hot paths.
2. Identify any per-record scans, repeated metadata merges, string lowercasing, shared mutable caches, or test-isolation risks.
3. Recommend the cheapest focused tests and any benchmark that matters.
4. Do not edit files unless the lead asks.

Return:
- hot path risks
- isolation risks
- recommended focused test commands
- any benchmark command worth running
```

## Phase 2: Integrate Squad A First

- [ ] **Step 2.1: Review Squad A findings**

Accept only changes that prove docs parser noise or namespace identity with tests. Reject broad filters that hide real public Salesforce rows.

- [ ] **Step 2.2: Run Squad A tests**

Run:

```bash
go test ./internal/surfaceledger -count=1
```

Expected: pass.

- [ ] **Step 2.3: Refresh and compare top rows**

Run:

```bash
tmp_a="$(mktemp -d /tmp/glade-overnight-system-a.XXXXXX)"
go run ./cmd/glade compat surface refresh \
  --docs "example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run" \
  --tooling-completions testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp_a"
go run ./cmd/glade compat surface packet \
  --ledger "$tmp_a/SURFACE_LEDGER.json" \
  --area Core.Runtime.SystemAndStdlib \
  --out "$tmp_a/Core.Runtime.SystemAndStdlib.md"
sed -n '50,120p' "$tmp_a/Core.Runtime.SystemAndStdlib.md"
```

Expected: false rows such as `Apex.*`, `Appendices`, `Custom.*`, or `Documentation` are gone or explained. No surface failures.

## Phase 3: Integrate Result, Error, Exception Shape

- [ ] **Step 3.1: Add failing fixture or surfaceledger tests first**

If Squad B did not add tests, add them before code changes. Minimum expected coverage:

```text
surface rows for DeleteResult/EmptyRecycleBinResult/Error/Exception constructors are either normalized, implemented with fixture evidence, or explicitly unsupported with fixture evidence.
```

- [ ] **Step 3.2: Integrate exact shape/evidence**

Patch only the exact rows proven by existing local behavior. Keep Database result behavior in existing runtime paths.

- [ ] **Step 3.3: Validate**

Run:

```bash
go test ./internal/typesys ./internal/surfaceledger ./internal/compat -count=1
```

Expected: pass.

## Phase 4: Integrate DMLOptions

- [ ] **Step 4.1: Write focused tests**

Required test fixture shape:

```apex
@isTest private class CoreRuntimeDMLOptionsTest {
  @isTest static void optionsShapeAndAllOrNone() {
    Database.DMLOptions opts = new Database.DMLOptions();
    opts.optAllOrNone = false;
    opts.allowFieldTruncation = false;
    System.assertEquals(false, opts.optAllOrNone);
    System.assertEquals(false, opts.allowFieldTruncation);

    Account good = new Account(Name = 'Good');
    Account bad = new Account();
    List<Database.SaveResult> results = Database.insert(new List<Account>{good, bad}, opts);
    System.assertEquals(2, results.size());
    System.assert(results[0].isSuccess());
    System.assert(!results[1].isSuccess());
  }
}
```

If `Database.insert(List<SObject>, Database.DMLOptions)` is not locally supported, first add explicit unsupported evidence for that overload. Do not silently ignore options on a method that accepts them.

- [ ] **Step 4.2: Implement minimal behavior**

Minimum local behavior:

```text
DMLOptions is constructible.
Documented properties are readable/writable.
optAllOrNone maps to existing all-or-none behavior for supported Database DML options overloads.
Side-effect-only headers are stored but do not fire assignment rules or email.
```

- [ ] **Step 4.3: Validate**

Run:

```bash
go test ./internal/vm ./internal/dml ./internal/compat -run 'DMLOptions|TestRunDocumentedFixtures' -count=1
```

Expected: pass.

## Phase 5: Integrate Approval, BusinessHours, Answers

- [ ] **Step 5.1: Write unsupported tests before runtime code**

For each unsupported service method, fixture expectation must be exact:

```json
{
  "error": {
    "type": "UnsupportedFeature",
    "message": "unsupported call \"BusinessHours.add local business hours metadata\""
  }
}
```

Use exact messages chosen by implementation. Keep one fixture file per family if that is clearer.

- [ ] **Step 5.2: Add exact static shape**

Add method signatures so compile and ledger shape are exact:

```text
Approval.process(Approval.ProcessRequest)
Approval.process(Approval.ProcessRequest, Boolean)
BusinessHours.add(String, Datetime, Long)
BusinessHours.addGmt(String, Datetime, Long)
BusinessHours.nextStartDate(String, Datetime)
Answers.findSimilar(Question)
```

If docs show `Id` instead of `String` for BusinessHours ID, use public docs. Do not guess.

- [ ] **Step 5.3: Add stable unsupported dispatch**

Runtime must return `UnsupportedFeature`, not a generic method-not-found or type error.

- [ ] **Step 5.4: Validate**

Run:

```bash
go test ./internal/vm ./internal/compat -run 'Approval|BusinessHours|Answers|TestRunDocumentedFixtures' -count=1
```

Expected: pass.

## Phase 6: Capability Rows And Generated Docs

- [ ] **Step 6.1: Add capability rows**

Add exact entries to `internal/capability/stdlib.go` for every closed row. Use:

```text
supported: implemented and fixture-backed
partial: common local behavior works but documented gaps remain
unsupported: exact stable UnsupportedFeature diagnostic exists
```

- [ ] **Step 6.2: Regenerate docs**

Run:

```bash
go run ./cmd/glade compat dashboard --output docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade compat gaps --output docs/KNOWN_GAPS.md
go run ./cmd/glade compat stdlib --output docs/STDLIB_COVERAGE.md
```

Expected: no command output or only normal success output.

- [ ] **Step 6.3: Check generated docs**

Run:

```bash
go run ./cmd/glade compat dashboard --check docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade compat gaps --check docs/KNOWN_GAPS.md
go run ./cmd/glade compat stdlib --check docs/STDLIB_COVERAGE.md
```

Expected:

```text
docs/COMPATIBILITY_DASHBOARD.md: up to date
docs/KNOWN_GAPS.md: up to date
docs/STDLIB_COVERAGE.md: up to date
```

## Phase 7: Final Surface Ratchet

- [ ] **Step 7.1: Run focused package gate**

Run:

```bash
go test ./internal/typesys ./internal/sema ./internal/vm ./internal/surfaceledger ./internal/capability ./internal/compat ./internal/repoguard -count=1
```

Expected: pass.

- [ ] **Step 7.2: Run final surface refresh**

Run:

```bash
tmp_final="$(mktemp -d /tmp/glade-overnight-system-final.XXXXXX)"
go run ./cmd/glade compat surface refresh \
  --docs "example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run" \
  --tooling-completions testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp_final"
go run ./cmd/glade compat surface packet \
  --ledger "$tmp_final/SURFACE_LEDGER.json" \
  --area Core.Runtime.SystemAndStdlib \
  --out "$tmp_final/Core.Runtime.SystemAndStdlib.md"
jq -r '.summary' "$tmp_final/SURFACE_LEDGER.json"
sed -n '50,130p' "$tmp_final/Core.Runtime.SystemAndStdlib.md"
```

Expected:

```text
failures: parser=0 docsOrgMismatch=0 staleGlade=0 passiveServiceRisk=0
```

The target families should be absent from the top unreviewed missing-shape list or explicitly documented as deferred with a reason.

- [ ] **Step 7.3: Update plan/todo closeout**

Update:

```text
docs/plans/2026-06-04-salesforce-agent-surface-todo.md
docs/plans/2026-06-04-salesforce-vertical-priority-overlay.md
```

Required closeout text:

```text
Reviewed refresh:
implemented=<n> partial=<n> passive=<n> explicitUnsupported=<n>
gaps: missingShape=<n> missingBehavior=<n> missingEvidence=<n>
failures: parser=0 docsOrgMismatch=0 staleGlade=0 passiveServiceRisk=0

Rows closed:
- <exact row>

Rows deferred:
- <exact row>: <reason>

Next top row:
- <exact row>
```

- [ ] **Step 7.4: Final diff checks**

Run:

```bash
git diff --check
git status --short
git diff --stat
```

Expected: no whitespace errors. Dirty files are expected; report them.

## Stop Rules

Stop and write a blocked note if any condition repeats after two focused attempts:

- A public-doc behavior cannot be determined from local docs or public Salesforce docs.
- A locally modeled behavior would require workflow, approval engine, business-hours metadata, email delivery, or org services not loaded by Glade.
- A test starts requiring full example-project gates to prove a narrow stdlib row.
- A performance change adds measurable slowdown to `go test ./internal/vm` or `go test ./internal/compat`.
- Two squads need incompatible edits to the same hot runtime path.

Do not stay up chasing full parity. Make the job fit the night.

## Final Report Template

```text
Goal: Core.Runtime.SystemAndStdlib overnight burndown
Baseline:
Final:
Rows closed:
Rows moved to explicit unsupported:
Rows deferred:
Subagent squads used:
Files changed:
Validation:
Performance notes:
Next best goal:
```
