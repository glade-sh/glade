@IsTest
private class SetupMethodFail {
    @TestSetup static void seed() { insert as system new Account(Name = 'ok'); }
    @IsTest static void first() { Account a = [SELECT Id FROM Account LIMIT 1]; throw new CvException('METHOD_CUSTOM'); }
    @IsTest static void second() { System.assert(true); }
}
