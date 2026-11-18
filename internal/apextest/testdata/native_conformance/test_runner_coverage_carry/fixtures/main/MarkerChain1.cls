public class MarkerChain1 implements Queueable {
    public void execute(QueueableContext ctx) {
        insert new Account(Name = 'MK-C1');
        System.enqueueJob(new MarkerChain2());
    }
}
