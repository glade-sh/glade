Salesforce Developer Edition scratch org, captured 2026-11-24 and deleted after the run. See the fixture's `RUN-FACTS.txt` for the requested features and the uncaptured deliverability setting.

At Apex API 62.0 and 67.0, anonymous Apex and `@IsTest` called `Messaging.sendEmail(messages, false)`. A nonexistent well-formed org-wide sender ID returned `INVALID_CROSS_REFERENCE_KEY: invalid cross reference id`. An empty address lookup supplied a null ID and the call returned success. The test methods intentionally fail assertions to expose their results.

Rows and sources: `internal/apextest/testdata/conformance/absent_org_wide_email`.
