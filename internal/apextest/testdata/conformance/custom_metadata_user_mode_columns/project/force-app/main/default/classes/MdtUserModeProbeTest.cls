@IsTest private class MdtUserModeProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(MdtUserModeProbe.observe()));
    }
}
