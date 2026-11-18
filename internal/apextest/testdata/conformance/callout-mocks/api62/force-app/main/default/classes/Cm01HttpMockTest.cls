@IsTest private class Cm01HttpMockTest {
 @IsTest static void observe() {
  CalloutMockRecorder mock=new CalloutMockRecorder();
  Test.setMock(HttpCalloutMock.class,mock);
  HttpRequest q=new HttpRequest();
  q.setEndpoint('https://mock.invalid/one'); q.setMethod('POST');
  q.setHeader('X-Req','asked'); q.setBody('payload'); q.setTimeout(20000);
  Integer before=Limits.getCallouts();
  HttpResponse r=new Http().send(q);
  Integer after=Limits.getCallouts();
  Map<String,Object> o=new Map<String,Object>{'code'=>r.getStatusCode(),'status'=>r.getStatus(),'body'=>r.getBody(),
   'header'=>r.getHeader('X-Mock'),'calls'=>mock.requests.size(),'delta'=>after-before,'limit'=>Limits.getLimitCallouts(),'req'=>mock.requests[0]};
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
