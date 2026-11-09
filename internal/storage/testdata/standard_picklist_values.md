# Standard picklist allowlist

`standard_picklist_values.json.gz` is a generated, sorted JSON array of values
from the [reviewed public catalog](https://github.com/glade-sh/glade/blob/capture-revision-002657554188335611102428712372866564827417835253969247806662978913607723094863/internal/storage/standard_describe_catalog.json.gz).
Custom entity suffixes (`__c`, `__e`, `__mdt`, `__x`) are excluded. Public platform
namespace values remain allowed. This is a distribution baseline, not proof
that every feature-specific value exists in every Salesforce org.

The allowlist also includes the standard Account.Type choices documented in
Salesforce Trailhead's [Create Record Types](https://trailhead.salesforce.com/content/learn/projects/customize-a-salesforce-object/create-record-types).
That lesson identifies Prospect, Customer - Direct, Customer - Channel,
Channel Partner/Reseller, Installation Partner, Technology Partner, and Other.
The independently captured API 62.0 and 67.0 observations in
`internal/apextest/testdata/conformance/picklist_defaults/org.tsv` confirm the exact
API spelling `Channel Partner / Reseller`, including spaces around the slash.
These choices add five values beyond the reviewed public catalog; they do not
add any values taken solely from an org export.

The allowlist additionally includes the four default Opportunity.Type choices
and three missing Lead.LeadSource choices independently observed through Apex
in an owned clean Developer scratch org. The complete ordered lists, labels,
and active/default flags agree at API 62.0 and 67.0 across anonymous, named,
and test-runner entries. See
`internal/apextest/testdata/conformance/standard_picklist_values/native/api62/org.tsv`
and the corresponding `api67` rows and `ORG-RUN.txt` files. This independent
capture supplements the reviewed distribution catalog; additions are not
extracted from the export being sanitized. Salesforce's
[Lead Source mapping](https://trailhead.salesforce.com/content/learn/superbadges/superbadge-data-quality-and-validation/fix-and-import-data)
also documents the five LeadSource labels. These seven additions are standard
labels without customer or package identifiers.

The allowlist also contains `Pickup`, independently observed through named Apex
at API 68.0 in a fresh Developer scratch org. The complete entry, label, active
and default flags are in `standard_describe_api68_drift/P0-field-access/org.tsv`;
it was observed after an explicit field grant and is not extracted from the
describe export. This generic platform term contains no customer or package
identifier. The observation makes no claim about other profiles or API versions.

Regenerate and verify from the pinned public catalog and the separately reviewed
additions above:

```sh
python3 scripts/generate-standard-picklist-allowlist.py
python3 scripts/generate-standard-picklist-allowlist.py --check
```

The resulting unique allowlist contains 59,050 values.
