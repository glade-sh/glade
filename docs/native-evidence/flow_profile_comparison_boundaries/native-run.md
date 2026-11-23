# Native capture: $Profile.Name comparison case and spacing

The same decision is run with Title set to the exact profile name, the lower-cased name, and the name padded with spaces. Salesforce answered `yes|yes|yes`: the comparison ignores case, and the padded value matches because the stored Title is trimmed on save.

Fixture: `internal/apextest/testdata/conformance/flow_profile_comparison_boundaries` (org.tsv, RUN-FACTS.txt, api62 and api67 sources). Rows: API 62.0 and 67.0, anonymous and @IsTest routes. Each run used a fresh 30-day scratch org from a campaign Dev Hub, one org per API version; Salesforce ids are masked. Every org was deleted and verified gone after the run.
