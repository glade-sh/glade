# Docs-First Salesforce Surface Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Salesforce docs scrape the contract source for Glade platform shape, type precision, sema behavior, and corpus proof so missing implementations are found by gates before users hit them.

**Architecture:** Keep maintenance scanners and corpus runners in sibling `glade-tools`. Keep product behavior in `glade`: generated platform symbols, semantic analysis, metadata indexing, SOQL typing, and runtime defaults. The gate is four layers: docs contract, generated Glade shape, semantic behavior fixture, and public corpus classification.

**Tech Stack:** Go, Node.js generator scripts, local Salesforce docs at `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run`, sibling `/Users/matt/Dev/glade-tools`, `glade check`, Surface Ledger JSON, public corpus under `/Users/matt/Dev/glade-corpus/public`.

---

## Current Evidence

Fresh binary:

```bash
cd /Users/matt/Dev/glade
go build -o /tmp/glade-public-corpus-check ./cmd/glade
```

Public corpus sweep:

```bash
/tmp/glade-public-corpus-check check --project <project> --format json --no-progress
```

Sweep output:

```text
/tmp/glade-public-corpus-check-20260622062209
```

Corpus totals:

```text
projects: 102
projects with nonzero exit: 39
projects with diagnostics: 42
diagnostics: 3137
```

Diagnostic buckets:

```text
duplicate-indexing: 1529
sema-core: 1427
soql-schema: 99
performance-advisory: 63
parse: 19
```

Top diagnostic codes:

```text
1529 GLADETYPE001
443  GLADESEMA006
193  GLADESEMA023
150  GLADESEMA018
149  GLADESEMA008
114  GLADESEMA009
68   GLADESEMA004
66   GLADESEMA011
63   GLADESEMA021
51   GLADESEMA_QUERY_FIELD
40   GLADESEMA_QUERY_RELATIONSHIP
```

Surface refresh:

```bash
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools surface refresh \
  --docs "/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run" \
  --tooling-completions /Users/matt/Dev/glade/testdata/generated/tooling_system_symbols.json.gz \
  --out /tmp/glade-surface-refresh-20260622
```

Surface refresh result:

```text
implemented=130284
passive=47492
stubNoOp=262
explicitUnsupported=6347
missingShape=5818
failures=0
```

The important split:

```text
5752 missing-shape rows are unknown docs pages.
66 missing-shape rows are apex rows.
41 of the apex rows are commercepayments.
25 of the apex rows are ConnectApi.
```

Stub inventory:

```bash
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools stub-inventory \
  --source /Users/matt/Dev/glade/example-projects/stubs \
  --output /tmp/glade-stub-inventory-20260622.json
```

Stub inventory result:

```text
system stub classes: 7095
system stub properties: 27022
platform generated properties: 27327
```

Type-loss proof:

```bash
rg -n '^\s*(global|public)\s+\w+\s*\{ get; set; \}' \
  /Users/matt/Dev/glade/example-projects/stubs/apex-system-stubs -g '*.cls' | wc -l
```

Result:

```text
27008
```

Concrete miss:

```text
Docs: apex/apex_connectapi_output_managed_content_version_collection.md
  items: List<ConnectApi.ManagedContentVersion>

Stub: example-projects/stubs/apex-system-stubs/ConnectApi/ManagedContentVersionCollection.cls
  global  items { get; set; }

Generated: internal/typesys/system_stub_symbols_generated.go
  {Name: "items", Type: "Object"}
```

The old screen marked this row as known because the property name existed. It did not compare the type. That is the main process failure.

---

## Work Boundaries

- Do not edit `docs/superpowers/plans/2026-06-22-salesforce-sema-coverage.md`; another agent owns it.
- Do not add corpus runners or docs scanners to base `glade`.
- Do not add project-specific exceptions to product code.
- Do not count `GLADEPERF*` as implementation misses.
- Do not count parse errors as Salesforce surface misses until a focused parser repro proves valid Apex.
- Keep generated reports under `/tmp` or `glade-tools/docs/generated` unless a checked product artifact already owns that data.

## Execution Approach

Use parallel subagent squads, but do not let every squad swing at the same log. The work has one trunk and several branches.

The trunk:

```text
docs contract extraction -> source-specific ledger comparison -> generated Glade symbols -> sema fixtures -> corpus proof
```

Run one lead integrator in `/Users/matt/Dev/glade`. Give each squad its own worktree for any code edits. Keep paired `glade` and `glade-tools` worktrees as siblings so existing `go.mod replace` paths still resolve.

Recommended worktree layout:

