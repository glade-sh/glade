@IsTest
private class R3EventTest {
    @TestSetup static void seed() {
        EventBus.publish(new CovEvt__e(Tag__c = 'setup'));
    }
    @IsTest static void observe() {
        System.assert(false, R3Facts.facts());
    }
    @IsTest static void observeAgain() {
        System.assert(false, R3Facts.facts());
    }
}
