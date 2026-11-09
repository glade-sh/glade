@IsTest private class ApprovalLockCaptureTest { @IsTest static void capture() { System.assert(false,'LOCK_ROWS|'+JSON.serialize(ApprovalLockCapture.observe())); } }