```text
/Users/matt/Dev/.worktrees/docs-contract/glade
/Users/matt/Dev/.worktrees/docs-contract/glade-tools
/Users/matt/Dev/.worktrees/surface-ledger/glade
/Users/matt/Dev/.worktrees/surface-ledger/glade-tools
/Users/matt/Dev/.worktrees/symbol-generation/glade
/Users/matt/Dev/.worktrees/symbol-generation/glade-tools
/Users/matt/Dev/.worktrees/sema-contracts/glade
/Users/matt/Dev/.worktrees/sema-contracts/glade-tools
/Users/matt/Dev/.worktrees/corpus-proof/glade
/Users/matt/Dev/.worktrees/corpus-proof/glade-tools
```

Each squad returns a short packet:

```text
branch/worktree:
files changed:
tests run:
new gates added:
remaining rows or diagnostics:
risks:
```

The lead integrator reviews each packet, runs the squad's focused tests, then merges into the integration branch. Run the full proof gate only after all phase gates pass.

Do not merge code that only lowers a corpus count without adding a docs, ledger, generator, sema, or classifier gate. A shaved count without a gate does not hold.

## Phase Plan

### Phase 0: Baseline and Ownership

**Goal:** Freeze the numbers and split the work without overlap.

**Owner:** Lead integrator.

**Inputs:**

```text
/tmp/glade-public-corpus-check-20260622062209
/tmp/glade-surface-refresh-20260622
/tmp/glade-stub-inventory-20260622.json
```

**Steps:**

- [ ] Create an integration branch from `/Users/matt/Dev/glade` `main`.
- [ ] Create sibling `glade` and `glade-tools` worktrees for each squad.
- [ ] Copy or regenerate the corpus, surface, and stub baseline reports.
- [ ] Publish the baseline packet in the plan or a `/tmp` handoff note.
- [ ] Assign file ownership from the squad matrix below.

**Exit gate:**

```bash
cd /Users/matt/Dev/glade
go build -o /tmp/glade-public-corpus-check ./cmd/glade

cd /Users/matt/Dev/glade-tools
go test ./internal/apexdocs ./internal/surfaceledger ./internal/capability ./internal/toolcli -count=1
```

Expected: build succeeds; existing tool tests pass or known failures are recorded before squads start.

### Phase 1: Contract and Ledger Foundation

**Goal:** Make the docs scrape produce typed contracts and make Surface Ledger fail when Glade disagrees.

**Parallel squads:**

```text
Squad A: Docs Contract Extraction
  Owns Task 1.
  Edits only /Users/matt/Dev/glade-tools/internal/apexdocs.

Squad B: Surface Ledger Type Comparison
  Owns Task 2 after Squad A publishes the contract shape.
  Edits only /Users/matt/Dev/glade-tools/internal/surfaceledger.

Squad C: Surface Check CLI Gate
  Owns Task 3 after Squad B publishes gap names.
  Edits only /Users/matt/Dev/glade-tools/internal/toolcli and report wiring.
```

Squad B can start by adding source-specific fields and tests with hand-built rows. Squad C can start by adding CLI test scaffolds with synthetic reports. They must rebase once Squad A lands the real contract fields.

**Exit gate:**

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/apexdocs ./internal/surfaceledger ./internal/toolcli -count=1
go run ./cmd/glade-tools surface refresh \
  --docs "/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run" \
  --tooling-completions /Users/matt/Dev/glade/testdata/generated/tooling_system_symbols.json.gz \
  --out /tmp/glade-surface-refresh-typed
go run ./cmd/glade-tools surface check \
  --input /tmp/glade-surface-refresh-typed/surface-ledger.json
```

Expected: `surface check` fails on current return-type or parameter mismatches before generator work lands. That failure is desired. It proves the gate can see the gap.

### Phase 2: Shape Generation

**Goal:** Make Glade generated symbols consume docs contracts instead of falling back to `Object`.

**Parallel squads:**

```text
Squad D: Stub Symbol Generation
  Owns Task 4.
  Edits scripts/generate-system-stub-symbols.mjs, generated system symbols, typesys tests, and repo guard.

Squad E: Product Namespace Generation
  Owns Task 5.
  Edits glade-tools capability generation and generated product namespace symbols.

Squad F: Missing Apex Shape Families
  Owns Task 10.
  Starts with commercepayments and ConnectApi rows from the typed ledger.
