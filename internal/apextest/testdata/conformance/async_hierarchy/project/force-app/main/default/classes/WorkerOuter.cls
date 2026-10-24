public virtual class WorkerOuter extends WorkerMiddle {
 protected virtual override String start() { return 'outer'; }
 private WorkerMiddle successor;
 private CompletionBase finalizer;
 public String callback() {
  this.successor=new WorkerOuter();
  this.finalizer=new Completion(this);
  WorkerRoot alias=this.successor;
  alias.remember(this);
  return this.finalizer.accept(this.successor);
 }
 public class Variant extends WorkerOuter {}
 private class Completion extends CompletionBase {
  private final WorkerOuter conductor;
  public Completion(WorkerOuter conductor) { this.conductor = conductor; }
  public override String complete() { return this.conductor.start(); }
 }
 public override void execute(QueueableContext context) { System.attachFinalizer(new Completion(this)); }
 private static void mutate(List<WorkerRoot> rows) { for(WorkerRoot row:rows) row.touch(); }
 public String run() {
  Completion listener=new Completion(this);
  WorkerRoot broad=this;
  broad.touch();
  broad.remember(this);
  mutate(new List<WorkerRoot>{this});
  return listener.complete();
 }
}
