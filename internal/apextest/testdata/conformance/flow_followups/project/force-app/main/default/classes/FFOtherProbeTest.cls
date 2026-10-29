@IsTest
private class FFOtherProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'FF_OTHER|' + JSON.serialize(FFOtherProbe.observe()));
    }
}
