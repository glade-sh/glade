# Apex Oracle Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `oaer compat oracle` as a generator-driven Salesforce oracle factory that inventories every public Apex surface, generates deep parameterized probes, runs them against Salesforce and OAER, and stores enough evidence to reproduce local Apex behavior with confidence.

**Architecture:** Extend the existing oracle, probe, docs inventory, catalog, stub inventory, stub contract, and Salesforce coverage machinery. The command owns the durable model and artifacts. Shell scripts are generated from the command so long runs can continue for days or weeks without agent memory, chat tokens, or hand-written probe work.

**Tech Stack:** Go 1.26, `cmd/oaer`, `internal/oaercli`, `internal/oracle`, `internal/probe`, `internal/capability`, `internal/apexdocs`, `internal/apextest`, `internal/vm`, Salesforce CLI scratch orgs, Tooling API, generated Apex test classes, JSONL ledgers, deterministic `.oaer/oracle` run artifacts.

---

## Hard Line

This is not a small probe harness.

The job is to learn Apex behavior from Salesforce, then make local execution match it or fail with explicit unsupported diagnostics. The harness must cover the finite public Apex surface: language constructs, type system, stdlib classes, methods, constructors, properties, enums, exceptions, SObjects, SOQL, SOSL fences, DML, triggers, tests, async, metadata-shaped APIs, side effects, and org-shape dependent behavior.

It cannot test every possible runtime value. No machine can. It can enumerate every public surface and generate systematic parameter domains for each callable member. That is the correct shape. For each surface it records compile shape, invocation shape, return shape, exception behavior, mutation, side effects, limits, logs, and final state. When one value exposes a new edge, the command adds it to the domain ledger so future runs keep it.

No `supported` claim without oracle evidence or corpus evidence. No hand-written thousands of probes. No guessing from memory.

## Current Timber Already In The Repo

The first implementation should reuse these pieces instead of building a second harness:

- `internal/oracle`: existing normalized run, diff, local runner, Salesforce runner, report, and artifacts.
- `internal/oaercli/compat_oracle.go`: existing `oaer compat oracle-tests` command.
- `internal/probe`: existing org/local probe runner, debug parsing, deploy flow, stub contract probe code, and generated stub contract support.
- `internal/capability`: existing catalog, Salesforce coverage, standard object coverage, stub behavior, stub inventory, and stub contract reports.
- `internal/apexdocs`: existing local Salesforce docs inventory.
- `probes/sfdx`: existing deployable scratch-org probe project.
- `docs/fixtures/oracle`: existing minimal oracle fixtures.
- `.oaer/runs`: existing run artifact convention from CLI DX work.

The new command should make these one workbench.

## Command Surface

Add `oaer compat oracle` as the new front door. Keep `oaer compat oracle-tests` as a compatibility alias for the old test-comparison path.

```bash
oaer compat oracle doctor
oaer compat oracle inventory
oaer compat oracle domains
oaer compat oracle plan
oaer compat oracle generate
oaer compat oracle scripts
oaer compat oracle run-salesforce
oaer compat oracle run-oaer
oaer compat oracle diff
oaer compat oracle promote
oaer compat oracle report
oaer compat oracle next
oaer compat oracle resume
oaer compat oracle check
```

Each subcommand must support `--json`. Long-running subcommands must support `--run-id`, `--runs-dir`, `--area`, `--shard`, `--shard-count`, `--limit`, and `--resume` where the option makes sense.

## Artifact Contract

The artifacts matter as much as the code. They are what keeps week-long runs from turning into smoke.

### Checked generated artifacts

These files are deterministic and may be checked when updated on purpose:

```text
docs/generated/apex-oracle/INVENTORY.json
docs/generated/apex-oracle/DOMAINS.json
docs/generated/apex-oracle/PROBE_MANIFEST.json
docs/generated/apex-oracle/WORK_QUEUE.json
docs/generated/apex-oracle/COVERAGE.json
docs/generated/apex-oracle/KNOWN_ENV_REQUIREMENTS.json
docs/generated/apex-oracle/README.md
```

### Promoted fixtures

These files are reviewed oracle truth:

```text
docs/fixtures/oracle/<area>/<probe-id>.salesforce.json
docs/fixtures/oracle/<area>/<probe-id>.oaer.json
docs/fixtures/oracle/<area>/<probe-id>.diff.json
docs/fixtures/oracle/<area>/baseline.json
```

### Run artifacts

These files are not edited by hand:

```text
.oaer/oracle/runs/<run-id>/manifest.json
.oaer/oracle/runs/<run-id>/ledger.jsonl
.oaer/oracle/runs/<run-id>/work-queue.json
.oaer/oracle/runs/<run-id>/generated/sfdx/force-app/main/default/classes/*.cls
.oaer/oracle/runs/<run-id>/generated/scripts/*.sh
.oaer/oracle/runs/<run-id>/salesforce/raw/*.json
.oaer/oracle/runs/<run-id>/salesforce/logs/*.log
.oaer/oracle/runs/<run-id>/salesforce/observations/*.json
.oaer/oracle/runs/<run-id>/oaer/observations/*.json
.oaer/oracle/runs/<run-id>/diffs/*.json
.oaer/oracle/runs/<run-id>/reports/summary.md
.oaer/oracle/runs/<run-id>/reports/gaps.json
.oaer/oracle/runs/<run-id>/reports/coverage.json
.oaer/oracle/runs/latest.json
```

