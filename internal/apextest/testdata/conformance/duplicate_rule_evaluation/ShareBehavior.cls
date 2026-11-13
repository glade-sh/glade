public class ShareBehavior {
    public static Boolean predictMode = false;
    static String comp(List<Integer> xs) {
        xs.sort();
        List<String> out = new List<String>();
        Integer i = 0;
        while (i < xs.size()) {
            Integer j = i;
            while (j + 1 < xs.size() && xs[j + 1] == xs[j] + 1) j++;
            out.add(j == i ? String.valueOf(xs[i]) : String.valueOf(xs[i]) + '-' + String.valueOf(xs[j]));
            i = j + 1;
        }
        return String.join(out, '.');
    }
    private with sharing class Vis {
        public Map<Id,Contact> m() { return new Map<Id,Contact>([SELECT Id, Description FROM Contact]); }
    }
    static String shape(Datacloud.DuplicateResult dr, Map<Id,Contact> vis) {
        List<Integer> labels = new List<Integer>();
        Integer hidden = 0, n = 0, fieldOk = 0;
        for (Datacloud.MatchResult mr : dr.getMatchResults()) {
            for (Datacloud.MatchRecord m : mr.getMatchRecords()) {
                n++;
                Id i = m.getRecord().Id;
                if (vis.containsKey(i)) labels.add(Integer.valueOf(vis.get(i).Description.substringAfterLast('-'))); else hidden++;
                try { m.getRecord().get('Description'); fieldOk++; } catch (Exception e) { }
            }
        }
        return dr.getDuplicateRule() + ':members=' + String.valueOf(n) + ';userCanQuery=' + comp(labels) + ';notQueryable=' + String.valueOf(hidden) + ';descriptionOnReturnedRecord=' + String.valueOf(fieldOk) + ';allowSave=' + String.valueOf(dr.isAllowSave());
    }
    public static String core(String key, Boolean hid) {
        Contact c = hid ? new Contact(FirstName = 'Shareo', LastName = 'Hidzq' + key, Hid__c = 'hid' + key) : new Contact(FirstName = 'Shareo', LastName = 'Probe' + key, Email = 'sh' + key + '@sx.example');
        List<String> out = new List<String>();
        Map<Id,Contact> vis = new Vis().m();
        try {
            List<Datacloud.FindDuplicatesResult> f = Datacloud.FindDuplicates.findDuplicates(new List<Contact>{ c });
            List<String> rs = new List<String>();
            for (Datacloud.DuplicateResult dr : f[0].getDuplicateResults()) rs.add(shape(dr, vis));
            rs.sort();
            out.add('find=' + (rs.isEmpty() ? 'none' : String.join(rs, '+')));
        } catch (Exception e) { out.add('find=EX:' + e.getTypeName() + ':' + e.getMessage()); }
        Database.SaveResult sr = Database.insert(c, false);
        if (sr.isSuccess()) out.add('dml=S');
        else {
            Database.Error e = sr.getErrors()[0];
            if (e instanceof Database.DuplicateError) out.add('dml=D:' + shape(((Database.DuplicateError) e).getDuplicateResult(), vis));
            else out.add('dml=E:' + String.valueOf(e.getStatusCode()) + ':' + e.getMessage());
        }
        return String.join(out, '|');
    }
    static void seeds(Integer key, Integer n, Integer startLabel, Id owner, String descr, Id acct) {
        List<Contact> l = new List<Contact>();
        for (Integer i = 0; i < n; i++) {
            Contact c = new Contact(FirstName = 'Shareo', LastName = 'Probe' + key, Email = 'sh' + key + '@sx.example', Description = 'k' + key + '-' + String.valueOf(startLabel + i));
            if (descr != null) c.Title = descr;
            if (owner != null) c.OwnerId = owner;
            if (acct != null) c.AccountId = acct;
            l.add(c);
        }
        Database.DMLOptions o = new Database.DMLOptions(); o.DuplicateRuleHeader.allowSave = true;
        Database.insert(l, o);
    }
    static void fixtures(String key, User u) {
        Database.DMLOptions o = new Database.DMLOptions(); o.DuplicateRuleHeader.allowSave = true;
        Account a = new Account(Name = 'Shareacct Zq' + key + 'x');
        insert a;
        if (key == '1') seeds(1, 1, 0, null, null, a.Id);
        else if (key == '2') {
            seeds(2, 1, 0, null, null, a.Id);
            Contact c = [SELECT Id FROM Contact WHERE LastName = 'Probe2' LIMIT 1];
            insert new ContactShare(ContactId = c.Id, UserOrGroupId = u.Id, ContactAccessLevel = 'Read');
        }
        else if (key == '3') seeds(3, 1, 0, null, 'crit', a.Id);
        else if (key == '4') { seeds(4, 2, 0, null, null, a.Id); seeds(4, 1, 2, u.Id, null, a.Id); }
        else if (key == '5') { Contact c = new Contact(FirstName = 'Seedo', LastName = 'Hidseed5', Hid__c = 'hid5', Description = 'k5-0', OwnerId = u.Id, AccountId = a.Id); Database.insert(c, o); }
        else if (key == '6') { seeds(6, 60, 0, null, null, a.Id); seeds(6, 60, 60, u.Id, null, a.Id); }
    }
    static String invoke(String key, Boolean hid, String via) {
        if (via == 'with') return ShareWith.run(key, hid);
        if (via == 'without') return ShareWithout.run(key, hid);
        if (via == 'inherited') return ShareInherited.run(key, hid);
        return core(key, hid);
    }
    static String run(String key, Boolean hid, String via) {
        if (predictMode) { fixtures(key, new User(Id = UserInfo.getUserId())); return invoke(key, hid, via); }
        if (!Test.isRunningTest()) return invoke(key, hid, via);
        User u = [SELECT Id FROM User WHERE LastName = 'Sharing Reader' AND IsActive = true LIMIT 1];
        fixtures(key, u);
        String r;
        System.runAs(u) { r = invoke(key, hid, via); }
        return r;
    }
    public static String hidden() { return run('1', false, 'core'); }
    public static String sharedManual() { return run('2', false, 'core'); }
    public static String sharedCriteria() { return run('3', false, 'core'); }
    public static String mixed() { return run('4', false, 'core'); }
    public static String overflow() { return run('6', false, 'core'); }
    public static String fieldHidden() { return run('5', true, 'core'); }
    public static String classWith() { return run('1', false, 'with'); }
    public static String classWithout() { return run('1', false, 'without'); }
    public static String classInherited() { return run('1', false, 'inherited'); }
    // admin-only baseline for the same fixtures
    public static String adminBaseline() { return run('4', false, 'core'); }
    public static String pRun(String name) {
        predictMode = true;
        if (name == 'hidden') return hidden(); if (name == 'sharedManual') return sharedManual(); if (name == 'sharedCriteria') return sharedCriteria();
        if (name == 'mixed') return mixed(); if (name == 'overflow') return overflow(); if (name == 'fieldHidden') return fieldHidden();
        if (name == 'classWith') return classWith(); if (name == 'classWithout') return classWithout(); if (name == 'classInherited') return classInherited();
        return adminBaseline();
    }
    public static void committedFixtures() {
        User u = [SELECT Id FROM User WHERE LastName = 'Sharing Reader' AND IsActive = true LIMIT 1];
        for (String k : new List<String>{ '1', '2', '3', '4', '5', '6' }) fixtures(k, u);
    }
}
