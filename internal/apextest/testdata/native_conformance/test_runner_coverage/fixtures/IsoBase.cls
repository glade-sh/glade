@IsTest
private class IsoBase {
    static Integer state = 7;
    static Id setupId;
    @TestSetup static void seed() {
        Account a = new Account(Name = 'ISO_BASELINE');
        insert a;
        state = 99;
        setupId = a.Id;
        System.debug('ISO_SETUP ' + a.Id);
    }
    private static Account baseline() {
        System.assertEquals(7, state);
        System.assertEquals(null, setupId);
        Account a = [SELECT Id, Name FROM Account LIMIT 1];
        System.assertEquals('ISO_BASELINE', a.Name);
        System.debug('ISO_METHOD_ID ' + a.Id);
        return a;
    }
    @IsTest static void mutate() {
        Account a = baseline();
        a.Name = 'CHANGED'; update a;
        state = 101;
    }
    @IsTest static void remove() {
        delete baseline();
        state = 102;
    }
}