### Observation shape

Every probe observation must record enough information to rebuild the behavior locally:

```json
{
  "schemaVersion": 1,
  "probeId": "stdlib.System.String.substring.instance.0037",
  "surfaceId": "System.String.substring(Integer,Integer)",
  "source": "salesforce",
  "apiVersion": "61.0",
  "orgShapeId": "enterprise-personaccounts-multicurrency-sites-cache",
  "area": "stdlib.string",
  "mode": "org-diff",
  "className": "OAER_Oracle_stdlib_string_0007",
  "methodName": "probe_0037",
  "apexSourceHash": "sha256:...",
  "parameters": [
    {"name": "receiver", "type": "String", "domain": "ascii_short", "value": "abcdef"},
    {"name": "beginIndex", "type": "Integer", "domain": "zero", "value": 0},
    {"name": "endIndex", "type": "Integer", "domain": "length", "value": 6}
  ],
  "compile": {"accepted": true, "diagnostics": []},
  "runtime": {
    "status": "pass",
    "returnType": "String",
    "returnValue": "abcdef",
    "exceptionType": "",
    "exceptionMessage": ""
  },
  "mutation": {"receiverChanged": false, "parameterChanges": []},
  "events": [],
  "limits": [],
  "sideEffects": [],
  "finalRecords": [],
  "rawArtifacts": []
}
```

If a probe fails to compile, the observation still records the source, surface, parameter domains, diagnostics, and org shape. Compile failures are behavior too.

## What "All Apex" Means In This Plan

The inventory must classify every known public surface into one of these areas:

- `language.syntax`: grammar constructs, expressions, statements, annotations, modifiers, sharing, namespace references, generics, literals, operators.
- `language.types`: primitives, casts, boxing, null, Object, Type, interfaces, enums, exceptions, generics, overload resolution.
- `stdlib.core`: `System`, `String`, `Integer`, `Long`, `Decimal`, `Double`, `Boolean`, `Date`, `Datetime`, `Time`, `Blob`, `Id`, collections, `JSON`, `Pattern`, `Matcher`, `Math`, `Crypto`, `EncodingUtil`.
- `stdlib.platform`: `Database`, `Schema`, `Test`, `Limits`, `Messaging`, `Http`, `RestContext`, `ApexPages`, `PageReference`, `Auth`, `ConnectApi`, `Site`, `Network`, `Cache`, `Metadata`, `System.Callable`, `System.StubProvider`.
- `data.query`: SOQL, aggregate SOQL, relationship queries, dynamic SOQL, binds, null ordering, query exceptions, SOSL unsupported fences.
- `data.dml`: insert, update, upsert, delete, undelete, merge, allOrNone, partial results, validation errors, duplicate errors, record type behavior.
- `data.triggers`: trigger context maps, operation flags, order of execution, recursion, addError, old/new values, transaction rollback.
- `tests`: `@IsTest`, `@testSetup`, `System.assert*`, `Test.startTest`, `Test.stopTest`, async drain, mocks, runAs, test isolation.
- `async`: future, queueable, batch, schedulable, platform events where available.
- `sobjects`: standard SObjects, custom objects, external IDs, relationship fields, polymorphic fields, record types, describe results.
- `metadata`: custom metadata, labels, static resources, custom settings, permissions, layouts, tabs, Visualforce pages, Lightning/Aura import surfaces.
- `server_api`: Salesforce-shaped REST and Tooling behavior that `oaer server` claims.
- `org_shape`: feature flags and edition differences such as Person Accounts, MultiCurrency, Sites, Communities, Platform Cache, State/Country picklists.

Each inventory row must have a status:

```text
unknown
inventory_only
compile_shape_known
runtime_shape_known
salesforce_observed
oaer_matched
oaer_unsupported
env_required
manual_review
```

`unknown` is allowed in the ledger. It is not allowed in a release claim.

## Parameter Domain Strategy

A single happy-path call is a dull axe. Every callable member needs generated parameter domains.

### Domain rows

Create `docs/generated/apex-oracle/DOMAINS.json` from code, not by hand. It defines reusable value sets by Apex type and behavior purpose.

Examples:

