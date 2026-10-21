@IsTest private class MeadowDecisionProbeTest {
    @TestSetup static void seedRecords() {
        MeadowDecisionProbe.seed();
    }

    @IsTest static void observeRatingChanges() {
        Map<String, String> answers = MeadowDecisionProbe.observe();
        System.debug('PROBE|'+JSON.serialize(answers));
        System.assert(false, 'PROBE|'+JSON.serialize(answers));
    }
}
