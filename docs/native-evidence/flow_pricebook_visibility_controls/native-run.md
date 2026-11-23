# Native capture: standard pricebook lookup visibility controls in tests

Controls for the pricebook row: an @IsTest(SeeAllData=true) class using the org's standard pricebook and an @IsTest class using a pricebook it inserts. Both lookups find their record (`yes`), so only the test-isolated standard pricebook id is hidden from the Flow lookup.

Fixture: `internal/apextest/testdata/conformance/flow_pricebook_visibility_controls` (org.tsv, RUN-FACTS.txt, api62 and api67 sources). Rows: API 62.0 and 67.0, the @IsTest route. Each run used a fresh 30-day scratch org from a campaign Dev Hub, one org per API version; Salesforce ids are masked. Every org was deleted and verified gone after the run.
