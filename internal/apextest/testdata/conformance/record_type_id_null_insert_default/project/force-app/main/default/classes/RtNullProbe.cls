public class RtNullProbe {
    static String err(Exception e) { return e.getTypeName().replace('System.', '') + ':' + e.getMessage(); }
    static String rtName(Id rt) { return rt == null ? 'null' : [SELECT DeveloperName FROM RecordType WHERE Id = :rt].DeveloperName; }
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        V35Rt__c plain = new V35Rt__c(Name = 'plain');
        try { insert plain; m.put('plainInsert', rtName(plain.RecordTypeId)); } catch (Exception e) { m.put('plainInsert', err(e)); }
        V35Rt__c explicitNull = new V35Rt__c(Name = 'explicitNull', RecordTypeId = null);
        try { insert explicitNull; m.put('explicitNullInsert', 'ok ' + rtName(explicitNull.RecordTypeId)); } catch (Exception e) { m.put('explicitNullInsert', err(e)); }
        Database.SaveResult sr = Database.insert(new V35Rt__c(Name = 'partial', RecordTypeId = null), false);
        m.put('explicitNullPartial', sr.isSuccess() ? 'ok' : String.valueOf(sr.getErrors()[0].getStatusCode()) + ' ' + sr.getErrors()[0].getMessage() + ' ' + sr.getErrors()[0].getFields());
        V35Rt__c viaPut = new V35Rt__c(Name = 'viaPut');
        viaPut.put('RecordTypeId', null);
        try { insert viaPut; m.put('putNullInsert', 'ok'); } catch (Exception e) { m.put('putNullInsert', err(e)); }
        V35Rt__c cleared = new V35Rt__c(Name = 'cleared');
        insert cleared;
        cleared.RecordTypeId = null;
        try { update cleared; m.put('updateToNull', 'ok ' + rtName([SELECT RecordTypeId FROM V35Rt__c WHERE Id = :cleared.Id].RecordTypeId)); } catch (Exception e) { m.put('updateToNull', err(e)); }
        delete [SELECT Id FROM V35Rt__c];
        return m;
    }
}
