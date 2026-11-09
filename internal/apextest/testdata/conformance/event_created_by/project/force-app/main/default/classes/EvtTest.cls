@IsTest
private class EvtTest {
    @IsTest static void t() {
        Test.startTest();
        Database.SaveResult sr = EventBus.publish(new Log__e(Msg__c = 'x'));
        System.assert(sr.isSuccess());
        Test.getEventBus().deliver();
        Test.stopTest();
        List<LogRec__c> r = [SELECT Usr__c, CB__c, CD__c FROM LogRec__c];
        System.debug('ROWS=' + r.size());
        System.debug('ROW0=' + r);
        System.assertEquals(1, r.size());
        System.assertEquals(UserInfo.getUserId(), r[0].Usr__c, 'usr');
        System.assertNotEquals(null, r[0].CD__c, 'cd');
        System.assertEquals(String.valueOf(UserInfo.getUserId()), r[0].CB__c, 'cb');
    }
}
