Owned synthetic capture, 2026-11-07 UTC (v0.3.5 lane C3, Doot k6c2f8a1d93e): NULLS FIRST/LAST on standard lookups.
Org: org A of whatid_targets_tiebreak (fresh one-day plain Developer scratch org from a campaign Dev Hub, Latest Release,
max API 68.0, en_US, America/Los_Angeles). Org ID not retained. Synthetic data only.
Field list: REST global describe at 67.0 (579 createable, queryable standard objects), then each object's describe;
447 reference fields are createable, nillable and sortable (project/targets.json).
Generic probe (source "generic"): project/generate_generic_probe.py wrote NullCore62/67 (helpers) and NullProbe62_*/67_*
(one @IsTest method per object and up to five fields). For each field it creates two parents of the first creatable
target type (Account first; Users through System.runAs; otherwise two existing rows), renames the lower-Id parent
'zz null probe' and the higher 'aa null probe' when Name is updateable, inserts three children (lookup null, lower
parent, higher parent) with required fields filled from describe, runs SELECT Id FROM <Obj> WHERE Id IN :ids ORDER BY
<field> NULLS FIRST | NULLS LAST | ASC | DESC | DESC NULLS LAST, and rolls back. Orders are written as N (null),
L (lower parent Id), H (higher parent Id). describe facets come from the same run. A field it could not build is a row
with "skip" and the reason; at 67.0 more inserts failed ("fields being inaccessible") because of describe defaults.
Hand probe (source "hand"): NullCoreHand62/67 build 33 core lookups with explicit constructors and insert as system.
Facts: fieldDefinition67 (FieldDefinition DataType, IsIndexed, IsPolymorphicForeignKey, IsNameField, IsCompound,
RelationshipName) and entityParticle67 (IsIdLookup, IsCalculated, IsHighScaleNumber), queried once at API 67.0.
This is a stratified sample, not all ~300 lookups: 270 probed rows over 156 fields and 59 objects (core CRM, Field
Service, consent, DevOps, user provisioning, email), 662 skip rows.
Findings, identical at 62.0 and 67.0 wherever both probed, and between the generic and hand probes:
 Honour (NLH, LHN, NLH, NHL, HLN): Contact.ReportsToId, Opportunity.Pricebook2Id, Order.Pricebook2Id, WorkOrder
 lookups, ServiceContract.AccountId, Location.ParentLocationId and every lookup on the newer objects probed.
 Ignore (NLH, NLH, NLH, HLN, HLN: nulls first ascending, last descending): 31 lookups on Account, Asset, Campaign,
 Case, CommSubscriptionConsent (EngagementChannelTypeId), Contact, Contract, EmailMessage, Event, Lead,
 Opportunity, Order, Task and UserRole (listed in internal/soql/null_order_standard_lookups.go).
 Sorting follows the parent Id, not the parent Name. No describe, FieldDefinition or EntityParticle facet separates the
 two groups (Contact.AccountId and Contact.ReportsToId have identical facts). CommSubscriptionConsent.DataUsePurposeId
 returned insertion order (NLH) for every clause and is left unexplained.
Cleanup: org deleted; ScratchOrgInfo shows Status Deleted.
