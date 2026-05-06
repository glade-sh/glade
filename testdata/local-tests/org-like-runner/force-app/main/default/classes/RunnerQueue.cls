public class RunnerQueue implements Queueable {
  public void execute(QueueableContext context) {
    RunnerState.asyncRan = RunnerState.asyncRan + 1;
  }
}
