# Phase 5: Wire, LDS, And Apex Controller Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan with parallel subagent squads. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Make local LWC data flows useful in both hosts: Apex wire, imperative Apex, `getRecord`, `getObjectInfo`, selected record mutation helpers, and field helper functions.

**Architecture:** Browser shims call Glade server routes. Server routes execute local Apex through the VM and read or mutate `storage.OrgState`. A later LDS cache phase can add refresh and notification behavior.

**Tech Stack:** Go HTTP handlers, Glade VM `InvokeLWCMethod`, local storage, `lwcruntime` modules, wire adapter tests, browser tests.

---

## Feature Delivered

Developers can test real `@AuraEnabled` controllers and record-backed LWC screens without deploying to Salesforce, whether the component runs in `/lwc/preview/*` or inside a Visualforce Lightning Out page.

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
- Test data: `testdata/local-tests/lightning-out-vf/force-app/main/default/lwc/apexWireHost/*`
- Test data: `testdata/local-tests/lightning-out-vf/force-app/main/default/lwc/recordWireHost/*`

## Parallel Subagent Squads

Use parallel subagent squads where files do not overlap. The coordinator integrates one patch at a time.

- Apex squad owns imperative Apex and wire parameter behavior.
- LDS server squad owns record routes and storage mutation.
- Runtime squad owns wire adapter behavior and future cache, refresh, and notification work.
- Module squad owns generated shim exports.
- Visualforce host squad owns the same wire, Apex, and LDS browser cases inside `/apex/<PageName>`.
- Review squad runs Go, Node, and browser tests.

## Implementation Steps

- [x] Preserve existing Apex wire route `/lightning/wire/apex` and add imperative route `/lightning/apex/<class>/<method>`.
- [x] Match Salesforce Apex parameter rule: method params are passed as an object with properties matching Apex parameter names; `undefined` suppresses wire invocation; `null` invokes the method with null.
- [x] Add overload diagnostic: if multiple `@AuraEnabled` overloads match, return `GLADELWC013 overloaded AuraEnabled method unsupported`.
- [x] Extend `lightning/uiRecordApi` exports for the current local contract:
  - `getRecord`
  - `getFieldValue`
  - `getFieldDisplayValue`
  - `getObjectInfo`
  - `createRecord`
  - `updateRecord`
  - `deleteRecord`
- [x] Leave `getRecords`, `getObjectInfos`, picklist wires, `notifyRecordUpdateAvailable`, and `refreshApex` as future LDS expansion unless implemented with tests in both hosts.
- [x] Add record mutation routes. Mutations must update `storage.OrgState` and return UI API-shaped records.
- [x] Add browser fixture proving Apex wire, imperative Apex, `getRecord`, `getObjectInfo`, and mutation helper behavior in one component.
- [x] Add Visualforce Lightning Out browser fixture proving the same Apex wire, imperative Apex, `getRecord`, `getObjectInfo`, and mutation helper behavior inside `/apex/<PageName>`.

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

```bash
(cd lwcruntime && npm run build && node --test test/visualforce-dev-server.test.mjs)
```

Fixture-manifest comparison:

```bash
(cd ../glade-tools && go run ./cmd/glade-plugin-compat lwc capture --target-org oaer-probe-max --project ../glade/testdata/local-tests/lwc-shell --include-hosts lightning-shell,visualforce-lightning-out --targets apex-wire,imperative-apex,record-wire --out /tmp/glade-lwc-wire-capture.json)
```

## Done Gate

- Apex wire and imperative Apex execute real local controllers.
- `getRecord`, `getObjectInfo`, field helpers, and record mutations pass tests.
- Batch record APIs, picklist wires, refresh, and cache notifications remain named future work unless their tests exist in this phase.
- Unsupported UI API modules return named diagnostics rather than generic thrown errors.
- Every supported wire, Apex, LDS, and cache feature passes in both the Lightning shell and Visualforce Lightning Out host, or the support ledger names the host-specific gap.
