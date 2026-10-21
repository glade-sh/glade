@IsTest
private class StringRecordMapProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(StringRecordMapProbe.run()));
    }
}
