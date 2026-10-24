public class PendingCalloutProbe {
 public static String observe(String route) {
  if(route=='beforeStart') {insert new Account(Name='Before');Test.startTest();}
  if(route=='insideStart') {Test.startTest();insert new Account(Name='Inside');}
  if(route=='rollbackRelease' || route=='rollbackOnly') {Savepoint point=Database.setSavepoint();insert new Account(Name='Temporary');Database.rollback(point);if(route=='rollbackRelease')Database.releaseSavepoint(point);}
  if(route=='savepoint') Database.setSavepoint();
  if(route=='empty') insert new List<Account>();
  if(route=='afterStop') {Test.startTest();insert new Account(Name='Inside');Test.stopTest();}
  if(route=='soap') insert new Account(Name='SOAP pending');
  if(route=='enqueue') System.enqueueJob(new Worker());
  HttpRequest request=new HttpRequest();request.setEndpoint('https://example.com');request.setMethod('GET');
  try {
   if(route=='soap') {Map<String,Object> response=new Map<String,Object>();WebServiceCallout.invoke(new PendingCalloutProbe(),new PendingCalloutProbe(),response,new String[]{'https://example.com','action','urn:synthetic','request','urn:synthetic','response','PendingCalloutProbe'});}
   else new Http().send(request);
   return 'ok';
  }catch(Exception error){return error.getTypeName()+'|'+error.getMessage();}
 }
 public class Worker implements Queueable {public void execute(QueueableContext context) {}}
}
