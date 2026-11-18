@IsTest
private class SetupLimitFail {
    @TestSetup static void seed() {
        for (Integer i = 0; i < 101; i++) { List<Account> r = [SELECT Id FROM Account LIMIT 1]; }
    }
    @IsTest static void first() { System.debug('BODY'); }
    @IsTest static void second() { System.assert(true); }
}
