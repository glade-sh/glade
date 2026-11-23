public class RunAsProbe {
    static String err(Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
    static String fmt(List<SObject> rows, String[] names) {
        List<String> parts = new List<String>();
        for (SObject r : rows) { List<String> vs = new List<String>(); for (String n : names) { vs.add(String.valueOf(r.get(n))); } parts.add(String.join(vs, ';')); }
        parts.sort();
        return rows.size() + '#' + String.join(parts, ' / ');
    }
    public static String share(Id did) {
        Map<String,String> got = new Map<String,String>();
        try { got.put('s_total', String.valueOf([SELECT Id FROM Synth_Doc__Share].size())); } catch (Exception e) { got.put('s_total', err(e)); }
        try { got.put('s_d1', String.valueOf([SELECT Id FROM Synth_Doc__Share WHERE ParentId = :did].size())); } catch (Exception e) { got.put('s_d1', err(e)); }
        try { got.put('s_manual', String.valueOf([SELECT Id FROM Synth_Doc__Share WHERE RowCause = 'Manual'].size())); } catch (Exception e) { got.put('s_manual', err(e)); }
        try { got.put('parent_total', String.valueOf([SELECT Id FROM Synth_Doc__c].size())); } catch (Exception e) { got.put('parent_total', err(e)); }
        try {
            Synth_Doc__c own = new Synth_Doc__c(Name = 'own');
            insert own;
            got.put('own_shares', String.valueOf([SELECT Id FROM Synth_Doc__Share WHERE ParentId = :own.Id].size()));
            got.put('own_rowcauses', String.join(new List<String>(new Set<String>(rc([SELECT RowCause FROM Synth_Doc__Share WHERE ParentId = :own.Id]))), ','));
            got.put('s_total_after', String.valueOf([SELECT Id FROM Synth_Doc__Share].size()));
        } catch (Exception e) { got.put('own', err(e)); }
        return JSON.serialize(got);
    }
    static List<String> rc(List<Synth_Doc__Share> l) { List<String> r = new List<String>(); for (Synth_Doc__Share s : l) r.add(s.RowCause); return r; }
    public static String mdt() {
        Map<String,String> got = new Map<String,String>();
        String[] all = new String[]{'MasterLabel','DeveloperName','Val__c','QualifiedApiName','Label'};
        try { got.put('m_user_all', fmt(Database.query('SELECT MasterLabel, DeveloperName, Val__c, QualifiedApiName, Label FROM Synth__mdt WITH USER_MODE'), all)); } catch (Exception e) { got.put('m_user_all', err(e)); }
        try { got.put('m_user_ml', fmt(Database.query('SELECT MasterLabel FROM Synth__mdt WITH USER_MODE'), new String[]{'MasterLabel'})); } catch (Exception e) { got.put('m_user_ml', err(e)); }
        try { got.put('m_user_val', fmt(Database.query('SELECT Val__c FROM Synth__mdt WITH USER_MODE'), new String[]{'Val__c'})); } catch (Exception e) { got.put('m_user_val', err(e)); }
        try { got.put('m_user_dn', fmt(Database.query('SELECT DeveloperName FROM Synth__mdt WITH USER_MODE'), new String[]{'DeveloperName'})); } catch (Exception e) { got.put('m_user_dn', err(e)); }
        try { got.put('m_sys_all', fmt(Database.query('SELECT MasterLabel, DeveloperName, Val__c, QualifiedApiName, Label FROM Synth__mdt WITH SYSTEM_MODE'), all)); } catch (Exception e) { got.put('m_sys_all', err(e)); }
        try { got.put('m_plain_all', fmt(Database.query('SELECT MasterLabel, DeveloperName, Val__c, QualifiedApiName, Label FROM Synth__mdt'), all)); } catch (Exception e) { got.put('m_plain_all', err(e)); }
        try { got.put('m_static_user', fmt([SELECT MasterLabel, DeveloperName, Val__c FROM Synth__mdt WITH USER_MODE], new String[]{'MasterLabel','DeveloperName','Val__c'})); } catch (Exception e) { got.put('m_static_user', err(e)); }
        return JSON.serialize(got);
    }
    public static String describeProbe() {
        Map<String,String> got = new Map<String,String>();
        try { Schema.DescribeSObjectResult d = Synth__mdt.SObjectType.getDescribe(); got.put('m_desc', d.isAccessible() + ',' + d.isQueryable()); } catch (Exception e) { got.put('m_desc', err(e)); }
        try { Map<String, Schema.SObjectField> fm = Synth__mdt.SObjectType.getDescribe().fields.getMap(); got.put('m_fields', fm.get('Val__c').getDescribe().isAccessible() + ',' + fm.get('MasterLabel').getDescribe().isAccessible() + ',' + fm.get('DeveloperName').getDescribe().isAccessible()); } catch (Exception e) { got.put('m_fields', err(e)); }
        return JSON.serialize(got);
    }
    public static String who() {
        try { return [SELECT Profile.Name FROM User WHERE Id = :UserInfo.getUserId()].Profile.Name; } catch (Exception e) { return 'noprofile'; }
    }
}
