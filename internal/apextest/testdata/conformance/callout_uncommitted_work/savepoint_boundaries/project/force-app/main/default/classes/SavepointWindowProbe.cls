public class SavepointWindowProbe {
 public class Reply implements HttpCalloutMock {
  public HttpResponse respond(HttpRequest request) {HttpResponse response=new HttpResponse();response.setStatusCode(204);return response;}
 }
 public class SoapReply implements WebServiceMock {
  public void doInvoke(Object stub,Object request,Map<String,Object> response,String endpoint,String soapAction,String requestName,String responseNamespace,String responseName,String responseType) {}
 }
 public static void mocks() {Test.setMock(HttpCalloutMock.class,new Reply());Test.setMock(WebServiceMock.class,new SoapReply());}
 public static String observe(String route) {
  try {
   if(route=='soap') {WebServiceCallout.invoke(new SavepointWindowProbe(),new SavepointWindowProbe(),new Map<String,Object>(),new String[]{'https://native.invalid','action','urn:probe','request','urn:probe','response','SavepointWindowProbe'});}
   else {HttpRequest request=new HttpRequest();request.setEndpoint('https://native.invalid');request.setMethod('GET');new Http().send(request);}
   return 'ok';
  }catch(Exception failure) {return failure.getTypeName()+'|'+failure.getMessage();}
 }
 public static void emit(String key,String route) {System.assert(false,'OBSERVED|'+key+'|'+observe(route));}
}
