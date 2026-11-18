@IsTest private class Cm08SetMockMismatchTest {
 @IsTest static void observe() {
  Map<String,Object> o=new Map<String,Object>();
  try {
   Test.setMock(HttpCalloutMock.class,new SoapRecorder());
   o.put('registered',true);
   HttpRequest q=new HttpRequest(); q.setEndpoint('https://mock.invalid/mismatch'); q.setMethod('GET');
   HttpResponse r=new Http().send(q); o.put('result',r.getStatusCode());
  } catch(Exception e) { o.put('error',e.getTypeName()+'|'+e.getMessage()); }
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
