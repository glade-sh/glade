public class ListPartialDmlProbe {
    static String err(Exception e) { return e.getTypeName().replace('System.', '') + ':' + e.getMessage(); }
    static String res(Database.SaveResult r) { return r.isSuccess() ? 'ok' : 'row ' + String.valueOf(r.getErrors()[0].getStatusCode()) + ' ' + r.getErrors()[0].getMessage(); }
    static Map<String,Object> matrixRows() {
        Map<String,Object> m = new Map<String,Object>();
        for (String n : new List<String>{ 'UserLicense', 'Profile', 'ApexClass', 'RecordType' }) {
            Map<String,Object> o = new Map<String,Object>();
            Schema.SObjectType t = Schema.getGlobalDescribe().get(n);
            try { Database.SaveResult r = Database.insert(t.newSObject(), false); o.put('singlePartial', res(r)); } catch (Exception e) { o.put('singlePartial', err(e)); }
            try { Database.SaveResult[] r = Database.insert(new List<SObject>{ t.newSObject() }, false); o.put('listPartial', res(r[0])); } catch (Exception e) { o.put('listPartial', err(e)); }
            try { Database.SaveResult[] r = Database.insert(new List<SObject>{ t.newSObject() }, true); o.put('listAllOrNone', res(r[0])); } catch (Exception e) { o.put('listAllOrNone', err(e)); }
            try { Database.SaveResult[] r = Database.insert(new List<SObject>{ t.newSObject(), t.newSObject() }, false); o.put('list2Partial', res(r[0]) + '|' + r.size()); } catch (Exception e) { o.put('list2Partial', err(e)); }
            try { Database.SaveResult[] r = Database.insert(new List<SObject>{ t.newSObject() }, false, AccessLevel.SYSTEM_MODE); o.put('listPartialSystem', res(r[0])); } catch (Exception e) { o.put('listPartialSystem', err(e)); }
            try { Database.SaveResult[] r = Database.insert(new List<SObject>{ t.newSObject() }); o.put('listDefault', res(r[0])); } catch (Exception e) { o.put('listDefault', err(e)); }
            m.put(n, o);
        }
        return m;
    }

    static String ures(Database.UpsertResult r) { return r.isSuccess() ? 'ok' : 'row ' + String.valueOf(r.getErrors()[0].getStatusCode()) + ' ' + r.getErrors()[0].getMessage(); }
        static String dres(Database.DeleteResult r) { return r.isSuccess() ? 'ok' : 'row ' + String.valueOf(r.getErrors()[0].getStatusCode()) + ' ' + r.getErrors()[0].getMessage(); }
    static Map<String,Object> opsRows() {
        Map<String,Object> m = new Map<String,Object>();
        for (String n : new List<String>{ 'UserLicense', 'Profile', 'RecordType', 'Organization', 'ApexLog' }) {
            Map<String,Object> o = new Map<String,Object>();
            Schema.SObjectType t = Schema.getGlobalDescribe().get(n);
            Id fakeId = Id.valueOf(t.getDescribe().getKeyPrefix() + '000000000001');
            try { Database.SaveResult[] r = Database.insert(new List<SObject>{ t.newSObject() }, false); o.put('insertList', res(r[0])); } catch (Exception e) { o.put('insertList', err(e)); }
            try { Database.SaveResult[] r = Database.update(new List<SObject>{ t.newSObject(fakeId) }, false); o.put('updateList', res(r[0])); } catch (Exception e) { o.put('updateList', err(e)); }
            try { Database.SaveResult r = Database.update(t.newSObject(fakeId), false); o.put('updateSingle', res(r)); } catch (Exception e) { o.put('updateSingle', err(e)); }
            try { Database.UpsertResult[] r = Database.upsert(new List<SObject>{ t.newSObject() }, false); o.put('upsertList', ures(r[0])); } catch (Exception e) { o.put('upsertList', err(e)); }
            try { Database.UpsertResult r = Database.upsert(t.newSObject(), false); o.put('upsertSingle', ures(r)); } catch (Exception e) { o.put('upsertSingle', err(e)); }
            try { Database.DeleteResult[] r = Database.delete(new List<SObject>{ t.newSObject(fakeId) }, false); o.put('deleteList', dres(r[0])); } catch (Exception e) { o.put('deleteList', err(e)); }
            try { Database.DeleteResult r = Database.delete(t.newSObject(fakeId), false); o.put('deleteSingle', dres(r)); } catch (Exception e) { o.put('deleteSingle', err(e)); }
            try { Database.DeleteResult[] r = Database.delete(new List<Id>{ fakeId }, false); o.put('deleteIdList', dres(r[0])); } catch (Exception e) { o.put('deleteIdList', err(e)); }
            m.put(n, o);
        }
        return m;
    }
    public static Map<String,Object> matrix() { return new Map<String,Object>{ 'matrix' => matrixRows() }; }
    public static Map<String,Object> ops() { return new Map<String,Object>{ 'ops' => opsRows() }; }
    public static Map<String,Object> observe() { Map<String,Object> m = matrix(); m.putAll(ops()); return m; }
}
