public class GladeQueueProbe implements Queueable {
    private final String tag;
    public GladeQueueProbe(String tag) {
        this.tag = tag;
    }
    public void execute(QueueableContext context) {
        if (tag.startsWith('fail')) {
            throw new IllegalArgumentException('probe failure ' + tag);
        }
        insert new Account(Name = 'glade-queue-probe-' + tag);
    }
}
