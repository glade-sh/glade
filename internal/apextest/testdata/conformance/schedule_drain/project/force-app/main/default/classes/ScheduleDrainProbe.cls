public class ScheduleDrainProbe {
 public class InnerWork implements Schedulable {
  public void execute(SchedulableContext context) { insert new Account(Name='Schedule drain ran'); }
 }
 public static void scheduleWork(Boolean innerWork) {
  Schedulable work;
  if (innerWork) { work = new InnerWork(); } else { work = new ScheduleDrainTop(); }
  System.schedule('Drain followup', '0 0 0 1 1 ? 2099', work);
 }
 public class QueueWork implements Queueable {
  private Boolean innerWork;
  public QueueWork(Boolean value) { innerWork = value; }
  public void execute(QueueableContext context) {
   insert new Account(Name='Schedule queue ran');
   scheduleWork(innerWork);
  }
 }
}
