public class CedarSave67 {
    static String res(Database.SaveResult r) {
        if (r.isSuccess()) { return 'ok'; }
        Database.Error e = r.getErrors()[0];
        return 'err:' + e.getStatusCode() + ':' + e.getMessage() + ':' + e.getFields();
    }
    static String ins(SObject s) { return res(Database.insert(s, false)); }
    static String upd(SObject s) { return res(Database.update(s, false)); }
    static final List<String> CASES = new List<String>{ 'vr_owner_blank', 'vr_owner_prefix', 'vr_created_blank', 'vr_createddate_blank', 'vr_lmb_blank' };
    public static String owner() {
        List<String> out = new List<String>{ 'api=67.0' };
        Account a = new Account(Name = 'OwnerAcc');
        insert a;
        List<String> all = new List<String>(CASES);
        all.add('vr_shape');
        for (String c : all) {
            out.add('T.' + c + '=' + ins(new Task(Subject = c, WhatId = a.Id)));
            if (c != 'vr_shape') { out.add('C.' + c + '=' + ins(new Cedar_Probe__c(Name = 'p', Case__c = c))); }
        }
        for (String c : CASES) {
            Task t = new Task(Subject = 'plain', WhatId = a.Id);
            insert t;
            t.Subject = c;
            out.add('Tu.' + c + '=' + upd(t));
            Cedar_Probe__c p = new Cedar_Probe__c(Name = 'p', Case__c = 'plain');
            insert p;
            p.Case__c = c;
            out.add('Cu.' + c + '=' + upd(p));
        }
        return String.join(out, ' ');
    }
    public static String objectType() {
        List<String> out = new List<String>{ 'api=67.0' };
        for (String c : new List<String>{ 'ot_isnull', 'ot_isblank', 'ot_notnull', 'ot_other' }) {
            out.add(c + '=' + ins(new Cedar_Probe__c(Name = 'p', Case__c = c)));
        }
        return String.join(out, ' ');
    }
    public static String bind() {
        insert new Account(Name = 'BindAcc');
        Integer lit = [SELECT COUNT() FROM PermissionSet WHERE Name = :'Cedar_Probe_Access'];
        Integer acc = [SELECT COUNT() FROM Account WHERE Name = :'BindAcc'];
        Integer expr = [SELECT COUNT() FROM Account WHERE Name = :('Bind' + 'Acc')];
        Integer lst = [SELECT COUNT() FROM Account WHERE Name IN :new List<String>{ 'BindAcc' }];
        Integer plain = [SELECT COUNT() FROM Account WHERE Name = 'BindAcc'];
        List<Account> rows = [SELECT Name FROM Account WHERE Name = :'BindAcc' LIMIT 1];
        return 'api=67.0 lit=' + lit + ' acc=' + acc + ' expr=' + expr + ' list=' + lst + ' plain=' + plain + ' row=' + rows[0].Name;
    }
    public static String person() {
        List<String> out = new List<String>{ 'api=67.0' };
        Id rt = Schema.SObjectType.Account.getRecordTypeInfosByDeveloperName().get('PersonAccount').getRecordTypeId();
        for (String last : new List<String>{ 'Plain', 'VrBlocked', 'TrgBlocked' }) {
            Account pa = new Account(LastName = last, FirstName = 'F', RecordTypeId = rt);
            String r = ins(pa);
            Integer contacts = pa.Id == null ? -1 : [SELECT COUNT() FROM Contact WHERE AccountId = :pa.Id];
            out.add(last + '=' + r + ' contacts=' + contacts);
        }
        Account person = [SELECT Id FROM Account WHERE LastName = 'Plain' AND IsPersonAccount = true LIMIT 1];
        out.add('contactOnPerson=' + ins(new Contact(LastName = 'Biz', AccountId = person.Id)));
        Account biz = new Account(Name = 'BizAcc');
        insert biz;
        out.add('contactOnBusiness=' + ins(new Contact(LastName = 'Biz', AccountId = biz.Id)));
        out.add('businessVr=' + ins(new Contact(LastName = 'VrBlocked', AccountId = biz.Id)));
        out.add('businessTrg=' + ins(new Contact(LastName = 'TrgBlocked', AccountId = biz.Id)));
        return String.join(out, ' ');
    }
    public static String flow() {
        Account a = new Account(Name = 'FlowAcc');
        insert a;
        Id rt = Schema.SObjectType.Task.getRecordTypeInfosByDeveloperName().get('CedarTaskRT').getRecordTypeId();
        Task t = new Task(Subject = 'flow_probe', WhatId = a.Id, RecordTypeId = rt);
        String r = ins(t);
        Task q = [SELECT AccountId, Cedar_Account__c FROM Task WHERE Id = :t.Id];
        List<Cedar_Probe__c> logs = [SELECT Name, Case__c FROM Cedar_Probe__c WHERE Case__c = 'FlowAcc' OR Name = 'CedarTaskRT'];
        Task t2 = new Task(Subject = 'flow_probe');
        String r2 = ins(t2);
        Task q2 = [SELECT AccountId, Cedar_Account__c FROM Task WHERE Id = :t2.Id];
        List<Cedar_Probe__c> logs2 = [SELECT Id, Name, Case__c FROM Cedar_Probe__c WHERE Id NOT IN :logs];
        return 'api=67.0 insert=' + r + ' accountIdIsWhat=' + (q.AccountId == a.Id) + ' cedarAccountIsWhat=' + (q.Cedar_Account__c == a.Id)
            + ' logs=' + logs.size() + (logs.isEmpty() ? '' : ' name=' + logs[0].Name + ' case=' + logs[0].Case__c)
            + ' bare=' + r2 + ' bareAccount=' + q2.AccountId + ' bareCedar=' + q2.Cedar_Account__c + ' bareLogs=' + logs2.size()
            + (logs2.isEmpty() ? '' : ' bareNameBlank=' + String.isBlank(logs2[0].Name) + ' bareCase=' + logs2[0].Case__c);
    }
    public static String page() {
        PageReference p = Page.CedarMeetingReport;
        return 'api=67.0 url=' + p.getUrl();
    }
}
