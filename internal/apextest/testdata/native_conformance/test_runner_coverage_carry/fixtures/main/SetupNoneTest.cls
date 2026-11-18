@IsTest
private class SetupNoneTest {
    @TestSetup static void seed() {
        insert new Account(Name = 'MK-SYNC');
    }
    @IsTest static void observe() {
        System.assert(false, AsyncFacts.facts());
    }
    @IsTest static void observeAgain() {
        System.assert(false, AsyncFacts.facts());
    }
}
