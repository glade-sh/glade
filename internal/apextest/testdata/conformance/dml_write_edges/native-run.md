# DML write boundaries

## RUN-FACTS

- API 62.0 started at 07:27:11 UTC and API 67.0 at 07:28:14 UTC on 2026-11-25.
- Salesforce CLI 2.159.7 deployed the synthetic project to two fresh Developer scratch orgs from different Dev Hubs. Both scratch org deletions were verified.
- Each class and trigger used the API version in its row. Each anonymous run used that version for the CLI request and called the versioned probe class. The named route ran the three `@IsTest` methods synchronously.
- The probe permission set was assigned before either route. An anonymous debug marker and an intentional terminal assertion in each named method carried the observations. The exporter decoded the CLI's escaped pipe characters and masked the synthetic lookup ID in the error. Blank brackets and spaces are significant.
- Both API versions and both routes returned the same result for every primary case.

## NEIGHBOR PROBES

- Existing, omitted and soft-deleted lookup targets are controls. Self lookups, required lookups, master-detail, lookup updates, sparse local schemas and lookups from custom to standard objects keep their existing rules.
- The overlong non-key field and deleted parent are controls for attempted-insert upsert failures. Matching-key updates, standard ID-lookup selectors, explicit IDs and REST responses are outside the captured change.
- The text cases record caller, stored and trigger values. The runtime change targets plain Text in before-insert triggers. Before-update and Text Area values are observations outside this change.
