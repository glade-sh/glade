@IsTest private class ParentProjectionProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(ParentProjectionProbe.observe()));
    }
}
