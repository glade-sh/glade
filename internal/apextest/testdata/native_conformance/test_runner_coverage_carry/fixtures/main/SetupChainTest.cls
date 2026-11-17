@IsTest
private class SetupChainTest {
    @TestSetup static void seed() {
        System.enqueueJob(new MarkerChain1());
    }
    @IsTest static void observe() {
        System.assert(false, AsyncFacts.facts());
    }
    @IsTest static void observeAgain() {
        System.assert(false, AsyncFacts.facts());
    }
}
