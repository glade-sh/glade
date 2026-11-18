@IsTest
private class CvSetupCovPosTest {
    @TestSetup static void seed() {
        CvSetupCovPos.value();
    }
    @IsTest static void callsAgain() {
        System.assertEquals(42, CvSetupCovPos.value());
    }
}
