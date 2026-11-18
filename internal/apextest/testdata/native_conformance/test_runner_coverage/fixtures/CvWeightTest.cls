@IsTest
private class CvWeightTest {
    @IsTest static void t() {
        System.assertEquals(1, CvBig.used());
        System.assertEquals(1, CvSmall.f());
    }
}