```json
{
  "String": [
    {"id": "null", "apex": "null", "meaning": "null input"},
    {"id": "empty", "apex": "''", "meaning": "empty string"},
    {"id": "blank", "apex": "' '", "meaning": "single blank"},
    {"id": "ascii_short", "apex": "'abcdef'", "meaning": "ordinary short value"},
    {"id": "with_newline", "apex": "'a\\nb'", "meaning": "escaped newline"},
    {"id": "numeric_text", "apex": "'123.45'", "meaning": "parse candidate"},
    {"id": "bad_numeric_text", "apex": "'abc'", "meaning": "parse failure candidate"}
  ],
  "Integer": [
    {"id": "null", "apex": "null"},
    {"id": "zero", "apex": "0"},
    {"id": "one", "apex": "1"},
    {"id": "negative_one", "apex": "-1"},
    {"id": "max", "apex": "2147483647"},
    {"id": "min", "apex": "-2147483648"}
  ],
  "List<T>": [
    {"id": "null", "apex": "null"},
    {"id": "empty", "apex": "new List<T>()"},
    {"id": "one", "apex": "new List<T>{T_VALUE_1}"},
    {"id": "with_null", "apex": "new List<T>{null}"},
    {"id": "many", "apex": "new List<T>{T_VALUE_1,T_VALUE_2,T_VALUE_3}"}
  ]
}
```

Use pairwise generation for multi-parameter methods. Use targeted full Cartesian only for small domains and known-sensitive APIs. Record the generation strategy on each probe so the run can be expanded later.

### Required parameter behaviors

For every method or constructor, generate probes for:

- compile acceptance for each overload.
- static and instance dispatch where applicable.
- null receiver when syntax permits dynamic paths.
- null parameters.
- empty values.
- boundary numeric and date values.
- invalid type or invalid value when Salesforce reports a stable exception.
- polymorphic SObject values.
- inserted versus unsaved SObject values.
- collection mutation after call.
- return type and serialized return value.
- exception type, message, and catchability.
- limit increments where Salesforce exposes them.
- side effects and final record state for DML, email, file, async, cache, page, and context APIs.

## Generated Apex Probe Pattern

Generated probes must be Apex test classes, not only anonymous Apex. Anonymous Apex stays useful for fast shape checks. Test classes give isolation, rollback, `startTest`, `stopTest`, async drain, DML, SOQL, triggers, and stable logs.

Each generated class contains many small probe methods. Each method emits a structured marker:

```apex
System.debug(LoggingLevel.ERROR, 'OAER_ORACLE ' + JSON.serialize(payload));
```

For tests where `System.debug` does not fire after a fatal path, the generated probe should wrap the target call and record exception type and message before asserting the marker exists.

A generated probe method should follow this shape:

```apex
@IsTest
private class OAER_Oracle_stdlib_string_0007 {
    @IsTest
    static void probe_0037() {
        Map<String, Object> payload = new Map<String, Object>();
        payload.put('probeId', 'stdlib.System.String.substring.instance.0037');
        payload.put('surfaceId', 'System.String.substring(Integer,Integer)');
        payload.put('parameters', new List<Object>{'abcdef', 0, 6});
        try {
            String receiver = 'abcdef';
            Object resultValue = receiver.substring(0, 6);
            payload.put('status', 'pass');
            payload.put('returnType', 'String');
            payload.put('returnValue', resultValue);
            payload.put('exceptionType', null);
            payload.put('exceptionMessage', null);
        } catch (Exception ex) {
            payload.put('status', 'exception');
            payload.put('returnType', null);
            payload.put('returnValue', null);
            payload.put('exceptionType', ex.getTypeName());
            payload.put('exceptionMessage', ex.getMessage());
        }
        System.debug(LoggingLevel.ERROR, 'OAER_ORACLE ' + JSON.serialize(payload));
    }
}
```

The generator owns this pattern. Agents do not write these by hand.

## Script Factory

The command must generate shell scripts for long runs. These scripts save time and tokens. An agent should run a script and read one compact report, not reason through thousands of CLI calls.

```bash
oaer compat oracle scripts \
  --work-queue docs/generated/apex-oracle/WORK_QUEUE.json \
  --runs-dir .oaer/oracle/runs \
  --run-id full-2026-05-22 \
  --target-org oaer-probe-lab \
  --shard-count 128 \
  --output .oaer/oracle/runs/full-2026-05-22/generated/scripts
```

Generated scripts:

```text
00-doctor.sh
01-inventory.sh
02-domains.sh
03-plan.sh
04-generate-shard-000.sh
05-deploy-shard-000.sh
06-run-salesforce-shard-000.sh
07-run-oaer-shard-000.sh
08-diff-shard-000.sh
09-report-shard-000.sh
resume-failed.sh
resume-infra-errors.sh
promote-passing.sh
nightly-full.sh
next-agent-batch.sh
```

Every script must be idempotent. Every script must write its own status row to `ledger.jsonl`. Every script must exit non-zero on infrastructure failure and leave a resumable work item.

## Work Queue Model

The work queue is the center beam.

Each work item should look like this:

```json
{
  "id": "work.stdlib.string.0007",
  "area": "stdlib.string",
  "shard": 7,
  "priority": 10,
  "mode": "org-diff",
  "surfaceIds": ["System.String.substring(Integer,Integer)"],
  "probeIds": ["stdlib.System.String.substring.instance.0037"],
  "generatedClass": "OAER_Oracle_stdlib_string_0007",
  "envRequirements": [],
  "status": "planned",
  "attempts": 0,
  "lastError": "",
  "artifacts": []
}
```

