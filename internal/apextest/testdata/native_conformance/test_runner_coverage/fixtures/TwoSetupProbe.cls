@IsTest
private class TwoSetupProbe {
    @TestSetup static void first() {
        System.debug('SETUP_FIRST');
        insert new Account(Name = 'SETUP_FIRST');
    }
    @TestSetup static void second() {
        System.debug('SETUP_SECOND');
        insert new Account(Name = 'SETUP_SECOND');
    }
    @IsTest static void countRows() {
        System.assertEquals(2, [SELECT COUNT() FROM Account]);
    }
}
