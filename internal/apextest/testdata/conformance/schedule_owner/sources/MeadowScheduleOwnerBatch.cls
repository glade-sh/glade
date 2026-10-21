public class MeadowScheduleOwnerBatch implements Database.Batchable<SObject> {
  public Database.QueryLocator start(Database.BatchableContext context) { return Database.getQueryLocator([SELECT Id FROM Account WHERE Id=null]); }
  public void execute(Database.BatchableContext context, List<SObject> scope) {}
  public void finish(Database.BatchableContext context) {}
 }
