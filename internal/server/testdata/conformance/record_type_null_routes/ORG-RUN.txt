Owned synthetic capture, 2026-11-08 UTC (v0.3.6): an explicit null RecordTypeId through REST and UI API routes.
One fresh one-day Developer scratch org from a campaign Dev Hub, project/config/project-scratch-def.json. Release: max API 68.0.
Phases, cumulative: noProfileDefault deploys project/ without profiles (V35Rt__c with active Alpha and Beta; V35NoRt__c
without record types). profileDefault adds Admin.profile-meta.xml (Alpha visible default, Beta visible). The running user
has the System Administrator profile. Each case ran at REST version 62.0 and 67.0.
Requests go through sf api request rest; RecordType.DeveloperName is read back with SOQL. IDs are masked in org.tsv.
 rest.post.*     POST sobjects/<object>, RecordTypeId unset, null or Alpha.
 ui.post.*       POST ui-api/records, the same fields.
 rest.patch.*    PATCH sobjects/<object>/<id>, RecordTypeId null, or another field.
 ui.patch.null   PATCH ui-api/records/<id>, RecordTypeId null.
 collections.*   composite/sobjects, POST with allOrNone true and false, PATCH.
 composite.*     composite subrequest POST.
Findings, the same at both versions:
 With a profile default, a null insert fails with INVALID_CROSS_REFERENCE_KEY on RecordTypeId on every route, an unset
 insert takes the default, and a null update succeeds and the record ends on the default (it was seeded on the default, so these rows alone do not show a change of type).
 Without a profile default, a null insert succeeds with no record type, and Alpha is rejected as not valid for the user.
 An object without record types rejects the field: REST INVALID_FIELD, UI API POST_BODY_PARSE_ERROR.
 The REST and UI rule matches the Apex rows in record_type_id_null_insert_default.
Cleanup: org deleted (sf org delete scratch); ScratchOrgInfo Status recorded as Deleted.

unknown_field.tsv: 2026-11-08 UTC, a second fresh one-day org from a campaign Dev Hub, {"edition":"Developer"}, nothing deployed.
Same request method, at REST version 62.0 and 67.0, on Account: POST and PATCH of Nope__c, PATCH of RecordTypeId null, and
composite/sobjects POST and PATCH of Nope__c with allOrNone false. Every row is INVALID_FIELD "No such column '<field>' on
sobject of type Account" with no fields; REST answers 400 and collections 200 with success false.
Cleanup: org deleted (sf org delete scratch); ScratchOrgInfo Status recorded as Deleted.

allornone.tsv: 2026-11-08 UTC, a third fresh one-day org from a campaign Dev Hub, {"edition":"Developer"}, nothing deployed.
composite/sobjects with allOrNone true at REST version 62.0 and 67.0 on Account: POST good row plus unknown field, POST good
row plus missing Name, PATCH good row plus null Name. Every answer is HTTP 200. The good row reports
ALL_OR_NONE_OPERATION_ROLLED_BACK "Record rolled back because not all records were valid and the request was using AllOrNone
header"; a POST's rows have no id, a PATCH's rows have theirs. DELETE was attempted and did not run: sf api request rest refused
the request client side, so delete has no row. An earlier org for this capture stopped at that error; it was deleted too.
Cleanup: orgs deleted (sf org delete scratch); ScratchOrgInfo Status recorded as Deleted.
