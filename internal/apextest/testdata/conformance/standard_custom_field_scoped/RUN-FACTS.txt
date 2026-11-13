Scoped USER_MODE inserts of custom fields on a standard object (Account), with AccessLevel.USER_MODE.withPermissionSetId.
org: fresh 30-day Developer scratch org on a campaign Dev Hub with ApexUserModeWithPermset enabled, deleted after the capture (delete verified). Org id, usernames, alias and host are not recorded.
capture date: 2026-11-13 (UTC). Class source API 62.0, 67.0 and 68.0 (class apiVersion).
metadata: Account.Probe_A__c and Account.Probe_B__c, Text(255); permission set Probe_Access grants edit on Probe_A__c only.
route: named @IsTest methods. scoped* run as a Standard User under System.runAs with Probe_Access supplied as the scope; admin* run as the default administrator without System.runAs. Anonymous execution could not access the ungranted field and is not an outcome for DML.
observation: the assertion message; "ok" means the insert saved; "fail <status>:<message>" is a failed SaveResult.
