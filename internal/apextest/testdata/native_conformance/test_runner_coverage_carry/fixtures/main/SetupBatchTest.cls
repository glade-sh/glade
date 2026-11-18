@IsTest
private class SetupBatchTest {
    @TestSetup static void seed() {
        insert new Account(Name = 'MK-SEED');
        Database.executeBatch(new MarkerBatch());
    }
    @IsTest static void observe() {
        System.assert(false, AsyncFacts.facts());
    }
    @IsTest static void observeAgain() {
        System.assert(false, AsyncFacts.facts());
    }
}
