# Native run

Owned synthetic Flow probes in a fresh 30-day Developer scratch org, 2026-11-08, source API 62.0 and 67.0. Deployed classes, flows and project at each API. The isTest method intentionally asserts OBS|{}: the async path did not run during stopTest. The committed route calls APProbe.enqueueFlow() with sf apex run; after asynchronous delivery, a read-only Obs__c permission set allowed REST readback. The scheduled route activated a one-time schedule in the org local timezone and used the same readback. The read-only grant covers only Obs__c.Data__c, not Probe__c.Val__c. Both committed routes ran as Standard, reads and standard Account insert succeeded, and user-mode writes of Val__c were denied at both APIs. The 62 test was run before the readback grant and before APProbe.enqueueFlow was added; its probe body was otherwise identical. The persisted project is the 67 deployment; for 62 replay change API versions, async67 to async, and scheduled67 to scheduled62. The org was deleted; ScratchOrgInfo Status Deleted.

## Recorded facts

```text
source_api=62.0,67.0
route=istest,async,scheduled
native_rows=6
```
