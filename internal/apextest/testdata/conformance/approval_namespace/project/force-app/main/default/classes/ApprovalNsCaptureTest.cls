@IsTest private class ApprovalNsCaptureTest {
 @IsTest static void capture() { System.assert(false, 'R10_ROWS|' + JSON.serialize(ApprovalNsCapture.observe())); }
}
