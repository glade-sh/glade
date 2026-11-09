public class LFActivityProbe {
    static Map<Id, String> names = new Map<Id, String>();
    static List<String> out;
    static void row(String id, Object value) { out.add(id + '|' + String.valueOf(value)); }
    static String acc(Id activityId, String obj) {
        SObject r = Database.query('SELECT AccountId FROM ' + obj + ' WHERE Id = :activityId');
        Id a = (Id) r.get('AccountId');
        return a == null ? 'null' : names.get(a);
    }
    static Event ev(Id what, Id who) {
        return new Event(Subject = 'LF E', WhatId = what, WhoId = who, ActivityDateTime = Datetime.now(), DurationInMinutes = 30);
    }
    public static List<String> run() {
        out = new List<String>();
        Account a1 = new Account(Name = 'LF A1'), a2 = new Account(Name = 'LF A2'), a3 = new Account(Name = 'LF A3');
        insert new List<Account>{a1, a2, a3};
        names.put(a1.Id, 'A1'); names.put(a2.Id, 'A2'); names.put(a3.Id, 'A3');
        Opportunity o = new Opportunity(Name = 'LF O', StageName = 'Prospecting', CloseDate = Date.today(), AccountId = a2.Id);
        insert o;
        Case c = new Case(Subject = 'LF C', AccountId = a3.Id);
        insert c;
        Campaign cp = new Campaign(Name = 'LF Cp');
        insert cp;
        Contact ct = new Contact(LastName = 'LF Ct', AccountId = a3.Id);
        Contact cn = new Contact(LastName = 'LF Cn');
        insert new List<Contact>{ct, cn};
        LF_Probe__c p = new LF_Probe__c(Name = 'LF P');
        insert p;
        Task t = new Task(Subject = 'LF T', WhatId = a1.Id);
        insert t;
        row('T01 insert what=Account A1', acc(t.Id, 'Task'));
        t.WhatId = o.Id; update t;
        row('T02 update what=Opportunity(A2)', acc(t.Id, 'Task'));
        t.WhatId = c.Id; update t;
        row('T03 update what=Case(A3)', acc(t.Id, 'Task'));
        t.WhatId = cp.Id; update t;
        row('T04 update what=Campaign', acc(t.Id, 'Task'));
        t.WhatId = p.Id; update t;
        row('T05 update what=custom', acc(t.Id, 'Task'));
        t.WhatId = a1.Id; update t;
        row('T06 update what=Account A1', acc(t.Id, 'Task'));
        t.WhatId = null; update t;
        row('T07 update what=null', acc(t.Id, 'Task'));
        t.WhoId = ct.Id; update t;
        row('T08 update who=Contact(A3) what=null', acc(t.Id, 'Task'));
        t.WhatId = cp.Id; update t;
        row('T09 update who=Contact(A3) what=Campaign', acc(t.Id, 'Task'));
        t.WhatId = a1.Id; update t;
        row('T10 update who=Contact(A3) what=Account A1', acc(t.Id, 'Task'));
        t.WhoId = cn.Id; t.WhatId = null; update t;
        row('T11 update who=Contact(no account) what=null', acc(t.Id, 'Task'));
        t.Subject = 'LF T2'; update t;
        row('T12 update subject only', acc(t.Id, 'Task'));
        Task t2 = new Task(Subject = 'LF T', WhatId = o.Id);
        insert t2;
        row('T13 insert what=Opportunity(A2)', acc(t2.Id, 'Task'));
        o.AccountId = a1.Id; update o;
        row('T14 after Opportunity AccountId->A1', acc(t2.Id, 'Task'));
        Task t3 = new Task(Subject = 'LF T', WhoId = ct.Id);
        insert t3;
        row('T15 insert who=Contact(A3)', acc(t3.Id, 'Task'));
        Task t4 = new Task(Subject = 'LF T', WhoId = ct.Id, WhatId = cp.Id);
        insert t4;
        row('T16 insert who=Contact(A3) what=Campaign', acc(t4.Id, 'Task'));
        Task t5 = new Task(Subject = 'LF T', WhatId = p.Id);
        insert t5;
        row('T17 insert what=custom', acc(t5.Id, 'Task'));
        Task t6 = new Task(Subject = 'LF T', WhatId = c.Id);
        insert t6;
        row('T18 insert what=Case(A3)', acc(t6.Id, 'Task'));
        Task t7 = new Task(Subject = 'LF T', WhatId = a1.Id);
        insert t7;
        t7.WhatId = cp.Id; update t7;
        row('T19 insert A1 then what=Campaign', acc(t7.Id, 'Task'));
        Event e = ev(a1.Id, null);
        insert e;
        row('E01 insert what=Account A1', acc(e.Id, 'Event'));
        e.WhatId = o.Id; update e;
        row('E02 update what=Opportunity', acc(e.Id, 'Event'));
        e.WhatId = cp.Id; update e;
        row('E03 update what=Campaign', acc(e.Id, 'Event'));
        e.WhatId = null; e.WhoId = ct.Id; update e;
        row('E04 update who=Contact(A3) what=null', acc(e.Id, 'Event'));
        Event e2 = ev(cp.Id, null);
        insert e2;
        row('E05 insert what=Campaign', acc(e2.Id, 'Event'));
        Event e3 = ev(o.Id, null);
        insert e3;
        row('E06 insert what=Opportunity(A1 now)', acc(e3.Id, 'Event'));
        return out;
    }
}
