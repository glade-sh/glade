public class StopBoundaryIterable implements Database.Batchable<Integer>, Database.Stateful {
    public Iterable<Integer> start(Database.BatchableContext ctx) { StopOrderMk.mark('as'); return new List<Integer>{1}; }
    public void execute(Database.BatchableContext ctx, List<Integer> scope) { StopOrderMk.mark('ax'); }
    public void finish(Database.BatchableContext ctx) { StopOrderMk.mark('af'); }
}
