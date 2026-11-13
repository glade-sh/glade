# Native run: stopTest batch shapes

The `@IsTest` capture used Developer edition scratch orgs with no additional
features or settings, one fresh 30-day org per API. Both were deleted and
verified gone. API 62.0 ran from 2026-11-13 19:33:03 to 19:33:27 UTC;
API 67.0 ran from 19:33:41 to 19:34:05 UTC. Class and trigger metadata used
the corresponding API version. Salesforce CLI 2.157.8 and the campaign
capture driver adjusted for a 30-day org ran `StopShapeTest`.

Account markers recorded execution order through an intentional assertion
after `Test.stopTest()`. The 16 observations are in `org.tsv`; record and
org identifiers are omitted. The synthetic Apex and metadata are in
`project/`, copied from the existing boundary fixture; source hashes for the
shape classes are in `RUN-FACTS.txt`. Each row is one observation.
