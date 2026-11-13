@IsTest private class StopBoundaryLocalTest {
    @IsTest static void empty() {
        Test.startTest();
        Database.executeBatch(new StopBoundaryEmpty());
        System.enqueueJob(new StopOrderQ('k', false));
        Test.stopTest();
        System.assertEquals('stop=ok|seq=0:as,1:af,2:k|createdid=as,af,k', StopOrderMk.observe(false));
    }
}
