@IsTest
private class IdleCollectionProbeTest {
    @IsTest
    static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(IdleCollectionProbe.observe()));
    }
}
