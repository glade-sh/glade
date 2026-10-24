@IsTest private class HierarchyOverloadTest {
 @IsTest static void observe() { System.assert(false,'OBSERVED|'+JSON.serialize(HierarchyOverloadProbe.observe(false))); }
 @IsTest static void observeGrant() { System.assert(false,'OBSERVED|'+JSON.serialize(HierarchyOverloadProbe.observe(true))); }
}
