public class AsyncPathProbe {
 public class SubmitParcel implements Queueable { public void execute(QueueableContext context) { insert new AsyncParcel__c(Name='async',Source__c='initial'); } }
 public static Map<String,Object> observe(String key) {
  Map<String,Object> out=new Map<String,Object>(); out.put('exception',null);
  try {
   if(key=='SP10') { Test.startTest(); System.enqueueJob(new SubmitParcel()); Test.stopTest(); }
   else {
    if(key!='SP01'&&key!='SP03') Test.startTest();
    Savepoint point; if(key=='SP07') point=Database.setSavepoint();
    AsyncParcel__c row=new AsyncParcel__c(Name=key=='SP05'?'other':(key=='SP06'?'fault':'async'),Source__c='initial'); insert row;
    out.put('before',[SELECT count() FROM AsyncParcel__c WHERE Delayed__c=true]);
    if(key=='SP03') Test.startTest();
    if(key=='SP04') {row.Source__c='changed';update row;}
    if(key=='SP05') {row.Name='update';update row;}
    if(key=='SP07') Database.rollback(point);
    if(key!='SP01') Test.stopTest();
   }
   out.put('rows',[SELECT count() FROM AsyncParcel__c]);
   out.put('delayed',[SELECT count() FROM AsyncParcel__c WHERE Delayed__c=true]);
   out.put('immediate',[SELECT count() FROM AsyncParcel__c WHERE Immediate__c=true]);
   out.put('jobs',[SELECT count() FROM AsyncApexJob]);
   out.put('interviews',[SELECT count() FROM FlowInterview]);
  } catch(Exception failure) {out.put('exception',failure.getTypeName());}
  return out;
 }
 public static void emit(String key) { System.assert(false,'ASYNC_PATH|'+key+'|'+JSON.serialize(observe(key))); }
}
