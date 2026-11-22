# Native capture: after-save create Flow decision on $Profile.Name against a record field

A Contact after-save create Flow decides `$Profile.Name EqualTo $Record.Title` (elementReference right side) and writes the outcome to Description. Three Contacts are inserted: Title equal to the running profile name, a different Title, and no Title. Salesforce answered `no|yes|no` in Title order (different, equal, null): only the equal Title takes the match branch, and a null Title does not match.

Fixture: `internal/apextest/testdata/conformance/flow_direct_profile_decision` (org.tsv, RUN-FACTS.txt, api62 and api67 sources). Rows: API 62.0 and 67.0, anonymous and @IsTest routes. Each run used a fresh 30-day scratch org from a campaign Dev Hub, one org per API version; Salesforce ids are masked. Every org was deleted and verified gone after the run.
