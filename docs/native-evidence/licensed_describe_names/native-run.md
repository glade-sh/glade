# Native run: AssessmentStagedData, DocGenerationBatchProcess, RecordAccessEvent__e

Question: do these three names resolve in any org Glade can reach?
An earlier packet (internal/storage/testdata/standard_licensed_describe_case273)
recorded NOT_FOUND describes for an Enterprise trial and a scratch org. This run
adds a scratch org with the DocGen feature, repeats the trial and plain scratch
describes at both versions, and records compile and describe behavior in Apex.

Orgs (no ids or usernames are recorded):
- `plain`: fresh 30-day Developer-edition scratch org, default settings.
- `docgen-feature`: fresh 30-day Developer-edition scratch org with features
  DocGen, DocumentChecklist and IndustriesActionPlan. The describe list gained
  DocumentGenerationProcess, DocGenerationQueryResult and related names; none
  of the three names under test appeared.
- `enterprise-trial`: the authorized Enterprise trial org, read-only describes.

Routes and source API 62.0 and 67.0:
- `rest-describe`: GET /services/data/vNN.0/sobjects/NAME/describe. `Account`
  is the control (FOUND). `RecordAccessEvent` (no suffix) is an extra spelling.
- `anonymous`: Tooling executeAnonymous at the API under test. Statements end in
  a thrown `DmlException('ok')`, so `ok` means the statement compiled. `OBS`
  rows return the observed value.
- `named` and `istest`: one metadata deploy per API of a public class and an
  `@IsTest` class per case; `deploy_status=Failed` carries the compile problem.
- Controls: `R04` uses a name that exists nowhere, `R90` and `R91` use Account,
  `T01` to `T04` use unknown `__e`, `__mdt` and `__c` names.

Result: the three names behave exactly like a name that does not exist. A
declaration, `new`, a static query and the `.SObjectType` token fail to compile
with `Invalid type: NAME` or `Variable does not exist: NAME`; a dynamic query
throws `QueryException: sObject type 'NAME' is not supported`; the global
describe lacks the name. `RecordAccessEvent__e` is not a platform event in any
reached org. Unknown `__e`, `__mdt` and `__c` names also fail on the
`.SObjectType` token (`T01` to `T03`).
