@IsTest
private class EvtRunAs {
    static User mk() {
        Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
        return new User(Alias='ru1', Email='ru1@example.com', EmailEncodingKey='UTF-8', LastName='RunAsOne', LanguageLocaleKey='en_US', LocaleSidKey='en_US', ProfileId=p.Id, TimeZoneSidKey='America/Chicago', Username='ru1.' + System.currentTimeMillis() + '@example.com');
    }
    @IsTest static void deliverInsideRunAs() {
        User u = mk(); insert u;
        insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = [SELECT Id FROM PermissionSet WHERE Name = 'QAll'].Id);
        System.runAs(u) {
            Test.startTest();
            System.assert(EventBus.publish(new Log__e(Msg__c = 'x')).isSuccess());
            Test.getEventBus().deliver();
            Test.stopTest();
        }
        List<LogRec__c> r = [SELECT Usr__c, CB__c FROM LogRec__c];
        System.assertEquals(1, r.size());
        System.assertEquals(u.Id, r[0].Usr__c, 'insideRunAs cb is runAs user; got ' + r[0].Usr__c + ' running ' + UserInfo.getUserId());
        System.assertEquals(String.valueOf(u.Id), r[0].CB__c, 'text');
    }
    @IsTest static void deliverOutsideRunAs() {
        User u = mk(); insert u;
        insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = [SELECT Id FROM PermissionSet WHERE Name = 'QAll'].Id);
        System.runAs(u) {
            System.assert(EventBus.publish(new Log__e(Msg__c = 'x')).isSuccess());
        }
        Test.getEventBus().deliver();
        List<LogRec__c> r = [SELECT Usr__c, CB__c FROM LogRec__c];
        System.assertEquals(1, r.size());
        System.assertEquals(u.Id, r[0].Usr__c, 'outsideRunAs cb is runAs user; got ' + r[0].Usr__c + ' running ' + UserInfo.getUserId());
    }
    @IsTest static void publishOutsideDeliverInside() {
        User u = mk(); insert u;
        insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = [SELECT Id FROM PermissionSet WHERE Name = 'QAll'].Id);
        Id me = UserInfo.getUserId();
        System.assert(EventBus.publish(new Log__e(Msg__c = 'x')).isSuccess());
        System.runAs(u) {
            Test.getEventBus().deliver();
        }
        List<LogRec__c> r = [SELECT Usr__c, CB__c FROM LogRec__c];
        System.assertEquals(1, r.size());
        System.assertEquals(me, r[0].Usr__c, 'pubOutside cb is publisher; got ' + r[0].Usr__c + ' runAs ' + u.Id);
    }
}
