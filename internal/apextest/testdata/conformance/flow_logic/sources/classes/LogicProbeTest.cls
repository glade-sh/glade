@IsTest
private class LogicProbeTest {
    @IsTest
    static void observe() {
        Map<String, Object> answers = LogicProbe.observe();
        System.assert(false, 'PROBE|' + JSON.serialize(answers));
    }
}
