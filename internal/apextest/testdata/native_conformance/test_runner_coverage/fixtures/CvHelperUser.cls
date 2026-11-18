@IsTest
private class CvHelperUser {
    @IsTest static void t() {
        System.assertEquals(1, CvHelperTest.helpOne());
        System.assertEquals(1, CvHelperPlain.helpOne());
    }
}
