public class StopOrderMk {
    public static void mark(String name) {
        insert new Account(Name = name, NumberOfEmployees = [SELECT count() FROM Account]);
    }
    @future public static void fut(String name) {
        mark(name);
    }
    @future public static void futEnqueue(String name, String next) {
        mark(name);
        System.enqueueJob(new StopOrderQ(next, false));
    }
    public static String observe(Boolean cron) {
        List<String> seq = new List<String>();
        Integer i = 0;
        for (Account a : [SELECT Name FROM Account ORDER BY NumberOfEmployees]) {
            seq.add(i + ':' + a.Name);
            i++;
        }
        List<String> ids = new List<String>();
        for (Account a : [SELECT Name FROM Account ORDER BY Id]) {
            ids.add(a.Name);
        }
        String out = 'stop=ok|seq=' + String.join(seq, ',') + '|createdid=' + String.join(ids, ',');
        if (cron) {
            CronTrigger ct = [SELECT State, TimesTriggered FROM CronTrigger LIMIT 1];
            out += '|cron=' + ct.State + ':' + ct.TimesTriggered;
        }
        return out;
    }
}
