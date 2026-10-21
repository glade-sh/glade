@IsTest private class SecuredProjectionProbeTest { @IsTest static void observe() { System.assert(false,'OBS|'+JSON.serialize(SecuredProjectionProbe.observe())); } }
