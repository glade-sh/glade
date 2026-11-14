# Native run: stopTest ordering

`StopOrderTest` ran through the `@IsTest` route at APIs 62.0 and 67.0.
The original capture record available here does not specify its timestamp,
org edition or driver. The repeat control documented in
`../async_stop_mix/RUN-FACTS.txt` reproduced all 13 methods at both APIs on
2026-11-13 at 16:39 UTC using Salesforce CLI 2.157.8. It used a fresh 30-day
scratch org per API, deleted and verified gone; the edition and exact driver
script are not recorded.

Synthetic Account markers and an intentional assertion after
`Test.stopTest()` record execution order. The observations and their repeat
notes are in `org.tsv`; record and org identifiers are omitted. The synthetic
Apex and metadata are in `project/`. The scheduled case notes a difference
between the nested and top-level class runs; the fixture retains that
qualification.
