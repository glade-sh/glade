public class SchemaProbe {
    static String ex(Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
    static String q(String soql) {
        try { List<SObject> l = Database.query(soql); return 'ok:' + l.size(); } catch (Exception e) { return ex(e); }
    }
    static String d(String name) {
        try {
            Schema.SObjectType t = Schema.getGlobalDescribe().get(name);
            if (t == null) return 'absent';
            Schema.DescribeSObjectResult r = t.getDescribe();
            List<String> f = new List<String>(r.fields.getMap().keySet());
            f.sort();
            return 'present;fields=' + f.size() + ';' + String.join(f, ',');
        } catch (Exception e) { return ex(e); }
    }
    public static Map<String, Object> observe() {
        Map<String, Object> r = new Map<String, Object>();
        r.put('q_network', q('SELECT Id FROM Network'));
        r.put('q_networkmember', q('SELECT Id FROM NetworkMember'));
        r.put('q_share', q('SELECT Id FROM Widget__Share'));
        r.put('q_share_cols', q('SELECT Id, ParentId, UserOrGroupId, AccessLevel, RowCause FROM Widget__Share'));
        r.put('q_mdt_masterlabel', q('SELECT MasterLabel, DeveloperName, Flag__c FROM Cfg__mdt'));
        r.put('q_mdt_language', q('SELECT Language, NamespacePrefix, Label FROM Cfg__mdt'));
        r.put('q_event', q('SELECT Id FROM RecordAccessEvent__e'));
        r.put('gd_network', d('network'));
        r.put('gd_networkmember', d('networkmember'));
        r.put('gd_share', d('widget__share'));
        r.put('gd_mdt', d('cfg__mdt'));
        r.put('gd_event', d('recordaccessevent__e'));
        try {
            Database.SaveResult sr = EventBus.publish((SObject) Type.forName('RecordAccessEvent__e').newInstance());
            r.put('publish_event', 'success=' + sr.isSuccess());
        } catch (Exception e) { r.put('publish_event', ex(e)); }
        return r;
    }
}
