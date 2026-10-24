public virtual class DispatchOuter extends DispatchMiddle {
 public DispatchOuter() {super(7);}
 protected virtual override String beginWork() { String ignored=super.beginWork();return 'outer'; }
 private DispatchCompletion listener;
 public String finishAliased() {
  this.listener=new Completion(this);
  this.beginWork();
  DispatchRoot broad=this;
  broad.touch();
  DispatchMiddle middle=this;
  middle.touch();
  List<DispatchRoot> items=new List<DispatchRoot>{this};
  for(DispatchRoot item:items) item.touch();
  return this.listener.complete();
 }
 public String finishDirect() { return new Completion(this).complete(); }
 public DispatchOuter prepare() {
  this.pending=new Completion(this);
  this.beginWork();
  DispatchRoot.Queued broad=this;
  broad.touch();
  return this;
 }
 protected override DispatchCompletion makeCompletion() {return new Completion(this);}
 private class Completion extends DispatchCompletion {
  private final DispatchOuter conductor;
  public Completion(DispatchOuter conductor) { this.conductor=conductor; }
  public override String complete() { return this.conductor.beginWork(); }
 }
}
