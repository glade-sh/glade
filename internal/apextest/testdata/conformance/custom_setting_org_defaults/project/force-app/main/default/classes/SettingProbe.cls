public class SettingProbe {
    static String ex(Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
    public static Map<String, Object> s01_org_update() {
        Map<String, Object> r = new Map<String, Object>();
        try { Prefs__c a = Prefs__c.getOrgDefaults(); r.put('null', a == null); r.put('id', a.Id); a.Flag__c = true; try { update a; r.put('dml','ok'); } catch (Exception e) { r.put('dml', ex(e)); } r.put('count', [SELECT COUNT() FROM Prefs__c]); r.put('id_after', a.Id != null); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s02_org_upsert() {
        Map<String, Object> r = new Map<String, Object>();
        try { Prefs__c a = Prefs__c.getOrgDefaults(); a.Flag__c = true; try { upsert a; r.put('dml','ok'); } catch (Exception e) { r.put('dml', ex(e)); } r.put('count', [SELECT COUNT() FROM Prefs__c]); r.put('id_set', a.Id != null); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s03_org_insert() {
        Map<String, Object> r = new Map<String, Object>();
        try { Prefs__c a = Prefs__c.getOrgDefaults(); a.Flag__c = true; try { insert a; r.put('dml','ok'); } catch (Exception e) { r.put('dml', ex(e)); } List<Prefs__c> l = [SELECT SetupOwnerId, Flag__c FROM Prefs__c]; r.put('count', l.size()); if (!l.isEmpty()) { r.put('owner_is_org', l[0].SetupOwnerId == UserInfo.getOrganizationId()); r.put('owner_is_user', l[0].SetupOwnerId == UserInfo.getUserId()); } } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s04_org_db_update() {
        Map<String, Object> r = new Map<String, Object>();
        try { Prefs__c a = Prefs__c.getOrgDefaults(); a.Flag__c = true; Database.SaveResult sr = Database.update(a, false); r.put('success', sr.isSuccess()); if (!sr.isSuccess()) { r.put('code', String.valueOf(sr.getErrors()[0].getStatusCode())); r.put('msg', sr.getErrors()[0].getMessage()); } } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s05_org_list_update() {
        Map<String, Object> r = new Map<String, Object>();
        try { Prefs__c a = Prefs__c.getOrgDefaults(); a.Flag__c = true; try { update new List<Prefs__c>{a}; r.put('dml','ok'); } catch (Exception e) { r.put('dml', ex(e)); } r.put('count', [SELECT COUNT() FROM Prefs__c]); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s06_org_update_start_stop() {
        Map<String, Object> r = new Map<String, Object>();
        try { Prefs__c a = Prefs__c.getOrgDefaults(); a.Flag__c = true; Test.startTest(); try { update a; r.put('dml','ok'); } catch (Exception e) { r.put('dml', ex(e)); } Test.stopTest(); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s07_row_then_org_update() {
        Map<String, Object> r = new Map<String, Object>();
        try { insert new Prefs__c(SetupOwnerId = UserInfo.getOrganizationId(), Flag__c = false); Prefs__c a = Prefs__c.getOrgDefaults(); r.put('id_set', a.Id != null); a.Flag__c = true; try { update a; r.put('dml','ok'); } catch (Exception e) { r.put('dml', ex(e)); } r.put('flag_after', Prefs__c.getOrgDefaults().Flag__c); r.put('count', [SELECT COUNT() FROM Prefs__c]); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s08_org_update_twice() {
        Map<String, Object> r = new Map<String, Object>();
        try { Prefs__c a = Prefs__c.getOrgDefaults(); a.Flag__c = true; try { update a; } catch (Exception e) { r.put('first', ex(e)); } Prefs__c b = Prefs__c.getOrgDefaults(); r.put('same', a == b); r.put('b_flag', b.Flag__c); r.put('b_id_set', b.Id != null); try { update b; r.put('second','ok'); } catch (Exception e) { r.put('second', ex(e)); } } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s09_instance_update() {
        Map<String, Object> r = new Map<String, Object>();
        try { Prefs__c d = Prefs__c.getInstance(); r.put('null', d == null); r.put('owner_is_user', d.SetupOwnerId == UserInfo.getUserId()); d.Flag__c = true; try { update d; r.put('dml','ok'); } catch (Exception e) { r.put('dml', ex(e)); } r.put('count', [SELECT COUNT() FROM Prefs__c]); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s10_userrow_instance_update() {
        Map<String, Object> r = new Map<String, Object>();
        try { insert new Prefs__c(SetupOwnerId = UserInfo.getUserId(), Flag__c = false); Prefs__c d = Prefs__c.getInstance(); r.put('id_set', d.Id != null); d.Flag__c = true; try { update d; r.put('dml','ok'); } catch (Exception e) { r.put('dml', ex(e)); } r.put('flag_after', [SELECT Flag__c FROM Prefs__c LIMIT 1].Flag__c); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s11_org_set_owner_update() {
        Map<String, Object> r = new Map<String, Object>();
        try { Prefs__c a = Prefs__c.getOrgDefaults(); a.SetupOwnerId = UserInfo.getOrganizationId(); a.Flag__c = true; try { update a; r.put('dml','ok'); } catch (Exception e) { r.put('dml', ex(e)); } r.put('count', [SELECT COUNT() FROM Prefs__c]); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s12_org_set_owner_upsert() {
        Map<String, Object> r = new Map<String, Object>();
        try { Prefs__c a = Prefs__c.getOrgDefaults(); a.SetupOwnerId = UserInfo.getOrganizationId(); a.Flag__c = true; try { upsert a; r.put('dml','ok'); } catch (Exception e) { r.put('dml', ex(e)); } r.put('count', [SELECT COUNT() FROM Prefs__c]); r.put('id_set', a.Id != null); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s13_list_values() {
        Map<String, Object> r = new Map<String, Object>();
        try { ListPrefs__c g = ListPrefs__c.getValues('missing'); r.put('values_null', g == null); r.put('all_size', ListPrefs__c.getAll().size()); ListPrefs__c h = ListPrefs__c.getInstance('missing'); r.put('instance_null', h == null); insert new ListPrefs__c(Name = 'present', Flag__c = true); ListPrefs__c p = ListPrefs__c.getValues('present'); r.put('present_flag', p.Flag__c); p.Flag__c = false; update p; r.put('after_update', [SELECT Flag__c FROM ListPrefs__c LIMIT 1].Flag__c); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s14_org_insert_then_get() {
        Map<String, Object> r = new Map<String, Object>();
        try { Prefs__c a = Prefs__c.getOrgDefaults(); a.Flag__c = true; insert a; Prefs__c b = Prefs__c.getOrgDefaults(); r.put('same_id', b.Id == a.Id); r.put('b_id_set', b.Id != null); r.put('b_owner_org', b.SetupOwnerId == UserInfo.getOrganizationId()); r.put('a_owner_org', a.SetupOwnerId == UserInfo.getOrganizationId()); a.Flag__c = false; try { update a; r.put('dml','ok'); } catch (Exception e) { r.put('dml', ex(e)); } r.put('flag_stored', [SELECT Flag__c FROM Prefs__c LIMIT 1].Flag__c); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s15_org_upsert_twice() {
        Map<String, Object> r = new Map<String, Object>();
        try { Prefs__c a = Prefs__c.getOrgDefaults(); a.Flag__c = true; upsert a; a.Flag__c = false; upsert a; r.put('count', [SELECT COUNT() FROM Prefs__c]); r.put('flag_stored', [SELECT Flag__c FROM Prefs__c LIMIT 1].Flag__c); r.put('a_owner_org', a.SetupOwnerId == UserInfo.getOrganizationId()); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s16_org_stale_after_insert() {
        Map<String, Object> r = new Map<String, Object>();
        try { Prefs__c a = Prefs__c.getOrgDefaults(); insert new Prefs__c(SetupOwnerId = UserInfo.getOrganizationId(), Flag__c = false); a.Flag__c = true; try { update a; r.put('dml','ok'); } catch (Exception e) { r.put('dml', ex(e)); } r.put('count', [SELECT COUNT() FROM Prefs__c]); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s17_instance_insert() {
        Map<String, Object> r = new Map<String, Object>();
        try { Prefs__c d = Prefs__c.getInstance(); d.Flag__c = true; try { insert d; r.put('dml','ok'); } catch (Exception e) { r.put('dml', ex(e)); } List<Prefs__c> l = [SELECT SetupOwnerId FROM Prefs__c]; r.put('count', l.size()); if (!l.isEmpty()) { r.put('owner_org', l[0].SetupOwnerId == UserInfo.getOrganizationId()); r.put('owner_user', l[0].SetupOwnerId == UserInfo.getUserId()); } } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s18_org_delete_then_get() {
        Map<String, Object> r = new Map<String, Object>();
        try { insert new Prefs__c(SetupOwnerId = UserInfo.getOrganizationId(), Flag__c = true); Prefs__c a = Prefs__c.getOrgDefaults(); r.put('id_set', a.Id != null); delete a; Prefs__c b = Prefs__c.getOrgDefaults(); r.put('after_delete_id_set', b.Id != null); r.put('after_delete_flag', b.Flag__c); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s19_org_insert_owner_shape() {
        Map<String, Object> r = new Map<String, Object>();
        try { Prefs__c a = Prefs__c.getOrgDefaults(); insert a; Prefs__c s = [SELECT SetupOwnerId FROM Prefs__c LIMIT 1]; r.put('len', String.valueOf(s.SetupOwnerId).length()); r.put('eq_org', s.SetupOwnerId == UserInfo.getOrganizationId()); r.put('where_org', [SELECT COUNT() FROM Prefs__c WHERE SetupOwnerId = :UserInfo.getOrganizationId()]); r.put('where_org_str', [SELECT COUNT() FROM Prefs__c WHERE SetupOwnerId = :String.valueOf(UserInfo.getOrganizationId())]); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s20_new_insert_owner_shape() {
        Map<String, Object> r = new Map<String, Object>();
        try { insert new Prefs__c(Flag__c = true); Prefs__c s = [SELECT SetupOwnerId FROM Prefs__c LIMIT 1]; r.put('len', String.valueOf(s.SetupOwnerId).length()); r.put('eq_org', s.SetupOwnerId == UserInfo.getOrganizationId()); r.put('where_org', [SELECT COUNT() FROM Prefs__c WHERE SetupOwnerId = :UserInfo.getOrganizationId()]); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
    public static Map<String, Object> s21_org_upsert_then_query_update() {
        Map<String, Object> r = new Map<String, Object>();
        try { Prefs__c a = Prefs__c.getOrgDefaults(); a.Flag__c = true; upsert a; Prefs__c s = [SELECT Id, Flag__c FROM Prefs__c WHERE SetupOwnerId = :UserInfo.getOrganizationId() LIMIT 1]; s.Flag__c = false; update s; r.put('flag', [SELECT Flag__c FROM Prefs__c LIMIT 1].Flag__c); } catch (Exception e) { r.put('uncaught', ex(e)); }
        return r;
    }
}
