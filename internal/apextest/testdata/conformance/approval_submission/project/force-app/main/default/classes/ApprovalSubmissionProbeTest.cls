@IsTest private class ApprovalSubmissionProbeTest {
    @IsTest static void missing() { System.assert(false,'APPROVAL_ROWS|'+JSON.serialize(ApprovalSubmissionProbe.observeGroup('missing'))); }
    @IsTest static void inactive() { System.assert(false,'APPROVAL_ROWS|'+JSON.serialize(ApprovalSubmissionProbe.observeGroup('inactive'))); }
    @IsTest static void no_applicable() { System.assert(false,'APPROVAL_ROWS|'+JSON.serialize(ApprovalSubmissionProbe.observeGroup('no_applicable'))); }
    @IsTest static void pending() { System.assert(false,'APPROVAL_ROWS|'+JSON.serialize(ApprovalSubmissionProbe.observeGroup('pending'))); }
    @IsTest static void null_id() { System.assert(false,'APPROVAL_ROWS|'+JSON.serialize(ApprovalSubmissionProbe.observeGroup('null_id'))); }
    @IsTest static void success() { System.assert(false,'APPROVAL_ROWS|'+JSON.serialize(ApprovalSubmissionProbe.success())); }
}
