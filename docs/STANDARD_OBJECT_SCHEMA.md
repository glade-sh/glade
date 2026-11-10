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
edit that file by hand. It preserves the describe `restrictedPicklist` facet.
The packed-describe decoder carries the same facet into runtime fields.

To refresh only this facet from the authoritative embedded describe input while
preserving the existing reviewed feature spread and all other field metadata:

```sh
node ../glade-tools/scripts/generate-standard-schema.mjs internal/storage/standard_describe_catalog.json.gz internal/storage/standard_schema_generated.go --refresh-restricted-picklists
```

This mode leaves fields without an explicit describe facet unchanged.

Describe restriction and DML enforcement have different scopes. The explicit
configuration-field table in `internal/storage/standard_picklist_validation.go`
excludes divisions, currency codes, locale/language/time-zone keys, and state
and country integration codes from catalog enforcement. Describe still exposes
the native restricted facet. The locale-boundary captures demonstrate why a
single describe snapshot is not a complete source of DML values.

The exclusion applies only to restrictions supplied by the embedded catalog.
Storage and describe conversions retain that provenance. Explicit project and
pinned-schema restrictions remain enforced, as does the existing native-backed
`User.TimeZoneSidKey` rule. Persisted restrictions without a provenance marker
keep their existing validation. Locale formatting and other existing validation
paths are unchanged.

Project StandardValueSet declarations supply the active describe values through
schema loading. Native also permits Order.Draft after a deployment omits it
from describe; the separate per-field DML value table preserves that captured
case without inferring acceptance for other omitted values. The native `project-valuesets` capture
covers ContractStatus, OrderStatus, and TaskStatus at APIs 62/67 on anonymous
and test routes. Standard fields without an explicit value-set reference use
these captured bindings; other explicitly named field value sets use the same
loader. No custom picklist gains standard baseline values.

