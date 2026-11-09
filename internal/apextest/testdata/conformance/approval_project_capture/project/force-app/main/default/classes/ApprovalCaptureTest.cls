@IsTest private class ApprovalCaptureTest {
 @IsTest static void capture() { System.assert(false, 'R10_ROWS|' + JSON.serialize(ApprovalCapture.observe())); }
}
