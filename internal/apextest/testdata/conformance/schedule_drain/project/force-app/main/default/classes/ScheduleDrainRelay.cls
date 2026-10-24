public class ScheduleDrainRelay implements Schedulable {
 private Boolean innerWork;
 public ScheduleDrainRelay(Boolean value) { innerWork = value; }
 public void execute(SchedulableContext context) {
  insert new Account(Name='Schedule relay ran');
  ScheduleDrainProbe.scheduleWork(innerWork);
 }
}
