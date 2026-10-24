@IsTest private class ApprovalSubmissionProbeTest {
    @IsTest static void mixed_late() { System.assert(false,'APPROVAL_ROWS|'+JSON.serialize(ApprovalSubmissionProbe.observeGroup('mixed_late'))); }
    @IsTest static void mixed_early() { System.assert(false,'APPROVAL_ROWS|'+JSON.serialize(ApprovalSubmissionProbe.observeGroup('mixed_early'))); }
    @IsTest static void mixed_success() { System.assert(false,'APPROVAL_ROWS|'+JSON.serialize(ApprovalSubmissionProbe.observeGroup('mixed_success'))); }
    @IsTest static void mixed_workitems() { System.assert(false,'APPROVAL_ROWS|'+JSON.serialize(ApprovalSubmissionProbe.observeGroup('mixed_workitems'))); }
    @IsTest static void mixed_reject() { System.assert(false,'APPROVAL_ROWS|'+JSON.serialize(ApprovalSubmissionProbe.observeGroup('mixed_reject'))); }
    @IsTest static void single_approve() { System.assert(false,'APPROVAL_ROWS|'+JSON.serialize(ApprovalSubmissionProbe.observeGroup('single_approve'))); }
    @IsTest static void single_reject() { System.assert(false,'APPROVAL_ROWS|'+JSON.serialize(ApprovalSubmissionProbe.observeGroup('single_reject'))); }
    @IsTest static void single_invalid() { System.assert(false,'APPROVAL_ROWS|'+JSON.serialize(ApprovalSubmissionProbe.observeGroup('single_invalid'))); }
}
