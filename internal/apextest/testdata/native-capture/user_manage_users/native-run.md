Captured 2026-11-07 in a fresh one-day Developer scratch org without a namespace; locale en_US, time zone America/Los_Angeles, maximum API 68.0. Org deletion was verified.

Test route: project/force-app/main/default/classes/ManageUsersCapture.cls deployed at apiVersion 62.0 and then 67.0 (sf project deploy start --api-version <api>) and run with sf apex run test --synchronous --api-version <api>. Each method inserts two Standard Users (Standard User profile, no role) as the administrator. Unless it is the control, it inserts a PermissionSet with the listed Permissions* flags true and assigns it to the first user. Inside System.runAs(first user) it records the assigned ManageUsers, ManageInternalUsers and ModifyAllData flags and User describe isUpdateable, then updates Title on the target: the second user ("other") or the administrator ("admin"). It ends with System.assert(false, 'CAP|<probe>|<json>'); all state rolls back. "storedTitle" is the administrator's re-query after runAs; "grant" is "ok" or the insert error.
The class was deployed three times. The first two runs used shorter flag lists whose PermissionSet inserts failed on dependencies ("Permission ManageUsers depends on permission(s): DelegatedTwoFactor, FreezeUsers, MonitorLoginHistory"; "Permission ViewSetup depends on permission(s): ViewRoles"). org.tsv keeps the third run, where every grant is "ok".

org.tsv columns: source_api, route, method, outcome (native test outcome), message (verbatim native result message). User ids in messages differ per org.

Findings (identical at 62.0 and 67.0):
- No grant (controlOther) or ViewAllUsers + ViewSetup + ViewRoles (viewAllUsersOther): updating another Standard User fails with DmlException INSUFFICIENT_ACCESS_ON_CROSS_REFERENCE_ENTITY; Title is unchanged.
- Manage Internal Users without Manage Users or Modify All Data (manageInternalUsers*): updating another Standard User or the administrator saves, with update and with Database.update(r, false).
- Manage Users (which requires Manage Internal Users) without Modify All Data (manageUsers*): the same updates save.
- User describe isUpdateable and Title isUpdateable are true in every case.
Not captured: anonymous route (System.runAs is test-only; anonymous Apex runs as the administrator), API 63.0-66.0, other fields than Title (ProfileId, IsActive), portal or partner targets, explicit with/without sharing classes, USER_MODE DML.

The synthetic fixture class was renamed after capture for the public tree.
