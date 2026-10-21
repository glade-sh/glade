public class TransactionStaticQueue implements Queueable {
 private Boolean afterCommit;
 public TransactionStaticQueue(Boolean value) {afterCommit=value;}
 public void execute(QueueableContext context) {TransactionStaticState.publishOne(afterCommit);}
}
