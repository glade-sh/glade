public class StopOrderChain implements Queueable {
    private String p;
    private Integer idx;
    private Integer total;
    public StopOrderChain(String p, Integer idx, Integer total) {
        this.p = p;
        this.idx = idx;
        this.total = total;
    }
    public void execute(QueueableContext ctx) {
        StopOrderMk.mark(p + idx);
        if (idx < total) {
            try {
                System.enqueueJob(new StopOrderChain(p, idx + 1, total));
            } catch (Exception e) {
                StopOrderMk.mark(p + idx + '!' + e.getTypeName());
            }
        }
    }
}
