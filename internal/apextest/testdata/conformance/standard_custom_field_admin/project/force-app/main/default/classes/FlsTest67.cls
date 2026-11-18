@IsTest
private class FlsTest67 {
    static User mk(Boolean ps) {
        Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User'];
        String k = String.valueOf(Math.abs(Crypto.getRandomInteger()));
        User u = new User(Alias = 'su' + k.left(5), Email = 'su@example.invalid', EmailEncodingKey = 'UTF-8', LastName = 'T', LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles', UserName = 'su' + k + '@example.invalid.glade');
        insert u;
        if (ps) { insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = [SELECT Id FROM PermissionSet WHERE Name = 'Probe_Access'].Id); }
        return u;
    }

    static String run(Boolean asUser, Boolean ps, String f, String op) {
        User u = asUser ? mk(ps) : null;
        if (!asUser && ps) { insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = [SELECT Id FROM PermissionSet WHERE Name = 'Probe_Access'].Id); }
        String[] res = new String[]{'?'};
        if (asUser) { System.runAs(u) { res[0] = doit(f, op); } } else { res[0] = doit(f, op); }
        String st;
        try { List<Account> back = Database.query('SELECT ' + f + ' FROM Account WHERE Name = \'x\''); st = back.isEmpty() ? 'norow' : String.valueOf(back[0].get(f)); } catch (Exception e) { st = 'readfail'; }
        return op + ' ' + f + ' u=' + asUser + ' ps=' + ps + ' => ' + res[0] + ' stored=' + st;
    }
    static String err(Exception e) { return e.getTypeName() + ':' + e.getMessage(); }
    static String doit(String f, String op) {
        try {
            if (op == 'sel') { Database.query('SELECT ' + f + ' FROM Account LIMIT 1'); return 'ok'; }
            Account a = new Account(Name = 'x');
            if (op == 'upd' || op == 'dbu') { insert a; }
            a.put(f, 'v');
            if (op == 'ins') { insert a; return 'ok'; }
            if (op == 'upd') { update a; return 'ok'; }
            Database.SaveResult sr;
            if (op == 'dbit') { sr = Database.insert(a, true); }
            if (op == 'dbif') { sr = Database.insert(a, false); }
            if (op == 'dbu') { sr = Database.update(a, false); }
            if (sr.isSuccess()) { return 'ok'; }
            return 'fail ' + sr.getErrors()[0].getStatusCode() + ':' + sr.getErrors()[0].getMessage();
        } catch (Exception e) { return err(e); }
    }
    @IsTest static void uA_ins() { System.assert(false, run(true, true, 'Probe_A__c', 'ins')); }
    @IsTest static void uA_upd() { System.assert(false, run(true, true, 'Probe_A__c', 'upd')); }
    @IsTest static void uA_dbit() { System.assert(false, run(true, true, 'Probe_A__c', 'dbit')); }
    @IsTest static void uA_dbif() { System.assert(false, run(true, true, 'Probe_A__c', 'dbif')); }
    @IsTest static void uA_dbu() { System.assert(false, run(true, true, 'Probe_A__c', 'dbu')); }
    @IsTest static void uA_sel() { System.assert(false, run(true, true, 'Probe_A__c', 'sel')); }
    @IsTest static void uB_ins() { System.assert(false, run(true, true, 'Probe_B__c', 'ins')); }
    @IsTest static void uB_upd() { System.assert(false, run(true, true, 'Probe_B__c', 'upd')); }
    @IsTest static void uB_dbit() { System.assert(false, run(true, true, 'Probe_B__c', 'dbit')); }
    @IsTest static void uB_dbif() { System.assert(false, run(true, true, 'Probe_B__c', 'dbif')); }
    @IsTest static void uB_dbu() { System.assert(false, run(true, true, 'Probe_B__c', 'dbu')); }
    @IsTest static void uB_sel() { System.assert(false, run(true, true, 'Probe_B__c', 'sel')); }
    @IsTest static void uAnops_ins() { System.assert(false, run(true, false, 'Probe_A__c', 'ins')); }
    @IsTest static void uAnops_upd() { System.assert(false, run(true, false, 'Probe_A__c', 'upd')); }
    @IsTest static void uAnops_dbit() { System.assert(false, run(true, false, 'Probe_A__c', 'dbit')); }
    @IsTest static void uAnops_dbif() { System.assert(false, run(true, false, 'Probe_A__c', 'dbif')); }
    @IsTest static void uAnops_dbu() { System.assert(false, run(true, false, 'Probe_A__c', 'dbu')); }
    @IsTest static void uAnops_sel() { System.assert(false, run(true, false, 'Probe_A__c', 'sel')); }
    @IsTest static void aA_ins() { System.assert(false, run(false, false, 'Probe_A__c', 'ins')); }
    @IsTest static void aA_upd() { System.assert(false, run(false, false, 'Probe_A__c', 'upd')); }
    @IsTest static void aA_dbit() { System.assert(false, run(false, false, 'Probe_A__c', 'dbit')); }
    @IsTest static void aA_dbif() { System.assert(false, run(false, false, 'Probe_A__c', 'dbif')); }
    @IsTest static void aA_dbu() { System.assert(false, run(false, false, 'Probe_A__c', 'dbu')); }
    @IsTest static void aA_sel() { System.assert(false, run(false, false, 'Probe_A__c', 'sel')); }
    @IsTest static void aB_ins() { System.assert(false, run(false, false, 'Probe_B__c', 'ins')); }
    @IsTest static void aB_upd() { System.assert(false, run(false, false, 'Probe_B__c', 'upd')); }
    @IsTest static void aB_dbit() { System.assert(false, run(false, false, 'Probe_B__c', 'dbit')); }
    @IsTest static void aB_dbif() { System.assert(false, run(false, false, 'Probe_B__c', 'dbif')); }
    @IsTest static void aB_dbu() { System.assert(false, run(false, false, 'Probe_B__c', 'dbu')); }
    @IsTest static void aB_sel() { System.assert(false, run(false, false, 'Probe_B__c', 'sel')); }
    @IsTest static void pA_ins() { System.assert(false, run(false, true, 'Probe_A__c', 'ins')); }
    @IsTest static void pA_upd() { System.assert(false, run(false, true, 'Probe_A__c', 'upd')); }
    @IsTest static void pA_dbit() { System.assert(false, run(false, true, 'Probe_A__c', 'dbit')); }
    @IsTest static void pA_dbif() { System.assert(false, run(false, true, 'Probe_A__c', 'dbif')); }
    @IsTest static void pA_dbu() { System.assert(false, run(false, true, 'Probe_A__c', 'dbu')); }
    @IsTest static void pA_sel() { System.assert(false, run(false, true, 'Probe_A__c', 'sel')); }
    @IsTest static void pB_ins() { System.assert(false, run(false, true, 'Probe_B__c', 'ins')); }
    @IsTest static void pB_upd() { System.assert(false, run(false, true, 'Probe_B__c', 'upd')); }
    @IsTest static void pB_dbit() { System.assert(false, run(false, true, 'Probe_B__c', 'dbit')); }
    @IsTest static void pB_dbif() { System.assert(false, run(false, true, 'Probe_B__c', 'dbif')); }
    @IsTest static void pB_dbu() { System.assert(false, run(false, true, 'Probe_B__c', 'dbu')); }
    @IsTest static void pB_sel() { System.assert(false, run(false, true, 'Probe_B__c', 'sel')); }
}
