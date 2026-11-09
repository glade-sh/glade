public with sharing class MdtUserModeProbe {
    static String fmt(List<SObject> rows, String[] names) {
        List<String> parts = new List<String>();
        for (SObject r : rows) { List<String> vs = new List<String>(); for (String n : names) { vs.add(String.valueOf(r.get(n))); } parts.add(String.join(vs, ';')); }
        parts.sort();
        return rows.size() + '#' + String.join(parts, ' / ');
    }
    static String err(Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
    public static Map<String,String> observe() {
        Map<String,String> got = new Map<String,String>();
        try { List<Synth__mdt> r = [SELECT MasterLabel, DeveloperName, Custom__c FROM Synth__mdt]; got.put('f1_sys', fmt(r, new String[]{'MasterLabel','DeveloperName','Custom__c'})); } catch (Exception e) { got.put('f1_sys', err(e)); }
        try { List<Synth__mdt> r = [SELECT MasterLabel, DeveloperName, Custom__c FROM Synth__mdt WITH USER_MODE]; got.put('f1_user', fmt(r, new String[]{'MasterLabel','DeveloperName','Custom__c'})); } catch (Exception e) { got.put('f1_user', err(e)); }
        try { List<Synth__mdt> r = [SELECT QualifiedApiName FROM Synth__mdt]; got.put('f2_sys', fmt(r, new String[]{'QualifiedApiName'})); } catch (Exception e) { got.put('f2_sys', err(e)); }
        try { List<Synth__mdt> r = [SELECT QualifiedApiName FROM Synth__mdt WITH USER_MODE]; got.put('f2_user', fmt(r, new String[]{'QualifiedApiName'})); } catch (Exception e) { got.put('f2_user', err(e)); }
        try { List<Synth__mdt> r = [SELECT Label FROM Synth__mdt]; got.put('f3_sys', fmt(r, new String[]{'Label'})); } catch (Exception e) { got.put('f3_sys', err(e)); }
        try { List<Synth__mdt> r = [SELECT Label FROM Synth__mdt WITH USER_MODE]; got.put('f3_user', fmt(r, new String[]{'Label'})); } catch (Exception e) { got.put('f3_user', err(e)); }
        try { List<Synth__mdt> r = [SELECT NamespacePrefix FROM Synth__mdt]; got.put('f4_sys', fmt(r, new String[]{'NamespacePrefix'})); } catch (Exception e) { got.put('f4_sys', err(e)); }
        try { List<Synth__mdt> r = [SELECT NamespacePrefix FROM Synth__mdt WITH USER_MODE]; got.put('f4_user', fmt(r, new String[]{'NamespacePrefix'})); } catch (Exception e) { got.put('f4_user', err(e)); }
        try { List<Synth__mdt> r = [SELECT MasterLabel, DeveloperName, Custom__c, QualifiedApiName, Label, NamespacePrefix FROM Synth__mdt]; got.put('f5_sys', fmt(r, new String[]{'MasterLabel','DeveloperName','Custom__c','QualifiedApiName','Label','NamespacePrefix'})); } catch (Exception e) { got.put('f5_sys', err(e)); }
        try { List<Synth__mdt> r = [SELECT MasterLabel, DeveloperName, Custom__c, QualifiedApiName, Label, NamespacePrefix FROM Synth__mdt WITH USER_MODE]; got.put('f5_user', fmt(r, new String[]{'MasterLabel','DeveloperName','Custom__c','QualifiedApiName','Label','NamespacePrefix'})); } catch (Exception e) { got.put('f5_user', err(e)); }
        try { List<Synth__mdt> r = [SELECT MasterLabel FROM Synth__mdt]; got.put('f6_sys', fmt(r, new String[]{'MasterLabel'})); } catch (Exception e) { got.put('f6_sys', err(e)); }
        try { List<Synth__mdt> r = [SELECT MasterLabel FROM Synth__mdt WITH USER_MODE]; got.put('f6_user', fmt(r, new String[]{'MasterLabel'})); } catch (Exception e) { got.put('f6_user', err(e)); }
        try { List<Synth__mdt> r = [SELECT Id FROM Synth__mdt]; got.put('f9_sys', String.valueOf(r.size())); } catch (Exception e) { got.put('f9_sys', err(e)); }
        try { List<Synth__mdt> r = [SELECT Id FROM Synth__mdt WITH USER_MODE]; got.put('f9_user', String.valueOf(r.size())); } catch (Exception e) { got.put('f9_user', err(e)); }
        try { List<Synth__mdt> r = [SELECT MasterLabel, DeveloperName FROM Synth__mdt WHERE MasterLabel = 'Alpha Label']; got.put('f7_sys', fmt(r, new String[]{'MasterLabel','DeveloperName'})); } catch (Exception e) { got.put('f7_sys', err(e)); }
        try { List<Synth__mdt> r = [SELECT MasterLabel, DeveloperName FROM Synth__mdt WHERE MasterLabel = 'Alpha Label' WITH USER_MODE]; got.put('f7_user', fmt(r, new String[]{'MasterLabel','DeveloperName'})); } catch (Exception e) { got.put('f7_user', err(e)); }
        try { List<Synth__mdt> r = [SELECT MasterLabel, DeveloperName FROM Synth__mdt ORDER BY MasterLabel DESC]; got.put('f8_sys', fmt(r, new String[]{'MasterLabel','DeveloperName'})); } catch (Exception e) { got.put('f8_sys', err(e)); }
        try { List<Synth__mdt> r = [SELECT MasterLabel, DeveloperName FROM Synth__mdt WITH USER_MODE ORDER BY MasterLabel DESC]; got.put('f8_user', fmt(r, new String[]{'MasterLabel','DeveloperName'})); } catch (Exception e) { got.put('f8_user', err(e)); }
        try { List<Synth__mdt> r = [SELECT Label, DeveloperName FROM Synth__mdt WHERE Label = 'Beta Label']; got.put('f10_sys', fmt(r, new String[]{'Label','DeveloperName'})); } catch (Exception e) { got.put('f10_sys', err(e)); }
        try { List<Synth__mdt> r = [SELECT Label, DeveloperName FROM Synth__mdt WHERE Label = 'Beta Label' WITH USER_MODE]; got.put('f10_user', fmt(r, new String[]{'Label','DeveloperName'})); } catch (Exception e) { got.put('f10_user', err(e)); }
        try { List<Synth__mdt> r = [SELECT QualifiedApiName FROM Synth__mdt WHERE QualifiedApiName = 'Alpha']; got.put('f11_sys', fmt(r, new String[]{'QualifiedApiName'})); } catch (Exception e) { got.put('f11_sys', err(e)); }
        try { List<Synth__mdt> r = [SELECT QualifiedApiName FROM Synth__mdt WHERE QualifiedApiName = 'Alpha' WITH USER_MODE]; got.put('f11_user', fmt(r, new String[]{'QualifiedApiName'})); } catch (Exception e) { got.put('f11_user', err(e)); }
        try { got.put('dyn_f1_user', fmt(Database.query('SELECT MasterLabel, DeveloperName, Custom__c FROM Synth__mdt WITH USER_MODE'), new String[]{'MasterLabel','DeveloperName','Custom__c'})); } catch (Exception e) { got.put('dyn_f1_user', err(e)); }
        try { got.put('dyn_f1_al', fmt(Database.query('SELECT MasterLabel, DeveloperName, Custom__c FROM Synth__mdt', AccessLevel.USER_MODE), new String[]{'MasterLabel','DeveloperName','Custom__c'})); } catch (Exception e) { got.put('dyn_f1_al', err(e)); }
        try { got.put('dyn_f1_bind', fmt(Database.queryWithBinds('SELECT MasterLabel, DeveloperName, Custom__c FROM Synth__mdt', new Map<String,Object>(), AccessLevel.USER_MODE), new String[]{'MasterLabel','DeveloperName','Custom__c'})); } catch (Exception e) { got.put('dyn_f1_bind', err(e)); }
        try { got.put('dyn_f1_sys', fmt(Database.query('SELECT MasterLabel, DeveloperName, Custom__c FROM Synth__mdt'), new String[]{'MasterLabel','DeveloperName','Custom__c'})); } catch (Exception e) { got.put('dyn_f1_sys', err(e)); }
        try { got.put('dyn_f3_user', fmt(Database.query('SELECT Label FROM Synth__mdt WITH USER_MODE'), new String[]{'Label'})); } catch (Exception e) { got.put('dyn_f3_user', err(e)); }
        try { got.put('dyn_f3_al', fmt(Database.query('SELECT Label FROM Synth__mdt', AccessLevel.USER_MODE), new String[]{'Label'})); } catch (Exception e) { got.put('dyn_f3_al', err(e)); }
        try { got.put('dyn_f3_bind', fmt(Database.queryWithBinds('SELECT Label FROM Synth__mdt', new Map<String,Object>(), AccessLevel.USER_MODE), new String[]{'Label'})); } catch (Exception e) { got.put('dyn_f3_bind', err(e)); }
        try { got.put('dyn_f3_sys', fmt(Database.query('SELECT Label FROM Synth__mdt'), new String[]{'Label'})); } catch (Exception e) { got.put('dyn_f3_sys', err(e)); }
        try { got.put('dyn_f5_user', fmt(Database.query('SELECT MasterLabel, DeveloperName, Custom__c, QualifiedApiName, Label, NamespacePrefix FROM Synth__mdt WITH USER_MODE'), new String[]{'MasterLabel','DeveloperName','Custom__c','QualifiedApiName','Label','NamespacePrefix'})); } catch (Exception e) { got.put('dyn_f5_user', err(e)); }
        try { got.put('dyn_f5_al', fmt(Database.query('SELECT MasterLabel, DeveloperName, Custom__c, QualifiedApiName, Label, NamespacePrefix FROM Synth__mdt', AccessLevel.USER_MODE), new String[]{'MasterLabel','DeveloperName','Custom__c','QualifiedApiName','Label','NamespacePrefix'})); } catch (Exception e) { got.put('dyn_f5_al', err(e)); }
        try { got.put('dyn_f5_bind', fmt(Database.queryWithBinds('SELECT MasterLabel, DeveloperName, Custom__c, QualifiedApiName, Label, NamespacePrefix FROM Synth__mdt', new Map<String,Object>(), AccessLevel.USER_MODE), new String[]{'MasterLabel','DeveloperName','Custom__c','QualifiedApiName','Label','NamespacePrefix'})); } catch (Exception e) { got.put('dyn_f5_bind', err(e)); }
        try { got.put('dyn_f5_sys', fmt(Database.query('SELECT MasterLabel, DeveloperName, Custom__c, QualifiedApiName, Label, NamespacePrefix FROM Synth__mdt'), new String[]{'MasterLabel','DeveloperName','Custom__c','QualifiedApiName','Label','NamespacePrefix'})); } catch (Exception e) { got.put('dyn_f5_sys', err(e)); }
        return got;
    }
}