Statuses:

```text
planned
generated
deployed
salesforce_running
salesforce_done
oaer_done
diff_done
promoted
blocked_env
blocked_compile
blocked_infra
blocked_unsupported
```

The command must update work items without losing prior raw evidence.

## Report Shape

The report must answer three questions without reading raw logs:

1. What Apex surfaces do we know?
2. What did Salesforce do?
3. What does OAER still need to match?

Required report sections:

- Inventory totals by area and status.
- Probe totals by area, mode, and shard.
- Salesforce observation totals by status.
- OAER comparison totals by outcome.
- Unknown public surfaces.
- Environment-required surfaces.
- Top mismatch families with one minimal repro each.
- Suggested owner package: parser, sema, VM, SOQL, DML, storage, schema, apxtest, server, stdlib, docs.
- Next command to run.
- Next agent batch of no more than 25 compact tasks.

`oaer compat oracle next --limit 25 --json` should be the token-saving handle for future agents.

## File Structure

### New package files

```text
internal/oracle/inventory.go
internal/oracle/inventory_test.go
internal/oracle/domain.go
internal/oracle/domain_test.go
internal/oracle/probe_spec.go
internal/oracle/probe_spec_test.go
internal/oracle/plan.go
internal/oracle/plan_test.go
internal/oracle/generate.go
internal/oracle/generate_test.go
internal/oracle/scripts.go
internal/oracle/scripts_test.go
internal/oracle/ledger.go
internal/oracle/ledger_test.go
internal/oracle/promote.go
internal/oracle/promote_test.go
internal/oracle/coverage.go
internal/oracle/coverage_test.go
internal/oracle/next.go
internal/oracle/next_test.go
```

### Existing package files to extend

```text
internal/oracle/model.go
internal/oracle/normalize.go
internal/oracle/diff.go
internal/oracle/salesforce_runner.go
internal/oracle/local_runner.go
internal/oracle/report.go
internal/oaercli/cli.go
internal/oaercli/compat_oracle.go
internal/oaercli/cli_test.go
internal/probe/stub_contract_probe.go
internal/capability/stub_contracts.go
internal/capability/salesforce_coverage.go
internal/capability/catalog.go
```

### Generated or fixture paths

```text
docs/generated/apex-oracle/README.md
docs/generated/apex-oracle/INVENTORY.json
docs/generated/apex-oracle/DOMAINS.json
docs/generated/apex-oracle/PROBE_MANIFEST.json
docs/generated/apex-oracle/WORK_QUEUE.json
docs/generated/apex-oracle/COVERAGE.json
docs/fixtures/oracle/<area>/baseline.json
probes/oracle/templates/ProbeClass.cls.tmpl
probes/oracle/templates/ProbeRouter.cls.tmpl
```

## Implementation Tasks

### Task 1: Add The `compat oracle` Dispatcher

**Files:**

- Modify: `internal/oaercli/cli.go`
- Modify: `internal/oaercli/compat_oracle.go`
- Modify: `internal/oaercli/cli_test.go`

- [ ] Add `oracle` to the `compat` subcommand switch.
- [ ] Keep `oracle-tests` as an alias to the existing comparison command.
- [ ] Add help text for every new subcommand.
- [ ] Add tests that `oaer compat oracle --help` lists `inventory`, `generate`, `scripts`, `run-salesforce`, `run-oaer`, `diff`, `report`, `next`, `resume`, and `check`.
- [ ] Add tests that `oaer compat oracle-tests --check docs/fixtures/oracle/fixture-corpus.json --json` still works.

Validation:

```bash
go test -count=1 ./internal/oaercli -run 'TestRunCompatOracle|TestRunCompatHelp'
```

Expected: pass.

### Task 2: Build The Unified Apex Inventory

**Files:**

- Create: `internal/oracle/inventory.go`
- Create: `internal/oracle/inventory_test.go`
- Modify: `internal/capability/catalog.go`
- Modify: `internal/capability/stub_inventory.go`
- Modify: `internal/capability/stub_contracts.go`

- [ ] Define `ApexSurface`, `ApexMember`, `ApexParameter`, `ApexSourceRef`, and `Inventory` in `internal/oracle`.
- [ ] Merge docs catalog entries, stub inventory rows, stub contract rows, standard object coverage, current capability rows, and existing oracle fixtures.
- [ ] Give every surface a stable `surfaceId`.
- [ ] Classify every row by area, kind, namespace, type, member, signature, source, and current evidence status.
- [ ] Emit deterministic JSON sorted by `surfaceId`.
- [ ] Mark conflicting source facts as `manual_review`, not as support.

Command shape:

```bash
go run ./cmd/oaer compat oracle inventory \
  --docs-source "/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper (1)/salesforce-docs" \
  --stubs example-projects/stubs \
  --output docs/generated/apex-oracle/INVENTORY.json
```

Validation:

```bash
go test -count=1 ./internal/oracle ./internal/capability -run 'Inventory|Stub|Catalog'
go run ./cmd/oaer compat oracle inventory --json | head -c 4000
```

