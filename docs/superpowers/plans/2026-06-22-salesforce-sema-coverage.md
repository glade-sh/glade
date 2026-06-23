# Salesforce Sema Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the Salesforce coverage gaps exposed by the local docs pass and corpus checks so valid projects report no semantic diagnostics. Remaining diagnostics must be limited to actual metadata, file, or source-code issues and must be listed project by project.

**Architecture:** Keep product behavior in `glade`: type loading, metadata indexing, semantic checks, standard stubs, SOQL typing, and runtime stubs. Keep broad corpus runners, dashboards, and generated maintenance ledgers out of this repo; use `/tmp` reports or first-party `glade-tools` workflows for corpus sweeps.

**Tech Stack:** Go, Apex metadata parsers, generated Salesforce standard symbols, `glade check`, local Salesforce docs at `/Users/matt/Dev/glade/example-projects/Salesforce Docs Scraper/salesforce-docs-expanded-run`, private corpus under `/Users/matt/Dev/glade-corpus/private`, public corpus under `/Users/matt/Dev/glade-corpus/public`.

---

## Evidence From This Pass

Fresh binary:

```bash
go build -o /tmp/glade-plan-pass ./cmd/glade
```

Private checks:

- `/Users/matt/Dev/glade-corpus/private/payment-workspace`: 8 diagnostics. Two look like Glade implementation gaps on `this.request.PaymentToken` and `this.request.CurrencyIsoCode`; six look like missing package metadata fields or relationship metadata.
- `/Users/matt/Dev/glade-corpus/private/portal-workspace`: one `GLADEPERF001` warning. No semantic implementation failure found in this pass.

Public spot checks:

- `agent-script-recipes`: 65 diagnostics. Most are missing `Flow.Interview.<FlowName>` static types.
- `ApexKit`: 75 diagnostics. Main cluster is inherited `Object.equals/hashCode/toString` and overload resolution.
- `apex-dml-mocking`: 45 diagnostics. Main cluster is `Object.equals`; two are child relationship subquery collection typing.
- `sfdx-mass-action-scheduler`: 19 diagnostics. Main cluster is `Database.Batchable`, `Iterable`, `Iterator`, constructors, and overload resolution.
- `EDA`: 35 diagnostics. Main cluster is `Schema`/`Type`/describe typing, collection constructors, nested enums, overloads, and `User.Profile`.
- `apex-rollup`: 21 diagnostics. Main cluster is `Flow.Interview`, custom metadata string fields, CDC change event types, null coalescing, and method resolution.
- `az-insurance`: 1 diagnostic. `ConnectApi.ManagedContentVersionCollection.items` loses element type.

Prior public corpus run:

- `GLADETYPE001`: 1529. Mostly duplicate project-shape indexing in a few repos.
- `GLADESEMA006`: 471.
- `GLADESEMA023`: 198.
- `GLADESEMA018`: 159.
- `GLADESEMA008`: 149.
- `GLADESEMA009`: 118.
- `GLADESEMA004`: 68.
- `GLADESEMA011`: 66.
- `GLADESEMA021`: 63.
- `GLADEPERF001`: 57.

Docs checked locally:

- `apex/apex_class_system_Location.md`
- `apex/flow_interview_class.md`
- `apex/apex_methods_system_approval.md`
- `apex/apex_methods_system_database.md`
- `apex/apex_class_System_Object.md`
- `apex-guide/langCon_apex_collections_maps_keys_userdefined.md`
- `apex/apex_methods_system_list.md`
- `apex/apex_methods_system_map.md`
- `apex/apex_methods_system_set.md`
- `apex-guide/apex_batch_interface.md`
- `apex/apex_methods_system_type.md`
- `apex/apex_methods_system_schema.md`
- `apex/apex_ConnectAPI_ManagedContent_static_methods.md`
- `apex/apex_ConnectAPI_Orchestration_static_methods.md`

---

## Acceptance Gate

- [ ] Build a fresh binary from this repo before every corpus sweep:

  ```bash
  go build -o /tmp/glade-sema-pass ./cmd/glade
  ```

