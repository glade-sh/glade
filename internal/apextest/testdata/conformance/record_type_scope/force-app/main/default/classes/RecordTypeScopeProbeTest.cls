@IsTest private class RecordTypeScopeProbeTest { @IsTest static void observeScope() { System.assert(false,'RECORD_TYPE_SCOPE_RESULT|'+JSON.serialize(RecordTypeScopeProbe.observe())); } }
