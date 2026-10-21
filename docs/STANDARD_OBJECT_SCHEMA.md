# Standard Object Schema Baseline

`glade` treats Salesforce standard object metadata as runtime data, not as
project metadata. SFDX projects usually include only custom object metadata and
custom-field deltas, so the local runtime has to provide the standard spread
that Salesforce normally exposes through describe calls.

## Source Of Truth

The checked-in standard baseline lives in
`internal/storage/standard_schema_generated.go`. It is generated from public
Salesforce describe responses and carries a broad Enterprise feature spread
including Sales Cloud, Service Cloud, Person Accounts, Orders, Quotes, Products
and Schedules, Multi-Currency, State and Country Picklists, and Order
Management.

Regenerate the catalog with:

```bash
cd ../glade-tools
mkdir -p tmp/standard-describes
for obj in Account Contact Opportunity OpportunityContactRole Lead Order OrderItem Quote Pricebook2 Product2 Campaign CampaignMember Case Asset Contract Task Event User RecordType EmailTemplate ContentVersion ContentDocument ContentDocumentLink Attachment Document Organization UserRole Profile PermissionSet PermissionSetAssignment; do
  sf sobject describe --sobject "$obj" --target-org "$SF_TARGET_ORG" --json > "tmp/standard-describes/$obj.json"
done
node scripts/generate-standard-schema.mjs tmp/standard-describes ../glade/internal/storage/standard_schema_generated.go
```

The generator writes `internal/storage/standard_schema_generated.go`. Do not
edit that file by hand.

## Broad Describe Catalog Sanitization

The embedded `standard_describe_catalog.json.gz` is the authoritative input for
`standard_describe_catalog_v2.pack` and its generated Go index. Before packing a
new export from an authorized clean scratch org, run the first-party sanitizer
with the reviewed public picklist allowlist. It removes custom fields, custom reference targets and child rows,
non-master record types, and unreviewed picklist values. Platform namespace
values remain supported when present in the allowlist.

From the Glade checkout, with a sibling tools checkout:

```sh
node ../glade-tools/scripts/sanitize-standard-describe-catalog.mjs /tmp/standard-describe-export.json.gz internal/storage/testdata/standard_picklist_values.json.gz internal/storage/standard_describe_catalog.json.gz
node ../glade-tools/scripts/generate-standard-describe-pack.mjs internal/storage/standard_describe_catalog.json.gz internal/storage/standard_describe_catalog_v2.pack internal/storage/standard_describe_child_relationships_v2.pack internal/storage/standard_describe_catalog_v2_index_generated.go
```

See `internal/storage/testdata/standard_picklist_values.md` for the allowlist's
public provenance and regeneration. Review changes to the allowlist separately
from new org exports; never derive additions from the export being sanitized.
Both generators are deterministic. Run the storage catalog/digest tests and
update digest pins only after inspecting the intended data changes.

## Catalog boundaries

A clean-org export and the public picklist allowlist are separate inputs.
Neither an exported describe response nor a shape count proves runtime
behavior. Preserve the reviewed public allowlist provenance; do not promote
org-specific picklists, custom entities, or non-master record types into the
standard baseline.

The catalog can expose shape for excluded products. DataWeave, ConnectApi,
and Slack remain unsupported regardless of a generated object row. Local Files
behavior includes `ContentDocumentLinks` child queries on linked records;
relationship shape alone does not provide the hosted Files service.

## Stub Field Overlay

The describe baseline is the authoritative source for rich metadata such as key
prefixes, picklists, record types, and feature-gated fields. It does not cover
every platform object that large legacy projects reference. Glade therefore also
keeps a generated SObject field overlay derived from public Apex stub shape
data.

Regenerate the overlay with:

```bash
cd ../glade-tools
node scripts/generate-sobject-stub-overlay.mjs "$GLADE_APEX_SOBJECT_STUBS" ../glade/internal/storage/standard_sobject_stub_overlay_generated.go
```

The overlay generator reads API shape from an owned SObject stub corpus:
standard object names, field names, field labels, simple Apex field types, and
available relationship hints. It writes
`internal/storage/standard_sobject_stub_overlay_generated.go`. Do not edit that
file by hand.

The overlay is intentionally field-only. It fills broad compile/runtime gaps for
standard objects such as `AsyncApexJob` without replacing the richer
describe-driven catalog where Salesforce describe data exists.

System, Schema, Database, and product namespace Apex type shapes are refreshed
separately from the system stub corpus:

```bash
node scripts/generate-system-stub-symbols.mjs "$GLADE_APEX_SYSTEM_STUBS" internal/typesys/system_stub_symbols_generated.go
```

## Runtime Behavior

`internal/storage.EnsureStandardObjectFields` merges the generated base standard
schema and then the SObject stub field overlay into local object definitions.
Existing project metadata wins, so custom labels, project-defined fields,
validation rules, and record types are not clobbered.

Feature-gated standard fields are kept out of the base shape:

- `PersonAccounts` adds Account person fields such as `IsPersonAccount`,
  `PersonEmail`, `PersonContactId`, `PersonMailing*`, and the Person Account
  record types.
- `MultiCurrency` keeps the existing local behavior of adding
  `CurrencyIsoCode`, while the generated catalog also records standard
  object-specific currency field metadata captured from Salesforce.

`internal/sobject.BuildDescribeRegistry` now routes project metadata through the
same merge path, and `Schema.getGlobalDescribe` / `Schema.describeSObjects`
also expose known standard object shape without first seeding org records. Schema
describe, SOQL projection validation, DML validation, REST describe, and test
execution all see the same standard baseline.

## Current Baseline

The describe-driven generated spread covers these standard objects:

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

The active standard-object coverage report currently tracks 5,473 known
standard SObjects, all marked as `shape`, with 93,978 fields and 19,897
relationships. Twenty-eight locally modeled standard objects are marked as
`behavior`: Account, Attachment, CampaignMember, CampaignMemberStatus, Contact,
ContentDistribution, ContentDocument, ContentDocumentLink, ContentVersion,
Document, EmailMessage, EmailMessageRelation, FieldPermissions, Lead,
ObjectPermissions, Opportunity, OpportunityLineItem, PermissionSet,
PermissionSetAssignment, PermissionSetGroup, PermissionSetGroupComponent,
Pricebook2, PricebookEntry, Product2, Profile, RecordType, SetupEntityAccess,
and User.

The checked SObject stub inventory is maintained by the first-party compat
plugin.
It currently contains 1,373 SObject stub classes, zero source objects missing
active shape, and zero supported-feature fields missing active shape. The
remaining field gaps are feature-gated fields such as Person Account and
State/Country picklist fields. Shape entries improve SObject token lookup, SOQL
projection, DML/default field handling, and schema describe access, but they do
not assert full runtime behavior unless the row is marked `behavior`.
