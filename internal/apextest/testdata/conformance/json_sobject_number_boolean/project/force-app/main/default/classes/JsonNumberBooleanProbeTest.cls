@IsTest private class JsonNumberBooleanProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(JsonNumberBooleanProbe.observe()));
    }
}
