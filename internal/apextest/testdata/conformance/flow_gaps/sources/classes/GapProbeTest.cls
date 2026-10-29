@IsTest
private class GapProbeTest {
    @IsTest
    static void observe() {
        Map<String, Object> answers = GapProbe.observe();
        System.assert(false, 'PROBE|' + JSON.serialize(answers));
    }
}
