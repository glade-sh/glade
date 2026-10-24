@IsTest
private class HeapLimitCheckpointTest {
    @IsTest static void listSync() {
        HeapLimitProbe.grow('list', 50000000);
        System.assert(false, HeapLimitProbe.marker('list', 'sync', 'checkpoint'));
    }
    @IsTest static void mapSync() {
        HeapLimitProbe.grow('map', 50000000);
        System.assert(false, HeapLimitProbe.marker('map', 'sync', 'checkpoint'));
    }
    @IsTest static void blobSync() {
        HeapLimitProbe.grow('blob', 50000000);
        System.assert(false, HeapLimitProbe.marker('blob', 'sync', 'checkpoint'));
    }
    @IsTest static void listQueueable() {
        Test.startTest();
        System.enqueueJob(new HeapLimitQueueable('list', 50000000));
        Test.stopTest();
        System.assert(false, HeapLimitProbe.marker('list', 'queueable', 'checkpoint'));
    }
    @IsTest static void mapQueueable() {
        Test.startTest();
        System.enqueueJob(new HeapLimitQueueable('map', 50000000));
        Test.stopTest();
        System.assert(false, HeapLimitProbe.marker('map', 'queueable', 'checkpoint'));
    }
    @IsTest static void blobQueueable() {
        Test.startTest();
        System.enqueueJob(new HeapLimitQueueable('blob', 50000000));
        Test.stopTest();
        System.assert(false, HeapLimitProbe.marker('blob', 'queueable', 'checkpoint'));
    }
    @IsTest static void listFuture() {
        Test.startTest();
        HeapLimitFuture.grow('list', 50000000);
        Test.stopTest();
        System.assert(false, HeapLimitProbe.marker('list', 'future', 'checkpoint'));
    }
    @IsTest static void mapFuture() {
        Test.startTest();
        HeapLimitFuture.grow('map', 50000000);
        Test.stopTest();
        System.assert(false, HeapLimitProbe.marker('map', 'future', 'checkpoint'));
    }
    @IsTest static void blobFuture() {
        Test.startTest();
        HeapLimitFuture.grow('blob', 50000000);
        Test.stopTest();
        System.assert(false, HeapLimitProbe.marker('blob', 'future', 'checkpoint'));
    }
}
