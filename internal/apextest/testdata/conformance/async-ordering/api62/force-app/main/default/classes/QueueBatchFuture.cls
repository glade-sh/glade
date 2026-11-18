public class QueueBatchFuture implements Queueable {
 String token;
 public QueueBatchFuture(String token) { this.token=token; }
 public void execute(QueueableContext context) {
  AsyncProbe.log('queue-body-'+token+'|job='+String.valueOf(context.getJobId()));
  Id job=Database.executeBatch(new BatchFuture(token),1);
  AsyncProbe.log('queue-batch-submitted-'+(job!=null));
 }
}
