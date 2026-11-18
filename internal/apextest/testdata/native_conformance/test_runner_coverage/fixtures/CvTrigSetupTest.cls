@IsTest
private class CvTrigSetupTest {
    @TestSetup static void seed() {
        insert as system new Account(Name = 'cv-trig');
    }
    @IsTest static void body() {
        System.assertEquals(1, [SELECT count() FROM Account WITH SYSTEM_MODE]);
    }
}
