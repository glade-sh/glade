@IsTest private class MeadowFlowBindingProbeTest {
    @IsTest static void observeRecordBindings() {
        Map<String, Boolean> answers = MeadowFlowBindingProbe.observe();
        System.debug('PROBE|'+JSON.serialize(answers));
        System.assert(false, 'PROBE|'+JSON.serialize(answers));
    }
}
