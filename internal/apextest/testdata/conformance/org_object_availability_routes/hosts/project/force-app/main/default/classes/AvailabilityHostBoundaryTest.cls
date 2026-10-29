@IsTest private class AvailabilityHostBoundaryTest { @IsTest static void capture() { System.assert(false,'OBSERVED|capture|'+JSON.serialize(AvailabilityHostBoundary.observe())); } }
