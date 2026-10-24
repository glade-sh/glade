@IsTest private class HierarchyBoundaryTest { @IsTest static void observe() { System.assert(false,'OBSERVED|'+JSON.serialize(HierarchyBoundaryProbe.all())); } }
