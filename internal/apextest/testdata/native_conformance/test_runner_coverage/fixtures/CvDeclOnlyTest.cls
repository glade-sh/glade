@IsTest
private class CvDeclOnlyTest {
    @IsTest static void t() {
        System.assertNotEquals(null, new CvDeclOnly());
    }
}
