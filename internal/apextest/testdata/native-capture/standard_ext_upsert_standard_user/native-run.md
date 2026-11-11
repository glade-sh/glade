# Native run

Owned synthetic Account external ID probe, fresh 30-day Developer scratch org, 2026-11-09 UTC. Deployed sources at API 62.0 and 67.0; sf apex run test ran ExtProbeTest with a freshly created Standard User inside System.runAs. An ExtKey permission set granting read and edit on the key field was deployed, but assigned only in the granted rows. Text__c remained ungranted. For granted rows, the test source inserts a PermissionSetAssignment for the test user immediately before runAs; the included project is the ungranted source. Each method deliberately asserts to transport the complete 36 observations per set/API. Ungranted USER_MODE external key throws SObjectException at 62 and 67; default mode is system at 62 and user mode at 67. With the key granted and no optional text, all controls succeed; with ungranted optional text, user-mode writes fail at the field check instead. The org was deleted; ScratchOrgInfo Status Deleted.

## Recorded facts

```text
source_api=62.0,67.0
route=istest
native_rows=144
```
