@IsTest
private class SetupTwoQTest {
    @TestSetup static void seed() {
        System.enqueueJob(new MarkerQ());
        System.enqueueJob(new MarkerQ());
    }
    @IsTest static void observe() {
        System.assert(false, AsyncFacts.facts());
    }
    @IsTest static void observeAgain() {
        System.assert(false, AsyncFacts.facts());
    }
}
