@IsTest private class StaticCallbackProbeTest {
    @IsTest static void observe() {
        Test.startTest();
        System.schedule('Owned static schedule ' + Datetime.now().getTime(), '0 0 0 1 1 ? 2099', new StaticScheduleProbe());
        System.enqueueJob(new StaticQueueProbe());
        Database.executeBatch(new StaticBatchProbe());
        Test.stopTest();
        List<String> markers = new List<String>();
        for (Account row : [SELECT Name FROM Account ORDER BY Name]) markers.add(row.Name);
        System.assert(false, 'OBS|' + JSON.serialize(markers));
    }
    @IsTest static void observeScheduledBatch() { Test.startTest(); System.scheduleBatch(new StaticBatchProbe(), 'Owned delayed batch ' + Datetime.now().getTime(), 1); Test.stopTest(); List<String> markers=new List<String>(); for(Account row:[SELECT Name FROM Account WHERE Name LIKE 'staticBatch%' ORDER BY Name]) markers.add(row.Name); System.assert(false, 'OBS|' + JSON.serialize(markers)); }
}
