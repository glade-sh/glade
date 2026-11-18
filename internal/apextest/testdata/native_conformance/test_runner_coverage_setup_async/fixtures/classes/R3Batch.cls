public class R3Batch implements Database.Batchable<SObject> {
    public Database.QueryLocator start(Database.BatchableContext ctx) {
        return Database.getQueryLocator([SELECT Id FROM Account WHERE Name = 'R3-SEED']);
    }
    public void execute(Database.BatchableContext ctx, List<SObject> scope) {
        insert new Account(Name = 'R3-EXEC');
    }
    public void finish(Database.BatchableContext ctx) {
        insert new Account(Name = 'R3-FINISH');
        System.enqueueJob(new R3Later());
    }
}
