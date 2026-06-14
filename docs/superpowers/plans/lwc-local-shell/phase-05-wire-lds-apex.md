# Phase 5: Wire, LDS, And Apex Controller Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make local LWC data flows useful: Apex wire, imperative Apex, LDS record wires, object info, picklists, record mutation, refresh, and cache notification.

**Architecture:** Browser shims call Glade server routes. Server routes execute local Apex through the VM and read or mutate `storage.OrgState`. A small LDS-like cache in `lwcruntime` makes refresh and notification behavior testable.

**Tech Stack:** Go HTTP handlers, Glade VM `InvokeLWCMethod`, local storage, `lwcruntime` modules, wire adapter tests, browser tests.

---

## Feature Delivered

Developers can test real `@AuraEnabled` controllers and record-backed LWC screens without deploying to Salesforce.

## Files

- Modify: `internal/lwcbrowser/salesforce_modules.go`
- Modify: `internal/lwcbrowser/salesforce_modules_test.go`
- Modify: `internal/server/lightning_wire.go`
- Create: `internal/server/lightning_record_api.go`
- Create: `internal/server/lightning_record_api_test.go`
- Modify: `internal/vm/ui_invocation.go`
- Modify: `lwcruntime/src/shims/wire-adapter.mjs`
- Create: `lwcruntime/src/shims/lds-cache.mjs`
- Add tests: `lwcruntime/test/lds-cache.test.mjs`
- Test data: `testdata/local-tests/lwc-shell/force-app/main/default/lwc/wireProbe/*`

## Parallel Squads

- Apex squad owns imperative Apex and wire parameter behavior.
- LDS server squad owns record routes and storage mutation.
- Runtime squad owns cache, refresh, and notification.
- Module squad owns generated shim exports.
- Review squad runs Go, Node, and browser tests.

## Implementation Steps

- [ ] Preserve existing Apex wire route `/lightning/wire/apex` and add imperative route `/lightning/apex/<class>/<method>`.
- [ ] Match Salesforce Apex parameter rule: method params are passed as an object with properties matching Apex parameter names; `undefined` suppresses wire invocation; `null` invokes the method with null.
- [ ] Add overload diagnostic: if multiple `@AuraEnabled` overloads match, return `GLADELWC013 overloaded AuraEnabled method unsupported`.
- [ ] Extend `lightning/uiRecordApi` exports:
  - `getRecord`
  - `getRecords`
  - `getFieldValue`
  - `getFieldDisplayValue`
  - `getObjectInfo`
  - `getObjectInfos`
  - `getPicklistValues`
  - `getPicklistValuesByRecordType`
  - `createRecord`
  - `updateRecord`
  - `deleteRecord`
  - `notifyRecordUpdateAvailable`
  - `refreshApex`
- [ ] Add route `/lightning/record-api/getRecords` for batch record requests.
- [ ] Add route `/lightning/record-api/picklist-values` from local schema picklist metadata and record type selection.
- [ ] Add record mutation routes. Mutations must update `storage.OrgState`, return UI API-shaped records, and trigger cache notifications in the browser.
- [ ] Add `lds-cache.mjs` with cache keys based on adapter name and stable JSON config. It must fan out notifications to active subscribers.
- [ ] Add cache tests:
  - two `getRecord` subscribers receive one network load and two emissions.
  - `updateRecord` updates local cache and notifies subscribers.
  - `notifyRecordUpdateAvailable([{recordId}])` reloads affected records.
  - `refreshApex(wiredValue)` reloads Apex wires.
- [ ] Add browser fixture proving Apex wire, imperative Apex, `getRecord`, `updateRecord`, and `notifyRecordUpdateAvailable` in one component.

## Verification

```bash
go test ./internal/server ./internal/lwcbrowser ./internal/vm -run 'Lightning|LWC|UIInvocation' -count=1
```

```bash
(cd lwcruntime && npm test -- --runInBand)
```

```bash
go test ./internal/server -run 'TestLightningWire|TestLightningRecordAPI' -count=1
```

Scratch comparison:

```bash
(cd ../glade-tools && go run ./cmd/glade-plugin-compat lwc capture --target-org oaer-probe-max --project ../glade/testdata/local-tests/lwc-shell --targets apex-wire,imperative-apex,record-wire --out /tmp/glade-lwc-wire-capture.json)
```

## Done Gate

- Apex wire and imperative Apex execute real local controllers.
- `getRecord`, `getRecords`, `getObjectInfo`, picklist wires, and record mutations pass tests.
- Cache refresh behavior is observable in browser tests.
- Unsupported UI API modules return named diagnostics rather than generic thrown errors.
