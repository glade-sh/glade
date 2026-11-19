public class MarkerThrow implements Queueable {
    public void execute(QueueableContext ctx) {
        insert new Account(Name = 'MK-T');
        throw new MarkerException('job-boom');
    }
}
