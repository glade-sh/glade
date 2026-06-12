# Local Apex and LWC Best Next Packets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. The coordinating agent should also use dispatching-parallel-agents and give each implementer a separate worktree.

**Goal:** Deepen the remaining high-value `partial` standard-library surfaces that local Apex tests, `@AuraEnabled` controllers, Visualforce pages, and LWC wire paths can use without contacting Salesforce services.

**Architecture:** Split work into independent product lanes. Each lane owns runtime code and focused tests in `glade`. One integration lane updates `/Users/matt/Dev/glade-tools` catalog rows, fixtures, and generated docs after product lanes merge. Keep the five true live-service rows unsupported.

**Tech Stack:** Go 1.26, Glade VM/runtime under `internal/vm`, local org storage under `internal/storage`, Visualforce/LWC browser support under `internal/visualforce`, `internal/lwc`, and `internal/lwcbrowser`, first-party compatibility tooling under `/Users/matt/Dev/glade-tools`.

---

## Baseline

Current `docs/STDLIB_COVERAGE.md` on `main` has:

- `supported`: 181
- `partial`: 78
- `unsupported`: 5
- `unknown`: 1

The five unsupported rows stay fenced:

- `Answers.findSimilar(Question)`
- `ResetPasswordResult.getPassword()`
- `TrailblazerIdentity.generateUserEmailVerificationToken(String,String,String)`
- `TrailblazerIdentity.getUserOrgInfo(List<String>)`
- `TrailblazerIdentity.splunkLog(String,String)`

Do not fake these with DTOs or no-op support. They require live identity, Trailblazer, Answers, or password-reset service behavior.

## Why These Packets Come First

These packets help local Apex running before they help the support table:

- LWC Apex wire calls serialize and deserialize controller DTOs.
- LWC record wires and Apex controllers lean on schema describe, field metadata, labels, and static resources.
- Controllers often use SOSL and `Search.find` for lookup/autocomplete paths.
- Visualforce and Lightning Out pages share page reference, message, and controller invocation state.
- Local test suites use async, event, continuation, and external-service hooks as test scaffolding.

These packets should move rows only when behavior improved. Some rows may stay `partial` with sharper notes. That is acceptable. The contract is useful runtime behavior plus honest coverage, not a prettier table.

## Parallel Worktree Setup

Run from `/Users/matt/Dev/glade`:

```bash
git worktree add -b codex/lwc-json-dto ../glade-lwc-json-dto HEAD
git worktree add -b codex/lwc-schema-permissions ../glade-lwc-schema-permissions HEAD
git worktree add -b codex/lwc-search-sosl ../glade-lwc-search-sosl HEAD
git worktree add -b codex/lwc-vf-bridge ../glade-lwc-vf-bridge HEAD
git worktree add -b codex/lwc-async-harness ../glade-lwc-async-harness HEAD
git worktree add -b codex/lwc-stdlib-integration ../glade-lwc-stdlib-integration HEAD
```

Expected: six worktrees are created and each has a `codex/` branch.

Each subagent must:

- Use GPT-5.5 medium if available. If not, use the strongest GPT-5 coding agent available.
- Work only inside its assigned worktree.
- Read this whole plan, then only the files named in its lane unless blocked.
- Add or strengthen tests before implementation.
- Avoid generated docs unless assigned to the integration lane.
- Return changed files, exact tests run, row/status recommendations, and remaining gaps.

The coordinator merges product lanes first. Then the integration lane updates `glade-tools` and generated docs.

## Phase 0: Baseline Checks

**Files:**

- Read: `docs/STDLIB_COVERAGE.md`
- Read: `/Users/matt/Dev/glade-tools/internal/capability/stdlib.go`
- Read: `/Users/matt/Dev/glade-tools/internal/capability/capability_test.go`

- [ ] **Step 1: Record current stdlib counts**

Run:

