@IsTest private class OwnerShapeProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(OwnerShapeProbe.observe())); }
}
