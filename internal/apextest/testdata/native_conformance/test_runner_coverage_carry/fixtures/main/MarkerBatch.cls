public class MarkerBatch implements Database.Batchable<SObject> {
    public Database.QueryLocator start(Database.BatchableContext ctx) {
        return Database.getQueryLocator([SELECT Id FROM Account WHERE Name = 'MK-SEED']);
    }
    public void execute(Database.BatchableContext ctx, List<SObject> scope) {
        insert new Account(Name = 'MK-B');
    }
    public void finish(Database.BatchableContext ctx) {
        insert new Account(Name = 'MK-BF');
    }
}
