@IsTest
private class PolicyProbe67 {
    static User mk(Boolean ps) {
        Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User'];
        String k = String.valueOf(Math.abs(Crypto.getRandomInteger()));
        User u = new User(Alias = 'su' + k.left(5), Email = 'su@example.invalid', EmailEncodingKey = 'UTF-8', LastName = 'T', LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles', UserName = 'su' + k + '@example.invalid.glade');
        insert u;
        if (ps) { insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = [SELECT Id FROM PermissionSet WHERE Name = 'Probe_Access'].Id); }
        return u;
    }
    static String fmt(List<Database.SaveResult> l) {
        String s = '';
        for (Database.SaveResult sr : l) { s += sr.isSuccess() ? 'ok;' : ('fail ' + sr.getErrors()[0].getStatusCode() + ':' + sr.getErrors()[0].getMessage() + ';'); }
        return s;
    }
    static String fmt(Database.SaveResult sr) { return fmt(new List<Database.SaveResult>{sr}); }
    static String run(Boolean ps, String f, String n) {
        User u = mk(ps);
        String[] res = new String[]{'?'};
        System.runAs(u) { res[0] = doit(f, n); }
        return n + ' ' + f + ' ps=' + ps + ' => ' + res[0];
    }
    static String doit(String f, String n) {
        String r = '?';
        Account a = new Account(Name = 'x');
        a.put(f, 'v');
        if (n == 'ins_def') { try { insert a; r='ok'; } catch (Exception e) { r = e.getTypeName() + ':' + e.getMessage(); } }
        if (n == 'ins_user') { try { insert as user a; r='ok'; } catch (Exception e) { r = e.getTypeName() + ':' + e.getMessage(); } }
        if (n == 'ins_sys') { try { insert as system a; r='ok'; } catch (Exception e) { r = e.getTypeName() + ':' + e.getMessage(); } }
        if (n == 'upd_def') { try { Account b = new Account(Name='x'); insert b; b.put(f,'v'); update b; r='ok'; } catch (Exception e) { r = e.getTypeName() + ':' + e.getMessage(); } }
        if (n == 'upd_user') { try { Account b = new Account(Name='x'); insert b; b.put(f,'v'); update as user b; r='ok'; } catch (Exception e) { r = e.getTypeName() + ':' + e.getMessage(); } }
        if (n == 'dbi_f_def') { try { r=fmt(Database.insert(a,false)); } catch (Exception e) { r = e.getTypeName() + ':' + e.getMessage(); } }
        if (n == 'dbi_f_user') { try { r=fmt(Database.insert(a,false,AccessLevel.USER_MODE)); } catch (Exception e) { r = e.getTypeName() + ':' + e.getMessage(); } }
        if (n == 'dbi_t_user') { try { r=fmt(Database.insert(a,true,AccessLevel.USER_MODE)); } catch (Exception e) { r = e.getTypeName() + ':' + e.getMessage(); } }
        if (n == 'dbi_f_sys') { try { r=fmt(Database.insert(a,false,AccessLevel.SYSTEM_MODE)); } catch (Exception e) { r = e.getTypeName() + ':' + e.getMessage(); } }
        if (n == 'ups_def') { try { upsert a; r='ok'; } catch (Exception e) { r = e.getTypeName() + ':' + e.getMessage(); } }
        if (n == 'ins_list_def') { try { insert new List<Account>{a}; r='ok'; } catch (Exception e) { r = e.getTypeName() + ':' + e.getMessage(); } }
        if (n == 'dbi_f_list2') { try { r=fmt(Database.insert(new List<Account>{new Account(Name='z'), a},false)); } catch (Exception e) { r = e.getTypeName() + ':' + e.getMessage(); } }
        return r;
    }
    @IsTest static void A_ins_def() { System.assert(false, run(true, 'Probe_A__c', 'ins_def')); }
    @IsTest static void A_ins_user() { System.assert(false, run(true, 'Probe_A__c', 'ins_user')); }
    @IsTest static void A_ins_sys() { System.assert(false, run(true, 'Probe_A__c', 'ins_sys')); }
    @IsTest static void A_upd_def() { System.assert(false, run(true, 'Probe_A__c', 'upd_def')); }
    @IsTest static void A_upd_user() { System.assert(false, run(true, 'Probe_A__c', 'upd_user')); }
    @IsTest static void A_dbi_f_def() { System.assert(false, run(true, 'Probe_A__c', 'dbi_f_def')); }
    @IsTest static void A_dbi_f_user() { System.assert(false, run(true, 'Probe_A__c', 'dbi_f_user')); }
    @IsTest static void A_dbi_t_user() { System.assert(false, run(true, 'Probe_A__c', 'dbi_t_user')); }
    @IsTest static void A_dbi_f_sys() { System.assert(false, run(true, 'Probe_A__c', 'dbi_f_sys')); }
    @IsTest static void A_ups_def() { System.assert(false, run(true, 'Probe_A__c', 'ups_def')); }
    @IsTest static void A_ins_list_def() { System.assert(false, run(true, 'Probe_A__c', 'ins_list_def')); }
    @IsTest static void A_dbi_f_list2() { System.assert(false, run(true, 'Probe_A__c', 'dbi_f_list2')); }
    @IsTest static void Anops_ins_def() { System.assert(false, run(false, 'Probe_A__c', 'ins_def')); }
    @IsTest static void Anops_ins_user() { System.assert(false, run(false, 'Probe_A__c', 'ins_user')); }
    @IsTest static void Anops_ins_sys() { System.assert(false, run(false, 'Probe_A__c', 'ins_sys')); }
    @IsTest static void Anops_upd_def() { System.assert(false, run(false, 'Probe_A__c', 'upd_def')); }
    @IsTest static void Anops_upd_user() { System.assert(false, run(false, 'Probe_A__c', 'upd_user')); }
    @IsTest static void Anops_dbi_f_def() { System.assert(false, run(false, 'Probe_A__c', 'dbi_f_def')); }
    @IsTest static void Anops_dbi_f_user() { System.assert(false, run(false, 'Probe_A__c', 'dbi_f_user')); }
    @IsTest static void Anops_dbi_t_user() { System.assert(false, run(false, 'Probe_A__c', 'dbi_t_user')); }
    @IsTest static void Anops_dbi_f_sys() { System.assert(false, run(false, 'Probe_A__c', 'dbi_f_sys')); }
    @IsTest static void Anops_ups_def() { System.assert(false, run(false, 'Probe_A__c', 'ups_def')); }
    @IsTest static void Anops_ins_list_def() { System.assert(false, run(false, 'Probe_A__c', 'ins_list_def')); }
    @IsTest static void Anops_dbi_f_list2() { System.assert(false, run(false, 'Probe_A__c', 'dbi_f_list2')); }
    @IsTest static void B_ins_def() { System.assert(false, run(true, 'Probe_B__c', 'ins_def')); }
    @IsTest static void B_ins_user() { System.assert(false, run(true, 'Probe_B__c', 'ins_user')); }
    @IsTest static void B_ins_sys() { System.assert(false, run(true, 'Probe_B__c', 'ins_sys')); }
    @IsTest static void B_upd_def() { System.assert(false, run(true, 'Probe_B__c', 'upd_def')); }
    @IsTest static void B_upd_user() { System.assert(false, run(true, 'Probe_B__c', 'upd_user')); }
    @IsTest static void B_dbi_f_def() { System.assert(false, run(true, 'Probe_B__c', 'dbi_f_def')); }
    @IsTest static void B_dbi_f_user() { System.assert(false, run(true, 'Probe_B__c', 'dbi_f_user')); }
    @IsTest static void B_dbi_t_user() { System.assert(false, run(true, 'Probe_B__c', 'dbi_t_user')); }
    @IsTest static void B_dbi_f_sys() { System.assert(false, run(true, 'Probe_B__c', 'dbi_f_sys')); }
    @IsTest static void B_ups_def() { System.assert(false, run(true, 'Probe_B__c', 'ups_def')); }
    @IsTest static void B_ins_list_def() { System.assert(false, run(true, 'Probe_B__c', 'ins_list_def')); }
    @IsTest static void B_dbi_f_list2() { System.assert(false, run(true, 'Probe_B__c', 'dbi_f_list2')); }
}
