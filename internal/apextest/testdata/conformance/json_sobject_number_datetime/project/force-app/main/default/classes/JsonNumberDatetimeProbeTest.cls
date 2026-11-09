@IsTest private class JsonNumberDatetimeProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(JsonNumberDatetimeProbe.observe()));
    }
}
