public class StopShapeLocator implements Database.Batchable<SObject> {
    public Database.QueryLocator start(Database.BatchableContext ctx) { StopOrderMk.mark('as'); return Database.getQueryLocator('SELECT Id FROM Contact'); }
    public void execute(Database.BatchableContext ctx, List<SObject> scope) { StopOrderMk.mark('ax'); }
    public void finish(Database.BatchableContext ctx) { StopOrderMk.mark('af'); }
}
