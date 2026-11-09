public class LsBatOuterSObj implements Database.Batchable<SObject> {
  public Database.QueryLocator start(Database.BatchableContext bc) { return Database.getQueryLocator([SELECT Id FROM Account]); }
  public void execute(Database.BatchableContext bc, List<SObject> scope) {}
  public void finish(Database.BatchableContext bc) {}
}