Expected: JSON includes `schemaVersion`, `surfaces`, `summary`, and no duplicate `surfaceId` values.

### Task 3: Add Parameter Domain Generation

**Files:**

- Create: `internal/oracle/domain.go`
- Create: `internal/oracle/domain_test.go`
- Create: `docs/generated/apex-oracle/DOMAINS.json`

- [ ] Define reusable domain values for primitive Apex types.
- [ ] Define domain templates for `List<T>`, `Set<T>`, `Map<K,V>`, `SObject`, enums, exceptions, dates, datetimes, blobs, IDs, and HTTP-ish values.
- [ ] Add pairwise generation for multi-parameter methods.
- [ ] Add targeted Cartesian expansion for small sensitive APIs such as string indexing, numeric parsing, date construction, collection indexing, and DML options.
- [ ] Record domain IDs and Apex expressions on every generated invocation.
- [ ] Add tests for stable domain output and bounded pairwise counts.

Command shape:

```bash
go run ./cmd/oaer compat oracle domains --output docs/generated/apex-oracle/DOMAINS.json
```

Validation:

```bash
go test -count=1 ./internal/oracle -run 'Domain|Pairwise'
go run ./cmd/oaer compat oracle domains --check docs/generated/apex-oracle/DOMAINS.json
```

Expected: deterministic domains and no unbounded Cartesian explosion.

### Task 4: Generate Deep Probe Specs

**Files:**

- Create: `internal/oracle/probe_spec.go`
- Create: `internal/oracle/probe_spec_test.go`
- Create: `internal/oracle/plan.go`
- Create: `internal/oracle/plan_test.go`
- Create: `docs/generated/apex-oracle/PROBE_MANIFEST.json`
- Create: `docs/generated/apex-oracle/WORK_QUEUE.json`

- [ ] Convert inventory surfaces into probe specs.
- [ ] Generate compile-shape probes for every public type and member.
- [ ] Generate runtime org-diff probes for methods, constructors, properties, operators, SOQL, DML, trigger, and test-runner surfaces.
- [ ] Generate passive DTO probes only for known passive objects where behavior is construction, property, getter, setter, or serialization shape.
- [ ] Generate explicit unsupported probes for cloud-only or environment-missing APIs.
- [ ] Generate env-required probes when a feature-specific scratch org is needed.
- [ ] Split probes into shards by area, estimated runtime, and Apex class size.
- [ ] Add a first-day `--area stdlib.string --limit 200` plan that proves the pipeline.

Command shape:

```bash
go run ./cmd/oaer compat oracle plan \
  --inventory docs/generated/apex-oracle/INVENTORY.json \
  --domains docs/generated/apex-oracle/DOMAINS.json \
  --area stdlib.string \
  --limit 200 \
  --manifest docs/generated/apex-oracle/PROBE_MANIFEST.json \
  --work-queue docs/generated/apex-oracle/WORK_QUEUE.json
```

Validation:

```bash
go test -count=1 ./internal/oracle -run 'ProbeSpec|Plan|WorkQueue'
go run ./cmd/oaer compat oracle plan --inventory docs/generated/apex-oracle/INVENTORY.json --domains docs/generated/apex-oracle/DOMAINS.json --area stdlib.string --limit 20 --json
```

Expected: each planned probe has `probeId`, `surfaceId`, `mode`, `parameters`, `area`, and shard assignment.

### Task 5: Generate Apex Classes And Router

**Files:**

- Create: `internal/oracle/generate.go`
- Create: `internal/oracle/generate_test.go`
- Create: `probes/oracle/templates/ProbeClass.cls.tmpl`
- Create: `probes/oracle/templates/ProbeRouter.cls.tmpl`

- [ ] Generate SFDX source under `.oaer/oracle/runs/<run-id>/generated/sfdx`.
- [ ] Generate Apex classes small enough for Salesforce limits.
- [ ] Generate one method per probe or compact grouped methods when safe.
- [ ] Emit `OAER_ORACLE` markers for result, exception, mutation, side effect, and final state payloads.
- [ ] Generate setup helpers for SObject, DML, SOQL, trigger, async, and metadata probes.
- [ ] Generate router classes only where batch execution needs them.
- [ ] Preserve raw generated Apex source hashes in the manifest.

Command shape:

```bash
go run ./cmd/oaer compat oracle generate \
  --work-queue docs/generated/apex-oracle/WORK_QUEUE.json \
  --run-id smoke-string \
  --runs-dir .oaer/oracle/runs \
  --area stdlib.string
```

Validation:

```bash
go test -count=1 ./internal/oracle -run 'Generate'
find .oaer/oracle/runs/smoke-string/generated/sfdx -type f -name '*.cls' | head -c 4000
```

Expected: generated Apex files exist and include `OAER_ORACLE` markers.

### Task 6: Generate The Script Factory

**Files:**

- Create: `internal/oracle/scripts.go`
- Create: `internal/oracle/scripts_test.go`

