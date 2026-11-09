@IsTest private class IndustriesProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(IndustriesProbe.observe()));
    }
}
