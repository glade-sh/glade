@IsTest private class ZephyrTextProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(ZephyrTextProbe.observe()));
    }
}
