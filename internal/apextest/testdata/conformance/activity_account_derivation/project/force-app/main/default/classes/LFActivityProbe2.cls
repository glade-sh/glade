public class LFActivityProbe2 {
    static Map<Id, String> names = new Map<Id, String>();
    static List<String> out;
    static void row(String id, Object value) { out.add(id + '|' + String.valueOf(value)); }
    static String acc(Id activityId) {
        Task r = [SELECT AccountId FROM Task WHERE Id = :activityId];
        return r.AccountId == null ? 'null' : names.get(r.AccountId);
    }
    public static List<String> run() {
        out = new List<String>();
        Account a1 = new Account(Name = 'LF B1'), a2 = new Account(Name = 'LF B2'), a3 = new Account(Name = 'LF B3');
        insert new List<Account>{a1, a2, a3};
        names.put(a1.Id, 'A1'); names.put(a2.Id, 'A2'); names.put(a3.Id, 'A3');
        Contract k = new Contract(AccountId = a2.Id, Status = 'Draft', StartDate = Date.today(), ContractTerm = 12);
        insert k;
        SObject lk = Schema.getGlobalDescribe().get('LF_Child__c').newSObject();
        lk.put('Name', 'LF L'); lk.put('Account__c', a2.Id);
        insert lk;
        SObject md = Schema.getGlobalDescribe().get('LF_MD__c').newSObject();
        md.put('Name', 'LF M'); md.put('Account__c', a2.Id);
        insert md;
        Contact ct = new Contact(LastName = 'LF Ct', AccountId = a3.Id);
        insert ct;
        Lead ld = new Lead(LastName = 'LF Ld', Company = 'LF Co');
        insert ld;
        Opportunity o = new Opportunity(Name = 'LF O', StageName = 'Prospecting', CloseDate = Date.today());
        insert o;
        Task t1 = new Task(Subject = 'LF', WhatId = k.Id); insert t1;
        row('U01 insert what=Contract(A2)', acc(t1.Id));
        Task t2 = new Task(Subject = 'LF', WhatId = lk.Id); insert t2;
        row('U02 insert what=custom lookup child(A2)', acc(t2.Id));
        Task t3 = new Task(Subject = 'LF', WhatId = md.Id); insert t3;
        row('U03 insert what=custom master-detail child(A2)', acc(t3.Id));
        Task t4 = new Task(Subject = 'LF', WhoId = ld.Id); insert t4;
        row('U04 insert who=Lead', acc(t4.Id));
        Task t5 = new Task(Subject = 'LF', WhoId = ct.Id); insert t5;
        ct.AccountId = a1.Id; update ct;
        row('U05 who=Contact after Contact AccountId A3->A1', acc(t5.Id));
        Task t6 = new Task(Subject = 'LF', WhatId = o.Id, WhoId = ct.Id); insert t6;
        row('U06 insert what=Opportunity(no account) who=Contact(A1)', acc(t6.Id));
        Task t7 = new Task(Subject = 'LF', WhatId = k.Id); insert t7;
        k.AccountId = a1.Id;
        try { update k; row('U07 Contract AccountId A2->A1', acc(t7.Id)); } catch (Exception e) { row('U07 Contract AccountId A2->A1', 'EXC ' + e.getMessage()); }
        lk.put('Account__c', a1.Id); update lk;
        row('U08 lookup child Account__c A2->A1', acc(t2.Id));
        Case cs = new Case(Subject = 'LF C', AccountId = a2.Id); insert cs;
        Task t11 = new Task(Subject = 'LF', WhatId = cs.Id, WhoId = ct.Id); insert t11;
        row('U11 insert what=Case(A2) who=Contact(A1)', acc(t11.Id));
        Task t12 = new Task(Subject = 'LF', WhatId = lk.Id, WhoId = ct.Id); insert t12;
        row('U12 insert what=custom lookup child(A1) who=Contact(A1)', acc(t12.Id));
        Contact c2 = new Contact(LastName = 'LF C2', AccountId = a3.Id); insert c2;
        Task t13 = new Task(Subject = 'LF', WhatId = md.Id, WhoId = c2.Id); insert t13;
        row('U13 insert what=custom master-detail child(A2) who=Contact(A3)', acc(t13.Id));
        Task t14 = new Task(Subject = 'LF', WhatId = k.Id, WhoId = c2.Id); insert t14;
        row('U14 insert what=Contract(A1) who=Contact(A3)', acc(t14.Id));
        Task t15 = new Task(Subject = 'LF', WhatId = cs.Id, WhoId = c2.Id); insert t15;
        row('U15 insert what=Case(A2) who=Contact(A3)', acc(t15.Id));
        t15.WhoId = null; update t15;
        row('U16 update what=Case who=null', acc(t15.Id));
        Task t10 = new Task(Subject = 'LF', WhatId = a1.Id); insert t10;
        delete a1;
        List<Task> gone = [SELECT AccountId FROM Task WHERE Id = :t10.Id];
        row('U10 after What Account deleted', gone.isEmpty() ? 'deleted' : String.valueOf(gone[0].AccountId));
        return out;
    }
}