```

Squad D and Squad E both need the Phase 1 docs contract artifact. They must not invent separate JSON shapes. One contract file feeds both generators.

**Exit gate:**

```bash
cd /Users/matt/Dev/glade
node scripts/generate-system-stub-symbols.mjs \
  example-projects/stubs/apex-system-stubs \
  internal/typesys/system_stub_symbols_generated.go \
  testdata/generated/apex_docs_contracts.json
go test ./internal/typesys ./internal/repoguard -count=1

cd /Users/matt/Dev/glade-tools
go test ./internal/capability -run ProductNamespace -count=1
```

Expected:

```text
ConnectApi.ManagedContentVersionCollection.items == List<ConnectApi.ManagedContentVersion>
ConnectApi.ManagedContentVersionCollection.managedContentTypes == Map<String,ConnectApi.ManagedContentType>
```

### Phase 3: Behavior Contracts

**Goal:** Turn docs-backed shape into semantic behavior, not just generated names.

**Parallel squads:**

```text
Squad G: Core Sema Contracts
  Owns Task 6 for System.Object, collection constructors, generic returns, Type, Schema, Batchable, Iterator.

Squad H: SOQL and Metadata Classification
  Owns Task 7.
  Keeps SOQL schema misses separate from Apex surface misses.

Squad I: Duplicate Indexing Gate
  Owns Task 9.
  Fixes or gates duplicate project discovery before corpus numbers are used as semantic proof.
```

Squad G works in product sema. Squad H works in metadata and classifier boundaries. Squad I works in project discovery and indexing. These lanes can run side by side after Phase 2 symbols compile.

**Exit gate:**

```bash
cd /Users/matt/Dev/glade
go test ./internal/sema ./internal/typesys ./internal/metadata ./internal/repoguard -count=1
```

Expected: docs-backed fixtures pass; duplicate indexing no longer dominates corpus diagnostics.

### Phase 4: Corpus Proof and Triage Automation

**Goal:** Make public corpus output a classified regression gate, not a pile of JSON.

**Parallel squads:**

```text
Squad J: Corpus Classifier
  Owns Task 8.
  Lives in glade-tools. Produces stable buckets and markdown/JSON summaries.

Squad K: Corpus Reduction Review
  Consumes the classifier output after Squads G, H, and I land.
  Opens targeted follow-up tasks only for buckets backed by docs contracts.
```

Squad J can start as soon as Phase 0 baseline reports exist. It does not need Phase 2 or Phase 3 fixes. Squad K waits until the classifier can compare before and after.

**Exit gate:**

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/corpus ./internal/toolcli -run Corpus -count=1
go run ./cmd/glade-tools corpus check \
  --glade /tmp/glade-public-corpus-check \
  --root /Users/matt/Dev/glade-corpus/public \
  --out /tmp/glade-corpus-proof-final
```

Expected: report lists implementation misses, metadata misses, parse misses, perf advisories, and duplicate-indexing separately. It must preserve raw diagnostic counts.

### Phase 5: Final Integration Gate

**Goal:** Prove the new machinery catches the old miss and gives a clean, actionable corpus view.

**Owner:** Lead integrator.

**Steps:**

- [ ] Rebase all squad branches onto the integration branch.
- [ ] Resolve generated-file conflicts by regenerating from the final docs contract.
- [ ] Run focused tests from each squad packet.
- [ ] Run the full proof gate in Task 11.
- [ ] Compare `/tmp/glade-corpus-proof-final` against `/tmp/glade-public-corpus-check-20260622062209`.
- [ ] Record remaining nonzero buckets with owner labels.

**Exit gate:**

```bash
cd /Users/matt/Dev/glade
go test ./...
scripts/smoke.sh

cd /Users/matt/Dev/glade-tools
go test ./...
go run ./cmd/glade-tools surface refresh \
  --docs "/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run" \
  --tooling-completions /Users/matt/Dev/glade/testdata/generated/tooling_system_symbols.json.gz \
  --out /tmp/glade-surface-refresh-final
go run ./cmd/glade-tools surface check \
  --input /tmp/glade-surface-refresh-final/surface-ledger.json
go run ./cmd/glade-tools corpus check \
  --glade /tmp/glade-public-corpus-check \
  --root /Users/matt/Dev/glade-corpus/public \
  --out /tmp/glade-corpus-proof-final
```

Expected: surface type mismatches are zero or listed as explicit unsupported gaps with evidence; corpus output is classified by root cause and no longer uses duplicate indexing as a stand-in for implementation state.

## Squad Prompt Templates

Use these prompts when dispatching subagents. Fill in only the branch name and current baseline paths.

### Squad A Prompt: Docs Contract Extraction

