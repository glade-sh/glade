public class DmlNotAllowedProbe {
    static String err(Exception e) { return e.getTypeName().replace('System.', '') + ':' + e.getMessage(); }
    static Object probe(String objectName) {
        Map<String,Object> m = new Map<String,Object>();
        Schema.SObjectType t = Schema.getGlobalDescribe().get(objectName);
        Id fakeId = Id.valueOf(t.getDescribe().getKeyPrefix() + '000000000001');
        List<SObject> existing = new List<SObject>{ t.newSObject(fakeId) };
        SObject fresh = t.newSObject();
        try { insert fresh; m.put('insert', 'ok'); } catch (Exception e) { m.put('insert', err(e)); }
        try { Database.SaveResult sr = Database.insert(t.newSObject(), false); m.put('dbInsertPartial', 'ok ' + sr.isSuccess() + (sr.isSuccess() ? '' : ' ' + sr.getErrors()[0].getStatusCode() + ' ' + sr.getErrors()[0].getMessage())); } catch (Exception e) { m.put('dbInsertPartial', err(e)); }
        try { upsert t.newSObject(); m.put('upsert', 'ok'); } catch (Exception e) { m.put('upsert', err(e)); }
        {
            try { update existing[0]; m.put('update', 'ok'); } catch (Exception e) { m.put('update', err(e)); }
            try { Database.delete(new List<SObject>{ existing[0] }, true, AccessLevel.SYSTEM_MODE); m.put('listDeleteSystem', 'ok'); } catch (Exception e) { m.put('listDeleteSystem', err(e)); }
            try { delete existing[0]; m.put('delete', 'ok'); } catch (Exception e) { m.put('delete', err(e)); }
        }
        m.put('createable', t.getDescribe().isCreateable());
        return m;
    }
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        for (String n : new List<String>{ 'Profile', 'ApexClass', 'ApexTrigger', 'RecordType', 'Organization', 'ApexLog', 'UserLicense' }) {
            try { m.put(n, probe(n)); } catch (Exception e) { m.put(n, 'probe ' + err(e)); }
        }
        return m;
    }
}
