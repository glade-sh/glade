@IsTest
private class HeapLimitThrowTest {
    @IsTest static void listSync() {
        HeapLimitProbe.grow('list', 60000000);
        System.assert(false, HeapLimitProbe.marker('list', 'sync', 'completed'));
    }
    @IsTest static void mapSync() {
        HeapLimitProbe.grow('map', 60000000);
        System.assert(false, HeapLimitProbe.marker('map', 'sync', 'completed'));
    }
    @IsTest static void blobSync() {
        HeapLimitProbe.grow('blob', 60000000);
        System.assert(false, HeapLimitProbe.marker('blob', 'sync', 'completed'));
    }
    @IsTest static void listQueueable() {
        Test.startTest();
        System.enqueueJob(new HeapLimitQueueable('list', 60000000));
        Test.stopTest();
        System.assert(false, HeapLimitProbe.marker('list', 'queueable', 'completed'));
    }
    @IsTest static void mapQueueable() {
        Test.startTest();
        System.enqueueJob(new HeapLimitQueueable('map', 60000000));
        Test.stopTest();
        System.assert(false, HeapLimitProbe.marker('map', 'queueable', 'completed'));
    }
    @IsTest static void blobQueueable() {
        Test.startTest();
        System.enqueueJob(new HeapLimitQueueable('blob', 60000000));
        Test.stopTest();
        System.assert(false, HeapLimitProbe.marker('blob', 'queueable', 'completed'));
    }
    @IsTest static void listFuture() {
        Test.startTest();
        HeapLimitFuture.grow('list', 60000000);
        Test.stopTest();
        System.assert(false, HeapLimitProbe.marker('list', 'future', 'completed'));
    }
    @IsTest static void mapFuture() {
        Test.startTest();
        HeapLimitFuture.grow('map', 60000000);
        Test.stopTest();
        System.assert(false, HeapLimitProbe.marker('map', 'future', 'completed'));
    }
    @IsTest static void blobFuture() {
        Test.startTest();
        HeapLimitFuture.grow('blob', 60000000);
        Test.stopTest();
        System.assert(false, HeapLimitProbe.marker('blob', 'future', 'completed'));
    }
}
