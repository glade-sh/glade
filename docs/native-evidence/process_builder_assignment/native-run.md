Fresh 30-day Developer scratch org, campaign Dev Hub, 2026-11-09. Synthetic project deployed at API 67.0.
Lead assignment rule sends Company=Route to a queue. The matched/header=true positive control confirms that rule fired.
The process and after-save flow each change Company from Other to Route in a nested update; the owner remains a user with or without the outer DML assignment header.
Named test probe and anonymous execution ran using source and invocation API 62.0 and 67.0; named sentinel or anonymous debug marker records the result. Anonymous assertion rolls back.
Scratch deletion returned success. The org identity matched both original named-test receipts and exactly one Dev Hub ScratchOrgInfo record, whose Status=Deleted; the matching ActiveScratchOrg count was zero.
