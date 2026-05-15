public class RunnerQueue implements Queueable {
  public void execute(QueueableContext context) {
    RunnerState.asyncRan = RunnerState.asyncRan + 1;
    insert new Widget__c(Name = 'Queueable');
  }
}
