public class FlowEmailLogProbe {
    static String err(Exception e) { return e.getTypeName().replace('System.', '') + ':' + e.getMessage(); }
    static Object run(Boolean log, Id recipient, Id related, String tag) {
        Map<String,Object> m = new Map<String,Object>();
        Integer tasks0 = [SELECT COUNT() FROM Task];
        Integer emails0 = [SELECT COUNT() FROM EmailMessage];
        try {
            Flow.Interview.createInterview('V35EmailLog', new Map<String,Object>{ 'varBody' => 'Body ' + tag, 'varLog' => log, 'varRecipient' => String.valueOf(recipient), 'varRelated' => related == null ? null : String.valueOf(related), 'varSubject' => 'Subj ' + tag }).start();
            m.put('flow', 'ok');
        } catch (Exception e) { m.put('flow', e.getTypeName()); }
        m.put('tasks', [SELECT COUNT() FROM Task] - tasks0);
        m.put('emails', [SELECT COUNT() FROM EmailMessage] - emails0);
        List<Task> ts = [SELECT Subject, Status, TaskSubtype, WhatId, WhoId, Priority FROM Task WHERE Subject LIKE :('%' + tag + '%')];
        List<String> rows = new List<String>();
        for (Task t : ts) { rows.add(t.Subject.replace(tag, 'T') + '~' + t.Status + '~' + t.TaskSubtype + '~' + (t.WhatId == related) + '~' + (t.WhoId == recipient) + '~' + t.Priority); }
        m.put('taskRows', rows);
        List<String> ems = new List<String>();
        for (EmailMessage em : [SELECT Subject, Status, RelatedToId, ActivityId, Incoming FROM EmailMessage WHERE Subject LIKE :('%' + tag + '%')]) { ems.add(em.Subject.replace(tag, 'T') + '~' + em.Status + '~' + (em.RelatedToId == related) + '~' + (em.ActivityId != null) + '~' + em.Incoming); }
        m.put('emailRows', ems);
        return m;
    }
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        String uniq = String.valueOf(Math.abs(Crypto.getRandomInteger()));
        Account a = new Account(Name = 'V35 Email Acct ' + uniq);
        insert a;
        Contact c = new Contact(LastName = 'V35Email' + uniq, Email = 'v35email' + uniq + '@example.com', AccountId = a.Id);
        insert c;
        m.put('logTrueContactRelated', run(true, c.Id, a.Id, 'LT1' + uniq));
        m.put('logFalseContactRelated', run(false, c.Id, a.Id, 'LF1' + uniq));
        m.put('logTrueContactNoRelated', run(true, c.Id, null, 'LT2' + uniq));
        return m;
    }
}
