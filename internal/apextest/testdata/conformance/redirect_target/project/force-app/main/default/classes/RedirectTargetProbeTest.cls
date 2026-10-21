@IsTest
private class RedirectTargetProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(RedirectTargetProbe.observe()));
    }
}