```text
You own docs contract extraction for Glade Salesforce surface closure.

Worktree:
  glade-tools: <absolute glade-tools worktree>

Read:
  /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-22-docs-first-salesforce-surface-closure.md
  Task 1 and Phase 1 only.

Goal:
  Parse typed Apex docs contracts from the expanded Salesforce docs scrape.

Scope:
  Modify only /Users/matt/Dev/glade-tools/internal/apexdocs.

Required proof:
  go test ./internal/apexdocs -count=1

Return:
  branch/worktree, files changed, tests run, sample extracted contracts, risks.
```

### Squad B Prompt: Surface Ledger Type Comparison

```text
You own source-specific type comparison in Surface Ledger.

Worktree:
  glade-tools: <absolute glade-tools worktree>

Read:
  /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-22-docs-first-salesforce-surface-closure.md
  Task 2 and Phase 1 only.

Goal:
  Preserve docs, org, and Glade return types separately and classify mismatches.

Scope:
  Modify only /Users/matt/Dev/glade-tools/internal/surfaceledger.

Required proof:
  go test ./internal/surfaceledger -count=1

Return:
  branch/worktree, files changed, tests run, mismatch examples, risks.
```

### Squad C Prompt: Surface Check CLI Gate

```text
You own the Surface Check CLI gate.

Worktree:
  glade-tools: <absolute glade-tools worktree>

Read:
  /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-22-docs-first-salesforce-surface-closure.md
  Task 3 and Phase 1 only.

Goal:
  Make surface check fail on return-type and parameter mismatches with explicit ceilings.

Scope:
  Modify only /Users/matt/Dev/glade-tools/internal/toolcli and surface report wiring.

Required proof:
  go test ./internal/toolcli ./internal/surfaceledger -count=1

Return:
  branch/worktree, files changed, tests run, CLI output examples, risks.
```

### Squad D Prompt: Stub Symbol Generation

```text
You own docs-backed system stub symbol generation.

Worktree:
  glade: <absolute glade worktree>
  glade-tools: <absolute glade-tools worktree>

Read:
  /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-22-docs-first-salesforce-surface-closure.md
  Task 4 and Phase 2 only.

Goal:
  Make untyped stub properties use docs contract types before falling back to Object.

Scope:
  Modify only generator, generated system symbols, typesys tests, repo guard, and the docs contract JSON.

Required proof:
  go test ./internal/typesys ./internal/repoguard -count=1

Return:
  branch/worktree, files changed, tests run, ConnectApi property proof, risks.
```

### Squad E Prompt: Product Namespace Generation

```text
You own docs-backed product namespace generation.

Worktree:
  glade: <absolute glade worktree>
  glade-tools: <absolute glade-tools worktree>

Read:
  /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-22-docs-first-salesforce-surface-closure.md
  Task 5 and Phase 2 only.

Goal:
  Stop product namespace generation from defaulting docs-known members to Object.

Scope:
  Modify only capability generation, capability tests, and generated product namespace symbols.

Required proof:
  go test ./internal/capability -run ProductNamespace -count=1
  go test ./internal/typesys -count=1

Return:
  branch/worktree, files changed, tests run, docs/tooling precedence examples, risks.
```

### Squad F Prompt: Missing Apex Shape Families

```text
You own the 66 Apex missing-shape rows.

Worktree:
  glade: <absolute glade worktree>
  glade-tools: <absolute glade-tools worktree>

Read:
  /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-22-docs-first-salesforce-surface-closure.md
  Task 10 and Phase 2 only.

Goal:
  Close commercepayments and ConnectApi missing-shape families by generated shape or explicit unsupported evidence.

Scope:
  Modify only docs contract generation, symbol generation, or explicit unsupported ledgers needed for those rows.

Required proof:
  surface refresh output with apex missing-shape rows reduced or justified.

Return:
  branch/worktree, files changed, tests run, before/after row counts, risks.
```

### Squad G Prompt: Core Sema Contracts

```text
You own docs-backed semantic contracts.

Worktree:
  glade: <absolute glade worktree>

Read:
  /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-22-docs-first-salesforce-surface-closure.md
  Task 6 and Phase 3 only.

Goal:
  Add sema fixtures and product fixes for docs-backed stdlib behavior.

Scope:
  Modify only internal/sema, internal/typesys, and internal/metadata files listed in Task 6.

Required proof:
  go test ./internal/sema ./internal/typesys ./internal/metadata -count=1

Return:
  branch/worktree, files changed, tests run, fixture list, risks.
```

### Squad H Prompt: SOQL and Metadata Classification

