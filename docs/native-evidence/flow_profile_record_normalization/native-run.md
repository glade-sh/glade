# Native capture: $Profile.Name decision against a space-padded record field

A Contact is inserted with Title padded by spaces; the Flow copies Title into Department and records the decision outcome. Salesforce answered `yes|storedSpace=false|flowSpace=false`: the stored Title and the value the Flow sees are both trimmed, and the decision matches.

Fixture: `internal/apextest/testdata/conformance/flow_profile_record_normalization` (org.tsv, RUN-FACTS.txt, api62 and api67 sources). Rows: API 62.0 and 67.0, anonymous and @IsTest routes. Each run used a fresh 30-day scratch org from a campaign Dev Hub, one org per API version; Salesforce ids are masked. Every org was deleted and verified gone after the run.
