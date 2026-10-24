public class TransactionBatch implements Database.Batchable<SObject> {
    public Iterable<SObject> start(Database.BatchableContext context) {
        TransactionDml.work(true,'BatchStart');
        return new List<SObject>{new Account(Name='scope')};
    }
    public void execute(Database.BatchableContext context,List<SObject> scope) {
        TransactionDml.work(false,'BatchExecute');
    }
    public void finish(Database.BatchableContext context) { TransactionDml.work(true,'BatchFinish'); }
}
