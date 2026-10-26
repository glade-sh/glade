public class StackDepthProbe {
    public static String submit(String token, String kind, Integer count, Integer cap, String overload, Boolean trap) {
        if (kind == 'future') { futureHop(token, 1, count); return null; }
        else if (kind == 'batch') { return Database.executeBatch(new StackBatchHop(token, 1, count, false), 1); }
        else {
            QueueHop job = new QueueHop(token, 1, count, overload, trap, kind == 'mixed');
            if (cap == null) { return System.enqueueJob(job); }
            else { AsyncOptions options = new AsyncOptions(); options.MaximumQueueableStackDepth = cap; return System.enqueueJob(job, options); }
        }
    }
    public static void save(String token, String value) { insert new Account(Name=token, Description=value); }
    public static List<String> readback(String token) {
        List<String> values = new List<String>();
        for (Account row : [SELECT Description FROM Account WHERE Name = :token]) { values.add(row.Description); }
        values.sort(); return values;
    }
    public class QueueHop implements Queueable {
        String token; Integer hop; Integer count; String overload; Boolean trap; Boolean mixed;
        public QueueHop(String token, Integer hop, Integer count, String overload, Boolean trap, Boolean mixed) {
            this.token=token; this.hop=hop; this.count=count; this.overload=overload; this.trap=trap; this.mixed=mixed;
        }
        public void child() {
            if (mixed) { mixedFuture(token); }
            else {
                QueueHop next = new QueueHop(token, hop+1, count, overload, trap, false);
                if (overload == 'delay') { System.enqueueJob(next, 0); }
                else if (overload == 'options') { System.enqueueJob(next, new AsyncOptions()); }
                else { System.enqueueJob(next); }
            }
        }
        public void execute(QueueableContext context) {
            String value = String.valueOf(hop)+':queue';
            if (hop < count) {
                if (trap) {
                    try { child(); value += '|accepted'; }
                    catch (Exception e) { value += '|'+e.getTypeName()+'|'+e.getMessage(); }
                } else { child(); value += '|accepted'; }
            }
            save(token, value);
        }
    }
    @future public static void futureHop(String token, Integer hop, Integer count) {
        String value=String.valueOf(hop)+':future';
        if (hop < count) {
            try { futureHop(token, hop+1, count); value += '|accepted'; }
            catch (Exception e) { value += '|'+e.getTypeName()+'|'+e.getMessage(); }
        }
        save(token, value);
    }
    @future public static void mixedFuture(String token) {
        String value='2:future';
        try { Database.executeBatch(new StackBatchHop(token, 3, 4, true), 1); value += '|accepted'; }
        catch (Exception e) { value += '|'+e.getTypeName()+'|'+e.getMessage(); }
        save(token, value);
    }
}
