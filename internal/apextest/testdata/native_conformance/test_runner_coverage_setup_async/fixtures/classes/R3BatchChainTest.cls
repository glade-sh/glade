@IsTest
private class R3BatchChainTest {
    @TestSetup static void seed() {
        insert new Account(Name = 'R3-SEED');
        Database.executeBatch(new R3Batch());
    }
    @IsTest static void observe() {
        System.assert(false, R3Facts.facts());
    }
    @IsTest static void observeAgain() {
        System.assert(false, R3Facts.facts());
    }
}
