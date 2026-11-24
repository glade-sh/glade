# Native run facts

Captured 2026-11-24 in a temporary scratch org, deleted after the run. Edition: Developer Edition (queried). Requested features: none; no additional features or settings were requested. Deliverability setting: not captured.

Anonymous Apex ran through `sf apex run` with each API version. The class was deployed with the corresponding class API version and run synchronously as Apex tests. Both use `Messaging.sendEmail(messages, false)`, with a syntactically valid `0D2` ID absent from the org or a missing address lookup whose empty result supplies null. Results are the exact success flag and status code/message, or exception type and message. The test methods deliberately fail an assertion containing the observed result to expose it through the test result; `Failed` describes that assertion, not the email operation. No trigger is deployed or invoked. Residue was not measured.

Additional capture on 2026-11-24 used a second temporary Developer Edition scratch org, deleted after the run. Anonymous and test class probes ran at API 63.0. An Account before-insert trigger at APIs 62.0, 63.0, and 67.0 sent the direct missing ID; anonymous Apex and a test class inserted and queried the record at the matching API version. The test assertion intentionally failed to expose the outcome. Residue was not measured.

Additional capture on 2026-11-24 used a temporary Developer Edition scratch org, deleted after the run. The direct and lookup probes ran in anonymous Apex and test classes at APIs 64.0, 65.0, and 66.0. The test assertions deliberately failed to expose results. Residue was not measured.
