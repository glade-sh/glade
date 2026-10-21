@IsTest private class AliasLoopProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(AliasLoopProbe.observe()));
    }
}
