@IsTest
private class IndexedInsertionProbeTest {
    @IsTest
    static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(IndexedInsertionProbe.observe()));
    }
}
