# Salesforce Surface Roadmap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce Glade's measured Salesforce Surface Ledger gaps by closing proof, small local-test rows, and narrow server API rows, while keeping hosted Salesforce services explicitly unsupported.

**Architecture:** Treat the Surface Ledger as the source of truth. Work from a fresh refresh, fix `SURFACE_FAILURES.md` before `SURFACE_GAPS.md`, and change only the packet area under work. Product runtime code stays in `/Users/matt/Dev/glade`; compatibility fixtures, capability rows, and ledger tooling stay in `/Users/matt/Dev/glade-tools`.

**Tech Stack:** Go, Glade VM/server packages, first-party `glade-tools` compat plugin, JSON compatibility fixtures, Salesforce docs mirror at `/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper/salesforce-docs`, generated tooling symbols at `/Users/matt/Dev/glade/testdata/generated/tooling_system_symbols.json.gz`.

---

## Worker Mission

Work in phases. Stop after each phase with before counts, after counts, commands run, and the next top row. Do not try to make Glade a hosted Salesforce replacement.

The best first work session is Phase 0 through Phase 2. That should reduce ledger uncertainty and close the low-risk local-test proof rows. Start Phase 3 only after Phase 2 has a green ratchet.

Current baseline from `/tmp/glade-surface-refresh-20260613`:

| Metric | Count |
| --- | ---: |
| total rows | 184373 |
| implemented | 129268 |
| partial | 35 |
| passive | 47578 |
| explicit unsupported | 1056 |
| stub/no-op | 318 |
| remaining | 6118 |
| missing shape | 1126 |
| missing evidence | 4878 |
| blank gap class | 114 |

The worker must refresh this at the start. These counts may drift.

## Hard Boundaries

- Keep maintenance commands, fixtures, dashboards, scanners, and generated support artifacts in `/Users/matt/Dev/glade-tools`.
- Keep product behavior, VM runtime, server runtime, SOQL, DML, schema, and test runner changes in `/Users/matt/Dev/glade`.
- Do not add broad `compat` or maintenance commands to base `glade`.
- Do not fake live Salesforce services. Use exact fixture evidence or explicit unsupported diagnostics.
- Do not implement Marketing Cloud AMPscript, Marketing Cloud Handlebars, full Aura rendering, full Visualforce rendering, PDF generation, live OAuth/session validation, live outbound email, live callouts, broad GraphQL, Pub/Sub, or every Tooling object.
- Do not add project-specific runtime exceptions.

## File Map

Use these files first. Add new files only when the local pattern calls for it.

`/Users/matt/Dev/glade`:

- `internal/vm/dispatch_static.go`: static platform method registration.
- `internal/vm/dispatch.go`: standard-library and generated platform dispatch.
- `internal/vm/request_runtime.go`: request, service, and unsupported integration boundaries.
- `internal/vm/dml_runtime.go`: DML, Approval, lock, and Database method behavior.
- `internal/vm/platform_passive_members.go`: passive generated platform member behavior.
- `internal/vm/generated_platform_runtime.go`: generated platform type fallback handling.
- `internal/vm/ui_invocation.go`: Aura/LWC Apex controller invocation.
- `internal/soql/parser.go` and `internal/soql/soql.go`: SOQL/SOSL parse and execution.
- `internal/server/server.go`, `internal/server/server_test.go`, `internal/server/composite_handlers.go`: local REST, Tooling, Composite, Bulk, and unsupported server boundaries.
- `internal/apextest/runner.go` and `internal/apextest/runner_test.go`: local test harness, page/controller loading, async drain.
- `internal/visualforce/*`: Visualforce metadata and controller contract support.

`/Users/matt/Dev/glade-tools`:

- `docs/fixtures/*.json`: exact compat fixture evidence and explicit unsupported fences.
- `internal/capability/*`: capability catalog and status docs.
- `internal/surfaceledger/*`: ledger merge, packet ownership, identity joins, reports.
- `internal/toolcli/compat_surface_command.go`: surface refresh, packet, gaps, explain, check commands.

## Phase 0: Fresh Baseline

**Purpose:** Get a clean ledger. This phase changes no files.

