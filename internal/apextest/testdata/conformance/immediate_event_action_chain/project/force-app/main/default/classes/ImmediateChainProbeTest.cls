@IsTest
private class ImmediateChainProbeTest {
    private static void forward(List<SObject> events) {
        List<Database.SaveResult> outcomes = EventBus.publish(events);
        System.assertEquals(true, outcomes[0].isSuccess());
    }
    private static List<String> outputs(String token) {
        List<String> result = new List<String>();
        String firstName = token+'-first';
        String secondName = token+'-second';
        for (Account record : [SELECT Name,Description FROM Account WHERE Name=:firstName OR Name=:secondName ORDER BY Name]) {
            result.add(record.Name+'|'+record.Description);
        }
        return result;
    }
    @IsTest static void observe() {
        Map<String,Object> result = new Map<String,Object>();
        Test.startTest();
        forward(new List<ImmediateChainNotice__e>{new ImmediateChainNotice__e(Token__c='Owned chain explicit')});
        result.put('beforeExplicit', outputs('Owned chain explicit'));
        Test.getEventBus().deliver();
        result.put('afterExplicit', outputs('Owned chain explicit'));
        EventBus.publish(new ImmediateChainNotice__e(Token__c='Owned chain stop'));
        result.put('beforeStop', outputs('Owned chain stop'));
        Test.stopTest();
        result.put('afterStop', outputs('Owned chain stop'));
        result.put('explicitAfterStop', outputs('Owned chain explicit'));
        result.put('explicitCount', outputs('Owned chain explicit').size());
        result.put('stopCount', outputs('Owned chain stop').size());
        System.assert(false, 'OBS|' + JSON.serialize(result));
    }
}
