@IsTest private class ChoiceBoundaryProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(ChoiceBoundaryProbe.observe())); }
}
