@IsTest
private class QueueableIdTextProbeTest {
    @IsTest static void observeEnqueuedIdText() {
        Test.startTest();
        Id enqueuedId = System.enqueueJob(new QueueableIdWorker(false));
        Test.stopTest();
        String enqueuedIdText = enqueuedId;
        System.assert(false, 'OBSERVED|observeEnqueuedIdText|' + JSON.serialize(new Map<String, Object>{'enqueuedIdTextLength' => enqueuedIdText.length()}));
    }

    @IsTest static void observeContextIdText() {
        Test.startTest();
        System.enqueueJob(new QueueableIdWorker(true));
        Test.stopTest();
    }
}
