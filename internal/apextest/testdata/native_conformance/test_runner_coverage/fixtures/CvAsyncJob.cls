public class CvAsyncJob implements Queueable {
    public void execute(QueueableContext ctx) {
        Integer x = 1;
        x += 1;
    }
}
