# Native run: Case entitlement fields in a plain Developer org

Question: the v0.3.1 audit saw Glade expose Case.EntitlementId in a plain
Developer shape while native describes did not. The conformance packet
org_feature_surfaces records `Case.EntitlementId` as present. Which is right?

Orgs (fresh 30-day Developer-edition scratch orgs; no ids or usernames kept):
- `plain` and `plain2`: default settings (two separate orgs).
- `shape` and `shape2`: feature StateAndCountryPicklist and the Lightning
  Experience setting (the org_feature_surfaces `currency_off` shape).
- `entitle`: the entitlementSettings.enableEntitlements setting from the
  scratch definition (the retrieved settings read `true`).
- `plain2-fls-granted`: `plain2` after one permission set granted read and
  edit on the 11 Case fields below to the running user.

Source API 62.0 and 67.0. Routes:
- `anonymous`: Tooling executeAnonymous at that API (cases.json; `E00` returns
  a JSON map of describe results, the others end in a thrown `ok`).
- `anonymous-calls-named-class` and `istest`: one deployed class (class API
  equal to the API under test) whose `observe()` returns the same kind of map,
  run twice per route (rows repeat). `staticQuery` selects EntitlementId,
  BusinessHoursId, IsStopped and MilestoneStatus; `dynamicQuery` selects
  EntitlementId.

Results:
- Inline anonymous Apex does not see these 11 Case fields in any baseline org:
  AssetWarrantyId, BusinessHoursId, EntitlementId, IsClosedOnCreate, IsStopped,
  MilestoneStatus, ProductId, ServiceContractId, SlaExitDate, SlaStartDate and
  StopStartDate. The describe map has 35 Case fields instead of 46, the static
  token and static query fail to compile (`Variable does not exist: ...`,
  `No such column ...`), and the setting `enableEntitlements` changes nothing.
- A deployed class (named code, called from anonymous Apex or run as a test)
  sees all 11 fields at 62.0 and 67.0 in every baseline org, with the same
  describe in the shape orgs.
- Inside the named class, the static and dynamic queries succeed at API 62.0.
  At API 67.0 they throw `System.QueryException: No such column 'EntitlementId'
  on entity 'Case'` in the baseline orgs.
- After the field permissions are granted, every route sees and queries all 11
  fields at both versions.
- Controls: `Contact.IsPersonAccount` and `Opportunity.SyncedQuoteId` are absent
  in every route; `Case.ContactId` is present; `Entitlement`,
  `EntitlementContact`, `MilestoneType`, `SlaProcess`, `CaseMilestone` and
  `ServiceContract` are describable.

So the audit and the older packet are both right for their route: the fields
are hidden from anonymous Apex and from API 67.0 queries because the default
administrator has no field-level security on them, and the permission set
control shows the field grant is the variable.

Not changed in Glade (parked): Glade shows the 11 fields in every route and
accepts the API 67.0 named queries. Matching native needs the default
administrator's field grants to cover these standard fields in anonymous
describe, token and query paths and in API 67.0 plain queries, the same
mechanism v0.3.5 added for optional custom fields. The describe-only rows are
matched by the existing named-route behavior and are pinned by the storage test.

Also found: the REST describe of Case lists 46 fields at 62.0 and 67.0 in `plain2`,
including `ServiceContractId` (reference to ServiceContract, label "Service
Contract ID") and `AssetWarrantyId` (reference to AssetWarranty, label "Asset
Warranty ID"), both createable, updateable and filterable, length 18. Glade's
Case catalog has 44 fields and lacks these two. Parked with the rest.
