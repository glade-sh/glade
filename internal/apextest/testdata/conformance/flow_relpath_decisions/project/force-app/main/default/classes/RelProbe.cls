public class RelProbe {
    public static String run(String key, String acctType, String parentType) {
        Account p = new Account(Name='Par', Type=parentType); insert p;
        Account a = new Account(Name='Acme', Type=acctType, ParentId=p.Id); insert a;
        Contact c = new Contact(LastName=key, AccountId=a.Id);
        try { insert c; } catch (Exception e) { return key + '|ERR|' + e.getTypeName() + ':' + e.getMessage(); }
        c = [SELECT Description FROM Contact WHERE Id = :c.Id];
        List<String> subs = new List<String>();
        for (Task t : [SELECT Subject, WhatId FROM Task WHERE WhatId = :a.Id]) subs.add(t.Subject);
        return key + '|' + c.Description + '|tasks=' + String.join(subs, ',');
    }
}