**Files:**
- Read: `/tmp/glade-surface-refresh-20260613/SURFACE_PROGRESS.md`
- Read: `/tmp/glade-surface-refresh-20260613/SURFACE_FAILURES.md`
- Read: `/tmp/glade-surface-refresh-20260613/SURFACE_GAPS.md`
- Read: `/Users/matt/Dev/glade-tools/internal/surfaceledger/packets.go`

- [ ] **Step 0.1: Check both worktrees**

Run:

```bash
cd /Users/matt/Dev/glade
git status --short
cd /Users/matt/Dev/glade-tools
git status --short
```

Expected: any dirty files are noted before work starts. Do not revert user changes.

- [ ] **Step 0.2: Refresh the Surface Ledger**

Run:

```bash
cd /Users/matt/Dev/glade-tools
tmp="$(mktemp -d)"
go run ./cmd/glade-tools compat surface refresh \
  --docs "/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper/salesforce-docs" \
  --tooling-completions ../glade/testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp"
printf '%s\n' "$tmp"
sed -n '1,120p' "$tmp/SURFACE_PROGRESS.md"
sed -n '1,40p' "$tmp/SURFACE_FAILURES.md"
```

Expected: `surface refresh: ok`. `SURFACE_FAILURES.md` should contain only its header. If failures appear, stop this plan and fix failures before gaps.

- [ ] **Step 0.3: Count the work by gap class**

Run:

```bash
python3 - "$tmp/SURFACE_LEDGER.json" <<'PY'
import json, sys
from collections import Counter
rows = json.load(open(sys.argv[1]))["rows"]
print("bucket", Counter(r.get("bucket", "") for r in rows))
print("gapClass", Counter((r.get("gapClass") or "blank") for r in rows if r.get("bucket") == "gap"))
print("gap product", Counter(r.get("product", "") for r in rows if r.get("bucket") == "gap"))
print("gap namespace", Counter(r.get("namespace", "") for r in rows if r.get("bucket") == "gap").most_common(30))
PY
```

Expected: a clear count for `missing-shape`, `missing-evidence`, and `blank`.

- [ ] **Step 0.4: Open packets for first work**

Run:

```bash
go run ./cmd/glade-tools compat surface packet --ledger "$tmp/SURFACE_LEDGER.json" --area Core.Runtime.SystemAndStdlib > "$tmp/core-stdlib.packet.md"
go run ./cmd/glade-tools compat surface packet --ledger "$tmp/SURFACE_LEDGER.json" --area Tests.AsyncAndIsolation > "$tmp/async-tests.packet.md"
go run ./cmd/glade-tools compat surface packet --ledger "$tmp/SURFACE_LEDGER.json" --area Query.Runtime.SOQLSOSL > "$tmp/query.packet.md"
go run ./cmd/glade-tools compat surface packet --ledger "$tmp/SURFACE_LEDGER.json" --area UI.LWCModules > "$tmp/lwc.packet.md"
sed -n '/Rows To Explain First/,+45p' "$tmp/core-stdlib.packet.md"
sed -n '/Rows To Explain First/,+35p' "$tmp/async-tests.packet.md"
```

Expected: the first packet rows are mostly `missing-evidence`, not missing behavior.

## Phase 1: Ledger Identity And Blank Gap Rows

