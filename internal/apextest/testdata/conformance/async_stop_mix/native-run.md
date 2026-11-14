# Native run: mixed stopTest ordering

The `@IsTest` capture ran at APIs 62.0 and 67.0 on 2026-11-13 at
16:39 UTC, using Salesforce CLI 2.157.8. Each API used a fresh 30-day
scratch org, deleted and verified gone. The available run record does not
state the org edition or the exact driver script. Class and trigger metadata
used the corresponding API version.

`StopOrderMixTest` submits asynchronous work and deliberately asserts the
observation after `Test.stopTest()`. Synthetic Account markers and their
in-job sequence record execution order. The 48 observations are in
`org.tsv`; record and org identifiers are omitted. The synthetic Apex and
metadata are in `project/`; source hashes and the repeat control are in
`RUN-FACTS.txt`. These are single observations per case and API.
