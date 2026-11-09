public class ListRunAsProbe {
    static Object probe() {
        Map<String,Object> m = new Map<String,Object>();
        try { m.put('plainCount', [SELECT COUNT() FROM V35List__c]); } catch (Exception e) { m.put('plainCount', e.getTypeName()); }
        try { m.put('userModeQuery', [SELECT Id FROM V35List__c WITH USER_MODE].size()); } catch (Exception e) { m.put('userModeQuery', e.getTypeName()); }
        try { m.put('systemModeQuery', [SELECT Id FROM V35List__c WITH SYSTEM_MODE].size()); } catch (Exception e) { m.put('systemModeQuery', e.getTypeName()); }
        try { m.put('dynamicSystem', Database.query('SELECT Id FROM V35List__c', AccessLevel.SYSTEM_MODE).size()); } catch (Exception e) { m.put('dynamicSystem', e.getTypeName()); }
        try { m.put('dynamicUser', Database.query('SELECT Id FROM V35List__c', AccessLevel.USER_MODE).size()); } catch (Exception e) { m.put('dynamicUser', e.getTypeName()); }
        try { m.put('countQuery', Database.countQuery('SELECT COUNT() FROM V35List__c')); } catch (Exception e) { m.put('countQuery', e.getTypeName()); }
        try { m.put('getAll', V35List__c.getAll().size()); } catch (Exception e) { m.put('getAll', e.getTypeName()); }
        try { m.put('getInstance', V35List__c.getInstance('alpha') == null ? 'null' : 'found'); } catch (Exception e) { m.put('getInstance', e.getTypeName()); }
        m.put('withSharing', ListRunAsWithSharing.observe());
        m.put('withoutSharing', ListRunAsWithoutSharing.observe());
        m.put('describeAccessible', V35List__c.SObjectType.getDescribe().isAccessible());
        m.put('describeCreateable', V35List__c.SObjectType.getDescribe().isCreateable());
        return m;
    }
    static Object probeHier() {
        Map<String,Object> m = new Map<String,Object>();
        try { m.put('plainCount', [SELECT COUNT() FROM V35Hier__c]); } catch (Exception e) { m.put('plainCount', e.getTypeName()); }
        try { m.put('userModeQuery', [SELECT Id FROM V35Hier__c WITH USER_MODE].size()); } catch (Exception e) { m.put('userModeQuery', e.getTypeName()); }
        try { m.put('systemModeQuery', [SELECT Id FROM V35Hier__c WITH SYSTEM_MODE].size()); } catch (Exception e) { m.put('systemModeQuery', e.getTypeName()); }
        try { m.put('dynamicSystem', Database.query('SELECT Id FROM V35Hier__c', AccessLevel.SYSTEM_MODE).size()); } catch (Exception e) { m.put('dynamicSystem', e.getTypeName()); }
        try { m.put('dynamicUser', Database.query('SELECT Id FROM V35Hier__c', AccessLevel.USER_MODE).size()); } catch (Exception e) { m.put('dynamicUser', e.getTypeName()); }
        try { m.put('countQuery', Database.countQuery('SELECT COUNT() FROM V35Hier__c')); } catch (Exception e) { m.put('countQuery', e.getTypeName()); }
        try { m.put('orgDefaults', V35Hier__c.getOrgDefaults() == null ? 'null' : String.valueOf(V35Hier__c.getOrgDefaults().Val__c)); } catch (Exception e) { m.put('orgDefaults', e.getTypeName()); }
        m.put('withSharing', ListRunAsWithSharing.observeHier());
        m.put('withoutSharing', ListRunAsWithoutSharing.observeHier());
        m.put('describeAccessible', V35Hier__c.SObjectType.getDescribe().isAccessible());
        m.put('describeCreateable', V35Hier__c.SObjectType.getDescribe().isCreateable());
        return m;
    }
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        delete [SELECT Id FROM V35List__c];
        delete [SELECT Id FROM V35Hier__c];
        insert new V35Hier__c(SetupOwnerId = UserInfo.getOrganizationId(), Val__c = 'a');
        insert new List<V35List__c>{ new V35List__c(Name = 'alpha', Val__c = 'a'), new V35List__c(Name = 'beta', Val__c = 'b') };
        m.put('admin', probe());
        m.put('adminHier', probeHier());
        if (!Test.isRunningTest()) { m.put('standard', 'not a test run'); return m; }
        Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
        String uniq = String.valueOf(Math.abs(Crypto.getRandomInteger()));
        User u = new User(Alias = 'v35u', Email = 'v35u' + uniq + '@example.com', EmailEncodingKey = 'UTF-8', LastName = 'V35', LanguageLocaleKey = 'en_US',
            LocaleSidKey = 'en_US', ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles', Username = 'v35u' + uniq + '@example.com');
        insert u;
        try {
            System.runAs(u) { m.put('standard', probe()); m.put('standardHier', probeHier()); }
        } catch (Exception e) { m.put('standard', e.getTypeName() + ':' + e.getMessage()); }
        return m;
    }
}