- [ ] Generate idempotent shell scripts for each shard.
- [ ] Make scripts call `oaer compat oracle` subcommands instead of duplicating logic.
- [ ] Add `resume-failed.sh`, `resume-infra-errors.sh`, `promote-passing.sh`, `nightly-full.sh`, and `next-agent-batch.sh`.
- [ ] Add script headers that record command, cwd, `oaer` commit, target org, run id, and shard.
- [ ] Use byte-capped log previews in generated scripts.
- [ ] Make every script write a ledger row.

Command shape:

```bash
go run ./cmd/oaer compat oracle scripts \
  --work-queue docs/generated/apex-oracle/WORK_QUEUE.json \
  --run-id smoke-string \
  --runs-dir .oaer/oracle/runs \
  --target-org oaer-probe-lab \
  --shard-count 4 \
  --output .oaer/oracle/runs/smoke-string/generated/scripts
```

Validation:

```bash
go test -count=1 ./internal/oracle -run 'Scripts'
bash -n .oaer/oracle/runs/smoke-string/generated/scripts/*.sh
```

Expected: scripts parse and contain only command calls plus logging.

### Task 7: Run Salesforce Shards And Capture Under-The-Hood Evidence

**Files:**

- Modify: `internal/oracle/salesforce_runner.go`
- Create: `internal/oracle/ledger.go`
- Create: `internal/oracle/ledger_test.go`

- [ ] Deploy generated SFDX classes by shard.
- [ ] Run generated tests by class or method.
- [ ] Fetch test results, Apex logs, compile diagnostics, and debug markers.
- [ ] Parse `OAER_ORACLE` markers into observations.
- [ ] Parse selected logs into method, SOQL, DML, trigger, exception, limit, and debug events when logs are enabled.
- [ ] Save raw Salesforce responses, raw logs, normalized observations, and ledger rows.
- [ ] Mark infrastructure failures separately from Salesforce behavior.
- [ ] Add `--fetch-logs`, `--log-level`, `--log-limit`, and `--wait` controls.

Command shape:

```bash
go run ./cmd/oaer compat oracle run-salesforce \
  --run-id smoke-string \
  --runs-dir .oaer/oracle/runs \
  --target-org oaer-probe-lab \
  --area stdlib.string \
  --shard 0 \
  --fetch-logs \
  --log-limit 200
```

Validation:

```bash
go test -count=1 ./internal/oracle -run 'Salesforce|Ledger'
```

Expected: unit tests pass. Live command writes observations when a scratch org is configured.

### Task 8: Run OAER Against The Same Probe Specs

**Files:**

- Modify: `internal/oracle/local_runner.go`
- Modify: `internal/apextest`
- Modify: `internal/vm`

- [ ] Run generated probe classes through OAER using the same source and parameters.
- [ ] Capture compile diagnostics in the same observation shape.
- [ ] Capture runtime return values, exceptions, debug payloads, side effects, limits, and final records.
- [ ] Add VM trace hooks only where needed for oracle observations.
- [ ] Preserve unsupported diagnostics as first-class observations.
- [ ] Save observations under `.oaer/oracle/runs/<run-id>/oaer/observations`.

Command shape:

```bash
go run ./cmd/oaer compat oracle run-oaer \
  --run-id smoke-string \
  --runs-dir .oaer/oracle/runs \
  --area stdlib.string \
  --shard 0
```

Validation:

```bash
go test -count=1 ./internal/oracle ./internal/apextest ./internal/vm -run 'Local|Oracle|Debug|Trace'
```

Expected: local observations use the same `probeId` and `surfaceId` values as Salesforce observations.

### Task 9: Diff, Classify, And Promote Evidence

**Files:**

- Modify: `internal/oracle/diff.go`
- Modify: `internal/oracle/normalize.go`
- Create: `internal/oracle/promote.go`
- Create: `internal/oracle/promote_test.go`

- [ ] Normalize IDs, timestamps, generated usernames, org IDs, async IDs, log IDs, stack line noise, and unordered map output.
- [ ] Diff compile behavior, return type, return value, exception type, exception message, mutation, events, limits, side effects, and final state.
- [ ] Classify mismatches by owner package.
- [ ] Promote passing or accepted unsupported observations into `docs/fixtures/oracle/<area>`.
- [ ] Refuse promotion when raw Salesforce evidence is missing.
- [ ] Refuse promotion when the Salesforce run has infrastructure errors.

Command shape:

```bash
go run ./cmd/oaer compat oracle diff \
  --run-id smoke-string \
  --runs-dir .oaer/oracle/runs \
  --area stdlib.string \
  --shard 0 \
  --json

go run ./cmd/oaer compat oracle promote \
  --run-id smoke-string \
  --runs-dir .oaer/oracle/runs \
  --area stdlib.string \
  --only pass
```

Validation:

```bash
go test -count=1 ./internal/oracle -run 'Diff|Normalize|Promote'
go run ./cmd/oaer compat oracle check --fixtures docs/fixtures/oracle --json
```

Expected: promoted fixtures are deterministic and checkable.

### Task 10: Add Reports And Token-Saving Next Work

**Files:**

