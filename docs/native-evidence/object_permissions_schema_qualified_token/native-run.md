# Native run: ObjectPermissions.PermissionsViewAllFields spelled through Schema

Question: the field is absent at source API 62.0 and present from 63.0
(case289_followups). Does Salesforce reject the same field when it is named
through `Schema.ObjectPermissions`, through a chained `.getDescribe()` or
`.getSObjectField()`, or through an `SObjectType.fields` member?

Capture: fresh 30-day Developer-edition scratch org with default settings.
Source API 62.0 and 67.0. Routes:

- `anonymous`: Tooling `executeAnonymous` at /services/data/v62.0 and v67.0.
  Each statement ends in a thrown `DmlException('ok')`, so `compiled=true`
  with `exceptionMessage` ending in `ok` means the statement compiled.
  Each case ran twice per API (the two rows are identical).
- `named` and `istest`: one metadata deploy per API of a public class and an
  `@IsTest` class per case (class apiVersion equal to the API under test).
  `deploy_status=Failed` carries the per-class compile problem.

Controls: `P07` (bare token, already captured at 62.0), `P08` and `P10`
(another ObjectPermissions field, accepted), `P09` (absent field, rejected).

Result: every spelling of the field is rejected at 62.0 with
`Variable does not exist: ...` and accepted at 67.0. The rejection message
names the field for member reads, and the longer path for `.getDescribe()`
chains. A different ObjectPermissions field is accepted at both versions.

No org id, username or credential is recorded here.
