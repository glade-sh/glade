# Restricted picklist source-version bisection

capture: soql-ordering-bisect
started: 2026-11-13T06:56:04Z
finished: 2026-11-13T07:00:14Z
apis: 63.0 64.0 65.0 66.0 67.0
routes: anonymous istest
cases: 12
sf: @salesforce/cli/2.157.8
hub: campaign Dev Hub (scratch org, 30 days, deleted after the run)
cases.tsv sha256: 131b6ddb0a4c732e
mode: live

Each observation deliberately raises System.AssertException to return all six clause results and roll back anonymous DML. Source-versioned helpers ran at 63.0, 64.0, 65.0 and 66.0, on anonymous and @IsTest routes. Anonymous calls also used transport 67.0. The compact table expands these 12 observations into 72 clause rows. Original 62.0/67.0 endpoints remain in the parent fixture and were not recaptured. The transition is source 66.0. Creation used a fresh 30-day Developer scratch org on an authorized campaign Dev Hub. The org was deleted and the alias no longer resolved. No identifiers or authentication material are retained.

One source-66/transport-67 anonymous attempt returned a network fetch failure and supplied no observation. A retry returned the recorded six-clause result. The first deletion attempt also returned a network failure. The retry returned status 0; the subsequent display returned NamedOrgNotFoundError.
