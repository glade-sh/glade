@IsTest private class ObjectAvailabilityChildrenTest { @IsTest static void capture() { System.assert(false,'OBSERVED|capture|'+JSON.serialize(ObjectAvailabilityChildren.observe())); } }
