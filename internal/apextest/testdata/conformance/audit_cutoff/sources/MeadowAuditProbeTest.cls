@IsTest private class MeadowAuditProbeTest { @IsTest static void observeRows(){ System.assert(false,'PROBE|'+JSON.serialize(MeadowAuditProbe.observe())); } }
