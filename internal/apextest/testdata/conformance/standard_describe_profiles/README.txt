Standard describe profile captures

Each profiles/<profile>.json is an isolated scratch definition. P0 is the
Developer baseline; P11 changes only the edition to Enterprise. P1-P9 vary
one feature or the explicitly listed settings group. P10 adds B2BCommerce to
the Communities configuration; Salesforce rejected B2BCommerce without that
prerequisite. P10/setup/rejected-definition.json and P10/capture-blocked.json
retain the rejected input and provisioning answer. P10 therefore compares to
P4, while the other profiles compare to P0. The baseline_profile column makes
this distinction explicit. PU is the original
nine-feature union control: it includes SalesUser and ServiceCloud and does
not include B2BCommerce or ContactsToMultipleAccounts. Do not attribute PU-only
differences to an individual feature or infer gates for combinations not run.

profile-status.tsv records capture status, exact definition digest and the
observed effective membership additions/removals relative to baseline_profile at both APIs.
feature-set-differences.tsv preserves each field difference separately for
anonymous default, anonymous granted, named default and effective membership.
Effective membership is positive named default presence union granted REST
presence over the 65 case-table objects. REST absence cannot disprove named
Apex presence. No fieldsComplete flag or installed feature-gate sidecar is
produced by these capture fixtures.

global-object-differences.tsv compares advertised clean-name standard inventories. REST
exports for individual profiles cover the 65 case-table objects plus objects
advertised beyond P0; P0 and PU export all advertised standard objects. An
unrequested object is not an observed absence. Per-object request statuses,
raw public describe metadata, separate anonymous/named field maps, native
EntityParticle rows and FieldDefinition counts are retained by API/context.

P0/org.tsv pins constructor, dynamic put/get/describe, rejection and boundary
routes as anonymous, named compilation and project test-runner observations.
P11/slice-org.tsv carries the same Enterprise visibility matrix. Each
slices.json retains the owned source and source digest. Other profile org.tsv
files export anonymous/named membership observations before and after the
readable field grant; setup contains the exact class and permission sources.

fls-delta.tsv is a same-org permission comparison, distinct from feature
profile differences. historical-extra-fields.tsv records each of the 416
historical candidates. Native named maps and granted REST can still disagree
on non-permissionable fields; visibility-boundaries.tsv preserves that limit.

Tenant-specific ManagedContentChannel.Domain picklist values are omitted.
They identify the source scratch and are not standard choice-list evidence.
All orgs are owned, ephemeral scratches; ORG-RUN.txt records definitions,
transport, grant scope, capture digests and verified deletion. CI consumes
only these fixtures and never connects to Salesforce.

Enterprise provisioning advertises Knowledge__ka and Knowledge__kav as custom
objects with no namespace. The fresh owned org had zero installed packages;
setup/knowledge-boundary-*.json records those checks. These two documented
Salesforce Knowledge objects are excluded by the clean-name export, and no
article bodies are captured. Unknown custom/package objects still refuse the
export. Native article__Share template choices in ObjectDataImport and its
ChangeEvent picklists are omitted, with exact paths in privacy-omissions-*.json.
Knowledge object references:
https://developer.salesforce.com/docs/platform/field-reference/references/field-reference/salesforce-field-reference-knowledge__ka.html
https://developer.salesforce.com/docs/platform/field-reference/references/field-reference/salesforce-field-reference-knowledge__kav.html
Commerce prerequisite:
https://developer.salesforce.com/docs/platform/sfdx-dev/guide/sfdx-dev-example-orgs-def-file-config-values.html

P11/version-bisect.tsv pins PermissionsViewLicenseUtilization on PermissionSet
and Profile across APIs 63-66, in addition to the 62/67 field maps. It first
appears at API 65 on anonymous and named routes. version-sources.json retains
the exact probe sources and matching hashes; these ran before the field grant.

P10 additionally sets commerceSettings.commerceEnabled=true. The complete
license-only control (same B2BCommerce + Communities features, without this
setting) is retained under P10/license-only, with its exact profile-definition.
That control exposes no WebStore in either named map. The enabled profile
exposes 46 WebStore fields at both APIs. P10/enablement-differences.tsv compares
the setting variant against that control; the control's field-differences.tsv
compares licensing against P4. These remain observed configuration differences,
not installed runtime gate rules.
Commerce setting reference:
https://developer.salesforce.com/docs/atlas.en-us.api_meta.meta/api_meta/meta_commercesettings.htm
