@IsTest private class NoRowSeedsTest {
 @IsTest static void observe() { System.assert(false, 'OBSERVED|' + JSON.serialize(NoRowSeedsProbe.observe())); }
}
