# Object-list signed-zero sort capture

capture: numeric-zero-object-sort
started: 2026-11-13T08:29:40Z
finished: 2026-11-13T08:30:36Z
apis: 62.0 67.0 68.0
routes: anonymous istest
cases: 18
sf: @salesforce/cli/2.157.8
hub: campaign Dev Hub (scratch org, 30 days, deleted after the run)
cases.tsv sha256: 4b16f47adc81e3a4
mode: live

All 18 invocations compiled. Assertion failures deliberately carry the observations.
The anonymous CLI stores these expected failures in its data envelope; rows were
extracted from data.exceptionMessage. Named rows come from result.tests[].Message.
The scratch org was deleted and the CLI verified it no longer resolves.

The source APIs and anonymous transport APIs match each row. Named tests use the
API declared in their class metadata. Both input orders, duplicate and same-sign
zeros, nonzero Double values, null, and Decimal zero controls are included.
