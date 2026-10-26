public class ReturnIdentityBatch implements Database.Batchable<SObject> {
    public Iterable<SObject> start(Database.BatchableContext context) { return new List<SObject>(); }
    public void execute(Database.BatchableContext context, List<SObject> scope) {}
    public void finish(Database.BatchableContext context) {}
}
