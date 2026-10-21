@IsTest private class SchemaCollectionProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(SchemaCollectionProbe.observe()));
    }
}
