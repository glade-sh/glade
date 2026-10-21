public class PredicateContextProbe {
 public static Map<String,Boolean> flags(){return new Map<String,Boolean>{'queueable'=>System.isQueueable(),'future'=>System.isFuture(),'batch'=>System.isBatch(),'scheduled'=>System.isScheduled(),'test'=>Test.isRunningTest()};}
 public static void mark(String label,String stage){insert new Account(Name=label+':'+stage,Description=JSON.serialize(flags()));}
 public class Work implements Queueable {
  String label;Integer step;
  public Work(String label,Integer step){this.label=label;this.step=step;}
  public void execute(QueueableContext context){mark(label,'body'+step);System.attachFinalizer(new Tail(label,step));}
 }
 public class Tail implements Finalizer {
  String label;Integer step;
  public Tail(String label,Integer step){this.label=label;this.step=step;}
  public void execute(FinalizerContext context){mark(label,'finalizer'+step);if(!System.isQueueable()&&step<2)System.enqueueJob(new Work(label,step+1));}
 }
 public static void start(String label){mark(label,'root');System.enqueueJob(new Work(label,0));}
 public static String report(String label){String pattern=label+':%';Map<String,Object> out=new Map<String,Object>();for(Account item:[SELECT Name,Description FROM Account WHERE Name LIKE :pattern ORDER BY Name])out.put(item.Name.substring(label.length()+1),JSON.deserializeUntyped(item.Description));return JSON.serialize(out);}
}