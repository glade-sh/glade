public class ShapeQueue implements Queueable {
    private Boolean follow;
    public ShapeQueue(Boolean follow) { this.follow = follow; }
    public void execute(QueueableContext context) {
        insert new Account(Name = 'shape-queue');
        if (follow) System.enqueueJob(new ShapeQueue(false));
    }
}
