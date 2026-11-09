@IsTest private class ApprovalIdentifierCaptureTest { @IsTest static void capture() { System.assert(false,'IDENTIFIER_ROWS|'+JSON.serialize(ApprovalIdentifierCapture.observe())); } }
