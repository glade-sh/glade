# Native run

Owned synthetic Contact custom external-ID probe (`Contact.RouteKey__c`), fresh 1-day Developer scratch org, 2026-11-10. The route is an Aura application whose Apex controller action (`@AuraEnabled`) ran the upsert as the default System Administrator, loaded in a headless Chrome session (Lightning controller route; not an Apex test and not anonymous Apex). `grant=none` means no permission grant on the key; `grant=permset` means a permission set granting read/edit on the key was assigned to the administrator. Class API 67.0 (rows with source_api 67.0 and the control) and 62.0 (redeployed class, assignment removed). Update rows upsert onto an existing record, new rows onto a fresh key. The ungranted 67.0 rows were captured twice. The org was deleted; deletion was verified.

## Recorded facts

```text
source_api=62.0,67.0
route=lightning-controller
administrator, key ungranted, 67.0: statement and Database.upsert (default and allOrNone=false) throw SObjectException "Field Contact.RouteKey__c is inaccessible in this context"
administrator, key granted by an assigned permission set, 67.0: all forms succeed
62.0, key ungranted: all forms succeed
```
