# Native capture: custom metadata Get Records filtered by a formula value

A Contact before-save create flow looks up custom metadata where a Text field equals a String formula, `TRIM(RIGHT(Title, LEN(Title) - FIND(". ", Title)))`, and assigns a field of the found record to Description. Salesforce answered `T-Alpha|T-Beta|||`: the match is case-insensitive after TRIM, and a missing, empty or null name finds nothing.

Fixture: `internal/apextest/testdata/conformance/flow_metadata_formula_lookup` (org.tsv, RUN-FACTS.txt, api62 and api67 sources). Rows cover Apex class API 62.0 and 67.0, anonymous and @IsTest routes. The flows were deployed at 67.0 in both orgs. Each run used a fresh 30-day scratch org from a campaign Dev Hub, one org per API version, and Salesforce ids are masked. Every org was deleted and verified gone after the run.
