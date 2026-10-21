@IsTest
private class CycleFinalizerProbeTest {
    @IsTest static void observe() {
        Test.startTest();
        System.enqueueJob(new CycleWorkerProbe());
        Test.stopTest();
        Integer markers = [SELECT COUNT() FROM Account WHERE Name = 'Cyclic observer'];
        System.assert(false, 'OBS|' + JSON.serialize(new Map<String, Object>{'markers' => markers}));
    }
}
