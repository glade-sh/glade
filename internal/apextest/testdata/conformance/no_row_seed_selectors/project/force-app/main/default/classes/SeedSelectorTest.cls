@IsTest private class SeedSelectorTest {
 @IsTest static void observe() { System.assert(false, 'OBSERVED|' + JSON.serialize(SeedSelectorProbe.observe())); }
}
