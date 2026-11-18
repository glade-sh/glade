public class TbBatch__A__ implements Database.Batchable<SObject> {
    public Database.QueryLocator start(Database.BatchableContext bc) { return Database.getQueryLocator('SELECT Id FROM Account LIMIT 1'); }
    public void execute(Database.BatchableContext bc, List<SObject> scope) {}
    public void finish(Database.BatchableContext bc) {}
}
