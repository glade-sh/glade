@IsTest private class ObjectAvailabilityRoutesTest { @IsTest static void capture() { System.assert(false,'OBSERVED|capture|'+JSON.serialize(ObjectAvailabilityRoutes.observe())); } }
