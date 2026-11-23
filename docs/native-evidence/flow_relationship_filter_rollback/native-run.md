# Native capture: relationship filter fault rollback

The relationship-filter case again, also counting the inserted FilterCheck Contacts afterwards. Salesforce reports CANNOT_EXECUTE_FLOW_TRIGGER for all three rows and `remaining=0`: each faulted insert is rolled back.

Fixture: `internal/apextest/testdata/conformance/flow_relationship_filter_rollback` (org.tsv, RUN-FACTS.txt, api62 and api67 sources). Rows: API 62.0 and 67.0, anonymous and @IsTest routes. Each run used a fresh 30-day scratch org from a campaign Dev Hub, one org per API version; Salesforce ids are masked. Every org was deleted and verified gone after the run.
