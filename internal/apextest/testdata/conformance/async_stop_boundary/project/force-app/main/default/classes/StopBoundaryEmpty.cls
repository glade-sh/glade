public class StopBoundaryEmpty implements Database.Batchable<Integer> {
    public Iterable<Integer> start(Database.BatchableContext ctx) { StopOrderMk.mark('as'); return new List<Integer>(); }
    public void execute(Database.BatchableContext ctx, List<Integer> scope) { StopOrderMk.mark('ax'); }
    public void finish(Database.BatchableContext ctx) { StopOrderMk.mark('af'); }
}
