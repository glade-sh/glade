@IsTest private class StandardParentProbeTest { @IsTest static void observe() { System.assert(false, 'OBS|'+JSON.serialize(StandardParentProbe.observe())); } }
