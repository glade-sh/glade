public class DeferredWorkProbe {
public static Map<String,Object> finalizerChild() { Test.startTest(); Id root=System.enqueueJob(new DeferredWorkProbe.Root(true)); Test.stopTest(); return DeferredWorkProbe.observe(root); }
public static Map<String,Object> bodyChild() { Test.startTest(); Id root=System.enqueueJob(new DeferredWorkProbe.Root(false)); Test.stopTest(); return DeferredWorkProbe.observe(root); }
public static Map<String,Object> noChild() { Test.startTest(); Id root=System.enqueueJob(new DeferredWorkProbe.Leaf()); Test.stopTest(); return DeferredWorkProbe.observe(root); }

 public class Root implements Queueable {
  private Boolean useFinalizer;
  public Root(Boolean value) { useFinalizer=value; }
  public void execute(QueueableContext context) {
   insert new Account(Name='deferred-root');
   if(useFinalizer) System.attachFinalizer(new Finish());
   else System.enqueueJob(new Leaf());
  }
 }
 public class Finish implements Finalizer {
  public void execute(FinalizerContext context) {
   insert new Account(Name='deferred-finalizer');
   System.enqueueJob(new Leaf());
  }
 }
 public class Leaf implements Queueable {
  public void execute(QueueableContext context) { insert new Account(Name='deferred-leaf'); }
 }
 public static Map<String,Object> observe(Id root) {
  List<AsyncApexJob> children=[SELECT Status FROM AsyncApexJob WHERE JobType='Queueable' AND Id != :root];
  List<AsyncApexJob> parents=[SELECT Status FROM AsyncApexJob WHERE Id = :root];
  return new Map<String,Object>{'rootStatus'=>parents[0].Status,'children'=>children.size(),'childStatus'=>children.isEmpty()?null:children[0].Status,'rootRows'=>[SELECT COUNT() FROM Account WHERE Name='deferred-root'],'finalizerRows'=>[SELECT COUNT() FROM Account WHERE Name='deferred-finalizer'],'leafRows'=>[SELECT COUNT() FROM Account WHERE Name='deferred-leaf']};
 }
 public static Map<String,Object> bodyChildCapped() { AsyncOptions options=new AsyncOptions(); options.MaximumQueueableStackDepth=2; Test.startTest(); Id root=System.enqueueJob(new DeferredWorkProbe.Root(false),options); Test.stopTest(); return DeferredWorkProbe.observe(root); }
 public static Map<String,Object> bodyChildDepthOne() { AsyncOptions options=new AsyncOptions(); options.MaximumQueueableStackDepth=1; Test.startTest(); Id root=System.enqueueJob(new DeferredWorkProbe.Root(false),options); Test.stopTest(); return DeferredWorkProbe.observe(root); }
 public static Map<String,Object> bodyTwoChildren() { Test.startTest(); Id root=System.enqueueJob(new DeferredWorkProbe.DoubleRoot()); Test.stopTest(); return DeferredWorkProbe.observe(root); }
 public class DoubleRoot implements Queueable {
 public void execute(QueueableContext context) { System.enqueueJob(new Leaf()); System.enqueueJob(new Leaf()); }
 }
 public static Map<String,Object> bodyChildDelay() { Test.startTest(); Id root=System.enqueueJob(new DeferredWorkProbe.OverloadRoot(false,0)); Test.stopTest(); return DeferredWorkProbe.observe(root); }
 public static Map<String,Object> bodyChildOptions() { Test.startTest(); Id root=System.enqueueJob(new DeferredWorkProbe.OverloadRoot(false,1)); Test.stopTest(); return DeferredWorkProbe.observe(root); }
 public static Map<String,Object> finalizerChildDelay() { Test.startTest(); Id root=System.enqueueJob(new DeferredWorkProbe.OverloadRoot(true,0)); Test.stopTest(); return DeferredWorkProbe.observe(root); }
 public static Map<String,Object> finalizerChildOptions() { Test.startTest(); Id root=System.enqueueJob(new DeferredWorkProbe.OverloadRoot(true,1)); Test.stopTest(); return DeferredWorkProbe.observe(root); }
 public static void enqueueOverloadLeaf(Integer route) {
 if(route==0) System.enqueueJob(new Leaf(),0);
 else System.enqueueJob(new Leaf(),new AsyncOptions());
 }
 public class OverloadRoot implements Queueable {
 private Boolean finish; private Integer route;
 public OverloadRoot(Boolean finish,Integer route) {this.finish=finish;this.route=route;}
 public void execute(QueueableContext context) {
 if(finish) System.attachFinalizer(new OverloadFinish(route));
 else DeferredWorkProbe.enqueueOverloadLeaf(route);
 }
 }
 public class OverloadFinish implements Finalizer {
 private Integer route;
 public OverloadFinish(Integer route) {this.route=route;}
 public void execute(FinalizerContext context) { DeferredWorkProbe.enqueueOverloadLeaf(route); }
 }
}
