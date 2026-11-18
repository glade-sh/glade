# Native run

Owned synthetic Account custom external-ID probe (`Account.A36External__c`, text, external ID, not unique), fresh 1-day Developer scratch org, 2026-11-10. The field was deployed by source with no profile or permission-set grant; `sf apex run --api-version <api>` ran the probe in anonymous Apex at 62.0 and 67.0. With no grant, anonymous Apex that names the field (`Account.A36External__c`, `new Account(A36External__c=...)`) fails to compile for the default System Administrator: "Field does not exist: A36External__c on Account". After a permission set granting read/edit on the field was assigned, the same upsert and insert succeed at both APIs (the 67.0 upsert row reports created=false because the 62.0 run had already persisted the record). The earlier owned DML-transaction conformance rows (`dml_transactions.json`) compile and run the same shape of statements, so their source org had the field visible to the administrator; the fixtures that reproduce them need the grant. The org was deleted; deletion was verified.

## Recorded facts

```text
source_api=62.0,67.0
route=anonymous
grant=none: compile error, field does not exist
grant=permset: upsert and insert succeed
```
