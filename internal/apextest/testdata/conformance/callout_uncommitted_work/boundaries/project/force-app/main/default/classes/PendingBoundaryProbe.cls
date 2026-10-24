public class PendingBoundaryProbe {
 public class Reply implements HttpCalloutMock {
  public HttpResponse respond(HttpRequest request) {HttpResponse response=new HttpResponse();response.setStatusCode(204);return response;}
 }
 public class SoapReply implements WebServiceMock {
  public void doInvoke(Object stub,Object request,Map<String,Object> response,String endpoint,String soapAction,String requestName,String responseNamespace,String responseName,String responseType) {}
 }
 public static String observe(String key) {
  if(Test.isRunningTest()) {Test.setMock(HttpCalloutMock.class,new Reply());Test.setMock(WebServiceMock.class,new SoapReply());}
  if(key.startsWith('before_') || key.startsWith('both_')) insert new Account(Name='Before boundary');
  Test.startTest();
  if(key.startsWith('inside_') || key.startsWith('both_')) insert new Account(Name='Inside boundary');
  if(!key.contains('_inside_')) Test.stopTest();
  if(key.startsWith('after_')) insert new Account(Name='After boundary');
  try {
   if(key.endsWith('_soap')) {Map<String,Object> response=new Map<String,Object>();WebServiceCallout.invoke(new PendingBoundaryProbe(),new PendingBoundaryProbe(),response,new String[]{'https://native.invalid','action','urn:probe','request','urn:probe','response','PendingBoundaryProbe'});}
   else {HttpRequest request=new HttpRequest();request.setEndpoint('https://native.invalid');request.setMethod('GET');new Http().send(request);}
   return 'ok';
  }catch(Exception failure) {return failure.getTypeName()+'|'+failure.getMessage();}
 }
 public static void emit(String key) {System.assert(false,'OBSERVED|'+key+'|'+observe(key));}
}
