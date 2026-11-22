# Site methods without a Site request context (ROAD-549)

Fixture: `internal/apextest/testdata/conformance/site_path_prefix/` (cases.tsv, org.tsv, RUN-FACTS.txt).

Captured 2026-11-22 with `advisor/ops/v2/capture.sh` (capture-v7, rename mode), fresh 1-day scratch orgs,
deleted and verified after each run. Probe source: `SiteProbe.cls.txt` (a try/catch wrapper around one
`Site.*` call, run from anonymous Apex and from one `@IsTest` method per call).

Configurations:

- `plain`: Developer edition, no features.
- `sites_communities`: Developer edition, feature `Communities`, setting `enableNetworksEnabled`; no Site
  record exists (no active Site). The scratch definition did not fail, but the run did not query for the
  enabled setting separately.

APIs: 62.0 and 67.0. Routes: anonymous and istest.

Result: all 13 callable methods answer identically in both configurations, both routes and both APIs.

| Method | Native |
| --- | --- |
| getPathPrefix, getBaseUrl, getBaseRequestUrl, getBaseSecureUrl, getErrorMessage | empty string |
| getName, getSiteId, getMasterLabel, getDomain, getSiteType, getAdminEmail, getOriginalUrl, getAnalyticsTrackingCode | null |
| getPrefix (anonymous) | compile error: `Method was removed after version 29.0: getPrefix` at 62.0 and 67.0 |

getCurrentSiteUrl also fails to compile ("Method was removed after version 29.0") when a class that calls it is deployed at 62.0.

Glade before the fix: with org.features Sites declared, `Site.getPathPrefix()` returned `local`, read from the
seeded local Site record. Native has no Site request context in anonymous or test execution, so the answer is
the empty string.
