@IsTest
private class QueueCapacityObservationTest {
    @IsTest
    static void syncCapacity() {
        Test.startTest();
        Map<String, Object> observation = new Map<String, Object>{
            'beforeLimit' => Limits.getLimitQueueableJobs(),
            'beforeCount' => Limits.getQueueableJobs(),
            'isQueueable' => System.isQueueable()
        };
        System.enqueueJob(new QueueCapacityProbe.EmptyWork());
        observation.put('afterLimit', Limits.getLimitQueueableJobs());
        observation.put('afterCount', Limits.getQueueableJobs());
        Test.stopTest();
        System.assert(false, 'OBSERVED|syncCapacity|' + JSON.serialize(observation));
    }

    @IsTest
    static void asyncCapacity() {
        Test.startTest();
        AsyncOptions options = new AsyncOptions();
        options.MaximumQueueableStackDepth = 5;
        System.enqueueJob(new QueueCapacityProbe.MeasureWork(), options);
        Test.stopTest();
        List<Account> observations = [SELECT Description FROM Account WHERE Name = 'Queue capacity observation'];
        Map<String, Object> observation = new Map<String, Object>{'rows' => observations.size()};
        if (!observations.isEmpty()) {
            observation.put('capacity', JSON.deserializeUntyped(observations[0].Description));
        }
        System.assert(false, 'OBSERVED|asyncCapacity|' + JSON.serialize(observation));
    }
}