```bash
awk -F'|' 'NR>8 && /^\|/ {
  status=$4
  gsub(/`/,"",status)
  gsub(/^[ \t]+|[ \t]+$/,"",status)
  counts[status]++
}
END {for (s in counts) print s, counts[s]}' docs/STDLIB_COVERAGE.md | sort
```

Expected on this baseline:

```text
partial 78
supported 181
unknown 1
unsupported 5
```

- [ ] **Step 2: Record partial and unsupported rows**

Run:

```bash
awk -F'|' 'NR>8 && /^\|/ {
  area=$2; api=$3; status=$4; notes=$5
  gsub(/^[ \t]+|[ \t]+$/,"",area)
  gsub(/^[ \t]+|[ \t]+$/,"",api)
  gsub(/`/,"",api)
  gsub(/`/,"",status)
  gsub(/^[ \t]+|[ \t]+$/,"",status)
  gsub(/^[ \t]+|[ \t]+$/,"",notes)
  if (status=="unsupported" || status=="partial") print status "\t" area "\t" api "\t" notes
}' docs/STDLIB_COVERAGE.md > /tmp/glade-stdlib-partial-unsupported-before.tsv
```

Expected: `/tmp/glade-stdlib-partial-unsupported-before.tsv` has 83 rows.

- [ ] **Step 3: Confirm generated docs match the catalog**

Run from `/Users/matt/Dev/glade-tools`:

```bash
go run ./cmd/glade-tools stdlib --check ../glade/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools dashboard --check ../glade/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools gaps --check ../glade/docs/KNOWN_GAPS.md
```

Expected:

```text
../glade/docs/STDLIB_COVERAGE.md: up to date
../glade/docs/COMPATIBILITY_DASHBOARD.md: up to date
../glade/docs/KNOWN_GAPS.md: up to date
```

- [ ] **Step 4: Dispatch the five product lanes**

Give each subagent one lane below. Do not dispatch the integration lane until product lanes have merged.

## Phase 1A: JSON DTO and Type Construction Packet

**Subagent lane:** `../glade-lwc-json-dto`

**Local value:** LWC Apex wires and imperative Apex calls pass JSON across the browser/server boundary. Local Apex tests also use `Type.forName(...).newInstance()` in factory-heavy code.

**Rows touched:**

- `JSON.deserialize`
- `JSON.deserializeStrict`
- `JSON.deserializeUntyped`
- `JSON.serialize`
- `JSON.serializePretty`
- `Type.forName`
- `Type.newInstance`

**Target status:** Keep JSON rows `partial` unless every tested edge in the row notes is covered. Move `Type.newInstance` from `partial` to `supported` only if zero-arg constructor dispatch parity lands and built-in rejection stays explicit.

**Files:**

- Modify: `internal/vm/json_runtime.go`
- Modify: `internal/vm/json_test.go`
- Modify: `internal/vm/construct_runtime.go`
- Modify: `internal/vm/method_dispatch.go`
- Modify: `internal/vm/type_coercion.go`
- Modify: `internal/vm/stdlib_test.go`
- Inspect and modify when SObject DTO coverage owns the failing case: `internal/vm/data_test.go`

- [ ] **Step 1: Add failing DTO coverage**

Add tests to `internal/vm/json_test.go`:

```apex
public class LwcDTO {
    public String name;
    public Status status;
    public List<Row> rows;
    public Map<String, Object> extra;
    public enum Status { Draft, Ready }
    public class Row {
        public String label;
        public Integer count;
    }
}

LwcDTO dto = (LwcDTO)JSON.deserialize(
    '{"name":"Widget","status":"Ready","rows":[{"label":"A","count":2}],"extra":{"ok":true}}',
    LwcDTO.class
);
System.assertEquals('Widget', dto.name);
System.assertEquals(LwcDTO.Status.Ready, dto.status);
System.assertEquals('A', dto.rows[0].label);
System.assertEquals(2, dto.rows[0].count);
System.assertEquals(true, dto.extra.get('ok'));
System.assert(JSON.serialize(dto).contains('"status":"Ready"'));
```

Expected before implementation: at least one enum, inner-class, or object-map assertion fails.

- [ ] **Step 2: Add failing Type constructor coverage**

Add tests to `internal/vm/stdlib_test.go`:

```apex
public class FactoryTarget {
    public String marker;
    public FactoryTarget() {
        marker = 'constructed';
    }
}

