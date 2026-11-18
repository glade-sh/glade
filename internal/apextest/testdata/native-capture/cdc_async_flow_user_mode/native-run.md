# Native run

Owned synthetic probes, fresh 30-day Developer scratch org, 2026-11-09 UTC. Deployed classes, CDC trigger, selected-entity channel member and async record-triggered Flow at API 62.0 then 67.0. sf apex run called APProbe.enqueueCDC(), inserting a custom record in a committed transaction. The CDC trigger inserts a Seed__c record in system mode, which starts an async Flow path that invokes APProbe; the Flow writes an observation. A read-only permission set for Obs__c.Data__c was assigned to the reading admin after the API 62 enqueue and before API 67; it grants no Probe__c field access. REST query read the committed observations. Both flows ran as AutomatedProcess; explicit USER_MODE custom writes were denied at 62 and succeeded at 67, while reads and Account insert succeeded at both APIs. The 67 project is included; for 62 substitute API versions and asyncap67 -> asyncap. The org was deleted; ScratchOrgInfo Status Deleted.

## Recorded facts

```text
source_api=62.0,67.0
route=committed_cdc_async
native_rows=2
```
