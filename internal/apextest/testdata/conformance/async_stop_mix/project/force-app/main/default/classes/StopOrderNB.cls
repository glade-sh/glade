public class StopOrderNB implements Database.Batchable<Integer> {
    private String p;
    private Boolean startQ;
    private Boolean execQ;
    private Boolean finQ;
    public StopOrderNB(String p, Boolean startQ, Boolean execQ, Boolean finQ) {
        this.p = p;
        this.startQ = startQ;
        this.execQ = execQ;
        this.finQ = finQ;
    }
    public Iterable<Integer> start(Database.BatchableContext ctx) {
        StopOrderMk.mark(p + 's');
        if (startQ) {
            StopOrderMx.enq(p + 'sq');
        }
        return new List<Integer>{ 1 };
    }
    public void execute(Database.BatchableContext ctx, List<Integer> scope) {
        StopOrderMk.mark(p + 'x');
        if (execQ) {
            StopOrderMx.enq(p + 'xq');
        }
    }
    public void finish(Database.BatchableContext ctx) {
        StopOrderMk.mark(p + 'f');
        if (finQ) {
            StopOrderMx.enq(p + 'fq');
        }
    }
}
