A Developer scratch org ran an after-save Account flow with an invocable Apex action. The action incremented a static counter. Anonymous Apex and an `@IsTest` method each observed one invocation at APIs 62.0, 67.0 and 68.0. Inserted Account records were cleaned up. The org was deleted and deletion verified.

In a separate fresh org, an otherwise matching record-triggered flow was validated with an absent action class and with a present class that had no invocable method. Salesforce rejected both deployments at flow APIs 62.0, 67.0 and 68.0. The present non-invocable class was deployed at API 68.0 during these validations. The org was deleted and deletion verified. Rows are in `internal/apextest/testdata/conformance/flow_record_action_dependency/org.tsv`.

The negative rows are metadata deployment observations. They do not establish a Salesforce runtime error for an active flow with a missing action; Salesforce did not accept those flow deployments.
