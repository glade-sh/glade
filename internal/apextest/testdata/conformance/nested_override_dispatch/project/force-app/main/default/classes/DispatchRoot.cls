public virtual class DispatchRoot {
 public DispatchRoot(Integer seed) {changes=seed;}
 protected DispatchCompletion pending;
 protected virtual DispatchCompletion makeCompletion() {return new DispatchCompletion();}
 protected Boolean scheduled=false;
 public void queue() {this.scheduled=true;this.beginWork();}
 public Integer changes=0;
 public List<DispatchRoot> children=new List<DispatchRoot>();
 public DispatchRoot touch() {changes++;children.add(this);return this;}
 protected virtual String beginWork() { return 'root'; }
 public virtual class Queued extends DispatchRoot implements Queueable {
  protected Queued(Integer seed) {super(seed);}
  protected virtual override String beginWork() { children.add(this);if(this.scheduled){this.scheduled=false;System.enqueueJob(this);}return 'queued'; }
  public void execute(QueueableContext context) {this.touch();this.pending=this.pending??this.makeCompletion();System.attachFinalizer(this.pending);this.touch();}
 }
}