```text
You own SOQL and metadata classification.

Worktree:
  glade: <absolute glade worktree>
  glade-tools: <absolute glade-tools worktree>

Read:
  /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-22-docs-first-salesforce-surface-closure.md
  Task 7 and Phase 3 only.

Goal:
  Keep SOQL schema and metadata misses separate from Apex surface misses.

Scope:
  Modify only metadata behavior and corpus classification needed for Task 7.

Required proof:
  focused SOQL/metadata tests plus classifier output showing separate bucket names.

Return:
  branch/worktree, files changed, tests run, bucket examples, risks.
```

### Squad I Prompt: Duplicate Indexing Gate

```text
You own duplicate indexing.

Worktree:
  glade: <absolute glade worktree>

Read:
  /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-22-docs-first-salesforce-surface-closure.md
  Task 9 and Phase 3 only.

Goal:
  Fix or gate duplicate project discovery so GLADETYPE001 stops hiding implementation signal.

Scope:
  Modify only project discovery, indexing, and repo guard tests needed for Task 9.

Required proof:
  focused indexing tests and corpus classifier before/after count for GLADETYPE001.

Return:
  branch/worktree, files changed, tests run, before/after duplicate count, risks.
```

### Squad J Prompt: Corpus Classifier

```text
You own the public corpus classifier.

Worktree:
  glade-tools: <absolute glade-tools worktree>

Read:
  /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-22-docs-first-salesforce-surface-closure.md
  Task 8 and Phase 4 only.

Goal:
  Add glade-tools corpus check with stable root-cause buckets and raw diagnostic preservation.

Scope:
  Modify only glade-tools corpus/toolcli files.

Required proof:
  go test ./internal/corpus ./internal/toolcli -run Corpus -count=1
  corpus check output over /Users/matt/Dev/glade-corpus/public

Return:
  branch/worktree, files changed, tests run, output paths, risks.
```

### Squad K Prompt: Corpus Reduction Review

```text
You own the final corpus reduction review.

Worktree:
  glade: <absolute glade worktree>
  glade-tools: <absolute glade-tools worktree>

Read:
  /Users/matt/Dev/glade/docs/superpowers/plans/2026-06-22-docs-first-salesforce-surface-closure.md
  Phase 4 and Phase 5 only.

Goal:
  Compare baseline corpus output against final classifier output and identify only docs-backed implementation misses.

Scope:
  Do not edit code unless the lead integrator assigns a focused follow-up.

Required proof:
  before/after markdown summary with bucket counts and owner labels.

Return:
  report path, count deltas, remaining implementation buckets, recommended next tasks.
```

---

## Implementation Tasks

### Task 1: Add Docs Contract Extraction in `glade-tools`

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/apexdocs/inventory.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/apexdocs/inventory_test.go`
- Create: `/Users/matt/Dev/glade-tools/internal/apexdocs/type_normalize.go`
- Create: `/Users/matt/Dev/glade-tools/internal/apexdocs/type_normalize_test.go`

- [ ] Add fields to `apexdocs.Member`:

```go
ReturnType string `json:"returnType,omitempty"`
PropertyType string `json:"propertyType,omitempty"`
Parameters []string `json:"parameters,omitempty"`
```

- [ ] Parse property tables with headers matching `Property Name`, `Type`, and `Available Version`.
- [ ] Parse `#### Signature` blocks into return type, member name, and parameter types.
- [ ] Normalize Salesforce markdown types:

```text
[String](...) -> String
[`ConnectApi.ManagedContentVersion`](...) -> ConnectApi.ManagedContentVersion
List<[String](...)> -> List<String>
Map<[String](...), [`ConnectApi.ManagedContentType`](...)> -> Map<String,ConnectApi.ManagedContentType>
sObject[] -> List<SObject>
```

- [ ] Strip zero-width spaces from docs names and types before comparison.
- [ ] Add tests using these docs snippets:

```text
ConnectApi.ManagedContentVersionCollection.items -> List<ConnectApi.ManagedContentVersion>
ConnectApi.ManagedContentVersionCollection.managedContentTypes -> Map<String,ConnectApi.ManagedContentType>
System.Object.equals(Object) -> Boolean
List<T>(Set<T>) constructor -> parameter Set<T>
```

- [ ] Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/apexdocs -count=1
```

Expected: pass.

### Task 2: Preserve Source-Specific Types in Surface Ledger

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/model.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/docs_snapshot.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/org_snapshot.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/glade_snapshot.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/merge.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/merge_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/report.go`

- [ ] Add source-specific contract fields:

