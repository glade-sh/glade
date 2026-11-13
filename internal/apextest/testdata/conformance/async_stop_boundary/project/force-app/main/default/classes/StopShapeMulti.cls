public class StopShapeMulti implements Database.Batchable<Integer> {
    public Iterable<Integer> start(Database.BatchableContext ctx) { StopOrderMk.mark('as'); return new List<Integer>{ 1, 2, 3 }; }
    public void execute(Database.BatchableContext ctx, List<Integer> scope) { StopOrderMk.mark('ax'); }
    public void finish(Database.BatchableContext ctx) { StopOrderMk.mark('af'); }
}
