@IsTest
private class CvSetupCovTest {
    @TestSetup static void seed() {
        CvSetupCov.makeRecord();
        System.assertEquals(42, CvSetupCov.value());
    }
    @IsTest static void inspectWithoutCallingHelper() {
        System.assertEquals(1, [SELECT count() FROM Account WHERE Name = 'setup-coverage-probe' WITH SYSTEM_MODE]);
    }
}