```go
DocsReturnType  string   `json:"docsReturnType,omitempty"`
OrgReturnType   string   `json:"orgReturnType,omitempty"`
GladeReturnType string   `json:"gladeReturnType,omitempty"`
DocsParameters  []string `json:"docsParameters,omitempty"`
OrgParameters   []string `json:"orgParameters,omitempty"`
GladeParameters []string `json:"gladeParameters,omitempty"`
```

- [ ] Keep existing `ReturnType` and `Parameters` as display fields only.
- [ ] Fill `DocsReturnType` and `DocsParameters` from docs inventory.
- [ ] Fill `OrgReturnType` and `OrgParameters` from tooling completions.
- [ ] Fill `GladeReturnType` and `GladeParameters` from `typesys.StandardPlatformSymbolView()`.
- [ ] Add gap classes:

```go
GapReturnTypeMismatch = "return-type-mismatch"
GapParameterMismatch = "parameter-mismatch"
```

- [ ] In `Classify`, mark a row as `BucketFailure` when docs or org has a concrete type and Glade has a different concrete type.
- [ ] Treat `Object` as concrete when docs has a narrower non-Object type.
- [ ] Add a test where docs says `List<ConnectApi.ManagedContentVersion>` and Glade says `Object`; expected `return-type-mismatch`.
- [ ] Add a test where docs says `Object` and Glade says `Object`; expected no mismatch.
- [ ] Update markdown reports so failures show docs type and Glade type.

- [ ] Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/surfaceledger -count=1
```

Expected: pass.

### Task 3: Make Surface Check Fail on Type Drift

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_surface_command.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_surface_command_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/report.go`

- [ ] Add `--max-return-type-mismatch <n>` and `--max-parameter-mismatch <n>` to `glade-tools surface check`.
- [ ] Default both to zero when the user passes no explicit ceiling.
- [ ] Keep `--max-missing-shape` behavior unchanged.
- [ ] Add CLI tests:

```text
surface check fails when return-type-mismatch=1 and ceiling is 0
surface check passes when return-type-mismatch=1 and ceiling is 1
surface check still reports missing-shape separately
```

- [ ] Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/toolcli ./internal/surfaceledger -count=1
```

Expected: pass.

### Task 4: Replace Stub Property Guessing With Docs Contracts

**Files:**
- Modify: `/Users/matt/Dev/glade/scripts/generate-system-stub-symbols.mjs`
- Modify: `/Users/matt/Dev/glade/internal/typesys/system_stub_symbols_generated.go`
- Modify: `/Users/matt/Dev/glade/internal/typesys/standard_symbols_test.go`
- Modify: `/Users/matt/Dev/glade/internal/repoguard/repo_standards_test.go`
- Create: `/Users/matt/Dev/glade/testdata/generated/apex_docs_contracts.json`

- [ ] Generate `testdata/generated/apex_docs_contracts.json` from `glade-tools` docs extraction.
- [ ] Make `scripts/generate-system-stub-symbols.mjs` accept that JSON as an optional input.
- [ ] When a stub property has no type and the docs contract has a type, use the docs type.
- [ ] Keep getter, setter, and constructor inference as fallback only after docs lookup fails.
- [ ] Add a generator test or repo-guard assertion for:

```text
ConnectApi.ManagedContentVersionCollection.items == List<ConnectApi.ManagedContentVersion>
ConnectApi.ManagedContentVersionCollection.managedContentTypes == Map<String,ConnectApi.ManagedContentType>
```

- [ ] Add a repo-guard assertion that a docs-backed property cannot generate as `Object` when docs has a narrower type.
- [ ] Regenerate:

```bash
cd /Users/matt/Dev/glade
node scripts/generate-system-stub-symbols.mjs \
  example-projects/stubs/apex-system-stubs \
  internal/typesys/system_stub_symbols_generated.go \
  testdata/generated/apex_docs_contracts.json
```

- [ ] Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/typesys ./internal/repoguard -count=1
```

Expected: pass.

### Task 5: Generate Product Namespace Shapes From Docs, Not `Object`

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/product_namespace_symbols.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/product_namespace_symbols_test.go`
- Modify: `/Users/matt/Dev/glade/internal/typesys/product_namespace_symbols_generated.go`

- [ ] Feed docs return types and property types into `BuildProductNamespaceSymbolSpecs`.
- [ ] Let tooling completions override docs only when tooling has a concrete type.
- [ ] Stop defaulting docs-derived properties and methods to `Object` when docs has a type.
- [ ] Add tests with one docs-only type, one tooling-only type, and one docs-plus-tooling type.
- [ ] Regenerate product namespace symbols.
- [ ] Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/capability -run ProductNamespace -count=1
cd /Users/matt/Dev/glade
go test ./internal/typesys -count=1
```

