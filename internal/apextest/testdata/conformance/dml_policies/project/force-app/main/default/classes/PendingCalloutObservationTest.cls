@IsTest private class PendingCalloutObservationTest {
 private class ResponseMock implements HttpCalloutMock {public HttpResponse respond(HttpRequest request){HttpResponse response=new HttpResponse();response.setStatusCode(204);return response;}}
 private class SoapMock implements WebServiceMock {public void doInvoke(Object stub,Object request,Map<String,Object> response,String endpoint,String soapAction,String requestName,String responseNamespace,String responseName,String responseType){}}
 @IsTest static void clean(){Test.setMock(HttpCalloutMock.class,new ResponseMock());Test.setMock(WebServiceMock.class,new SoapMock());System.assert(false,'OBSERVED|clean|'+PendingCalloutProbe.observe('clean'));}
 @IsTest static void beforeStart(){Test.setMock(HttpCalloutMock.class,new ResponseMock());Test.setMock(WebServiceMock.class,new SoapMock());System.assert(false,'OBSERVED|beforeStart|'+PendingCalloutProbe.observe('beforeStart'));}
 @IsTest static void insideStart(){Test.setMock(HttpCalloutMock.class,new ResponseMock());Test.setMock(WebServiceMock.class,new SoapMock());System.assert(false,'OBSERVED|insideStart|'+PendingCalloutProbe.observe('insideStart'));}
 @IsTest static void rollbackRelease(){Test.setMock(HttpCalloutMock.class,new ResponseMock());Test.setMock(WebServiceMock.class,new SoapMock());System.assert(false,'OBSERVED|rollbackRelease|'+PendingCalloutProbe.observe('rollbackRelease'));}
 @IsTest static void rollbackOnly(){Test.setMock(HttpCalloutMock.class,new ResponseMock());Test.setMock(WebServiceMock.class,new SoapMock());System.assert(false,'OBSERVED|rollbackOnly|'+PendingCalloutProbe.observe('rollbackOnly'));}
 @IsTest static void savepoint(){Test.setMock(HttpCalloutMock.class,new ResponseMock());Test.setMock(WebServiceMock.class,new SoapMock());System.assert(false,'OBSERVED|savepoint|'+PendingCalloutProbe.observe('savepoint'));}
 @IsTest static void empty(){Test.setMock(HttpCalloutMock.class,new ResponseMock());Test.setMock(WebServiceMock.class,new SoapMock());System.assert(false,'OBSERVED|empty|'+PendingCalloutProbe.observe('empty'));}
 @IsTest static void afterStop(){Test.setMock(HttpCalloutMock.class,new ResponseMock());Test.setMock(WebServiceMock.class,new SoapMock());System.assert(false,'OBSERVED|afterStop|'+PendingCalloutProbe.observe('afterStop'));}
 @IsTest static void soap(){Test.setMock(HttpCalloutMock.class,new ResponseMock());Test.setMock(WebServiceMock.class,new SoapMock());System.assert(false,'OBSERVED|soap|'+PendingCalloutProbe.observe('soap'));}
 @IsTest static void enqueue(){Test.setMock(HttpCalloutMock.class,new ResponseMock());Test.setMock(WebServiceMock.class,new SoapMock());System.assert(false,'OBSERVED|enqueue|'+PendingCalloutProbe.observe('enqueue'));}
}
