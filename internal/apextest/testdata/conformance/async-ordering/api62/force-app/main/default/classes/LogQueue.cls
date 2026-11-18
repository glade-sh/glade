public class LogQueue implements Queueable {
 String token;
 public LogQueue(String token) { this.token=token; }
 public void execute(QueueableContext context) { AsyncProbe.log('queue-body-'+token+'|job='+String.valueOf(context.getJobId())); }
}
