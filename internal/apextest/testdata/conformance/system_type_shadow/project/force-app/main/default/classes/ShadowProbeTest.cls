@IsTest private class ShadowProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(ShadowProbe.observe()));
    }
}