FactoryTarget made = (FactoryTarget)Type.forName('FactoryTarget').newInstance();
System.assertEquals('constructed', made.marker);
```

Also keep this rejection:

```apex
Type.forName('String').newInstance();
```

Expected rejection:

```text
unsupported call "Type.newInstance uninstantiable built-in String"
```

- [ ] **Step 3: Run focused tests and capture failures**

Run:

```bash
go test ./internal/vm -run 'TestExec(JSONDeserialize|JSONSerialize|TypeForName|TypeNewInstance|TypeForNameCreatesGenericCollections)'
```

Expected before implementation: the new tests fail and older tests still compile.

- [ ] **Step 4: Implement the DTO mapping**

In `internal/vm/json_runtime.go`:

- Resolve nested Apex class names through the same class lookup path used by normal construction.
- Map enum JSON strings to enum values when the target field type is an Apex enum.
- Preserve strict-mode unknown-field rejection for class targets.
- Preserve current SObject relationship behavior.
- Preserve `JSON.deserializeUntyped` primitive/list/map behavior.

In `internal/vm/type_coercion.go`:

- Keep enum coercion rules shared between JSON assignment and ordinary Apex assignment.
- Reject unsupported map key targets with the existing `JSONException` path.

- [ ] **Step 5: Implement constructor dispatch for Type.newInstance**

In `internal/vm/construct_runtime.go`:

- Keep built-in scalar, platform DTO, abstract, interface, and unknown-type fences.
- After allocating a local class object for `Type.newInstance`, invoke the zero-argument constructor through the same path used by `new ClassName()`.
- Do not invent parameterized-constructor dispatch.
- Do not run constructors for SObject values, list values, map values, or platform DTO shells.

- [ ] **Step 6: Verify this lane**

Run:

```bash
go test ./internal/vm -run 'TestExec(JSONDeserialize|JSONSerialize|JSONMalformed|TypeForName|TypeNewInstance|TypeForNameCreatesGenericCollections)'
go test ./internal/vm -run TestExecLwcControllerDTOs
go test ./internal/apextest -run 'Test.*(UIController|AuraEnabled|LocalApex)'
git diff --check
```

Expected: all commands exit 0. If `TestExecLwcControllerDTOs` does not exist yet, create it or fold the LWC DTO case into the nearest JSON test and report the exact test name.

**Done report:**

- Changed files.
- Which JSON edges now work.
- Whether `Type.newInstance` can move to `supported`.
- Any JSON row notes that must remain `partial`.

## Phase 1B: Schema, Permissions, and LWC Record Wire Packet

**Subagent lane:** `../glade-lwc-schema-permissions`

**Local value:** LWC record wire shims and Apex controllers need object info, field metadata, picklists, CRUD/FLS booleans, and local permission checks.

**Rows touched:**

- `AccessLevel.withPermissionSetId(String)`
- `FeatureManagement.checkPermission`
- `Schema.getGlobalDescribe()`
- `Schema.describeSObjects(List<String>)`
- `Schema.DescribeSObjectResult`
- `Schema.DescribeFieldResult`

**Target status:** Keep schema rows `partial` unless metadata-backed picklists, record types, object flags, and access booleans are complete for local metadata. Move `AccessLevel.withPermissionSetId` only if SOQL and DML both prove scoped permission behavior.

**Files:**

- Modify: `internal/vm/describe_runtime.go`
- Modify: `internal/vm/platform_passive_members.go`
- Modify: `internal/vm/permissions_runtime.go`
- Modify: `internal/vm/soql_runtime.go`
- Modify: `internal/vm/dispatch.go`
- Modify: `internal/vm/data_test.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/lwcbrowser/wire.go`
- Modify: `internal/lwcbrowser/salesforce_modules_test.go`
- Modify: `internal/server/lightning_test.go`
- Inspect and modify when metadata flags are missing from storage: `internal/storage/standard_fields.go`
- Inspect and modify when storage object metadata cannot expose the tested flag: `internal/storage/model.go`

- [ ] **Step 1: Add metadata-backed describe tests**

In `internal/vm/data_test.go`, add or extend tests for:

- `DescribeSObjectResult.getLabelPlural()`
- `DescribeSObjectResult.getKeyPrefix()`
- `DescribeSObjectResult.isQueryable()`
- `DescribeSObjectResult.isSearchable()`
- `DescribeFieldResult.getPicklistValues()`
- `DescribeFieldResult.isAccessible()`
- `DescribeFieldResult.isCreateable()`
- `DescribeFieldResult.isUpdateable()`
- `DescribeFieldResult.getReferenceTo()`

Use a local metadata fixture object under `testdata/local-tests/presentation-metadata` or an existing in-test org schema. Do not infer field behavior from field names.

- [ ] **Step 2: Add AccessLevel scoped permission tests**

In `internal/vm/data_test.go`, add one test proving:

```apex
AccessLevel scoped = AccessLevel.withPermissionSetId('0PS000000000998');
List<Account> rows = Database.queryWithBinds(
    'SELECT Id, Hidden__c FROM Account',
    new Map<String,Object>(),
    scoped
);
```

Expected behavior:

- The permission set ID controls object and field access checks.
- `SYSTEM_MODE` still bypasses local user-mode checks.
- Unknown permission set ID fails with the same local permission diagnostic as other user-mode failures.

- [ ] **Step 3: Add LWC record wire object-info tests**

In `internal/lwcbrowser/salesforce_modules_test.go` or `internal/server/lightning_test.go`, add tests for:

- schema field module import tokens such as `@salesforce/schema/Account.Name`
- object-info JSON for a local object
- record wire response with field labels and values from local storage

Use `testdata/local-tests/lightning-out-vf` or `testdata/local-tests/presentation-metadata`.

- [ ] **Step 4: Run focused failures**

Run:

```bash
go test ./internal/vm -run 'TestExecDescribe|TestExecFeatureManagement|TestExecAccessLevel'
go test ./internal/lwcbrowser ./internal/server -run 'Test.*(Schema|RecordWire|Lightning|Wire)'
```

Expected before implementation: new schema or LWC wire assertions fail.

- [ ] **Step 5: Implement metadata-backed describe completion**

In `internal/vm/describe_runtime.go` and `internal/vm/platform_passive_members.go`:

- Fill missing field/object getters from `storage.ObjectDefinition` and `storage.Field`.
- Use existing field metadata flags for access booleans.
- Keep missing metadata deterministic. Return empty maps/lists where Salesforce would expose no loaded metadata locally.
- Do not derive behavior from field names.

In `internal/vm/permissions_runtime.go` and `internal/vm/soql_runtime.go`:

- Thread `AccessLevel.withPermissionSetId` into supported SOQL permission checks.
- Keep DML scoped checks intact.

In `internal/lwcbrowser/wire.go`:

- Reuse the same local schema data for record-wire output.
- Do not add a browser-only metadata model.

- [ ] **Step 6: Verify this lane**

Run:

```bash
go test ./internal/vm -run 'TestExecDescribe|TestExecFeatureManagement|TestExecAccessLevel|TestExecDatabaseUserMode'
go test ./internal/lwcbrowser ./internal/server -run 'Test.*(Schema|RecordWire|Lightning|Wire)'
go test ./internal/storage -run 'Test.*(Standard|Model|Metadata)'
git diff --check
```

Expected: all commands exit 0.

**Done report:**

- Changed files.
- Exact describe getters improved.
- Whether any schema rows can move from `partial` to `supported`.
- Exact LWC record-wire behavior now covered.

## Phase 1C: Org-Backed Search and SOSL Packet

**Subagent lane:** `../glade-lwc-search-sosl`

**Local value:** Local Apex controllers and LWC lookup components often call SOSL or `Search.find`. Current local search is deterministic but still too dependent on fixed IDs or empty suggestions.

**Rows touched:**

- `Search.query / SOSL FIND`
- `Search.query(String,AccessLevel)`
- `Search.query(String,Object)`
- `Search.find`
- `Search.find(String,AccessLevel)`
- `Search.find(String,Object)`
- `Search.suggest`
- `Search.suggest(String,String,Search.SuggestionOption)`
- `Search.suggest(String,String,Search.SuggestionOption,AccessLevel)`

**Target status:** Keep rows `partial`. Move notes from fixed-results-only behavior to local org-backed deterministic search if implemented. Do not claim Salesforce ranking, snippets, language, synonym, or external search behavior.

**Files:**

- Modify: `internal/vm/soql_runtime.go`
- Modify: `internal/vm/platform_http_cache_resources.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/vm/runtime_state.go`
- Inspect and modify when search needs a storage iterator helper: `internal/storage/model.go`
- Inspect and modify when overload shape tests fail: `internal/typesys/standard_symbols_test.go`

- [ ] **Step 1: Add failing org-backed SOSL tests**

In `internal/vm/platform_test.go`, add tests that do not call `Test.setFixedSearchResults`:

```apex
insert new Account(Name = 'Nook Supply');
insert new Contact(LastName = 'Nook Buyer', Email = 'buyer@example.test');