**Purpose:** Remove false gaps where evidence exists but the ledger cannot join docs, fixtures, or Glade rows. This phase belongs in `glade-tools`.

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/*.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/*_test.go`
- Modify: `/Users/matt/Dev/glade-tools/docs/fixtures/*.json`
- Test: `/Users/matt/Dev/glade-tools/internal/surfaceledger`

- [ ] **Step 1.1: Print blank rows**

Run:

```bash
python3 - "$tmp/SURFACE_LEDGER.json" <<'PY'
import json, sys
rows = json.load(open(sys.argv[1]))["rows"]
for r in rows:
    if r.get("bucket") == "gap" and not r.get("gapClass"):
        print(r["surfaceId"], "|", r.get("product"), "|", r.get("gladeShape"), "|", r.get("gladeBehavior"), "|", r.get("evidence"), "|", r.get("notes", "")[:120])
PY
```

Expected: rows such as `unknown:apex_dynamic_soql`, `lwc:apex-wire.sobject-json`, and service-fenced Apex rows.

- [ ] **Step 1.2: Classify each blank row**

Use this rule:

- If `gladeBehavior=supported` and `evidence=fixture`, fix docs/source identity in `internal/surfaceledger`.
- If `gladeBehavior=none` and `evidence=fixture`, check whether the fixture should be `explicitUnsupported` or whether the row ID should join a known unsupported call.
- If the row is an LWC bridge row, keep it in UI/LWC and do not claim full LWC runtime.

- [ ] **Step 1.3: Write a failing surfaceledger test**

Add a focused test in `/Users/matt/Dev/glade-tools/internal/surfaceledger/merge_test.go` or the nearest existing test file. Use concrete rows from Step 1.1. A good first test shape:

```go
func TestClassifyFixtureBackedLWCBridgeRowDoesNotRemainBlankGap(t *testing.T) {
    row := SurfaceLedgerRow{
        SurfaceID:     "lwc:apex-wire.sobject-json",
        Product:       ProductLWC,
        Area:          AreaUI,
        Kind:          KindMethod,
        Docs:          SourceAbsent,
        Org:           SourceAbsent,
        GladeShape:    ShapeAbsent,
        GladeBehavior: BehaviorSupported,
        Evidence:      EvidenceFixture,
    }

    Classify(&row)
    if row.Bucket == BucketGap && row.GapClass == "" {
        t.Fatalf("blank gap remained after fixture evidence merge: %#v", row)
    }
}
```

If the first blank row under work is not an LWC row, use the same shape with that exact `SurfaceID`, `Product`, `Area`, and `Kind`. Keep the assertion exact.

- [ ] **Step 1.4: Run the failing test**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/surfaceledger -run TestClassifyFixtureBackedLWCBridgeRowDoesNotRemainBlankGap -count=1
```

Expected before the fix: FAIL with the blank gap still present, or a compile error that names the helper to use.

- [ ] **Step 1.5: Fix the join, not runtime behavior**

Modify only `internal/surfaceledger` merge/normalization code or fixture surface IDs. Do not touch `/Users/matt/Dev/glade/internal/vm` in this phase.

- [ ] **Step 1.6: Verify the phase**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/surfaceledger -count=1
tmp2="$(mktemp -d)"
go run ./cmd/glade-tools compat surface refresh \
  --docs "/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper/salesforce-docs" \
  --tooling-completions ../glade/testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp2"
python3 - "$tmp2/SURFACE_LEDGER.json" <<'PY'
import json, sys
from collections import Counter
rows = json.load(open(sys.argv[1]))["rows"]
print(Counter((r.get("gapClass") or "blank") for r in rows if r.get("bucket") == "gap"))
PY
go run ./cmd/glade-tools compat surface check --ledger "$tmp2/SURFACE_LEDGER.json" --max-parser-failures 0 --max-missing-shape 1126
```

Expected: blank gap count goes down. Missing shape does not increase.

- [ ] **Step 1.7: Commit**

Run:

```bash
cd /Users/matt/Dev/glade-tools
git add internal/surfaceledger docs/fixtures
git commit -m "fix: normalize surface ledger fixture evidence joins"
```

Expected: commit succeeds. If only fixture IDs changed, the message may be `test: align surface fixture ids`.

## Phase 2: Close Small Local-Test Evidence Rows

**Purpose:** Burn down missing evidence where Glade already has local behavior. This phase should not add broad new runtime behavior.

### Task 2A: Core Runtime And Stdlib Evidence

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/docs/fixtures/core-runtime-local-service-evidence-closeout.json`
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/*`
- Test: `/Users/matt/Dev/glade-tools/internal/capability`
- Test: `/Users/matt/Dev/glade/internal/vm`

- [ ] **Step 2A.1: Explain the first core rows**

Run:

```bash
cd /Users/matt/Dev/glade-tools
for id in \
  'apex:Approval.process(Approval.ProcessRequest,Boolean)' \
  'apex:BusinessHours.diff(String,Datetime,Datetime)' \
  'apex:BusinessHours.isWithin(String,Datetime)' \
  'apex:System.QuickAction.describeAvailableQuickActions(String)' \
  'apex:System.Search.find(String,Object)' \
  'apex:System.Test.enableChangeDataCapture()'
do
  go run ./cmd/glade-tools compat surface explain --ledger "$tmp/SURFACE_LEDGER.json" --id "$id"
done
```

Expected: rows show `gapClass: missing-evidence` or the current equivalent.

- [ ] **Step 2A.2: Add one fixture that exercises local service boundaries**

Create or extend `/Users/matt/Dev/glade-tools/docs/fixtures/core-runtime-local-service-evidence-closeout.json` with a single compatibility fixture covering:

- `Approval.process(Approval.ProcessRequest, Boolean)` returns a deterministic local approval result or explicit local boundary.
- `BusinessHours.diff` and `BusinessHours.isWithin` use the local week schedule model.
- `QuickAction.describeAvailableQuickActions(String)` and one perform/retrieve overload return local DTOs.
- `Search.find(String,Object)` or `Search.query(String,Object)` uses deterministic local search.
- `Request.getCurrent`, `Request.getRequestId`, and `Request.getQuiddity` return deterministic local request values.
- `Test.enableChangeDataCapture()` invokes the local test harness.

Use the existing fixture schema. Keep each `surfaceId` exact. Use short Apex snippets that assert observable local behavior.

- [ ] **Step 2A.3: Run fixture validation**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools compat validate docs/fixtures/core-runtime-local-service-evidence-closeout.json
go run ./cmd/glade-tools compat run docs/fixtures/core-runtime-local-service-evidence-closeout.json
```

Expected: validation and run pass. If a row returns an unsupported diagnostic by design, mark fixture evidence as `unsupported` and keep the message exact.

- [ ] **Step 2A.4: Run focused product tests**

Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/vm ./internal/apextest ./internal/soql
cd /Users/matt/Dev/glade-tools
go test ./internal/capability
```

Expected: all tests pass.

- [ ] **Step 2A.5: Refresh and check the core packet**

Run:

```bash
cd /Users/matt/Dev/glade-tools
tmp2="$(mktemp -d)"
go run ./cmd/glade-tools compat surface refresh \
  --docs "/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper/salesforce-docs" \
  --tooling-completions ../glade/testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp2"
go run ./cmd/glade-tools compat surface packet --ledger "$tmp2/SURFACE_LEDGER.json" --area Core.Runtime.SystemAndStdlib | sed -n '/Rows To Explain First/,+45p'
go run ./cmd/glade-tools compat surface check --ledger "$tmp2/SURFACE_LEDGER.json" --max-parser-failures 0 --max-missing-shape 1126
```

Expected: Core.Runtime.SystemAndStdlib missing-evidence count goes down. Missing shape does not increase.

- [ ] **Step 2A.6: Commit**

Run:

```bash
cd /Users/matt/Dev/glade-tools
git add docs/fixtures internal/capability
git commit -m "test: add local service surface evidence"
```

### Task 2B: Async And Test Harness Evidence

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/docs/fixtures/async-test-harness-local-evidence.json`
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/*`
- Test: `/Users/matt/Dev/glade/internal/apextest`

- [ ] **Step 2B.1: Target exact async rows**

Use the packet rows:

- `apex:System.Test.enableChangeDataCapture()`
- `apex:System.Test.invokeContinuationMethod(Object,Continuation)`
- `apex:System.Test.newSendEmailQuickActionDefaults(Id,Id)`
- `apex:System.Test.setCurrentPageReference(Object)`
- `apex:System.Test.testInstall(InstallHandler,Version)`
- `apex:System.Test.testInstall(InstallHandler,Version,Boolean)`
- `apex:System.Test.testNotificationActionHandler(Messaging.NotificationActionHandler,Messaging.ActionableNotification)`
- `apex:System.Test.testSandboxPostCopyScript(SandboxPostCopy,Id,Id,String)`
- `apex:System.Test.testSandboxPostCopyScript(SandboxPostCopy,Id,Id,String,Boolean)`
- `apex:System.Test.testUninstall(UninstallHandler)`

- [ ] **Step 2B.2: Add or extend one fixture**

Add short `@isTest` snippets that call the listed helpers and assert deterministic local IDs, PageReference values, or no live service contact. Keep each surface ID exact.

- [ ] **Step 2B.3: Run focused tests**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools compat validate docs/fixtures/async-test-harness-local-evidence.json
go run ./cmd/glade-tools compat run docs/fixtures/async-test-harness-local-evidence.json
cd /Users/matt/Dev/glade
go test ./internal/apextest ./internal/vm
```

Expected: fixture and tests pass.

- [ ] **Step 2B.4: Commit**

Run:

```bash
cd /Users/matt/Dev/glade-tools
git add docs/fixtures internal/capability
git commit -m "test: add async test harness surface evidence"
```

### Task 2C: Query, Schema, And LWC Evidence

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/docs/fixtures/data-platform-search-unsupported-api.json`
- Modify: `/Users/matt/Dev/glade-tools/docs/fixtures/ui-lwc-vf-local-bridge-evidence.json`
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/*`
- Test: `/Users/matt/Dev/glade/internal/soql`
- Test: `/Users/matt/Dev/glade/internal/vm`

- [ ] **Step 2C.1: Close query evidence rows with existing fixtures**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools compat surface packet --ledger "$tmp/SURFACE_LEDGER.json" --area Query.Runtime.SOQLSOSL | sed -n '/Rows To Explain First/,+70p'
go run ./cmd/glade-tools compat run docs/fixtures/data-platform-search-unsupported-api.json
```

Expected: identify whether `unknown:apex_dynamic_soql`, `unknown:apex_dynamic_sosl`, and SOQL guide rows need fixture IDs, evidence kind, or docs identity fixes.

- [ ] **Step 2C.2: Close LWC bridge rows without claiming LWC runtime**

Run:

```bash
go run ./cmd/glade-tools compat surface explain --ledger "$tmp/SURFACE_LEDGER.json" --id 'lwc:apex-wire.sobject-json'
go run ./cmd/glade-tools compat surface explain --ledger "$tmp/SURFACE_LEDGER.json" --id 'lwc:salesforce/uiRecordApi.getObjectInfo'
go run ./cmd/glade-tools compat run docs/fixtures/ui-lwc-vf-local-bridge-evidence.json
```

Expected: the fixture proves local Apex wire SObject JSON and object-info JSON only. Full LWC modules remain explicit unsupported.

- [ ] **Step 2C.3: Run focused product tests**

Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/soql ./internal/vm ./internal/lwcruntime
cd /Users/matt/Dev/glade-tools
go test ./internal/capability ./internal/surfaceledger
```

Expected: all tests pass.

- [ ] **Step 2C.4: Refresh and check**

Run:

```bash
cd /Users/matt/Dev/glade-tools
tmp3="$(mktemp -d)"
go run ./cmd/glade-tools compat surface refresh \
  --docs "/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper/salesforce-docs" \
  --tooling-completions ../glade/testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp3"
go run ./cmd/glade-tools compat surface check --ledger "$tmp3/SURFACE_LEDGER.json" --max-parser-failures 0 --max-missing-shape 1126
```

Expected: remaining count drops. No failures appear.

- [ ] **Step 2C.5: Commit**

Run:

```bash
cd /Users/matt/Dev/glade-tools
git add docs/fixtures internal/capability internal/surfaceledger
git commit -m "test: close query schema and LWC surface evidence"
```

## Phase 3: Narrow Server REST And Tooling Slice

**Purpose:** Add local server breadth where developer tools and AI clients expect Salesforce-shaped responses. Do not implement every documented route.

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/server/server.go`
- Modify: `/Users/matt/Dev/glade/internal/server/server_test.go`
- Modify: `/Users/matt/Dev/glade/internal/server/composite_handlers.go`
- Modify: `/Users/matt/Dev/glade-tools/docs/fixtures/integration-rest-server-apexrest-unsupported.json`
- Modify: `/Users/matt/Dev/glade-tools/docs/fixtures/integration-soapapi-unsupported.json`
- Modify: `/Users/matt/Dev/glade-tools/docs/fixtures/platform-unsupported-surfaces.json`

### Task 3A: Tooling Object Read Baseline

- [ ] **Step 3A.1: Start with supported local source objects**

Target these rows first:

- `tooling:ApexClass`
- `tooling:ApexTrigger`
- `tooling:ApexLog`
- `tooling:ApexTestResult`
- `tooling:ApexTestRunResult`
- `tooling:ApexCodeCoverage`
- `tooling:TraceFlag`
- `tooling:ContainerAsyncRequest`
- `tooling:ApexClassMember`
- `tooling:ApexTriggerMember`

- [ ] **Step 3A.2: Write failing server tests**

In `/Users/matt/Dev/glade/internal/server/server_test.go`, add table tests beside `TestToolingSObjectDiscoveryDescribeAndRecordRead` and `TestToolingQueryReadsLocalSourceMetadata`.

Test behaviors:

- `/services/data/vXX.X/tooling/sobjects` lists supported local Tooling names.
- `/services/data/vXX.X/tooling/sobjects/ApexClass/describe` returns fields needed by local query clients.
- Tooling query for `SELECT Id, Name, Body FROM ApexClass` reads project Apex source.
- Unknown Tooling objects still return stable unsupported or not found.

- [ ] **Step 3A.3: Run the failing tests**

Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/server -run 'TestTooling.*(Discovery|Query|SObject)' -count=1
```

Expected before implementation: FAIL on the new unsupported routes.

- [ ] **Step 3A.4: Implement only read-shaped local responses**

Use existing project loader and source metadata paths. Do not add mutation or deploy behavior. Keep POST/PATCH/DELETE unsupported unless an exact local model exists.

- [ ] **Step 3A.5: Verify Tooling**

Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/server -run 'TestTooling.*' -count=1
go test ./internal/server
```

Expected: server tests pass.

### Task 3B: Composite Tree, Batch, And Graph Validation

- [ ] **Step 3B.1: Keep generic Composite behavior separate**

Read:

```bash
sed -n '344,430p' /Users/matt/Dev/glade/internal/server/composite_handlers.go
sed -n '1817,2008p' /Users/matt/Dev/glade/internal/server/server_test.go
```

- [ ] **Step 3B.2: Add tests for one useful route at a time**

Start with Composite tree create for Account with one child Contact. Then add batch/graph orchestration over already supported subrequests. Keep unsupported validation tests for route families not implemented.

- [ ] **Step 3B.3: Run server tests**

Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/server -run 'TestComposite.*' -count=1
go test ./internal/server
```

Expected: supported routes pass. Unsupported routes still return `UNSUPPORTED_FEATURE` with stable messages.

### Task 3C: Server Ledger Evidence

- [ ] **Step 3C.1: Add server fixtures**

Add or extend `glade-tools` fixtures for the exact REST/Tooling rows changed. Each fixture must name the route and expected response shape.

- [ ] **Step 3C.2: Refresh server packets**

Run:

```bash
cd /Users/matt/Dev/glade-tools
tmp4="$(mktemp -d)"
go run ./cmd/glade-tools compat surface refresh \
  --docs "/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper/salesforce-docs" \
  --tooling-completions ../glade/testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp4"
go run ./cmd/glade-tools compat surface packet --ledger "$tmp4/SURFACE_LEDGER.json" --area Server.ToolingObjects | sed -n '/Rows To Explain First/,+60p'
go run ./cmd/glade-tools compat surface packet --ledger "$tmp4/SURFACE_LEDGER.json" --area Server.RESTResources | sed -n '/Rows To Explain First/,+60p'
go run ./cmd/glade-tools compat surface check --ledger "$tmp4/SURFACE_LEDGER.json" --max-parser-failures 0 --max-missing-shape 1126
```

Expected: targeted server rows move out of missing shape or gain explicit unsupported evidence. Untouched broad rows remain gaps.

- [ ] **Step 3C.3: Commit both repos**

Run:

```bash
cd /Users/matt/Dev/glade
git add internal/server
git commit -m "feat: add narrow local Tooling and Composite server coverage"
cd /Users/matt/Dev/glade-tools
git add docs/fixtures internal/capability
git commit -m "test: add server surface evidence"
```

## Phase 4: UI Controller Contracts Without Rendering

**Purpose:** Help Apex tests that depend on Aura/LWC/Visualforce controller contracts. Do not build a browser renderer.

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/vm/ui_invocation.go`
- Modify: `/Users/matt/Dev/glade/internal/apextest/runner.go`
- Modify: `/Users/matt/Dev/glade/internal/visualforce/*`
- Modify: `/Users/matt/Dev/glade-tools/docs/fixtures/ui-lwc-vf-local-bridge-evidence.json`

- [ ] **Step 4.1: Open the UI packet**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools compat surface packet --ledger "$tmp/SURFACE_LEDGER.json" --area UI.AuraComponents | sed -n '/Rows To Explain First/,+70p'
go run ./cmd/glade-tools compat surface packet --ledger "$tmp/SURFACE_LEDGER.json" --area UI.LWCModules | sed -n '/Rows To Explain First/,+50p'
```

- [ ] **Step 4.2: Add tests around controller invocation only**

In `/Users/matt/Dev/glade/internal/vm/ui_invocation.go` tests or `/Users/matt/Dev/glade/internal/apextest/runner_test.go`, prove:

- Static `@AuraEnabled` method invocation works with named params.
- SObject and DTO responses serialize through the same JSON path used by LWC.
- PageReference and ApexPages message state remain isolated per test.
- Full rendering methods such as `getContent` stay unsupported.

- [ ] **Step 4.3: Run focused tests**

Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/vm -run 'Test.*(UIInvocation|PageReference|ApexPages|InvokePage)' -count=1
go test ./internal/apextest -run 'TestRunResolves.*(Visualforce|Aura|LWC)' -count=1
```

Expected: controller contract tests pass. Rendering remains unsupported.

- [ ] **Step 4.4: Add fixture evidence**

Update `/Users/matt/Dev/glade-tools/docs/fixtures/ui-lwc-vf-local-bridge-evidence.json` with exact `surfaceId` entries for any row proven.

- [ ] **Step 4.5: Commit**

Run:

```bash
cd /Users/matt/Dev/glade
git add internal/vm internal/apextest internal/visualforce
git commit -m "feat: strengthen local UI controller contracts"
cd /Users/matt/Dev/glade-tools
git add docs/fixtures internal/capability
git commit -m "test: add UI controller surface evidence"
```

## Phase 5: Curated ConnectApi, Last

**Purpose:** Reduce high-count ConnectApi rows only where the local model is honest. This phase must not become a service simulation.

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/vm/platform_passive_members.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/generated_platform_runtime.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/dispatch_static.go`
- Modify: `/Users/matt/Dev/glade-tools/docs/fixtures/apex-connectapi-*.json`
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/*`

- [ ] **Step 5.1: Split ConnectApi rows into three piles**

Run:

```bash
cd /Users/matt/Dev/glade-tools
python3 - "$tmp/SURFACE_LEDGER.json" <<'PY'
import json, sys
from collections import Counter
rows = json.load(open(sys.argv[1]))["rows"]
connect = [r for r in rows if r.get("bucket") == "gap" and r.get("namespace") == "ConnectApi"]
print("gapClass", Counter(r.get("gapClass", "") for r in connect))
print("typeName", Counter(r.get("typeName", "") for r in connect).most_common(40))
for r in connect[:80]:
    print(r["surfaceId"], "|", r.get("gapClass"), "|", r.get("kind"), "|", r.get("gladeShape"), "|", r.get("gladeBehavior"))
PY
```

Classify as:

- Passive DTO shape: constructor, fields, enum values, clone, `toString`.
- Test setter or local getter: can be fixture-backed.
- Hosted service mutation/read: explicit unsupported.

- [ ] **Step 5.2: Start with shape rows, not service methods**

Target `missing-shape` DTO rows such as `ConnectApi.AbstractBaseSequenceInputRepresentation` only if the docs/source shape is clear and adding it helps compile local tests. Use generated platform shape patterns. Do not create fake service results.

- [ ] **Step 5.3: Add explicit unsupported fixtures for hosted calls**

For service methods like Chatter feed mutation, commerce cart mutation, CDP activation, orchestration publish, and social engagement, add exact unsupported fixtures. The diagnostic must say what local service boundary is not modeled.

- [ ] **Step 5.4: Verify ConnectApi**

Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/vm ./internal/apextest
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools compat run docs/fixtures/apex-connectapi-chatter.json docs/fixtures/apex-connectapi-commerce.json docs/fixtures/apex-connectapi-identity.json docs/fixtures/apex-connectapi-misc.json
go test ./internal/capability ./internal/surfaceledger
```

Expected: supported DTO/test rows pass. Hosted service rows fail with stable unsupported diagnostics.

- [ ] **Step 5.5: Refresh and commit**

Run:

```bash
cd /Users/matt/Dev/glade-tools
tmp5="$(mktemp -d)"
go run ./cmd/glade-tools compat surface refresh \
  --docs "/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper/salesforce-docs" \
  --tooling-completions ../glade/testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp5"
go run ./cmd/glade-tools compat surface packet --ledger "$tmp5/SURFACE_LEDGER.json" --area ConnectApi.PassiveDTOs | sed -n '/Rows To Explain First/,+80p'
go run ./cmd/glade-tools compat surface check --ledger "$tmp5/SURFACE_LEDGER.json" --max-parser-failures 0 --max-missing-shape 1126
cd /Users/matt/Dev/glade
git add internal/vm internal/apextest
git commit -m "feat: add curated ConnectApi local contracts"
cd /Users/matt/Dev/glade-tools
git add docs/fixtures internal/capability
git commit -m "test: add curated ConnectApi surface evidence"
```

## Phase 6: Packaged Product And External Product Fences

**Purpose:** Keep broad product namespaces honest. Add compile/load shapes only when a real project needs them.

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/vm/generated_platform_runtime.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/request_runtime.go`
- Modify: `/Users/matt/Dev/glade-tools/docs/fixtures/*unsupported*.json`
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/*`

- [ ] **Step 6.1: Print packaged product gaps**

Run:

```bash
cd /Users/matt/Dev/glade-tools
python3 - "$tmp/SURFACE_LEDGER.json" <<'PY'
import json, sys
from collections import Counter
namespaces = {"Slack", "commercepayments", "CartExtension", "RichMessaging", "wave", "reports", "sfsqlquery", "sfdatakit", "Datacloud", "healthcloudext"}
rows = [r for r in json.load(open(sys.argv[1]))["rows"] if r.get("bucket") == "gap" and r.get("namespace") in namespaces]
print(Counter(r.get("namespace", "") for r in rows))
for r in rows[:120]:
    print(r["surfaceId"], "|", r.get("gapClass"), "|", r.get("kind"))
PY
```

- [ ] **Step 6.2: Fence hosted product services**

For Slack, Commerce payments, Data Cloud, Wave, reports execution, Health Cloud service calls, and Rich Messaging mutations, prefer exact explicit unsupported fixtures. Only add passive DTO shape if source must compile.

- [ ] **Step 6.3: Verify fences**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools compat validate docs/fixtures/*unsupported*.json
go test ./internal/capability ./internal/surfaceledger
```

Expected: unsupported fixtures validate. No product runtime code claims a hosted service result.

- [ ] **Step 6.4: Commit**

Run:

```bash
cd /Users/matt/Dev/glade
git add internal/vm
git commit -m "fix: fence packaged platform service surfaces"
cd /Users/matt/Dev/glade-tools
git add docs/fixtures internal/capability
git commit -m "test: add packaged service unsupported evidence"
```

## Final Verification For Any Work Session

Run this before claiming the session is complete:

```bash
cd /Users/matt/Dev/glade
go test ./internal/vm ./internal/apextest ./internal/soql ./internal/server ./internal/repoguard
cd /Users/matt/Dev/glade-tools
go test ./internal/surfaceledger ./internal/capability ./internal/toolcli
tmp_final="$(mktemp -d)"
go run ./cmd/glade-tools compat surface refresh \
  --docs "/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper/salesforce-docs" \
  --tooling-completions ../glade/testdata/generated/tooling_system_symbols.json.gz \
  --out "$tmp_final"
sed -n '1,120p' "$tmp_final/SURFACE_PROGRESS.md"
sed -n '1,40p' "$tmp_final/SURFACE_FAILURES.md"
go run ./cmd/glade-tools compat surface check --ledger "$tmp_final/SURFACE_LEDGER.json" --max-parser-failures 0 --max-missing-shape 1126
```

Expected:

- No test failures.
- `SURFACE_FAILURES.md` has only the header.
- Missing shape does not increase.
- Target packet missing evidence or blank gaps decrease.
- Any newly unsupported behavior has exact fixture evidence.

## Handoff Report Format

End each agent run with:

```text
Branch:
Commits:
Fresh ledger path:
Before counts:
After counts:
Rows closed:
Rows intentionally fenced:
Commands run:
Failures:
Next top row:
Files changed:
```

## First Work Session Recommendation

Start with Phase 0. Then complete Phase 1 if blank rows still exist. Then do Phase 2A and Phase 2B. Stop there unless the final refresh is green and there is still time.

That gives a clean stump to stand on before the heavier server and ConnectApi work begins.