Expected: pass.

### Task 6: Add Docs-Backed Sema Contract Fixtures

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/sema/sema_test.go`
- Modify: `/Users/matt/Dev/glade/internal/sema/platform_signatures.go`
- Modify: `/Users/matt/Dev/glade/internal/sema/type_members.go`
- Modify: `/Users/matt/Dev/glade/internal/sema/body_calls.go`
- Modify: `/Users/matt/Dev/glade/internal/sema/body_ir.go`
- Modify: `/Users/matt/Dev/glade/internal/metadata/metadata.go`
- Modify: `/Users/matt/Dev/glade/internal/typesys/symbols.go`

- [ ] Add a table-driven sema fixture named `TestDocsBackedSalesforceContracts`.
- [ ] Include one fixture per docs-backed language or stdlib contract:

```apex
// System.Object inheritance
Boolean b = value.equals(other);
Integer h = value.hashCode();
String s = value.toString();

// Flow.Interview project-local type
Flow.Interview.Calculate_discounts flow = new Flow.Interview.Calculate_discounts(inputs);
flow.start();
Object output = flow.getVariableValue('Discount');

// Collection constructors
List<Id> ids = new List<Id>(accountMap.keySet());
Set<Id> idSet = new Set<Id>(ids);
Map<Id, Account> byId = new Map<Id, Account>(accounts);

// Type and Schema
Type t = Account.class;
Object instance = t.newInstance();
Map<String, Schema.SObjectType> gd = Schema.getGlobalDescribe();

// Batchable and iterator
Database.executeBatch(new MyBatch(), 50);
Iterator<Account> it = source.iterator();
```

- [ ] Each fixture must include the docs source path in the test name or test case struct.
- [ ] The Flow fixture must create a temp `.flow-meta.xml` file and prove metadata-derived static flow type generation.
- [ ] Add code to generate project-local `Flow.Interview.<FlowName>` symbols from metadata.
- [ ] Fix inherited Object member lookup for primitives, sObjects, user classes, enums, and generated platform DTOs.
- [ ] Fix collection constructor and generic method matching before overload diagnostics fire.
- [ ] Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/sema ./internal/metadata ./internal/typesys -count=1
```

Expected: pass.

### Task 7: Classify SOQL and Metadata Failures Separately

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/sema/sema_checks.go`
- Modify: `/Users/matt/Dev/glade/internal/storage/standard_schema_generated.go`
- Modify: `/Users/matt/Dev/glade/internal/storage/standard_sobject_stub_overlay_generated.go`
- Modify: `/Users/matt/Dev/glade/internal/storage/model_test.go`
- Modify: `/Users/matt/Dev/glade/internal/sema/sema_test.go`

- [ ] Add focused tests for current public corpus query clusters:

```text
Task.Activity fields
Task custom lookup relationship paths from local metadata
User relationship paths
child relationship subquery list typing
AggregateResult enhanced-for typing
Database.query assignment context
```

- [ ] Fix standard schema gaps only when public standard docs or generated standard schema support them.
- [ ] Classify local custom-field misses as project metadata issues when source metadata lacks the field.
- [ ] Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/sema ./internal/storage -count=1
```

Expected: pass.

### Task 8: Add a Public Corpus Classifier in `glade-tools`

**Files:**
- Create: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck.go`
- Create: `/Users/matt/Dev/glade-tools/internal/corpuscheck/corpuscheck_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/cli.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_command.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/plugin_command_test.go`

- [ ] Add command:

```bash
glade-tools corpus check --root <corpus-root> --glade <binary> --out <dir>
```

- [ ] The command must run `glade check --format json --no-progress` once per project.
- [ ] Emit:

```text
summary.tsv
diagnostics.tsv
by_code.tsv
by_project_code.tsv
by_stem.tsv
classified.tsv
```

- [ ] Classify diagnostics into:

```text
docs-contract-mismatch
generated-shape-gap
semantic-contract-gap
project-discovery-duplicate
project-metadata-missing
source-parse-error
performance-advisory
explicit-unsupported
unclassified
```

- [ ] Add `--fail-on-unclassified` and `--max-unclassified <n>`.
- [ ] Add tests using two tiny fake project JSON outputs and one invalid JSON output.
- [ ] Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/corpuscheck ./internal/toolcli -count=1
```

Expected: pass.

