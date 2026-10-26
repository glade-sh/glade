@IsTest private class AsyncIdentityProbeTest {
    @IsTest static void observe() {
        Test.startTest();
        String queueId = System.enqueueJob(new IdentityQueueProbe());
        String batchId = Database.executeBatch(new IdentityBatchProbe());
        String scheduleId = System.schedule('Owned identity schedule ' + Datetime.now().getTime(), '0 0 0 1 1 ? 2099', new IdentityScheduleProbe());
        Test.stopTest();
        System.assert(false, 'OBS|' + JSON.serialize(AsyncIdentityProbe.observe(queueId, batchId, scheduleId)));
    }
    @IsTest static void observeScheduledBatch() { Test.startTest(); System.scheduleBatch(new IdentityBatchProbe(), 'Owned delayed batch ' + Datetime.now().getTime(), 1); Test.stopTest(); Map<String,Object> markers=new Map<String,Object>(); for(Account row:[SELECT Name,Description FROM Account WHERE Name LIKE 'batch%' ORDER BY Name]) markers.put(row.Name, JSON.deserializeUntyped(row.Description)); System.assert(false, 'OBS|' + JSON.serialize(markers)); }
}
