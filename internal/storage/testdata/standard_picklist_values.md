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

Regenerate using the first-party tools checkout:

```sh
git show origin/main:internal/storage/standard_describe_catalog.json.gz > /tmp/public-standard-describe.json.gz
node ../glade-tools/scripts/sanitize-standard-describe-catalog.mjs --extract-allowlist /tmp/public-standard-describe.json.gz internal/storage/testdata/standard_picklist_values.json.gz
```

Use the linked public revision when reproducing this version of the allowlist.
Then add the five documented Account.Type choices missing from that revision
and sort the unique values by UTF-8 bytes before writing the gzip JSON array.
The sanitizer prints counts and never prints rejected values.

Finally add the seven independently captured Opportunity.Type and Lead.LeadSource
values above before sorting and writing the gzip array. The resulting unique
allowlist contains 59,049 values.
