public class TransactionFinalizer implements Finalizer {
    public void execute(FinalizerContext context) { TransactionDml.work(false,'Finalizer'); }
}
