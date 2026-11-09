public class PubAccessProbe {
    static User mk(String profileName, String alias) {
        Profile p = [SELECT Id FROM Profile WHERE Name = :profileName LIMIT 1];
        return new User(Alias = alias, Email = alias + '@example.com', EmailEncodingKey = 'UTF-8', LastName = 'Pub' + alias, LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', ProfileId = p.Id, TimeZoneSidKey = 'America/Chicago', Username = alias + '.' + System.currentTimeMillis() + '@example.com');
    }
    static Object results(List<Database.SaveResult> rs) {
        List<Object> out = new List<Object>();
        for (Database.SaveResult r : rs) {
            List<String> errs = new List<String>();
            if (!r.isSuccess()) for (Database.Error e : r.getErrors()) { errs.add(String.valueOf(e.getStatusCode()) + '|' + e.getMessage()); }
            out.add(new Map<String, Object>{'ok' => r.isSuccess(), 'errors' => errs});
        }
        return out;
    }
    static Object tryPublish(Integer count) {
        try {
            List<Pub__e> evts = new List<Pub__e>();
            for (Integer i = 0; i < count; i++) { evts.add(new Pub__e(Msg__c = 'm' + i)); }
            return results(EventBus.publish(evts));
        } catch (Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
    }
    static Object tryDml() {
        try { Database.insert(new Probe__c(Name = 'p', Val__c = 'x')); return 'ok'; } catch (Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
    }
    static Map<String, Object> asUser(String profileName, String alias, Boolean grant) {
        User u = mk(profileName, alias);
        insert u;
        if (grant) { insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = [SELECT Id FROM PermissionSet WHERE Name = 'PubGrant'].Id); }
        Map<String, Object> o = new Map<String, Object>();
        System.runAs(u) {
            Schema.DescribeSObjectResult d = Pub__e.SObjectType.getDescribe();
            o.put('createable', d.isCreateable());
            o.put('accessible', d.isAccessible());
            o.put('publishOne', tryPublish(1));
            o.put('publishTwo', tryPublish(2));
            o.put('dmlProbe', tryDml());
        }
        return o;
    }
    public static Map<String, Object> observe() {
        Map<String, Object> o = new Map<String, Object>();
        o.put('adminPublish', tryPublish(1));
        o.put('standardUserNoGrant', asUser('Standard User', 'su1', false));
        o.put('standardUserGrant', asUser('Standard User', 'su2', true));
        try { o.put('platformUserNoGrant', asUser('Standard Platform User', 'sp1', false)); } catch (Exception e) { o.put('platformUserNoGrant', e.getTypeName() + '|' + e.getMessage()); }
        try { o.put('minimumAccessNoGrant', asUser('Minimum Access - Salesforce', 'ma1', false)); } catch (Exception e) { o.put('minimumAccessNoGrant', e.getTypeName() + '|' + e.getMessage()); }
        return o;
    }
}
