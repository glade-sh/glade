@IsTest private class ScopeRefreshProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|'+JSON.serialize(ScopeRefreshProbe.observe())); } }