### Task 9: Fix Project Discovery Duplicate Indexing as a Gate, Not as Cleanup

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/project/project.go`
- Modify: `/Users/matt/Dev/glade/internal/project/project_test.go`
- Modify: `/Users/matt/Dev/glade/internal/typesys/symbols.go`
- Modify: `/Users/matt/Dev/glade/internal/typesys/standard_symbols_test.go`

- [ ] Add a project fixture with overlapping `sfdx-project.json` package directories.
- [ ] Add a second fixture with two different files declaring the same class.
- [ ] Index a canonical file path only once when package directories overlap.
- [ ] Preserve `GLADETYPE001` for true duplicate declarations in different files.
- [ ] Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/project ./internal/typesys -count=1
```

Expected: pass.

### Task 10: Close the 66 Apex Missing-Shape Rows by Family

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/typesys/system_stub_symbols_generated.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/generated_platform_runtime.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/platform_connectapi_runtime.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/platform_metadata_reports.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/platform_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/packets.go`

- [ ] For the 41 `commercepayments` rows, generate passive DTO shape where docs define DTO fields.
- [ ] For the 25 `ConnectApi` commerce/routing rows, generate passive DTO shape and explicit unsupported behavior for live service methods that cannot run locally.
- [ ] If a row is deliberately unsupported, set `GladeBehavior=unsupported` with evidence and notes. Do not leave it as missing shape.
- [ ] Add one VM passive DTO test per family:

```text
commercepayments.EnhancedPaymentDataInput.lineItems
commercepayments.LineItemInput.quantity
ConnectApi.CartFromQuoteInput
ConnectApi.CommerceQuotes.getQuoteDetail
ConnectApi.OptimizationFiles.FetchOptimizationFiles
```

- [ ] Re-run:

```bash
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools surface refresh \
  --docs "/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run" \
  --tooling-completions /Users/matt/Dev/glade/testdata/generated/tooling_system_symbols.json.gz \
  --out /tmp/glade-surface-refresh-after
jq '.summary.gaps["missing-shape"] // 0, .summary.failures' /tmp/glade-surface-refresh-after/SURFACE_LEDGER.json
```

Expected:

```text
0 apex missing-shape rows
0 return-type-mismatch rows
0 parameter-mismatch rows
```

Unknown guide pages can remain outside the Apex implementation gate.

### Task 11: Run the Full Proof Gate

**Files:**
- No product source file changes in this task.

- [ ] Build a fresh binary:

```bash
cd /Users/matt/Dev/glade
go build -o /tmp/glade-docs-first-proof ./cmd/glade
```

- [ ] Run product tests:

```bash
cd /Users/matt/Dev/glade
go test ./internal/apexast ./internal/metadata ./internal/project ./internal/typesys ./internal/sema ./internal/storage ./internal/vm ./internal/repoguard -count=1
```

- [ ] Run tool tests:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/apexdocs ./internal/surfaceledger ./internal/capability ./internal/corpuscheck ./internal/toolcli -count=1
```

- [ ] Run surface refresh:

```bash
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools surface refresh \
  --docs "/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run" \
  --tooling-completions /Users/matt/Dev/glade/testdata/generated/tooling_system_symbols.json.gz \
  --out /tmp/glade-surface-docs-first-proof
go run ./cmd/glade-tools surface check \
  --ledger /tmp/glade-surface-docs-first-proof/SURFACE_LEDGER.json \
  --max-missing-shape 0 \
  --max-return-type-mismatch 0 \
  --max-parameter-mismatch 0
```

- [ ] Run public corpus:

```bash
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools corpus check \
  --root /Users/matt/Dev/glade-corpus/public \
  --glade /tmp/glade-docs-first-proof \
  --out /tmp/glade-public-corpus-docs-first-proof \
  --fail-on-unclassified
```

Expected:

```text
unclassified=0
project-discovery-duplicate=0 unless each row names two different source files with real duplicate declarations
docs-contract-mismatch=0
generated-shape-gap=0
semantic-contract-gap=0
```

Remaining `project-metadata-missing`, `source-parse-error`, `performance-advisory`, and `explicit-unsupported` rows must be listed by project and source path.

---

## Completion Criteria

- [ ] Surface Ledger has source-specific docs, org, and Glade type data.
- [ ] A docs type that degrades to `Object` fails the ledger.
- [ ] The 66 Apex missing-shape rows are either implemented or explicitly unsupported with evidence.
- [ ] System stub generation consumes docs contracts before fallback inference.
- [ ] The public corpus runner classifies every diagnostic.
- [ ] Public corpus proof has zero unclassified implementation misses.
- [ ] No maintenance scanner or corpus dashboard lands in base `glade`.
