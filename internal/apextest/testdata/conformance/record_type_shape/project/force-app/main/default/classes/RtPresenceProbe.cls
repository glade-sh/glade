public class RtPresenceProbe {
    static String err(Exception e) { return e.getTypeName().replace('System.', '') + ':' + e.getMessage(); }
    static String rtName(Id rt) { return rt == null ? 'null' : [SELECT DeveloperName FROM RecordType WHERE Id = :rt].DeveloperName; }
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        List<String> infos = new List<String>();
        for (Schema.RecordTypeInfo i : V35Rt__c.SObjectType.getDescribe().getRecordTypeInfos()) { infos.add(i.getName() + ':' + i.isAvailable() + ':' + i.isDefaultRecordTypeMapping()); }
        infos.sort();
        m.put('infos', infos);
        V35NoRt__c control = new V35NoRt__c(Name = 'control');
        try { control.put('RecordTypeId', null); m.put('controlPut', 'ok'); } catch (Exception e) { m.put('controlPut', err(e)); }
        try { insert control; m.put('controlInsert', 'ok'); } catch (Exception e) { m.put('controlInsert', err(e)); }
        m.put('controlHasField', V35NoRt__c.SObjectType.getDescribe().fields.getMap().containsKey('recordtypeid'));
        Account acc = new Account(Name = 'V35 Acct');
        try { acc.put('RecordTypeId', null); insert acc; m.put('accountNullRt', 'ok'); } catch (Exception e) { m.put('accountNullRt', err(e)); }
        m.put('accountHasRtField', Account.SObjectType.getDescribe().fields.getMap().containsKey('recordtypeid'));
        delete [SELECT Id FROM V35NoRt__c];
        return m;
    }
}
