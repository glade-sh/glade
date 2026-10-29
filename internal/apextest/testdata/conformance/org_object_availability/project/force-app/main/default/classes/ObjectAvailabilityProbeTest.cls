@IsTest private class ObjectAvailabilityProbeTest { @IsTest static void capture() { System.assert(false, 'OBSERVED|capture|'+JSON.serialize(ObjectAvailabilityProbe.observe())); } }
