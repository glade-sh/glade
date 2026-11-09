@IsTest private class SchemaProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(SchemaProbe.observe()));
    }
}
