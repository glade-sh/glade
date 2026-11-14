public class StopOrderBatch implements Database.Batchable<Integer> {
    public Iterable<Integer> start(Database.BatchableContext ctx) {
        StopOrderMk.mark('bs');
        return new List<Integer>{ 1 };
    }
    public void execute(Database.BatchableContext ctx, List<Integer> scope) {
        StopOrderMk.mark('bx');
    }
    public void finish(Database.BatchableContext ctx) {
        StopOrderMk.mark('bf');
    }
}