- [ ] A valid project passes the implementation gate when `glade check --project .` emits no `GLADESEMA*` or false `GLADETYPE001` diagnostics.
- [ ] `GLADEPERF*` diagnostics stay separated from sema coverage. If a project has only real performance warnings, record that as source advisory work, not a Glade implementation miss.
- [ ] Remaining `GLADESEMA*` diagnostics must have a named reason: missing metadata file, missing field definition, duplicate source file, invalid Apex, or an explicit unsupported Salesforce surface with a follow-up issue.
- [ ] No project-specific exception may land in product code.

---

## Implementation Tasks

### 1. Add A Repeatable Local Failure Inventory

- [ ] Add a short, non-product checklist under this plan section as work proceeds. Do not add a corpus dashboard or scanner to base `glade`.
- [ ] Capture each corpus run in `/tmp` with one log file per project and a TSV summary:

  ```bash
  bin=/tmp/glade-sema-pass
  root=/Users/matt/Dev/glade-corpus/public
  out=/tmp/glade-public-sema-pass.$(date +%Y%m%d%H%M%S)
  mkdir -p "$out/logs"
  ```

- [ ] Classify every remaining diagnostic into one of these buckets: implemented by this plan, real metadata/file issue, real Apex issue, or new unsupported Salesforce surface.
- [ ] Keep the classification outside committed product code unless it becomes a focused regression test.

### 2. Flow.Interview Static Flow Types

The docs support `Flow.Interview.flowName`, `Flow.Interview.namespace.flowName`, `createInterview`, `start`, `getVariableValue`, and static flow variables as properties.

- [ ] Use the existing metadata flow index in `internal/metadata/metadata.go` as the source of project flow names.
- [ ] Generate project-local type symbols for:

  ```apex
  Flow.Interview.My_Flow
  Flow.Interview.namespace.My_Flow
  ```

- [ ] Give each generated flow interview type:
  - superclass or assignability to `Flow.Interview`
  - constructor `Flow.Interview.My_Flow(Map<String, Object> inputs)`
  - instance method `start()`
  - instance method `getVariableValue(String name)` returning `Object`
  - variable properties from `.flow-meta.xml` when the flow file declares them
- [ ] Map simple flow variable types to Apex types: `String`, `Boolean`, `Integer`, `Long`, `Decimal`, `Double`, `Date`, `DateTime`, `Object`, `SObject`, and `List<Object>` when the docs metadata cannot be narrowed.
- [ ] Add regression tests in `internal/sema` with a temp project containing one `.flow-meta.xml` and Apex that constructs, starts, and reads a generated interview type.
- [ ] Re-run `agent-script-recipes` and `apex-rollup`. The `Flow.Interview.*` unknown type cluster must disappear.

### 3. System.Location, Schema.Location, And Qualified Platform Aliases

The docs say compound geolocation fields use `System.Location`, while `Schema.Location` refers to the object token. The generated stubs include `Location`, but qualification must resolve in sema and runtime.

- [ ] Add tests for:

  ```apex
  System.Location home = Location.newInstance(47.0, -122.0);
  Double miles = System.Location.getDistance(home, home, 'mi');
  Double km = home.getDistance(home, 'km');
  ```

- [ ] Make qualified type lookup resolve `System.Location` to the standard `Location` class without losing static methods or properties.
- [ ] Keep `Schema.Location` on the schema/object side, not as the `System.Location` class.
- [ ] Audit other standard classes present unqualified in generated stubs and confirm qualified `System.<ClassName>` lookup works for them. Add a table-driven test for a representative set: `System.Address`, `System.Location`, `System.URL`, `System.JSON`, `System.Type`, and `System.UserInfo`.

### 4. Approval Namespace Completeness

The docs list `Approval.process` overloads for one request, one request plus `allOrNone`, list of requests, and list plus `allOrNone`.

