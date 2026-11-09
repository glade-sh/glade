@IsTest private class JsonScalarMismatchProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(JsonScalarMismatchProbe.observe()));
    }
}
