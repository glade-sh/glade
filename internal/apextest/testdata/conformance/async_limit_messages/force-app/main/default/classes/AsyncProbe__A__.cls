public class AsyncProbe__A__ implements Queueable, Database.Batchable<SObject> {
    String mode;
    public AsyncProbe__A__(String mode) { this.mode = mode; }
    public void execute(QueueableContext ctx) { run(mode, 'q'); }
    public Database.QueryLocator start(Database.BatchableContext bc) { return Database.getQueryLocator('SELECT Id FROM Account LIMIT 1'); }
    public void execute(Database.BatchableContext bc, List<SObject> scope) { run(mode, 'b'); }
    public void finish(Database.BatchableContext bc) {}
    @future public static void fut(String mode) { run(mode, 'f'); }
    public static String info() {
        return Limits.getQueries() + '|' + Limits.getLimitQueries() + '|' + Limits.getAggregateQueries() + '|' + Limits.getLimitAggregateQueries()
            + '|' + Limits.getLimitCpuTime() + '|' + Limits.getDmlStatements() + '|' + Limits.getLimitDmlStatements() + '|' + Limits.getLimitDmlRows()
            + '|' + Limits.getLimitQueryRows() + '|' + Limits.getLimitFutureCalls() + '|' + Limits.getLimitQueueableJobs() + '|' + Limits.getLimitHeapSize()
            + '|' + Limits.getLimitCallouts() + '|' + Limits.getLimitEmailInvocations() + '|' + Limits.getLimitSoslQueries();
    }
    public static void rec(String key, String val) {
        insert new Account(Name = 'AOBS-' + key, Description = val);
    }
    public static void run(String mode, String ctx) {
        String key = mode + '_' + ctx;
        if (mode == 'soql200') { for (Integer i = 0; i < 200; i++) { List<Account> r = [SELECT Id FROM Account WHERE Id = null]; } }
        else if (mode == 'soql201') { for (Integer i = 0; i < 201; i++) { List<Account> r = [SELECT Id FROM Account WHERE Id = null]; } }
        else if (mode.startsWith('agg')) {
            Integer n = Integer.valueOf(mode.substring(3));
            for (Integer i = 0; i < n; i++) {
                List<Account> r = [SELECT Id, (SELECT Id FROM Contacts), (SELECT Id FROM Opportunities), (SELECT Id FROM Cases), (SELECT Id FROM Tasks) FROM Account WHERE Id = null];
            }
        }
        else if (mode == 'dml150') { for (Integer i = 0; i < 149; i++) { insert new Contact(LastName = 'dmlprobe'); } }
        else if (mode == 'dml151') { for (Integer i = 0; i < 150; i++) { insert new Contact(LastName = 'dmlprobe'); } }
        else if (mode == 'cpu') { Integer x = 0; while (true) { x++; } }
        else if (mode == 'enq2') { System.enqueueJob(new AsyncNoop__A__()); System.enqueueJob(new AsyncNoop__A__()); }
        else if (mode == 'enq1') { System.enqueueJob(new AsyncNoop__A__()); }
        rec(key, info());
    }
}
