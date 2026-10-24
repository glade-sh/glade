public class ShapeBatch implements Database.Batchable<SObject> {
    public Database.QueryLocator start(Database.BatchableContext context) {
        return Database.getQueryLocator('SELECT Id FROM Account');
    }
    public void execute(Database.BatchableContext context, List<SObject> records) {
        insert new Account(Name = 'shape-batch');
    }
    public void finish(Database.BatchableContext context) {
        System.enqueueJob(new ShapeQueue(false));
    }
}
