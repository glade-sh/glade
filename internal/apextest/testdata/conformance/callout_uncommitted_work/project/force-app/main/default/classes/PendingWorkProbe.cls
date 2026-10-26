public class PendingWorkProbe {
 public class Reply implements HttpCalloutMock {
  public HttpResponse respond(HttpRequest request) {HttpResponse response=new HttpResponse();response.setStatusCode(204);return response;}
 }
 public class SoapReply implements WebServiceMock {
  public void doInvoke(Object stub,Object request,Map<String,Object> response,String endpoint,String soapAction,String requestName,String responseNamespace,String responseName,String responseType) {}
 }
 public class Worker implements Queueable,Database.AllowsCallouts {
  private Boolean exercise;
  public Worker(Boolean exercise) {this.exercise=exercise;}
  public void execute(QueueableContext context) {
   if(exercise) {Account row=new Account(Name='Async callout observation');insert row;String observed=send(false);row.Description=observed;update row;}
  }
 }
 @future(callout=true) public static void later() {}
 public static String send(Boolean soap) {
  try {
   if(soap) {Map<String,Object> response=new Map<String,Object>();WebServiceCallout.invoke(new PendingWorkProbe(),new PendingWorkProbe(),response,new String[]{'https://native.invalid','action','urn:probe','request','urn:probe','response','PendingWorkProbe'});}
   else {HttpRequest request=new HttpRequest();request.setEndpoint('https://native.invalid');request.setMethod('GET');new Http().send(request);}
   return 'ok';
  }catch(Exception failure) {return failure.getTypeName()+'|'+failure.getMessage();}
 }
 public static String observe(String key) {
  try {
   if(key=='UW15') {Test.setMock(HttpCalloutMock.class,new Reply());Test.startTest();System.enqueueJob(new Worker(true));Test.stopTest();return [SELECT Description FROM Account WHERE Name='Async callout observation' LIMIT 1].Description;}
   if(key!='UW17' && Test.isRunningTest()) {Test.setMock(HttpCalloutMock.class,new Reply());Test.setMock(WebServiceMock.class,new SoapReply());}
   if(key=='UW03') {insert new Account(Name='Before start');Test.startTest();}
   if(key=='UW04') {Test.startTest();insert new Account(Name='Inside start');}
   if(key=='UW05') Database.setSavepoint();
   if(key=='UW06') {Savepoint point=Database.setSavepoint();insert new Account(Name='Rolled back');Database.rollback(point);}
   if(key=='UW07') EventBus.publish(new ImmediateNotice__e());
   if(key=='UW08') EventBus.publish(new CommittedNotice__e());
   if(key=='UW09') System.enqueueJob(new Worker(false));
   if(key=='UW10') later();
   if(key=='UW11') {Messaging.SingleEmailMessage email=new Messaging.SingleEmailMessage();email.setToAddresses(new String[]{'probe@example.invalid'});email.setSubject('Probe');email.setPlainTextBody('Synthetic observation');Messaging.sendEmail(new Messaging.SingleEmailMessage[]{email});}
   if(key=='UW12') {List<Account> rows=[SELECT Id FROM Account LIMIT 1];}
   if(key=='UW16') Database.insert(new Account(),false);
   if(key=='UW01'||key=='UW13'||key=='UW14'||key=='UW17') insert new Account(Name='Pending callout observation');
   String value=send(key=='UW13');
   if(key=='UW02') insert new Account(Name='After callout');
   if(key=='UW14') value+='|callouts='+Limits.getCallouts();
   return value;
  }catch(Exception failure) {return failure.getTypeName()+'|'+failure.getMessage();}
 }
 public static String qualifiedFuture() {
  if(Test.isRunningTest()) Test.setMock(HttpCalloutMock.class,new Reply());
  PendingWorkProbe.later();
  return send(false);
 }
 public static void emit(String key) {System.assert(false,'OBSERVED|'+key+'|'+observe(key));}
}
