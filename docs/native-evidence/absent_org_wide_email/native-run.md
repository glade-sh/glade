Salesforce Developer scratch org on a campaign Dev Hub, captured 2026-11-24. The 30-day org was deleted and verified gone.

At Apex API 62.0 and 65.0, an absent org-wide email address was queried by display name with `WITH SECURITY_ENFORCED`. A caught exception left the newly constructed record's Id null. Anonymous Apex and an `@IsTest` method confirmed the null Id; a second `@IsTest` method asserting a non-null Id failed with the recorded assertion. The cases do not distinguish the caught exception's cause.

Rows: `internal/apextest/testdata/conformance/absent_org_wide_email/org.tsv`. There were no DML, email or asynchronous operations in these cases.