List<List<SObject>> rows = Search.query(
    'FIND {Nook*} IN ALL FIELDS RETURNING Account(Id, Name), Contact(Id, Name, Email)'
);
System.assertEquals(2, rows.size());
System.assertEquals(1, rows[0].size());
System.assertEquals(1, rows[1].size());
```

Add `Search.find` coverage:

```apex
Search.SearchResults results = Search.find(
    'FIND {Nook*} IN ALL FIELDS RETURNING Account(Id, Name), Contact(Id, Name)'
);
System.assertEquals(1, results.get('Account').size());
System.assertEquals(1, results.get('Contact').size());
```

Add `Search.suggest` coverage:

```apex
Search.SuggestionOption option = new Search.SuggestionOption();
option.setLimit(5);
Search.SuggestionResults suggestions = Search.suggest('Noo', 'Account', option);
System.assertEquals(1, suggestions.getSuggestionResults().size());
```

- [ ] **Step 2: Add access mode tests**

Add one test with `AccessLevel.USER_MODE` and one with `AccessLevel.SYSTEM_MODE`:

- USER_MODE applies existing object and field permission checks.
- SYSTEM_MODE returns local rows that match the query.

- [ ] **Step 3: Run focused failures**

Run:

```bash
go test ./internal/vm -run 'Test.*(SOSL|Search)'
```

Expected before implementation: new org-backed rows or suggestions fail.

- [ ] **Step 4: Implement deterministic local search**

In `internal/vm/soql_runtime.go`:

- Keep current parser handling for `RETURNING`, `WHERE`, `ORDER BY`, `LIMIT`, and `OFFSET`.
- When `Test.setFixedSearchResults` is not set, scan local org storage for each returned object.
- Match only text-like fields and ID fields. Use case-insensitive prefix matching for `*` suffix terms and case-insensitive contains matching otherwise.
- Return rows in object declaration order, then storage insertion order, unless `ORDER BY` is present.
- Apply existing projection and alias logic.
- Thread AccessLevel into existing permission checks.

In `internal/vm/platform_http_cache_resources.go`:

- Fill `SearchResult.getSnippet(String)` only with deterministic local field text if the row and field are present.
- Keep snippet empty for unsupported rich snippet behavior.

For `Search.suggest`:

- Return local `Search.SuggestionResult` rows based on matching name-like fields.
- Respect `Search.SuggestionOption.setLimit`.
- Do not add ranking, stemming, synonyms, or service-backed query suggestions.

- [ ] **Step 5: Verify this lane**

Run:

```bash
go test ./internal/vm -run 'Test.*(SOSL|Search|FixedSearchResults)'
go test ./internal/soql
git diff --check
```

Expected: all commands exit 0.

**Done report:**

- Changed files.
- Which Search rows have org-backed behavior.
- Remaining partial notes for ranking, snippets, language, and external services.

## Phase 1D: Visualforce, PageReference, and LWC Bridge Packet

**Subagent lane:** `../glade-lwc-vf-bridge`

**Local value:** Visualforce pages, Lightning Out hosts, LWC Apex wires, labels, static resources, and page state now meet in local server/runtime code. This packet tightens that bridge without broad browser work.

**Rows touched:**

- `ApexPages.Message`
- `PageReference`
- `Test.setCurrentPageReference(Object)`
- `Test.invokePage`
- `Messaging.SingleEmailMessage` only if page/controller tests require message DTO getters

**Target status:** Move rows only if lifecycle behavior now matches local Apex tests. Rendering remains a separate product surface. Do not broaden into full browser compatibility.

**Files:**

- Modify: `internal/vm/page_render.go`
- Modify: `internal/vm/ui_invocation.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/vm/ui_invocation_test.go`
- Modify: `internal/vm/runtime_state.go`
- Modify: `internal/server/visualforce.go`
- Modify: `internal/server/visualforce_test.go`
- Modify: `internal/server/lightning_test.go`
- Modify: `internal/lwcbrowser/bootstrap.go`
- Modify: `internal/lwcbrowser/bootstrap_test.go`
- Modify: `internal/lwcbrowser/setup_labels_test.go`
- Modify: `testdata/local-tests/lightning-out-vf/force-app/main/default/classes/ItemCtrl.cls`
- Inspect and modify when the Apex wire fixture must send the new DTO shape: `testdata/local-tests/lightning-out-vf/force-app/main/default/lwc/apexWireHost/apexWireHost.js`

- [ ] **Step 1: Add controller-state tests**

In `internal/vm/ui_invocation_test.go`, add tests for:

- current page URL and parameters visible inside an invoked `@AuraEnabled` method
- `ApexPages.addMessage` collected during a Visualforce action
- page messages isolated between test methods
- `PageReference.getParameters()` mutation preserved through `System.currentPageReference()`

- [ ] **Step 2: Add Lightning Out bridge tests**

In `internal/server/lightning_test.go`, add or extend tests with `testdata/local-tests/lightning-out-vf`:

- Apex wire request calls `ItemCtrl.getItems`.
- The response serializes a list of DTO or SObject rows.
- Label module imports return local Custom Labels.
- Static resource module imports return `/resource/<name>` URLs.
- Record wire output uses local storage and schema data.

If `third_party/lwc/node_modules` is absent, existing tests may skip. Do not make a non-deterministic network install part of this packet.

- [ ] **Step 3: Run focused failures**

Run:

```bash
go test ./internal/vm -run 'Test.*(UIInvocation|PageReference|ApexPages|InvokePage)'
go test ./internal/server ./internal/lwcbrowser -run 'Test.*(Lightning|Visualforce|Label|Resource|Wire)'
go test ./internal/apextest -run 'Test.*(Visualforce|Controller|Lightning)'
```

Expected before implementation: the new bridge assertions fail.

- [ ] **Step 4: Implement bridge behavior**

In `internal/vm/ui_invocation.go`:

- Keep `currentPage` and page parameters attached to controller invocation.
- Serialize controller return values through the same JSON path used by Apex wire responses.
- Keep errors structured and deterministic.

In `internal/vm/page_render.go` and `internal/server/visualforce.go`:

- Preserve `ApexPages.Message` state for the action lifecycle.
- Reset page state between test methods and requests.
- Keep Visualforce rendering separate from controller behavior.

In `internal/lwcbrowser/bootstrap.go` and tests:

- Reuse existing module shim helpers for labels, schema fields, and resources.
- Do not add a second Salesforce-module resolver.

- [ ] **Step 5: Verify this lane**

Run:

```bash
go test ./internal/vm -run 'Test.*(UIInvocation|PageReference|ApexPages|InvokePage)'
go test ./internal/server ./internal/lwcbrowser -run 'Test.*(Lightning|Visualforce|Label|Resource|Wire)'
go test ./internal/apextest -run 'Test.*(Visualforce|Controller|Lightning)'
git diff --check
```

Expected: all non-skipped tests pass.

**Done report:**

- Changed files.
- Which bridge behaviors now work.
- Whether tests skipped due missing LWC node modules.
- Any remaining boundary between runtime and rendering.

## Phase 1E: Async, Event, Continuation, and External-Service Test Harness Packet

**Subagent lane:** `../glade-lwc-async-harness`

**Local value:** Local Apex suites and LWC-backed controllers often call queueable work, event publication, continuations, and generated external-service facades. This packet should make the test harness useful without live services.

**Rows touched:**

- `System.enqueueJob(Object,Object)`
- `Test.startTest`
- `Test.stopTest`
- `Test.getEventBus()`
- `Test.getExternalService()`
- `Test.setMock`
- `WebServiceCallout.invoke(Object,Object,Map,List)`
- `WebServiceCallout.invoke(Object,Object,Map<String,Object>,List<String>)`

**Target status:** Keep live transport rows `partial`. Move local test harness notes from shape-only to behavior-backed if callback invocation, limits, and response materialization are covered.

**Files:**

- Modify: `internal/vm/async_job_runtime.go`
- Modify: `internal/vm/async_platform_runtime.go`
- Modify: `internal/vm/test_support_runtime.go`
- Modify: `internal/vm/runtime_state.go`
- Modify: `internal/vm/vm.go`
- Modify: `internal/vm/platform_passive_members.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/apextest/runner_test.go`
- Inspect and modify when overload shape tests fail: `internal/typesys/standard_symbols_test.go`

- [ ] **Step 1: Add async option and limit tests**

In `internal/vm/platform_test.go`, add tests proving:

- `AsyncOptions.MaximumQueueableStackDepth` rejects chains deeper than the local maximum.
- delay options stay explicitly unsupported.
- `Limits.getQueueableJobs()` and `Limits.getAsyncJobs()` count enqueues before and inside `Test.startTest()`.
- `Test.stopTest()` drains supported queueables in deterministic order.

- [ ] **Step 2: Add event bus tests**

Add tests proving:

- `EventBus.publish` queues local platform events.
- `Test.getEventBus().deliver()` invokes local platform-event triggers.
- `Limits.getPublishImmediateDML()` remains separate from ordinary DML counters.

- [ ] **Step 3: Add continuation and external-service tests**

Add tests proving:

- `Test.setContinuationResponse(label, response)` is visible through `Continuation.getResponse(label)`.
- `Test.invokeContinuationMethod(controller, methodName)` calls a local method and returns serialized response shape.
- `Test.getExternalService()` returns a deterministic local harness object and rejects live-service execution.

- [ ] **Step 4: Add SOAP mock materialization tests**

In `internal/vm/platform_test.go`, add coverage where `WebServiceCallout.invoke`:

- routes to a registered `WebServiceMock`
- increments callout limits once
- writes response values into the generated response map/list shape
- returns an empty response shell when no mock is registered

- [ ] **Step 5: Run focused failures**

Run:

```bash
go test ./internal/vm -run 'Test.*(Queueable|Async|EventBus|Continuation|ExternalService|WebServiceCallout|Limits)'
go test ./internal/apextest -run 'Test.*(Async|Event|Continuation)'
```

Expected before implementation: new harness assertions fail.

- [ ] **Step 6: Implement harness behavior**

In `internal/vm/async_job_runtime.go` and `internal/vm/async_platform_runtime.go`:

- Keep async queues per VM/test context.
- Enforce queueable stack depth from local `AsyncOptions`.
- Keep unsupported delay behavior explicit.
- Drain queueables and platform events only through supported `Test.stopTest()` and `Test.getEventBus().deliver()` paths.

In `internal/vm/test_support_runtime.go`:

- Store continuation responses by label in test context.
- Keep external-service harness deterministic and local.

In `internal/vm/vm.go`:

- Preserve `WebServiceCallout.invoke` mock dispatch.
- Improve response map/list materialization only for generated local shapes.
- Do not add outbound SOAP transport.

- [ ] **Step 7: Verify this lane**

Run:

```bash
go test ./internal/vm -run 'Test.*(Queueable|Async|EventBus|Continuation|ExternalService|WebServiceCallout|Limits)'
go test ./internal/apextest -run 'Test.*(Async|Event|Continuation)'
git diff --check
```

Expected: all commands exit 0.

**Done report:**

- Changed files.
- Exact harness behavior added.
- Remaining partial notes for live services, delay, and transport.

## Phase 2: Integration Lane

**Subagent lane:** `../glade-lwc-stdlib-integration`

Start only after product lanes merge into `main` or the coordinator integration branch.

**Files:**

- Modify: `/Users/matt/Dev/glade-tools/internal/capability/stdlib.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/capability_test.go`
- Add or modify fixtures under `/Users/matt/Dev/glade-tools/docs/fixtures/`
- Regenerate: `docs/STDLIB_COVERAGE.md`
- Regenerate: `docs/COMPATIBILITY_DASHBOARD.md`
- Regenerate: `docs/KNOWN_GAPS.md`
- Inspect and update if the user-facing support-map summary changes: `site/docs-src/guide/support-map.md`

- [ ] **Step 1: Refresh row recommendations from product lane reports**

Create `/tmp/glade-stdlib-integration-notes.md` with:

- rows that can move to `supported`
- rows that stay `partial` with better notes
- rows that stay `unsupported`
- focused tests proving each move

- [ ] **Step 2: Update catalog rows**

In `/Users/matt/Dev/glade-tools/internal/capability/stdlib.go`:

- Promote only rows with behavior-backed tests.
- Keep partial rows honest.
- Keep the five live-service unsupported rows fenced.
- Add local-model notes that name the boundary, such as "no external ranking", "no live transport", or "no full rendering lifecycle".

- [ ] **Step 3: Add fixture evidence**

Under `/Users/matt/Dev/glade-tools/docs/fixtures/`, add or update narrow fixtures:

- `core-runtime-json-dto-lwc-evidence.json`
- `data-platform-schema-lwc-record-wire-evidence.json`
- `query-runtime-local-search-sosl-evidence.json`
- `ui-lwc-vf-local-bridge-evidence.json`
- `async-test-harness-local-evidence.json`

Each fixture must name exact runtime behavior. Do not use broad labels without `surfaceId`.

- [ ] **Step 4: Run glade-tools focused tests**

Run from `/Users/matt/Dev/glade-tools`:

```bash
go test ./internal/capability ./internal/compat ./internal/surfaceledger
```

Expected:

```text
ok  	github.com/glade-sh/glade-tools/internal/capability
ok  	github.com/glade-sh/glade-tools/internal/compat
ok  	github.com/glade-sh/glade-tools/internal/surfaceledger
```

- [ ] **Step 5: Regenerate docs**

Run from `/Users/matt/Dev/glade-tools`:

```bash
go run ./cmd/glade-tools stdlib --output ../glade/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools dashboard --output ../glade/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools gaps --output ../glade/docs/KNOWN_GAPS.md
```

Expected: files are rewritten only where rows or notes changed.

- [ ] **Step 6: Verify generated docs**

Run from `/Users/matt/Dev/glade-tools`:

```bash
go run ./cmd/glade-tools stdlib --check ../glade/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools dashboard --check ../glade/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools gaps --check ../glade/docs/KNOWN_GAPS.md
```

Expected:

```text
../glade/docs/STDLIB_COVERAGE.md: up to date
../glade/docs/COMPATIBILITY_DASHBOARD.md: up to date
../glade/docs/KNOWN_GAPS.md: up to date
```

- [ ] **Step 7: Record after counts**

Run from `/Users/matt/Dev/glade`:

```bash
awk -F'|' 'NR>8 && /^\|/ {
  status=$4
  gsub(/`/,"",status)
  gsub(/^[ \t]+|[ \t]+$/,"",status)
  counts[status]++
}
END {for (s in counts) print s, counts[s]}' docs/STDLIB_COVERAGE.md | sort
```

Expected: `unsupported` remains 5. `partial` may drop only if product lanes truly closed rows.

## Phase 3: Coordinator Merge and Final Verification

- [ ] **Step 1: Merge product lanes one at a time**

Run from `/Users/matt/Dev/glade`:

```bash
git status --short --branch
git merge --no-ff codex/lwc-json-dto
git merge --no-ff codex/lwc-schema-permissions
git merge --no-ff codex/lwc-search-sosl
git merge --no-ff codex/lwc-vf-bridge
git merge --no-ff codex/lwc-async-harness
```

Expected: conflicts are limited to `internal/vm/platform_test.go`, `internal/vm/dispatch.go`, generated docs, or catalog notes. Resolve by keeping both tests unless duplicate coverage is exact.

- [ ] **Step 2: Merge integration lane**

Run:

```bash
git merge --no-ff codex/lwc-stdlib-integration
```

Expected: generated docs and catalog changes land after product code.

- [ ] **Step 3: Run focused product gates**

Run from `/Users/matt/Dev/glade`:

```bash
go test ./internal/vm
go test ./internal/apextest
go test ./internal/lwcbrowser ./internal/lwc ./internal/visualforce ./internal/server
go test ./internal/storage ./internal/soql ./internal/typesys
git diff --check
```

Expected: all commands exit 0. LWC tests that require missing `node_modules` may skip with an explicit skip message.

- [ ] **Step 4: Run glade-tools gates**

Run from `/Users/matt/Dev/glade-tools`:

```bash
go test ./internal/capability ./internal/compat ./internal/surfaceledger
go run ./cmd/glade-tools stdlib --check ../glade/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools dashboard --check ../glade/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools gaps --check ../glade/docs/KNOWN_GAPS.md
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 5: Optional broad gate**

Run from `/Users/matt/Dev/glade` only after focused gates pass:

```bash
go test ./...
```

Expected: exit 0.

- [ ] **Step 6: Cleanup worktrees after merge**

Run from `/Users/matt/Dev/glade`:

```bash
git worktree remove ../glade-lwc-json-dto
git worktree remove ../glade-lwc-schema-permissions
git worktree remove ../glade-lwc-search-sosl
git worktree remove ../glade-lwc-vf-bridge
git worktree remove ../glade-lwc-async-harness
git worktree remove ../glade-lwc-stdlib-integration
git branch -d codex/lwc-json-dto codex/lwc-schema-permissions codex/lwc-search-sosl codex/lwc-vf-bridge codex/lwc-async-harness codex/lwc-stdlib-integration
```

Expected: worktrees and merged local branches are removed.

## Final Done Criteria

- Product behavior is implemented for each accepted lane.
- Focused tests pass in `glade`.
- Catalog and generated docs pass checks in `glade-tools`.
- `unsupported` remains 5 unless the user explicitly changes live-service scope.
- Any row left `partial` has a concrete note naming what still is not modeled.
- The final report lists before/after counts and exact tests run.
