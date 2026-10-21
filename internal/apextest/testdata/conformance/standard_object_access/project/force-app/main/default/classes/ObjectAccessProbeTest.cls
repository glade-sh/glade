@IsTest private class ObjectAccessProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(ObjectAccessProbe.observe())); }
}