- Modify: `internal/oracle/report.go`
- Create: `internal/oracle/coverage.go`
- Create: `internal/oracle/coverage_test.go`
- Create: `internal/oracle/next.go`
- Create: `internal/oracle/next_test.go`

- [ ] Build coverage from inventory, probe manifest, Salesforce observations, OAER observations, and diffs.
- [ ] Report unknown, compile-shape-known, Salesforce-observed, OAER-matched, OAER-unsupported, env-required, and manual-review counts.
- [ ] Group failures into mismatch families.
- [ ] Emit a compact next-work batch with probe ID, minimal Apex repro, expected Salesforce observation, OAER actual observation, likely owner, and exact validation command.
- [ ] Keep `next` output capped and useful for agent work.

Command shape:

```bash
go run ./cmd/oaer compat oracle report \
  --run-id smoke-string \
  --runs-dir .oaer/oracle/runs \
  --output .oaer/oracle/runs/smoke-string/reports/summary.md

go run ./cmd/oaer compat oracle next \
  --run-id smoke-string \
  --runs-dir .oaer/oracle/runs \
  --limit 25 \
  --json
```

Validation:

```bash
go test -count=1 ./internal/oracle -run 'Report|Coverage|Next'
```

Expected: report names the next command and no raw log dump is needed for normal triage.

### Task 11: Add Resume And Check Gates

**Files:**

- Modify: `internal/oracle/ledger.go`
- Modify: `internal/oaercli/compat_oracle.go`
- Modify: `internal/oaercli/cli_test.go`

- [ ] Make `resume` read `ledger.jsonl` and `work-queue.json`.
- [ ] Resume planned, generated, infrastructure-failed, and incomplete shards.
- [ ] Avoid rerunning promoted passing observations unless `--force` is passed.
- [ ] Make `check` verify generated docs, fixtures, and promoted baselines.
- [ ] Make `check` fail if a `supported` capability lacks evidence.

Command shape:

```bash
go run ./cmd/oaer compat oracle resume \
  --run-id full-2026-05-22 \
  --runs-dir .oaer/oracle/runs \
  --target-org oaer-probe-lab

go run ./cmd/oaer compat oracle check \
  --inventory docs/generated/apex-oracle/INVENTORY.json \
  --manifest docs/generated/apex-oracle/PROBE_MANIFEST.json \
  --fixtures docs/fixtures/oracle \
  --json
```

Validation:

```bash
go test -count=1 ./internal/oracle ./internal/oaercli -run 'Resume|Check|Oracle'
```

Expected: interrupted runs can continue without hand cleanup.

## First Day Cut

The first working cut should not wait for all Apex. It should prove the factory with a deep but bounded area.

Run this sequence:

```bash
go run ./cmd/oaer compat oracle doctor --json

go run ./cmd/oaer compat oracle inventory \
  --stubs example-projects/stubs \
  --output docs/generated/apex-oracle/INVENTORY.json

go run ./cmd/oaer compat oracle domains \
  --output docs/generated/apex-oracle/DOMAINS.json

go run ./cmd/oaer compat oracle plan \
  --inventory docs/generated/apex-oracle/INVENTORY.json \
  --domains docs/generated/apex-oracle/DOMAINS.json \
  --area stdlib.string \
  --limit 200 \
  --manifest docs/generated/apex-oracle/PROBE_MANIFEST.json \
  --work-queue docs/generated/apex-oracle/WORK_QUEUE.json

go run ./cmd/oaer compat oracle generate \
  --work-queue docs/generated/apex-oracle/WORK_QUEUE.json \
  --run-id smoke-string \
  --runs-dir .oaer/oracle/runs \
  --area stdlib.string

go run ./cmd/oaer compat oracle scripts \
  --work-queue docs/generated/apex-oracle/WORK_QUEUE.json \
  --run-id smoke-string \
  --runs-dir .oaer/oracle/runs \
  --target-org oaer-probe-lab \
  --shard-count 4 \
  --output .oaer/oracle/runs/smoke-string/generated/scripts
```

Then run one generated shard against Salesforce and OAER:

```bash
bash .oaer/oracle/runs/smoke-string/generated/scripts/06-run-salesforce-shard-000.sh
bash .oaer/oracle/runs/smoke-string/generated/scripts/07-run-oaer-shard-000.sh
bash .oaer/oracle/runs/smoke-string/generated/scripts/08-diff-shard-000.sh
bash .oaer/oracle/runs/smoke-string/generated/scripts/09-report-shard-000.sh
```

Success means the artifact tree contains raw Salesforce evidence, OAER evidence, normalized observations, diffs, ledger rows, and a compact next-work report.

## Expansion Order

After the first cut, widen in this order:

