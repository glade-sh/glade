@IsTest
private class SetupThrowTest {
    @TestSetup static void seed() {
        System.enqueueJob(new MarkerThrow());
    }
    @IsTest static void observe() {
        System.assert(false, AsyncFacts.facts());
    }
    @IsTest static void observeAgain() {
        System.assert(false, AsyncFacts.facts());
    }
}
