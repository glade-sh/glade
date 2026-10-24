@IsTest private class ResultLiteralProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|'+JSON.serialize(ResultLiteralProbe.observe())); } }
