# Standard Object Schema Baseline

`oaer` treats Salesforce standard object metadata as runtime data, not as
project metadata. SFDX projects usually include only custom object metadata and
custom-field deltas, so the local runtime has to provide the standard spread
that Salesforce normally exposes through describe calls.

## Source Of Truth

The checked-in standard baseline is generated from public Salesforce describe
responses captured from the `oaer-probe-lab` scratch org. The scratch org shape
lives in `probes/sfdx/config/project-scratch-def.json` and enables a broad
Enterprise feature set including Sales Cloud, Service Cloud, Person Accounts,
Orders, Quotes, Products and Schedules, Multi-Currency, State and Country
Picklists, and Order Management.

Regenerate the catalog with:

```bash
mkdir -p tmp/standard-describes
for obj in Account Contact Opportunity OpportunityContactRole Lead Order OrderItem Quote Pricebook2 Product2 Campaign CampaignMember Case Asset Contract Task Event User RecordType EmailTemplate ContentVersion ContentDocument ContentDocumentLink Attachment Document Organization UserRole Profile PermissionSet PermissionSetAssignment; do
  sf sobject describe --sobject "$obj" --target-org oaer-probe-lab --json > "tmp/standard-describes/$obj.json"
done
node scripts/generate-standard-schema.mjs tmp/standard-describes internal/storage/standard_schema_generated.go
```

The generator writes `internal/storage/standard_schema_generated.go`. Do not
edit that file by hand.

## Runtime Behavior

`internal/storage.EnsureStandardObjectFields` merges the generated base standard
schema into local object definitions. Existing project metadata wins, so custom
labels, project-defined fields, validation rules, and record types are not
clobbered.

Feature-gated standard fields are kept out of the base shape:

- `PersonAccounts` adds Account person fields such as `IsPersonAccount`,
  `PersonEmail`, `PersonContactId`, `PersonMailing*`, and the Person Account
  record types.
- `MultiCurrency` keeps the existing local behavior of adding
  `CurrencyIsoCode`, while the generated catalog also records standard
  object-specific currency field metadata captured from Salesforce.

`internal/sobject.BuildDescribeRegistry` now routes project metadata through the
same merge path, so schema describe, SOQL projection validation, DML validation,
REST describe, and test execution all see the same standard baseline.

## Current Baseline

The initial generated spread covers these standard objects:

`Account`, `Contact`, `Opportunity`, `OpportunityContactRole`, `Lead`,
`Order`, `OrderItem`, `Quote`, `Pricebook2`, `Product2`, `Campaign`,
`CampaignMember`, `Case`, `Asset`, `Contract`, `Task`, `Event`, `User`,
`RecordType`, `EmailTemplate`, `ContentVersion`, `ContentDocument`,
`ContentDocumentLink`, `Attachment`, `Document`, `Organization`, `UserRole`,
`Profile`, `PermissionSet`, and `PermissionSetAssignment`.

The generator preserves field names, labels, display/storage types, required
flags for create-time required fields, references, relationship names,
picklists, record types, and key prefixes. It intentionally does not model full
layout metadata, permissions, security enforcement, automation behavior, or
feature provisioning side effects beyond the explicit feature-gated schema
overlays above.
