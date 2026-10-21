@IsTest
private class MeadowEventTypeProbeTest {
    @IsTest
    static void observeRows() {
        System.assert(false, 'PROBE|' + JSON.serialize(MeadowEventTypeProbe.observe()));
    }
}
