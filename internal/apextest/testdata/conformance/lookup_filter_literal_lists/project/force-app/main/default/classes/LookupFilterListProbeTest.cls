@IsTest private class LookupFilterListProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(LookupFilterListProbe.observe()));
    }
}
