@IsTest private class SeedInventoryTest {
 @IsTest static void observe() { System.assert(false, 'OBSERVED|' + JSON.serialize(SeedInventoryProbe.observe())); }
}
