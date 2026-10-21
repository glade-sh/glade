@IsTest
private class ImmediateEventFlowProbeTest {
    private static void forward(List<SObject> events) {
        List<Database.SaveResult> outcomes = EventBus.publish(events);
        System.assertEquals(true, outcomes[0].isSuccess());
    }
    @IsTest static void observe() {
        Map<String,Object> result = new Map<String,Object>();
        Test.startTest();
        forward(new List<ImmediateFlowNotice__e>{new ImmediateFlowNotice__e(Token__c='Owned Explicit Notice')});
        result.put('beforeExplicit', [SELECT COUNT() FROM Account WHERE Name='Owned Explicit Notice']);
        Test.getEventBus().deliver();
        result.put('afterExplicit', [SELECT COUNT() FROM Account WHERE Name='Owned Explicit Notice']);
        forward(new List<ImmediateFlowNotice__e>{new ImmediateFlowNotice__e(Token__c='Owned Stop Notice')});
        EventBus.publish(new ImmediateFlowNotice__e(Token__c='Owned Scalar Notice'));
        result.put('beforeStop', [SELECT COUNT() FROM Account WHERE Name='Owned Stop Notice']);
        Test.stopTest();
        result.put('afterStop', [SELECT COUNT() FROM Account WHERE Name='Owned Stop Notice']);
        result.put('afterScalar', [SELECT COUNT() FROM Account WHERE Name='Owned Scalar Notice']);
        result.put('explicitAfterStop', [SELECT COUNT() FROM Account WHERE Name='Owned Explicit Notice']);
        System.assert(false, 'OBS|' + JSON.serialize(result));
    }
}
