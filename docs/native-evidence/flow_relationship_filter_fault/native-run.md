# Native capture: Get Records filter on a relationship field path

A Contact after-save create Flow looks up Contact where `Account.Name` equals `$Record.Account.Name` and a second text filter holds a literal. Three Contacts are inserted with partial-success DML (Account One, Account Other, no Account). Salesforce faults every insert with CANNOT_EXECUTE_FLOW_TRIGGER.

Fixture: `internal/apextest/testdata/conformance/flow_relationship_filter_fault` (org.tsv, RUN-FACTS.txt, api62 and api67 sources). Rows: API 62.0 and 67.0, anonymous and @IsTest routes. Each run used a fresh 30-day scratch org from a campaign Dev Hub, one org per API version; Salesforce ids are masked. Every org was deleted and verified gone after the run.
