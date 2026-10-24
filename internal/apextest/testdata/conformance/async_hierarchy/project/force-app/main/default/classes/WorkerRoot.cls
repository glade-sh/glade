public virtual class WorkerRoot implements Queueable {
 public virtual void execute(QueueableContext context) {}
 public Integer changes=0;
 protected List<WorkerRoot> linked=new List<WorkerRoot>();
 public WorkerRoot remember(WorkerRoot child) { linked.add(child); return this; }
 public WorkerRoot touch() { changes++; return this; }
 protected virtual String start() { return 'root'; }
 public virtual class Queued extends WorkerRoot {
  protected virtual override String start() { return 'queued'; }
 }
}