- [ ] Update the standard symbol generator or overlay so `Approval.process` exists in generated symbols, not only in hand-written sema overlays.
- [ ] Support these signatures:

  ```apex
  Approval.ProcessResult process(Approval.ProcessRequest request)
  Approval.ProcessResult process(Approval.ProcessRequest request, Boolean allOrNone)
  List<Approval.ProcessResult> process(List<Approval.ProcessRequest> requests)
  List<Approval.ProcessResult> process(List<Approval.ProcessRequest> requests, Boolean allOrNone)
  ```

- [ ] Check runtime support in `internal/vm/approval_process_runtime.go`. Add list overload execution if missing.
- [ ] Add sema and VM tests for each overload.
- [ ] Regenerate standard symbols and verify generated output does not drop the method on the next run.

### 5. Inherited System.Object Methods And User Overrides

The docs say every Apex class inherits `equals(Object)`, `hashCode()`, and `toString()`. User-defined map keys also rely on `equals(Object)` and `hashCode()`.

- [ ] Fix member lookup so every non-void Apex type can resolve:

  ```apex
  Boolean b = x.equals(y);
  Integer h = x.hashCode();
  String s = x.toString();
  ```

- [ ] Ensure a user-defined `equals(Object)` or `hashCode()` method wins over the inherited `System.Object` method.
- [ ] Make collection-call diagnostics run after normal method lookup. A valid `equals(Object)` call must not become `GLADESEMA023 invalid collection call`.
- [ ] Add tests for primitive wrappers, strings, sObjects, user classes, enum values, and collection instances.
- [ ] Re-run `ApexKit`, `apex-dml-mocking`, `Apex-Opensource-Library`, and `ApexTestKit`. The `equals` cluster must disappear.

### 6. Generic Collection Constructors And Return Types

The docs list these constructors:

```apex
List<T>()
List<T>(List<T>)
List<T>(Set<T>)
Set<T>()
Set<T>(Set<T>)
Set<T>(List<T>)
Map<K,V>()
Map<K,V>(Map<K,V>)
Map<Id,SObjectSubType>(List<SObjectSubType>)
```

- [ ] Update collection constructor matching in sema so generic arguments bind from the declared target and the constructor argument.
- [ ] Preserve receiver generics for:
  - `List<T>.get(Integer) -> T`
  - `List<T>.clone() -> List<T>`
  - `List<T>.deepClone(...) -> List<T>`
  - `Map<K,V>.get(K) -> V`
  - `Map<K,V>.keySet() -> Set<K>`
  - `Map<K,V>.values() -> List<V>`
  - `Set<T>.clone() -> Set<T>`
- [ ] Add tests for common Apex idioms:

  ```apex
  List<Id> ids = new List<Id>(accountMap.keySet());
  Set<Id> idSet = new Set<Id>(ids);
  Map<Id, Account> byId = new Map<Id, Account>(accounts);
  Account a = byId.get(someId);
  ```

- [ ] Re-run `EDA`, `ApexKit`, and `apex-dml-mocking`. Constructor and collection method false positives must disappear.

### 7. Schema, Type, Describe, And `.class`

Docs support `Type.forName`, `Type.newInstance`, `Type.isAssignableFrom`, `.class` on Apex types, and schema describe maps.

- [ ] Add parser/sema support for `.class` on:
  - primitive types
  - collection types
  - sObject types
  - user classes
  - namespace-qualified classes
- [ ] Add or fix signatures for:

  ```apex
  Type.forName(String name)
  Type.forName(String namespace, String name)
  Object Type.newInstance()
  Boolean Type.isAssignableFrom(Type other)
  Map<String, Schema.SObjectType> Schema.getGlobalDescribe()
  List<Schema.DescribeSObjectResult> Schema.describeSObjects(List<String> names)
  ```

- [ ] Fix `Schema.SObjectType.getDescribe()` and `Schema.DescribeSObjectResult.fields.getMap()` chains.
- [ ] Preserve `Schema.SObjectField.getDescribe()` and field token maps.
- [ ] Add standard relationship metadata for `User.Profile` from the standard object surface.
- [ ] Add EDA-derived tests for describe variables, field maps, nested enum return values, and `User.Profile.Name`.
- [ ] Re-run `EDA`, `NPSP`, `at4dx`, `EnhancedLightningGrid`, and `OutboundFunds` from the public corpus.

