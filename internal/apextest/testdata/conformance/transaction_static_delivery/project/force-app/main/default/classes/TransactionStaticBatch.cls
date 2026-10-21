public class TransactionStaticBatch implements Database.Batchable<SObject> {
 private Boolean afterCommit;
 public TransactionStaticBatch(Boolean value) {afterCommit=value;}
 public Database.QueryLocator start(Database.BatchableContext context) {return Database.getQueryLocator([SELECT Id FROM Account WHERE Name='batch seed']);}
 public void execute(Database.BatchableContext context,List<SObject> scope) {}
 public void finish(Database.BatchableContext context) {TransactionStaticState.publishOne(afterCommit);}
}
