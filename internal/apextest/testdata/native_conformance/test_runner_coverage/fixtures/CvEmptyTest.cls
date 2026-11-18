@IsTest
private class CvEmptyTest {
    @IsTest static void createsEmptyInstance() {
        System.assertNotEquals(null, new CvEmpty());
    }
}
