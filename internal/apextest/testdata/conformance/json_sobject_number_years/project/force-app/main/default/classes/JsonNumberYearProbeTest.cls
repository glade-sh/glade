@IsTest private class JsonNumberYearProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(JsonNumberYearProbe.observe()));
    }
}
