public class TransactionQueue implements Queueable {
    Boolean setup; String label; Boolean finalizer;
    public TransactionQueue(Boolean setup, String label, Boolean finalizer) {
        this.setup=setup; this.label=label; this.finalizer=finalizer;
    }
    public void execute(QueueableContext context) {
        TransactionDml.work(setup,label);
        if(finalizer) System.attachFinalizer(new TransactionFinalizer());
    }
}
