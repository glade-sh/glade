public class EvtSubProbe {
    static Object tryRun(String what) {
        try {
            if (what == 'userModeQuery') { return [SELECT Id FROM Probe__c WITH USER_MODE].size(); }
            if (what == 'userModeInsert') { Database.insert(new Probe__c(Name = 'u', Val__c = 'x'), AccessLevel.USER_MODE); return 'ok'; }
            if (what == 'userModeInsertAccount') { Database.insert(new Account(Name = 'u'), AccessLevel.USER_MODE); return 'ok'; }
            if (what == 'systemInsert') { Database.insert(new Probe__c(Name = 's', Val__c = 'x'), AccessLevel.SYSTEM_MODE); return 'ok'; }
            if (what == 'userModeAccountQuery') { return [SELECT Id, Name FROM Account WITH USER_MODE].size(); }
        } catch (Exception e) {
            return e.getTypeName() + '|' + e.getMessage();
        }
        return null;
    }
    public static Map<String, Object> inSubscriber() {
        Map<String, Object> o = new Map<String, Object>();
        o.put('userType', UserInfo.getUserType());
        try { o.put('profileName', [SELECT Name FROM Profile WHERE Id = :UserInfo.getProfileId()].Name); } catch (Exception e) { o.put('profileName', e.getTypeName() + '|' + e.getMessage()); }
        Schema.DescribeSObjectResult p = Probe__c.SObjectType.getDescribe();
        o.put('probeAccessible', p.isAccessible());
        o.put('probeCreateable', p.isCreateable());
        o.put('probeUpdateable', p.isUpdateable());
        o.put('probeDeletable', p.isDeletable());
        Schema.DescribeFieldResult f = Probe__c.Val__c.getDescribe();
        o.put('valAccessible', f.isAccessible());
        o.put('valCreateable', f.isCreateable());
        o.put('valUpdateable', f.isUpdateable());
        Schema.DescribeSObjectResult g = Gov__c.SObjectType.getDescribe();
        o.put('govAccessible', g.isAccessible());
        o.put('govCreateable', g.isCreateable());
        o.put('govUpdateable', g.isUpdateable());
        o.put('govDeletable', g.isDeletable());
        o.put('govValAccessible', Gov__c.Val__c.getDescribe().isAccessible());
        o.put('govValCreateable', Gov__c.Val__c.getDescribe().isCreateable());
        o.put('govValUpdateable', Gov__c.Val__c.getDescribe().isUpdateable());
        Schema.DescribeSObjectResult a = Account.SObjectType.getDescribe();
        o.put('accountAccessible', a.isAccessible());
        o.put('accountCreateable', a.isCreateable());
        o.put('accountUpdateable', a.isUpdateable());
        o.put('accountNameUpdateable', Account.Name.getDescribe().isUpdateable());
        o.put('userModeQuery', tryRun('userModeQuery'));
        o.put('userModeInsert', tryRun('userModeInsert'));
        o.put('userModeInsertAccount', tryRun('userModeInsertAccount'));
        o.put('systemInsert', tryRun('systemInsert'));
        o.put('userModeAccountQuery', tryRun('userModeAccountQuery'));
        return o;
    }
    public static Map<String, Object> observe() {
        Database.insert(new Probe__c(Name = 'seed', Val__c = 'v'), AccessLevel.SYSTEM_MODE);
        Test.startTest();
        System.assert(EventBus.publish(new Evt__e(Note__c = 'n')).isSuccess());
        Test.stopTest();
        List<Obs__c> rows = Database.query('SELECT Data__c FROM Obs__c', AccessLevel.SYSTEM_MODE);
        Map<String, Object> o = new Map<String, Object>();
        o.put('rows', rows.size());
        o.put('subscriber', rows.isEmpty() ? null : JSON.deserializeUntyped(rows[0].Data__c));
        return o;
    }
}
