@IsTest private class JsonNumberDateProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(JsonNumberDateProbe.observe()));
    }
}
