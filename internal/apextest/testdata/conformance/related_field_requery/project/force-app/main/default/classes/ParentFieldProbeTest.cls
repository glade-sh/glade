@IsTest private class ParentFieldProbeTest { @IsTest static void observe() { System.assert(false,'OBS|'+JSON.serialize(ParentFieldProbe.observe())); } }
