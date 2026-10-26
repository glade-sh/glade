public class HeapLimitQueueable implements Queueable {
    private String kind;
    private Integer target;

    public HeapLimitQueueable(String kind, Integer target) {
        this.kind = kind;
        this.target = target;
    }

    public void execute(QueueableContext context) {
        HeapLimitProbe.grow(kind, target);
    }
}
