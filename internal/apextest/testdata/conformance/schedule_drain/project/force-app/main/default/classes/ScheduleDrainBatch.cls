public class ScheduleDrainBatch implements Database.Batchable<SObject> {
 private Boolean innerWork;
 public ScheduleDrainBatch(Boolean value) { innerWork = value; }
 public Iterable<SObject> start(Database.BatchableContext context) { return new List<SObject>(); }
 public void execute(Database.BatchableContext context, List<SObject> scope) {}
 public void finish(Database.BatchableContext context) {
  insert new Account(Name='Schedule batch finished');
  ScheduleDrainProbe.scheduleWork(innerWork);
 }
}
