@IsTest
private class UnsavedMapProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(UnsavedMapProbe.run()));
    }
}
