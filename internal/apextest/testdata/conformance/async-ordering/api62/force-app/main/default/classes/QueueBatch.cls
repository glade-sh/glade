public class QueueBatch implements Queueable {
 String token;
 public QueueBatch(String token) { this.token=token; }
 public void execute(QueueableContext context) {
  AsyncProbe.log('queue-body-'+token+'|job='+String.valueOf(context.getJobId()));
  Id job=Database.executeBatch(new BatchChild(token),1);
  AsyncProbe.log('queue-batch-submitted-'+(job!=null));
 }
}
