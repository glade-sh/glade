@IsTest
private class SetupFutureTest {
    @TestSetup static void seed() {
        MarkerFuture.go();
    }
    @IsTest static void observe() {
        System.assert(false, AsyncFacts.facts());
    }
    @IsTest static void observeAgain() {
        System.assert(false, AsyncFacts.facts());
    }
}
