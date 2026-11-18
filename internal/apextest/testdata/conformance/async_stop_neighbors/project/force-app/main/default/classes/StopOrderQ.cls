public class StopOrderQ implements Queueable {
    private String name;
    private Boolean callFuture;
    public StopOrderQ(String name, Boolean callFuture) {
        this.name = name;
        this.callFuture = callFuture;
    }
    public void execute(QueueableContext ctx) {
        StopOrderMk.mark(name);
        if (callFuture) {
            StopOrderMk.fut('e');
        }
    }
}