1. `stdlib.string`, `stdlib.integer`, `stdlib.decimal`, `stdlib.date`, `stdlib.datetime`, `stdlib.collections`.
2. `language.types`, `language.syntax`, overload dispatch, casts, null behavior, exceptions.
3. `data.soql`, `data.dml`, `data.triggers`.
4. `tests.lifecycle`, `tests.async`, `tests.mocks`, `tests.runAs`.
5. `schema.describe`, `sobjects.standard`, `sobjects.custom`, `metadata.custom_metadata`, `metadata.labels`, `metadata.resources`.
6. `stdlib.database`, `stdlib.schema`, `stdlib.test`, `stdlib.messaging`, `stdlib.http`.
7. `stdlib.apexpages`, `stdlib.pagereference`, Visualforce controller surfaces.
8. `stdlib.auth`, `stdlib.connectapi`, `stdlib.site`, `stdlib.network`, `stdlib.cache`, `stdlib.metadata`.
9. `server_api.rest`, `server_api.tooling`, local server parity gates.
10. Feature-specific org lanes for Person Accounts, MultiCurrency, Sites, Communities, Platform Cache, State/Country picklists, and other gated shape.

Each expansion starts with inventory and plan. Each expansion ends with report and next-work output.

## Environment Requirements

Some Salesforce behavior depends on org shape. The oracle must record that instead of flattening it.

Use separate org-shape IDs:

```text
base-enterprise
enterprise-personaccounts
enterprise-multicurrency
enterprise-sites-communities
enterprise-platform-cache
enterprise-state-country-picklists
enterprise-full-probe-lab
managed-package-consumer
```

A probe that cannot run in the current org gets `blocked_env`, not `fail`. The report should show the exact scratch org definition or setup command needed.

## Clean-Room Boundary

Allowed evidence:

- Public Salesforce docs.
- Public grammar behavior.
- Scratch-org black-box compile and runtime behavior.
- Tooling API and REST response shapes from owned orgs.
- Owned fixtures and generated probes.
- Example project behavior from local corpora.

Not allowed:

- Proprietary AER internals.
- Guessing behavior from undocumented implementation details.
- Promoting support from a stub shape alone.

## Validation Gates

Fast local gate:

```bash
go test -count=1 ./internal/oracle ./internal/oaercli ./internal/capability ./internal/probe
```

Generated artifact gate:

```bash
go run ./cmd/oaer compat oracle inventory --check docs/generated/apex-oracle/INVENTORY.json
go run ./cmd/oaer compat oracle domains --check docs/generated/apex-oracle/DOMAINS.json
go run ./cmd/oaer compat oracle check --fixtures docs/fixtures/oracle --json
```

Smoke live gate:

```bash
go run ./cmd/oaer compat oracle plan --area stdlib.string --limit 50 --json
go run ./cmd/oaer compat oracle generate --area stdlib.string --run-id oracle-smoke --runs-dir .oaer/oracle/runs
go run ./cmd/oaer compat oracle run-salesforce --area stdlib.string --run-id oracle-smoke --target-org oaer-probe-lab --runs-dir .oaer/oracle/runs
go run ./cmd/oaer compat oracle run-oaer --area stdlib.string --run-id oracle-smoke --runs-dir .oaer/oracle/runs
go run ./cmd/oaer compat oracle diff --area stdlib.string --run-id oracle-smoke --runs-dir .oaer/oracle/runs --json
```

Full long-run gate:

```bash
bash .oaer/oracle/runs/<run-id>/generated/scripts/nightly-full.sh
```

This command may run for weeks. That is fine. The ledger must make it restartable.

## Completion Criteria

The command is ready for real parity work when:

- `oaer compat oracle` exists and `oracle-tests` remains compatible.
- Inventory merges docs, stubs, catalog, standard objects, stub contracts, and current evidence.
- Domains generate parameter sweeps for primitives, collections, SObjects, enums, dates, exceptions, and platform objects.
- Probe manifests generate deep compile and runtime probes, not one happy path.
- Generated Apex classes emit `OAER_ORACLE` payloads with parameter domains and result details.
- Generated scripts can run shards without agent reasoning.
- Salesforce and OAER observations share the same schema.
- Diffs classify owner package and mismatch family.
- Reports show unknown surfaces, observed surfaces, matched surfaces, unsupported surfaces, and env-required surfaces.
- `next` gives compact, actionable work without raw log spelunking.
- Promoted fixtures contain raw Salesforce evidence or a pointer to the run artifact that contains it.

## Risks

- Salesforce API limits can slow full runs. Shard and resume from the ledger.
- Some APIs require special org setup. Mark `env_required` and keep separate org-shape lanes.
- Logs can be too large. Parse `OAER_ORACLE` markers first and fetch deep logs only for selected shards.
- Generated Apex can hit class limits. Keep classes small and shard by area.
- Pairwise generation can miss a multi-parameter edge. Store discovered edges back into domains and rerun affected probes.
- Passive DTO behavior can hide real behavior. Use passive mode only when Salesforce observation proves it or the contract is explicitly local-only.

## Working Rule For Agents

Do not spend tokens designing one-off probes when the generator can make them.

Use this loop:

```bash
go run ./cmd/oaer compat oracle next --run-id <run-id> --limit 25 --json
```

Pick one mismatch family. Patch the smallest runtime path. Run the exact validation command from `next`. Re-run the shard. Let the ledger record the new state.

Not bad when the tool does the carrying.
