@IsTest private class ApprovalSkipCaptureTest { @IsTest static void capture() { System.assert(false,'SKIP_ROWS|'+JSON.serialize(ApprovalSkipCapture.observe())); } }