### 8. SOQL Result, Relationship, And Subquery Typing

Valid SOQL must carry enough type information for field access after assignment.

- [ ] Type child relationship subqueries as `List<ChildSObject>` on the parent record.
- [ ] Add tests for:

  ```apex
  Opportunity opp = [
      SELECT Id, (SELECT Id, Role FROM OpportunityContactRoles)
      FROM Opportunity
      LIMIT 1
  ];
  Integer n = opp.OpportunityContactRoles.size();
  OpportunityContactRole role = opp.OpportunityContactRoles.get(0);
  ```

- [ ] Fix aggregate query enhanced-for typing so this form binds `row` as `AggregateResult`:

  ```apex
  for (AggregateResult row : [SELECT AccountId a, COUNT(Id) c FROM Contact GROUP BY AccountId]) {
      Object value = row.get('c');
  }
  ```

- [ ] Improve `Database.query` assignment-context inference where the query string is static and the left side supplies the target type.
- [ ] Re-run `apex-dml-mocking`, `sfdx-mass-action-scheduler`, and `LightningFlowComponents`.

### 9. Database.Batchable, Iterable, Iterator, And Interface Assignability

The docs define `Database.Batchable<T>` as a generic interface whose `start` returns `Database.QueryLocator` or `Iterable<T>`, and whose `execute` receives `List<T>`.

- [ ] Fix interface assignability so a class that implements `Database.Batchable<T>` passes as the first argument to `Database.executeBatch`.
- [ ] Preserve generic `T` through `start`, `execute`, `Iterable<T>`, and `Iterator<T>`.
- [ ] Resolve custom iterator methods:

  ```apex
  Boolean hasNext()
  T next()
  Iterator<T> iterator()
  ```

- [ ] Check constructor indexing for batch helper classes so a valid two-argument constructor resolves.
- [ ] Add tests modeled on `sfdx-mass-action-scheduler` for a batch class over `Map<String, Object>`, an iterable source, and a custom iterator.
- [ ] Re-run `sfdx-mass-action-scheduler`. The constructor, `executeBatch`, iterable, and aggregate-result diagnostics must disappear unless the source itself is invalid.

### 10. ConnectApi DTO Collections

The docs show `ConnectApi.ManagedContent.getManagedContentByIds` returns `ConnectApi.ManagedContentVersionCollection`; its item fields must carry concrete DTO types.

- [ ] Fix generated or overlay symbols for:
  - `ConnectApi.ManagedContentVersionCollection.items -> List<ConnectApi.ManagedContentVersion>`
  - `ConnectApi.ManagedContentVersion.contentNodes`
  - `ConnectApi.OrchestrationInstanceCollection.items -> List<ConnectApi.OrchestrationInstance>`
- [ ] Add tests that read a managed content collection and return a `List<ConnectApi.ManagedContentVersion>`.
- [ ] Re-run `az-insurance` and `LightningFlowComponents`.

### 11. Custom Metadata Fields And CDC Change Events

Custom metadata records should expose typed fields from project metadata. Change data capture objects should exist when the base object exists or metadata declares them.

- [ ] Verify custom metadata field type loading for `__mdt` records in `apex-rollup`.
- [ ] Fix string field member resolution so metadata fields like `LookupFieldOnLookupObject__c` support `toLowerCase()` and `endsWith()`.
- [ ] Generate or resolve `<SObject>ChangeEvent` types for known standard/custom objects when referenced.
- [ ] Give generated change event types core fields and sObject behavior; do not invent project-specific fields.
- [ ] Add tests for custom metadata string fields and `ContactPointAddressChangeEvent`.
- [ ] Re-run `apex-rollup`.

### 12. Package Namespace Artifacts And Field-Path Resolution

The private payment-workspace failure shows field-path typing can lose the declared type when a namespaced generated class is stored on `this`.

- [ ] Add a regression test matching this shape:

  ```apex
  public class GatewayService {
      private znu.PaymentGatewayRequest request;
      public void run() {
          String token = this.request.PaymentToken;
          String code = this.request.CurrencyIsoCode;
      }
  }
  ```

