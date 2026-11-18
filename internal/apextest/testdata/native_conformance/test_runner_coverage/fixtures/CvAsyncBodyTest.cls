@IsTest
private class CvAsyncBodyTest {
    @IsTest static void body() {
        Test.startTest();
        System.enqueueJob(new CvAsyncJob());
        Test.stopTest();
    }
}