The `broad-sample` captures exercise 14 more standard restricted fields across
Opportunity, Task/Event, Contract, Order, Quote, and ContentVersion, including
invalid insert/update and valid/null boundaries. All agree across APIs 62/67
and both routes; Case.Status remains an unrestricted control.

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
python3 scripts/generate-standard-field-order.py
```

The native Account, Lead, and Order field supplement is captured in
`internal/apextest/testdata/conformance/standard_field_supplement`. Apply its
verified missing fields to the broad input with
`python3 scripts/apply-standard-field-supplement.py`, then run the pack and
field-order generators above. Rich facets are taken from the existing P0
capture after matching the new API 62/67 rows. Picklist lists are included
only when every value is in the reviewed public allowlist; a partial list is
omitted so it cannot override a complete native compatibility overlay. The explicit policy sample for
`Lead.LastTransferDate` is regenerated with the field-policy generator below.
The supplement describes one Developer org and does not establish absence in
other feature profiles.

`python3 scripts/apply-standard-reference-supplement.py` replaces the
PrivacyRTBFRequest docs placeholders with the P0 API 67.0 describes, adds
PrivacyPolicyDefinition, and adds PrivacyRTBFRequest to every catalog reference
field whose P0 describe lists it at both API 62.0 and 67.0
(`plain_global_describe` confirms the Task and Event WhatId rows). Run the pack
and field-order generators after it. PrivacyPolicyDefinition keeps the API 67.0
field list; IsAllocatePermset is not in the API 62.0 describe. Regenerate the
names-only lookup with `python3 scripts/generate-standard-describe-catalog-names.py`.
The same script adds the objects that join the plain Developer global describe
after API 62.0 and have no P0 API 62.0 describe (DevopsEnvDeployment,
DevopsEnvDeploymentMbr and their Feed and History objects; the PayGroup and
PayPeriod placeholders), and adds DevopsEnvDeployment, DevopsEnvDeploymentMbr,
PayGroup, PayPeriod and SfdcPartnerSbscrOffer to the reference fields whose P0
API 67.0 describe lists them (`whatid_targets_tiebreak` confirms Task and Event
WhatId: 85 targets at 62.0, 93 at 67.0). The plain Developer object lists keep
them out of API 62.0; the scalars follow with
`GLADE_UPDATE_DESCRIBE_SCALARS=1 go test ./internal/storage -run TestStandardDescribeCatalogV2ScalarsMatchPack`.

`python3 scripts/apply-standard-restricted-supplement.py` sets the
restrictedPicklist facet on docs-placeholder picklists that the P0 describes
report as restricted at API 67.0 (and at 62.0 where the field exists). It
fills an empty value list only when both APIs list the same values and every
value is in the reviewed public allowlist. Run the pack and field-order
generators after it. Catalog provenance keeps DML enforcement limited to the
captured fields in `internal/storage/standard_picklist_validation.go`.

Objects whose field types come from the Apex SObject stub overlay type their
picklists as String. `python3 scripts/generate-standard-stub-restricted-picklists.py`
writes `internal/storage/standard_stub_restricted_picklists.json.gz` from the
P0 describes: every field reported as a restricted picklist at each API where
it exists. A value list is kept only when it is non-empty, equal at both APIs,
has no null label, no default entry and no dependent controller, and every
value is in the reviewed public allowlist. The runtime applies an entry only
to a field the stub overlay types String, with catalog provenance, so DML
enforcement is unchanged.

The plain Developer global describe lists in
`internal/apextest/testdata/conformance/plain_global_describe` feed
`internal/storage/plain_developer_objects_generated.go` through
`scripts/generate_order_management_objects.py`. A project whose declarations
are all captured plain Developer shapes sees only those standard objects.

The field-order generator writes `internal/storage/standard_field_order.tsv.gz`
from the same authoritative catalog. The packed describe format sorts fields;
this names-only projection preserves the original describe sequence for
`FIELDS(STANDARD)` whole-SObject JSON. It is decoded lazily on the first field
sequence lookup, without decoding the broad catalog. Do not edit it by hand.

See `internal/storage/testdata/standard_picklist_values.md` for the allowlist's
public provenance and regeneration. Review changes to the allowlist separately
from new org exports; never derive additions from the export being sanitized.
All generators are deterministic. Run the storage catalog/digest tests and
update digest pins only after inspecting the intended data changes.

## Catalog boundaries

A clean-org export and the public picklist allowlist are separate inputs.
Neither an exported describe response nor a shape count proves runtime
behavior. Preserve the reviewed public allowlist provenance; do not promote
org-specific picklists, custom entities, or non-master record types into the
standard baseline.

The catalog can expose shape for excluded products. DataWeave and Slack remain
unsupported regardless of a generated object row. Local Files
behavior includes `ContentDocumentLinks` child queries on linked records;
relationship shape alone does not provide the hosted Files service.

Some catalog fields name reference targets that no scratch org describes
(`Assessment.AssessmentStagedDataId`, `DocumentGenerationProcess.DocGenerationBatchProcessId`,
`DataStream`, `BusinessUnit`, and others). `referenceOnlyStandardObjectNames` in
`internal/storage/standard_reference_only_generated.go` lists every catalog reference
target that is not a known standard object. It is derived from the catalog, not
hand-written. After a catalog regeneration, run
`GLADE_UPDATE_REFERENCE_ONLY=1 go test -run TestGenerateReferenceOnlyStandardObjectNames ./internal/storage`
and review the diff. Field reference metadata admits these names so semantic analysis
does not reject the owning object. Reference-only membership does not register a
global standalone Apex type or materialize an object in test orgs.

Native API 62.0/67.0 captures cover all 99 names. Ninety-eight reject bare and
`Schema`-qualified construction, local declarations and static `SObjectType` access;
global describe and `Type.forName` return null. Six also have captured lowercase
construction, DML and null-describe dereference observations. The conformance guard
requires exact case bodies and both APIs/routes for every name.

`FieldServiceMobileExtension` has a captured named-body exception: construction,
local declarations, static/instance describe and global lookup are available in
named code, while anonymous source rejects it. Insert DML is rejected in named
code. Global lookup availability follows the executing class frame; an anonymous
caller still receives null after returning from a named helper. Bare and
`Schema`-qualified `Type.forName` return a type in named execution at APIs 62–66,
and null at API 67. Reflection stays null throughout Tooling anonymous transactions,
including inside a named helper. This allowance is separate from the global
standard-object catalog and preserves owned Apex class precedence.

See `internal/apextest/testdata/conformance/reference_only_names` for anonymous,
`@IsTest`, same-transaction context and version-bisection observations, including
an owned Apex class with the same name as a reference target. Describe and query
boundaries for the first two names are captured in
`internal/storage/testdata/standard_reference_targets_native`.

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

## Captured standard field policy

`internal/storage/standard_field_policy.json.gz` is a separate field-policy
sidecar. It leaves the generated schema and describe packs intact. Its version-1
format contains baseline `objects` (`present`, rich describe `fields`, and
explicit `absent` candidates), `gates` (object, field metadata, and an all-of
`features` condition), and SHA-256 `sources` relative to the owned
`internal/apextest/testdata/conformance/standard_describe_profiles` captures.

The baseline includes only the historical candidates and sample fields listed
in `cases.json`, on objects with a successful P0 describe. Positive named Apex
membership overrides REST absence; named-only fields keep their existing
catalog metadata. Rich field facets come from P0's granted API62 describe.
Absence is recorded only for those enumerated candidates, never for an
unrequested object or an arbitrary field missing from REST.

The `standard_industries_schema` captures add three independently observed
field gates to this sidecar: ContactsToMultipleAccounts exposes
AccountContactRelation.Roles as a multi-select picklist; B2BCommerce,
Communities, and enabled commerce expose PricebookEntry.ProductSellingModelId;
and `nameSettings.enableMiddleName` exposes User.MiddleName. The generator
checks isolated controls and both API endpoints against the combined-profile
anonymous and named rows. The middle-name setting is included in the project
feature signature, so warm overlays refresh when it changes. Captured
multi-select type metadata overrides the matching generated scalar stub only;
explicit project field definitions retain precedence.

The gate input is the effective-membership additions for isolated P1
(MultiCurrency), P2 (PersonAccounts) and P3 (StateAndCountryPicklist). The
generator requires identical API62/API67 gate membership and takes rich
metadata from the matching API62 granted describe. Four existing Person Account
address-code fields require the isolated PersonAccounts + StateAndCountryPicklist
capture in `standard_field_policy_combinations` on anonymous, named and test-runner
routes at both APIs. PU supplies their rich facets after that independent check;
PU-only differences never establish a gate. The existing Account.IsActive/HealthCloud and
Opportunity.SyncedQuoteId/Quotes gates are explicit `legacy` entries without
new field facets. Other profile, edition, provisioning and API-version deltas
are not attributed to new runtime gates by this sidecar.

The maintainer toolchain builds the sidecar from the native captures. Its `sources`
pin the digest of each published capture file. Verify the shipped sidecar against
those captures with `go test ./internal/storage -run StandardFieldPolicy`.

The generator reads API62 and API67 raw named maps, granted describes and global
inventories for P0-P11 and PU. It checks the published differences against those
raw captures and records every source digest. The `evidence.profiles` section
retains field/object additions and removals separately by API, including the
Enterprise and union controls. Other P4-P10 field differences remain evidence for a
separate sidecar expansion; reading them does not install new runtime field gates.
For API68, the generator applies the separately captured 67-to-68 granted REST,
named-map and global-inventory changes to the stored API67 observations before
comparing each profile to its captured baseline. When the resulting profile
delta equals API67, `evidence.profile_aliases` records the observed equality
instead of duplicating the delta. This retains the profile comparison; it does
not itself install API68 runtime fields or establish unobserved combinations.
Storage retains these deltas and exposes them through
`StandardDescribeCatalogProfileDelta(profile, api)`. Profile identifiers and API
versions are exact capture keys, such as `P10` and `67.0`; an uncaptured key
returns no result. The result names its baseline profile and separates added
and removed fields (pairs of object and field names) from added and removed
objects. P10 is relative to P4, while the other profiles are relative to P0.
An empty captured delta remains distinguishable from a missing capture.
Returned slices belong to the caller. This lookup preserves observations;
it does not select a profile from project features or attribute new field gates.
Missing native combination routes abort generation. JSON keys and fields are
sorted and gzip has no filename or timestamp. `--output` supports reproduction
in a temporary file; `--check` compares bytes without changing the output.
`TestStandardFieldPolicyNativeSidecarProvenance` binds every rich
field to its native describe and checks the input digests; the native storage
and project conformance tests independently verify membership and behavior.

API68 runtime describe additions are generated separately from the owned
`standard_describe_api68_drift` captures by the maintainer toolchain. Verify the
shipped sidecar against the published captures with
`go test ./internal/storage -run StandardAPI68Describe`.

The sidecar records source digests, exact profile definitions, per-route field
visibility, and new-object shapes. It selects only an observed edition and
feature combination. API62 and API67 remain separate from the API68 projection;
the generator checks the new-object version boundaries against both REST and
Apex observations. Native field grants establish the two captured named-only
field access transitions. Unobserved feature combinations do not acquire an
inferred profile. The P0 named RequestType picklist uses the independently captured list only
after the explicit field grant; its values must also be present in the separately
reviewed public allowlist above. Other profiles gain a list only when their own
native named-route capture is available.

Gates apply to catalog additions. Explicitly configured fields survive every
profile and refresh. A pre-overlay snapshot records their origin, including an
empty snapshot for a bare catalog. Older persisted overlays without that
snapshot retain their existing fields during migration: the old overlay marker
alone cannot prove which fields were generated. Newly generated fields continue
to leave when their enabling profile is removed. Non-Master record-type metadata
independently supplies `RecordTypeId`, including on objects whose bare catalog
omits it; the Master mapping alone does not enable an absent field.

API68-only object projections also use the pre-overlay field snapshot. A fresh
catalog-only definition records an empty configured set; an explicitly configured
field survives even if it equals an older generated field. A persisted definition
without a snapshot retains its old fields because their origin is unknown. This
metadata preserves local configuration; it does not infer visibility for an
uncaptured profile.

The org shape comes from the effective project: its root and all loaded source
dependencies, managed-package sources and shims, at every dependency depth.
Feature checks, runtime schema and startup-cache identity share that feature set.

The Account, Contact, EmailMessage, Lead and PricebookEntry constructor surfaces
are bound to their full positive named map and FLS-granted describe, including
captured reference relationship names. Contact, Lead and PricebookEntry require
the explicit object allowlist in `standard_constructor_complete/cases.json` and
its native `org.tsv` plus `ORG-RUN.txt`: missing-field rejection, accepted field
boundary and dynamic `put()` twins at APIs 62/67 on anonymous, named and test-runner
routes. The generator verifies the complete route matrix, source digests and
matching API surfaces before installing their constructor lists. A successful
describe on an unqualified object does not close its fallback. Known fields
from retained profiles whose field expansion is deferred keep their previous
constructor fallback, including reference relationship names. Captured baseline
absences and feature gates retain precedence. Their native field membership is
not a new constructor-acceptance claim; profile activation remains separate.
WebStore uses the union of the P8 and P10 provisioned surfaces, each verified
identical at API62/67; the isolated OrderManagement capture additionally pins
unknown-field rejection. Effective configured fields take precedence over these
catalog constructor lists. Profile-specific WebStore field availability remains
part of the separate feature-field expansion.
Constructor inference cannot manufacture fields outside that surface; custom
fields continue to require project metadata. EmailMessage additionally accepts the constructor-only `ToIds` field. Its null
and list constructors, unknown-field rejection and Subject boundary are captured
at APIs 62/67 in `standard_constructor_virtual_fields/{org.tsv,ORG-RUN.txt}` on
anonymous, named and test-runner routes. The field remains absent from the bare
describe catalog. This does not close other partially captured standard objects. Constructor availability for WebStore is
bound to the P8 OrderManagement and P10 enabled/license-only controls. The
`required_features_any_of` alternatives are `OrderManagement`, or all three of
`B2BCommerce`, `Communities`, and `CommerceEnabled`. The last marker
represents the captured `commerceSettings.commerceEnabled=true` setting;
licensing alone does not enable the object. Semantic constructor checks use the
index's feature profile on both anonymous and named routes, before accepting
source-inferred fields.

Account.RecordTypeId has a separate `record_types` enablement alternative:
a configured non-master record type also exposes it without Person Accounts.
The same-org baseline/record-type capture in
`standard_recordtype_field_policy/{org.tsv,ORG-RUN.txt}` proves both membership
and constructor compilation at APIs 62/67 on anonymous, named and test-runner
routes. The sidecar source hashes include that sibling oracle. Persisted
refreshes and semantic constructor checks retain project record types;
feature-generated record types are removed when their feature is disabled.
Explicit project RecordTypeId reference fields and EmailMessage.ToIds fields are
retained across overlay refreshes, independently of the bare catalog baseline.
The project loader maps the captured Commerce setting to CommerceEnabled for
both SFDX and CumulusCI scratch definitions.

EmailMessage also declares an insertion-only `input_fields` entry for ToIds,
separate from describe and constructor membership. The native
`standard_emailmessage_input_fields` matrix captures constructor/setter, null,
unknown-field and no-ToIds boundaries at APIs 62/67 on anonymous, named and
test-runner routes. Twenty successful insertions are replayed; four API67
named/test-runner non-null list insertions remain explicit typed-DML carries.
Their intermediate API boundary is unbisected. This admission policy does not
change update or matched-upsert field validation; upsert-create uses insertion.

### Incomplete project org shapes

A source tree is a metadata delta, not a complete inventory of the target org.
When the consuming project does not declare an org shape,
Glade retains existing catalog fields instead of removing them based on a fresh
scratch org's absence table. For example, Apex using `Account.RecordTypeId`
continues to work when the record types are configured outside the source tree.
This is an offline compatibility fallback, not a claim that the field exists in
a fresh Salesforce org.

A valid scratch definition declaring an edition, features, or settings, or an
explicit `org.features` list in `glade.yml` (including `[]`), selects the captured
field gates. Features from every loaded dependency still apply. A dependency's development
shape cannot prove that the consumer lacks a field. Declared fields
and non-Master record types remain positive evidence and are never withheld.
Unrelated configuration, malformed scratch definitions and an object's missing
metadata do not prove absence. Low-level storage calls with an explicit empty
feature profile continue to represent the captured bare org.

Uncertainty retains existing catalog fields; it does not enable an org feature,
provision a new feature-only object, synthesize every optional feature field, or
relax captured constructor-only and insertion-only restrictions. Existing legacy
feature gates remain in force. The internal uncertainty token participates in
schema and startup-cache signatures, so declared and unspecified shapes cannot
reuse each other's prepared overlays.

Applying a new project shape replaces its certainty state. Adding a runtime
capability (for example Platform Cache during a query) preserves that state;
it cannot turn incomplete source metadata into a declared bare org.

### Feature describe sidecar

`scripts/generate-standard-feature-describes.py` packs the owned feature-profile
captures into `internal/storage/standard_feature_describes.json.gz`. Run it after
updating those captures; use `python3 scripts/generate-standard-feature-describes.py
--check` to verify the committed sidecar without writing it. The bundle carries
captured object facets, field version and route membership, and explicitly observed
read-only fields. Describe create/update facets alone do not establish Apex write
behavior.
