@IsTest
private class R3DeliverTest {
    @TestSetup static void seed() {
        EventBus.publish(new CovEvt__e(Tag__c = 'setup'));
        Test.getEventBus().deliver();
    }
    @IsTest static void observe() {
        System.assert(false, R3Facts.facts());
    }
    @IsTest static void observeAgain() {
        System.assert(false, R3Facts.facts());
    }
}
