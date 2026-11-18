@IsTest
private class CvUnionTest {
    @IsTest static void aLeft() {
        System.assertEquals(10, CvUnion.choose(true));
    }
    @IsTest static void bRight() {
        System.assertEquals(20, CvUnion.choose(false));
    }
    @IsTest static void cIntentionalFailure() {
        Integer actual = CvUnion.choose(false);
        System.assertEquals(-1, actual, 'coverage-failure-probe');
    }
    @IsTest static void dUncaught() {
        Integer actual = CvUnion.choose(true);
        throw new CvException('uncaught-probe');
    }
}
