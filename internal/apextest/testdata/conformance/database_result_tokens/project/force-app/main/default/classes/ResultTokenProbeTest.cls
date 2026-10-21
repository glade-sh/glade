@IsTest private class ResultTokenProbeTest {
 @IsTest static void observe() { System.assert(false, 'OBSERVED|observe|' + JSON.serialize(ResultTokenProbe.observe())); }
}
