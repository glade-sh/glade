@IsTest private class LookupRestoreProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(LookupRestoreProbe.observe()));
    }
}