- [ ] Confirm package artifact loading provides `znu.PaymentGatewayRequest` fields from the namespace artifact.
- [ ] Fix `this.<field>.<member>` resolution so it uses the field's declared type, not the enclosing class.
- [ ] Re-run `/Users/matt/Dev/glade-corpus/private/payment-workspace`. Only real missing metadata fields or relationship metadata should remain.
- [ ] Re-run the private portal package workspaces.

### 13. Project Discovery And Duplicate Symbol Hygiene

The public corpus has large `GLADETYPE001` clusters from duplicate indexing. Valid projects with overlapping package directories should not create false duplicate classes.

- [ ] Inspect the top duplicate-symbol repos: `LightningFlowComponents`, `Apex-Opensource-Library`, and `source-deploy-retrieve`.
- [ ] De-duplicate source files by canonical path during project indexing.
- [ ] If `sfdx-project.json` lists overlapping package directories, index a file once.
- [ ] Preserve diagnostics for true duplicate Apex declarations in different files.
- [ ] Add project indexing tests with overlapping package dirs and with a real duplicate class pair.
- [ ] Re-run the duplicate-symbol repos. False `GLADETYPE001` counts must drop to zero.

### 14. Null Coalescing And Common Type Rules

The `apex-rollup` corpus shows a nullable metadata expression returning the wrong common type.

- [ ] Add tests for:

  ```apex
  RollupPluginParameter__mdt value =
      providedParameter ?? RollupPluginParameter__mdt.getInstance('Default');
  ```

- [ ] Fix common-type inference for null coalescing so identical concrete types stay concrete.
- [ ] Ensure `null ?? T` and `T ?? null` infer `T`.
- [ ] Re-run `apex-rollup`.

### 15. Overload Resolution And Static Helper Methods

Several repos still show valid helper methods as wrong-arity calls. The known clusters include `crud`, `getFLSForFieldOnObject`, `MA_StringUtils.matches`, and `tdtmClass.run`.

- [ ] Add failing tests from the smallest real call shapes before changing resolver behavior.
- [ ] Confirm method tables include instance, static, inherited, nested-class, namespace-qualified, and overloaded user methods.
- [ ] Fix overload selection to rank arity, assignability, generic binding, and namespace qualification before emitting `GLADESEMA004` or `GLADESEMA023`.
- [ ] Add tests for two overloads with the same name and different arity, plus one namespace-qualified helper.
- [ ] Re-run `ApexKit`, `sfdx-mass-action-scheduler`, and `EDA`.

---

## Verification Matrix

Run focused tests after each task cluster:

```bash
go test ./internal/sema
go test ./internal/typesys
go test ./internal/metadata
go test ./internal/project
go test ./internal/vm
```

Run the full product suite before corpus proof:

```bash
go test ./...
scripts/smoke.sh
```

Private corpus proof:

```bash
/tmp/glade-sema-pass check --project /Users/matt/Dev/glade-corpus/private/payment-workspace
/tmp/glade-sema-pass check --project /Users/matt/Dev/glade-corpus/private/portal-workspace
/tmp/glade-sema-pass check --project /Users/matt/Dev/glade-corpus/private/portal-package-workspace
```

Public corpus proof:

- [ ] Run all public repos with the fresh binary.
- [ ] Sort diagnostics by code and by message stem.
- [ ] Re-check the top ten diagnostic-producing repos after each major fix.
- [ ] Save the final public summary under `/tmp`.
- [ ] Report remaining diagnostics as project issues only when the exact missing metadata/file/source cause is named.

---

## Expected Closeout

- [ ] All docs-backed surfaces above have focused regression tests.
- [ ] Private payment workspace has no Glade semantic false positives.
- [ ] Private portal package workspaces have no Glade semantic false positives.
- [ ] Public corpus false positives in the named clusters are gone.
- [ ] Remaining public corpus diagnostics are listed as actual metadata, file, or source issues with project names.
- [ ] No maintenance scanner, corpus dashboard, or docs scraper code lands in base `glade`.
