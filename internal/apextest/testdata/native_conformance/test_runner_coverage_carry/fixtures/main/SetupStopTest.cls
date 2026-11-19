@IsTest
private class SetupStopTest {
    @TestSetup static void seed() {
        Test.startTest();
        System.enqueueJob(new MarkerQ());
        Test.stopTest();
    }
    @IsTest static void observe() {
        System.assert(false, AsyncFacts.facts());
    }
    @IsTest static void observeAgain() {
        System.assert(false, AsyncFacts.facts());
    }
}
