@IsTest
private class CvFailTest {
    @IsTest static void failsAfterCall() {
        CvFail.a();
        System.assertEquals(2, 1, 'fail-after-call');
        CvFail.b();
    }
    @IsTest static void throwsAfterCall() {
        CvFail.b();
        throw new CvException('thrown-after-call');
    }
}
