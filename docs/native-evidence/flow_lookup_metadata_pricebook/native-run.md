# Native capture: Get Records on custom metadata and on the standard pricebook

Two after-save create Flows. One looks up a custom metadata record by DeveloperName from `$Record.Title` (rows `yes|no|no` for an existing name, a missing name and a null Title). The other looks up Pricebook2 by Id from `$Record.Pricebook2Id` on an Opportunity. Anonymous Apex finds the standard pricebook (`yes`); inside a test that uses `Test.getStandardPricebookId()` the lookup finds nothing (`no`).

Fixture: `internal/apextest/testdata/conformance/flow_lookup_metadata_pricebook` (org.tsv, RUN-FACTS.txt, api62 and api67 sources). Rows: API 62.0 and 67.0, anonymous and @IsTest routes. Each run used a fresh 30-day scratch org from a campaign Dev Hub, one org per API version; Salesforce ids are masked. Every org was deleted and verified gone after the run.
