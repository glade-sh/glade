@IsTest private class MeadowParentTextProbeTest {
    @TestSetup static void seedRecords() {
        MeadowParentTextProbe.seed();
    }

    @IsTest static void observeParentText() {
        Map<String, Object> answers = MeadowParentTextProbe.observe();
        System.debug('PROBE|'+JSON.serialize(answers));
        System.assert(false, 'PROBE|'+JSON.serialize(answers));
    }
}
