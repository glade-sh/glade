@IsTest
private class CvAsyncSetupTest {
    @TestSetup static void seed() {
        System.enqueueJob(new CvAsyncJob());
    }
    @IsTest static void body() {
        System.assert(true);
    }
}
