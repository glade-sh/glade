public class StopOrderBoom implements Queueable {
    private String name;
    public StopOrderBoom(String name) {
        this.name = name;
    }
    public void execute(QueueableContext ctx) {
        StopOrderMk.mark(name);
        throw new StopOrderMx.BoomException('boom');
    }
}
