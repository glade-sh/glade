# Native run facts

Captured 2026-11-24 in a temporary scratch org, deleted after the run. Edition: Developer Edition (queried). Requested features: none; no additional features or settings were requested. Deliverability setting: not captured.

Anonymous Apex ran through `sf apex run` with each API version. The class was deployed with the corresponding class API version and run synchronously as Apex tests. Both use `Messaging.sendEmail(messages, false)`, with a syntactically valid `0D2` ID absent from the org or a missing address lookup whose empty result supplies null. Results are the exact success flag and status code/message, or exception type and message. The test methods deliberately fail an assertion containing the observed result to expose it through the test result; `Failed` describes that assertion, not the email operation. No trigger is deployed or invoked. Residue was not measured.
