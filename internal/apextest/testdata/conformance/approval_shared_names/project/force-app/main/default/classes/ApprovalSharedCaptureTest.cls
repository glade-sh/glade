@IsTest private class ApprovalSharedCaptureTest { @IsTest static void capture() { System.assert(false,'SHARED_ROWS|'+JSON.serialize(ApprovalSharedCapture.observe())); } }
