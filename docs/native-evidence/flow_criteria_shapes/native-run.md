# Native capture: remaining record-triggered Flow criteria and action shapes

Seven synthetic after-save and before-save create flows on Contact and Product2, each run from Apex with partial-success inserts:
- `$Profile.Name` EqualTo and NotEqualTo against a literal, a lower-cased literal and `$Record.Title` (rows `eq,n,n,n|eq,n,n,y|eq,n,n,y`: the comparison is case-insensitive, and a null Title differs).
- Get Records filtered by `$Record.ReportsTo.AccountId`, and a User lookup by `ContactId = $Record.ReportsToId`, where a null value matches internal users (`Acme RV,none|none,none|none,found`).
- A before-save Get Records of custom metadata by a custom Text field from a String variable (`T-Alpha|T-Beta||`).
- A loop over Get Records of Pricebook2 by Name that assigns an SObject variable's Id, followed by a PricebookEntry create. Anonymous Apex creates the standard entry. Inside a test without SeeAllData, the standard price book is not returned and the create fails with REQUIRED_FIELD_MISSING. A test-created price book named like the standard one fails with STANDARD_PRICE_NOT_DEFINED.
- Send Email with an address collection built by Add assignments and text-template subject and rich body: an unknown org-wide sender faults with INVALID_ARGUMENT_TYPE ("Org-Wide Email provided is not valid"), while the default and CurrentUser senders send.

Fixture: `internal/apextest/testdata/conformance/flow_criteria_shapes` (org.tsv, RUN-FACTS.txt, api62 and api67 sources). Rows cover Apex class API 62.0 and 67.0, anonymous and @IsTest routes. The flows were deployed at 67.0 in both orgs. Each run used a fresh 30-day scratch org from a campaign Dev Hub, one org per API version, and Salesforce ids are masked. Every org was deleted and verified gone after the run.
